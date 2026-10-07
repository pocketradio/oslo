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
var ErrRideNotFound = errors.New("ride not found")

type RideStore struct {
	database *pgxpool.Pool
}

// creates the postgres-backed ride store.
// callers use the store for transactional ride persistence and ownership checks.
func NewRideStore(database *pgxpool.Pool) *RideStore {
	return &RideStore{database: database}
}

// inserts a new ride and its initial outbox event in one transaction.
// this keeps ride creation and asynchronous matching durable together.
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
		VALUES ($1, $2, 'match_ride', jsonb_build_object('ride_id', $3::text))
	`, uuid.NewString(), ride.ID, ride.ID)
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

// looks up a previously created ride for one rider and idempotency key.
// a match lets repeated client requests return the original result safely.
func (s *RideStore) FindByIdempotencyKey(ctx context.Context, riderID, key string) (domain.Ride, error) {
	ride, err := scanRide(s.database.QueryRow(ctx, `
		SELECT
			id::text, rider_id::text, COALESCE(driver_id::text, ''), status,
			pickup_latitude, pickup_longitude,
			destination_latitude, destination_longitude,
			fare_cents, idempotency_key, matching_deadline, created_at, updated_at
		FROM rides
		WHERE rider_id = $1 AND idempotency_key = $2
	`, riderID, key))
	if err != nil {
		return domain.Ride{}, fmt.Errorf("find ride by idempotency key: %w", err)
	}

	return ride, nil
}

// loads one ride by its identifier.
// not-found results are translated into the store's domain error.
func (s *RideStore) FindByID(ctx context.Context, rideID string) (domain.Ride, error) {
	ride, err := scanRide(s.database.QueryRow(ctx, `
		SELECT
			id::text, rider_id::text, COALESCE(driver_id::text, ''), status,
			pickup_latitude, pickup_longitude,
			destination_latitude, destination_longitude,
			fare_cents, idempotency_key, matching_deadline, created_at, updated_at
		FROM rides
		WHERE id = $1
	`, rideID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ride{}, ErrRideNotFound
	}
	if err != nil {
		return domain.Ride{}, fmt.Errorf("find ride by id: %w", err)
	}

	return ride, nil
}

// locks a rider-owned ride, validates cancellation, and persists the change.
// the rider filter prevents another user from cancelling the ride.
func (s *RideStore) CancelByRider(ctx context.Context, riderID, rideID string) (domain.Ride, error) {
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return domain.Ride{}, fmt.Errorf("begin ride cancellation: %w", err)
	}
	defer tx.Rollback(ctx)

	ride, err := scanRide(tx.QueryRow(ctx, `
		SELECT
			id::text, rider_id::text, COALESCE(driver_id::text, ''), status,
			pickup_latitude, pickup_longitude,
			destination_latitude, destination_longitude,
			fare_cents, idempotency_key, matching_deadline, created_at, updated_at
		FROM rides
		WHERE id = $1 AND rider_id = $2
		FOR UPDATE
	`, rideID, riderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ride{}, ErrRideNotFound
	}
	if err != nil {
		return domain.Ride{}, fmt.Errorf("lock ride for cancellation: %w", err)
	}

	if err := ride.TransitionTo(domain.RideStatusCancelled); err != nil {
		return domain.Ride{}, err
	}

	err = tx.QueryRow(ctx, `
		UPDATE rides
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
		RETURNING updated_at
	`, ride.Status, ride.ID).Scan(&ride.UpdatedAt)
	if err != nil {
		return domain.Ride{}, fmt.Errorf("cancel ride: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Ride{}, fmt.Errorf("commit ride cancellation: %w", err)
	}

	return ride, nil
}

// locks a driver-owned ride, validates its next state, and persists the change.
// the driver filter prevents a valid driver from changing another driver's ride.
func (s *RideStore) AdvanceByDriver(ctx context.Context, driverID, rideID string, next domain.RideStatus) (domain.Ride, error) {
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return domain.Ride{}, fmt.Errorf("begin trip transition: %w", err)
	}
	defer tx.Rollback(ctx)

	// loads the ride
	ride, err := scanRide(tx.QueryRow(ctx, `
		SELECT
			id::text, rider_id::text, COALESCE(driver_id::text, ''), status,
			pickup_latitude, pickup_longitude,
			destination_latitude, destination_longitude,
			fare_cents, idempotency_key, matching_deadline, created_at, updated_at
		FROM rides
		WHERE id = $1 AND driver_id = $2
		FOR UPDATE
	`, rideID, driverID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ride{}, ErrRideNotFound
	}
	if err != nil {
		return domain.Ride{}, fmt.Errorf("lock ride for trip transition: %w", err)
	}

	// eg. assigned -> driver arriving is allowed
	// assigned -> completed is rejected
	if err := ride.TransitionTo(next); err != nil {
		return domain.Ride{}, err
	}

	if err := tx.QueryRow(ctx, `
		UPDATE rides
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
		RETURNING updated_at
	`, ride.Status, ride.ID).Scan(&ride.UpdatedAt); err != nil {
		return domain.Ride{}, fmt.Errorf("update trip status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Ride{}, fmt.Errorf("commit trip transition: %w", err)
	}
	return ride, nil
}

// maps one postgres ride row into the domain ride value.
// scan errors are returned unchanged so callers can classify them.
func scanRide(row pgx.Row) (domain.Ride, error) {
	var ride domain.Ride
	err := row.Scan(
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
	if err != nil {
		return domain.Ride{}, err
	}

	return ride, nil
}
