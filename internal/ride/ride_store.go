package ride

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pocketradio/oslo/internal/domain"
)

var ErrActiveRideExists = errors.New("rider already has an active ride")

type RideStore struct {
	database *pgxpool.Pool
}

func NewRideStore(database *pgxpool.Pool) *RideStore {
	return &RideStore{database: database}
}

func (s *RideStore) Create(ctx context.Context, ride domain.Ride) (domain.Ride, error) {
	tx, err := s.database.Begin(ctx) // starts transaction
	if err != nil {
		return domain.Ride{}, fmt.Errorf("begin ride transaction: %w", err)
	}
	defer tx.Rollback(ctx) // does nothing and returns if the commit succeeds

	err = tx.QueryRow(ctx, `
		INSERT INTO rides (
			id, rider_id, status,
			pickup_latitude, pickup_longitude,
			destination_latitude, destination_longitude,
			fare_cents, idempotency_key, matching_deadline
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`,
		ride.ID,
		ride.RiderID,
		ride.Status,
		ride.Pickup.Latitude,
		ride.Pickup.Longitude,
		ride.Destination.Latitude,
		ride.Destination.Longitude,
		ride.FareCents,
		ride.IdempotencyKey,
		ride.MatchingDeadline,
	).Scan(&ride.CreatedAt, &ride.UpdatedAt)
	if err != nil {
		_ = tx.Rollback(ctx)

		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) {
			switch postgresError.ConstraintName {
			case "rides_idempotency_unique":
				return s.FindByIdempotencyKey(ctx, ride.RiderID, ride.IdempotencyKey)
			case "rides_one_active_ride_per_rider":
				existing, findErr := s.FindByIdempotencyKey(ctx, ride.RiderID, ride.IdempotencyKey)
				if findErr == nil {
					return existing, nil
				}
				if !errors.Is(findErr, pgx.ErrNoRows) {
					return domain.Ride{}, findErr
				}
				return domain.Ride{}, ErrActiveRideExists
			}
		}

		return domain.Ride{}, fmt.Errorf("create ride: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, ride_id, event_type, payload)
		VALUES ($1, $2, 'match_ride', jsonb_build_object('ride_id', $2::text))
	`, uuid.NewString(), ride.ID)
	if err != nil {
		return domain.Ride{}, fmt.Errorf("create ride outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Ride{}, fmt.Errorf("commit ride transaction: %w", err)
	}

	return ride, nil
}

/* rides_idempotency_unique means the same rider and key already exist,
so the original ride is fetched and returned.
rides_one_active_ride_per_rider may mean either a retry or a new second request.
the key lookup returns the original ride for a retry, or rejects a different key. */

func (s *RideStore) FindByIdempotencyKey(ctx context.Context, riderID, key string) (domain.Ride, error) {
	var ride domain.Ride
	err := s.database.QueryRow(ctx, `
		SELECT
			id::text, rider_id::text, COALESCE(driver_id::text, ''), status,
			pickup_latitude, pickup_longitude,
			destination_latitude, destination_longitude,
			fare_cents, idempotency_key, matching_deadline, created_at, updated_at
		FROM rides
		WHERE rider_id = $1 AND idempotency_key = $2
	`, riderID, key).Scan(
		&ride.ID,
		&ride.RiderID,
		&ride.DriverID,
		&ride.Status,
		&ride.Pickup.Latitude,
		&ride.Pickup.Longitude,
		&ride.Destination.Latitude,
		&ride.Destination.Longitude,
		&ride.FareCents,
		&ride.IdempotencyKey,
		&ride.MatchingDeadline,
		&ride.CreatedAt,
		&ride.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ride{}, fmt.Errorf("find ride by idempotency key: %w", err)
	}
	if err != nil {
		return domain.Ride{}, fmt.Errorf("find ride by idempotency key: %w", err)
	}

	return ride, nil
}
