package runtime

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pocketradio/oslo/internal/config"
	"github.com/pocketradio/oslo/internal/outbox"
)

func RunPublisher(logger *slog.Logger, cfg config.Config) error {
	pool, err := openDatabase(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	sqsQueue, err := openQueue(cfg)
	if err != nil {
		return err
	}

	publisher := outbox.NewPublisher(pool, sqsQueue, 10, time.Second)
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("publisher started")
	publisher.Run(shutdown, func(err error) {
		logger.Error("outbox publisher failed", "error", err)
	})

	return nil
}
