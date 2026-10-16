package runtime

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pocketradio/oslo/internal/auth"
	"github.com/pocketradio/oslo/internal/config"
	"github.com/pocketradio/oslo/internal/driver"
	"github.com/pocketradio/oslo/internal/httpapi"
	"github.com/pocketradio/oslo/internal/matching"
	"github.com/pocketradio/oslo/internal/ride"
	"github.com/pocketradio/oslo/internal/user"
)

func RunAPI(logger *slog.Logger, cfg config.Config) error {
	pool, err := openDatabase(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	redisClient, err := openRedis(cfg)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	drivers := driver.NewService(driver.NewLocationStore(redisClient))
	tokens, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTLifetime)
	if err != nil {
		return err
	}
	users := user.NewService(user.NewStore(pool))
	rideRequests := ride.NewRequestService(ride.NewRideStore(pool))
	matchingService := matching.NewService(matching.NewStore(pool), drivers)
	rateLimiter, err := httpapi.NewRateLimiter(redisClient, cfg.HTTPRateLimitRequests, cfg.HTTPRateLimitWindow)
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
