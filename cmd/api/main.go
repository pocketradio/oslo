package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pocketradio/oslo/internal/auth"
	"github.com/pocketradio/oslo/internal/config"
	"github.com/pocketradio/oslo/internal/database"
	"github.com/pocketradio/oslo/internal/driver"
	"github.com/pocketradio/oslo/internal/httpapi"
	"github.com/pocketradio/oslo/internal/matching"
	"github.com/pocketradio/oslo/internal/outbox"
	"github.com/pocketradio/oslo/internal/queue"
	"github.com/pocketradio/oslo/internal/ride"
	"github.com/pocketradio/oslo/internal/user"
)

// starts the api process and writes any fatal startup error to the logs.
// exiting non-zero lets the host detect that the service did not start.
func main() {

	// writing logs as json to std o/p
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

// builds dependencies, starts background workers, and serves http traffic.
// it also coordinates signal-driven shutdown for the server and worker goroutines.
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 5*time.Second)

	pool, err := database.Open(connectCtx, cfg.DatabaseURL)
	connectCancel() // marks connectctx as cancelled and releases resources.
	if err != nil {
		return err
	}
	defer pool.Close()

	redisCtx, redisCancel := context.WithTimeout(context.Background(), 5*time.Second)
	redisClient, err := database.OpenRedis(redisCtx, cfg.RedisURL)
	redisCancel() // doenst close the redisclient , just cleans up resources assoc with the timeout context
	if err != nil {
		return err
	}

	defer redisClient.Close()

	queueCtx, queueCancel := context.WithTimeout(context.Background(), 5*time.Second)
	sqsQueue, err := queue.NewSQSQueue(queueCtx, cfg.SQSEndpoint, cfg.AWSRegion, cfg.SQSQueueURL)
	queueCancel()
	if err != nil {
		return err
	}

	drivers := driver.NewService(driver.NewLocationStore(redisClient))

	tokens, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTLifetime) // all user JWTs are signed with the same server secret
	if err != nil {
		return err
	}
	users := user.NewService(user.NewStore(pool))
	rideRequests := ride.NewRequestService(ride.NewRideStore(pool))
	outboxPublisher := outbox.NewPublisher(pool, sqsQueue, 10, time.Second)
	matchingService := matching.NewService(matching.NewStore(pool), drivers)
	rateLimiter, err := httpapi.NewRateLimiter(cfg.HTTPRateLimitRequests, cfg.HTTPRateLimitWindow)
	if err != nil {
		return err
	}
	queueWorker, err := queue.NewWorker(
		sqsQueue,
		sqsQueue,
		5,
		matchingService.HandleMessage,
		func(err error) {
			logger.Error("queue worker message failed", "error", err)
		},
	)
	if err != nil {
		return err
	}

	// router returns -> then WBL() runs, returns a new wrapper http.handler
	// then that is wrapped again 3 more times.
	// WRID -> WRL -> .... -> new router ;  so eveyr http req flows thru this
	// ReqID one is the outermost wrapper. it runs first when handler.serveHTTP(w,r) is called

	requestHandler := httpapi.NewRouter(pool, users, tokens, rideRequests, drivers, matchingService)
	requestHandler = httpapi.WithBodyLimit(cfg.HTTPMaxBodyBytes, requestHandler)
	requestHandler = httpapi.WithRequestDeadline(cfg.HTTPRequestTimeout, requestHandler)
	requestHandler = httpapi.WithRateLimit(rateLimiter, requestHandler)
	requestHandler = httpapi.WithRequestID(requestHandler)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           requestHandler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go drivers.RunStaleCleanup(shutdown, func(err error) {
		logger.Error("stale driver cleanup failed", "error", err)
	})
	go outboxPublisher.Run(shutdown, func(err error) {
		logger.Error("outbox publisher failed", "error", err)
	})
	go matchingService.RunExpiredOfferRecovery(shutdown, func(err error) {
		logger.Error("expired offer recovery failed", "error", err)
	})
	go func() {
		if err := queueWorker.ServeQueue(shutdown); err != nil {
			logger.Error("queue worker stopped", "error", err)
			stop()
		}
	}()

	serverError := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddr)
		serverError <- server.ListenAndServe()
	}()

	// blocks until either case succeeds
	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdown.Done():
		logger.Info("api shutting down")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return err
	}

	if err := <-serverError; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
