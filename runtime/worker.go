package runtime

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/pocketradio/oslo/internal/config"
	"github.com/pocketradio/oslo/internal/driver"
	"github.com/pocketradio/oslo/internal/matching"
	"github.com/pocketradio/oslo/internal/queue"
)

func RunWorker(logger *slog.Logger, cfg config.Config) error {
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

	sqsQueue, err := openQueue(cfg)
	if err != nil {
		return err
	}

	drivers := driver.NewService(driver.NewLocationStore(redisClient))
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

	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go drivers.RunStaleCleanup(shutdown, func(err error) {
		logger.Error("stale driver cleanup failed", "error", err)
	})
	go matchingService.RunExpiredOfferRecovery(shutdown, func(err error) {
		logger.Error("expired offer recovery failed", "error", err)
	})

	logger.Info("worker started")
	if err := queueWorker.ServeQueue(shutdown); err != nil {
		return err
	}

	return nil
}
