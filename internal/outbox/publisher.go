package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pocketradio/oslo/internal/queue"
)

type Publisher struct {
	database  *pgxpool.Pool
	queue     queue.Publisher
	batchSize int
	interval  time.Duration
}

type event struct {
	id        string
	eventType string
	payload   json.RawMessage
}

func NewPublisher(database *pgxpool.Pool, publisher queue.Publisher, batchSize int, interval time.Duration) *Publisher {
	return &Publisher{
		database:  database,
		queue:     publisher,
		batchSize: batchSize,
		interval:  interval,
	}
}

func (p *Publisher) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.publishBatch(ctx); err != nil {
				continue
			}
		}
	}
}

func (p *Publisher) publishBatch(ctx context.Context) error {
	tx, err := p.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin outbox transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id::text, event_type, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, p.batchSize)
	if err != nil {
		return fmt.Errorf("read outbox events: %w", err)
	}

	events := make([]event, 0, p.batchSize)

	for rows.Next() { // Next prepares next row for a READ

		var item event

		if err := rows.Scan(&item.id, &item.eventType, &item.payload); err != nil {
			rows.Close()
			return fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, item)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read outbox events: %w", err)
	}
	rows.Close()

	for _, item := range events {
		err := p.queue.Publish(ctx, queue.Message{
			ID:      item.id,
			Type:    queue.MessageType(item.eventType),
			Payload: item.payload,
		})
		if err != nil {
			return fmt.Errorf("publish outbox event %s: %w", item.id, err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE outbox_events
			SET published_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`, item.id); err != nil {
			return fmt.Errorf("mark outbox event %s published: %w", item.id, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit outbox transaction: %w", err)
	}

	return nil
}
