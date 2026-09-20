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
	"github.com/pocketradio/oslo/internal/httpapi"
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

	tokens, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTLifetime) // all user JWTs are signed with the same server secret
	if err != nil {
		return err
	}
	users := user.NewService(user.NewStore(pool))
	rideRequests := ride.NewRequestService(ride.NewRideStore(pool))

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(pool, users, tokens, rideRequests),
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM) // os.interrupt = ctrlC
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
