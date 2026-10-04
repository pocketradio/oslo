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

func main() {

	// writing logs as json to std o/p
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

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

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(pool, users, tokens, rideRequests, drivers, matchingService),
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
	go outboxPublisher.Run(shutdown)
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
