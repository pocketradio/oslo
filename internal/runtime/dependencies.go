package runtime

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/pocketradio/oslo/internal/config"
	"github.com/pocketradio/oslo/internal/database"
	"github.com/pocketradio/oslo/internal/queue"
)

func openDatabase(cfg config.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return database.Open(ctx, cfg.DatabaseURL)
}

func openRedis(cfg config.Config) (*redis.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return database.OpenRedis(ctx, cfg.RedisURL)
}

func openQueue(cfg config.Config) (*queue.SQSQueue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return queue.NewSQSQueue(ctx, cfg.SQSEndpoint, cfg.AWSRegion, cfg.SQSQueueURL)
}
