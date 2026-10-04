package matching

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pocketradio/oslo/internal/domain"
)

var (
	ErrOfferNotFound     = errors.New("offer not found")
	ErrOfferNotAvailable = errors.New("offer is not available")
	ErrOfferNotForDriver = errors.New("offer does not belong to driver")
	ErrRideNotMatchable  = errors.New("ride is not matchable")
)

type Store struct {
	database *pgxpool.Pool
}

// creates the postgres store used by the matching service.
func NewStore(database *pgxpool.Pool) *Store {
	return &Store{database: database}
}

// locks a ride and moves requested to matching.
// expired or already-processed rides are returned as not matchable; the original message is not requeued.
func (s *Store) PrepareRide(ctx context.Context, rideID string, now time.Time) (domain.Ride, error) {
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return domain.Ride{}, fmt.Errorf("begin ride matching: %w", err)
	}
	defer tx.Rollback(ctx)

	ride, err := lockRide(ctx, tx, rideID)
	if err != nil {
		return domain.Ride{}, err
	}

	// requested -> matching state change is valid.
	// if its offered, assigned, cancelled, failed, then its a duplicate Q message.

	switch ride.Status {
	case domain.RideStatusRequested:

		// past the deadline, so even req / matching -> failed.

		if !now.Before(ride.MatchingDeadline) {
			if err := ride.TransitionTo(domain.RideStatusFailed); err != nil {
				return domain.Ride{}, err
			}
			if err := updateRideStatus(ctx, tx, ride); err != nil {
				return domain.Ride{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return domain.Ride{}, fmt.Errorf("commit expired ride: %w", err)
			}
			return domain.Ride{}, ErrRideNotMatchable
		}

		// changes from req -> matching
		if err := ride.TransitionTo(domain.RideStatusMatching); err != nil {
			return domain.Ride{}, err
		}

		// if above ok, then persist to PG
		if err := updateRideStatus(ctx, tx, ride); err != nil {
			return domain.Ride{}, err
		}

	case domain.RideStatusMatching:
		if !now.Before(ride.MatchingDeadline) {

			// deadline passed, mark as failed
			if err := ride.TransitionTo(domain.RideStatusFailed); err != nil {
				return domain.Ride{}, err
			}

			// update pg status to failed
			if err := updateRideStatus(ctx, tx, ride); err != nil {
				return domain.Ride{}, err
			}

			// now committing the expired ride
			if err := tx.Commit(ctx); err != nil {
				return domain.Ride{}, fmt.Errorf("commit expired ride: %w", err)
			}
			// return if commit success
			return domain.Ride{}, ErrRideNotMatchable
		}

	default:
		return domain.Ride{}, ErrRideNotMatchable
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Ride{}, fmt.Errorf("commit ride matching: %w", err)
	}

	// msg handled successfully, so it can be deleted from SQS
	return ride, nil
}

// atomically inserts a pending offer, moves matching to offered
// and records the offer-timeout event in the same postgres tx
func (s *Store) CreateOffer(ctx context.Context, rideID, driverID string, offerID string, expiresAt time.Time) (bool, error) {
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin offer creation: %w", err)
	}
	defer tx.Rollback(ctx)

	ride, err := lockRide(ctx, tx, rideID)
	if err != nil {
		return false, err
	}
	if ride.Status != domain.RideStatusMatching || !time.Now().Before(ride.MatchingDeadline) {
		return false, nil
	}

	payload, err := json.Marshal(map[string]string{
		"ride_id":  rideID,
		"offer_id": offerID,
	})
	if err != nil {
		return false, fmt.Errorf("encode offer timeout: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO ride_offers (id, ride_id, driver_id, status, expires_at)
		VALUES ($1, $2, $3, 'pending', $4)
	`, offerID, rideID, driverID, expiresAt); err != nil {
		if isOfferConflict(err) {
			return false, nil
		}
		return false, fmt.Errorf("create ride offer: %w", err)
	}

	if err := ride.TransitionTo(domain.RideStatusOffered); err != nil {
		return false, err
	}

	//postgres update with the same tx
	if err := updateRideStatus(ctx, tx, ride); err != nil {
		return false, err
	}

	// outbox update
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (id, ride_id, event_type, payload)
		VALUES ($1, $2, 'offer_timeout', $3)
	`, uuid.NewString(), rideID, payload); err != nil {
		return false, fmt.Errorf("create offer timeout event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit ride offer: %w", err)
	}
	return true, nil
}

// locks a matching ride and marks it failed when no driver can be offered it.
func (s *Store) FailRide(ctx context.Context, rideID string) error {
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ride failure: %w", err)
	}
	defer tx.Rollback(ctx)

	ride, err := lockRide(ctx, tx, rideID)
	if err != nil {
		if err == ErrRideNotMatchable {
			return nil
		}
		return err
	}
	if ride.Status != domain.RideStatusMatching {
		return nil
	}
	if err := ride.TransitionTo(domain.RideStatusFailed); err != nil {
		return err
	}
	if err := updateRideStatus(ctx, tx, ride); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ride failure: %w", err)
	}
	return nil
}

// accepts or rejects a pending offer and updates the ride.
// acceptance assigns the ride. rejection returns it to matching and creates a retry event
func (s *Store) RespondToOffer(ctx context.Context, offerID, driverID string, accepted bool, now time.Time) error {
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin offer response: %w", err)
	}
	defer tx.Rollback(ctx)

	offer, ride, err := lockOfferAndRide(ctx, tx, offerID)
	if err != nil {
		return err
	}
	if offer.DriverID != driverID {
		return ErrOfferNotForDriver
	}
	if offer.Status != domain.OfferStatusPending {
		return ErrOfferNotAvailable
	}

	if accepted {
		if err := offer.Accept(now); err != nil {
			return err
		}
		if err := ride.TransitionTo(domain.RideStatusAssigned); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE rides SET status = $1, driver_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`, ride.Status, driverID, ride.ID); err != nil {
			return fmt.Errorf("assign ride: %w", err)
		}
	} else {
		if err := offer.Reject(now); err != nil {
			return err
		}

		// sets it to matching again
		if err := ride.TransitionTo(domain.RideStatusMatching); err != nil {
			return err
		}
		if err := updateRideStatus(ctx, tx, ride); err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]string{"ride_id": ride.ID})
		if err != nil {
			return fmt.Errorf("encode retry event: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id, ride_id, event_type, payload) VALUES ($1, $2, 'match_ride', $3)`, uuid.NewString(), ride.ID, payload); err != nil {
			return fmt.Errorf("create matching retry event: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE ride_offers SET status = $1, responded_at = $2 WHERE id = $3`, offer.Status, offer.RespondedAt, offer.ID); err != nil {
		return fmt.Errorf("update ride offer: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit offer response: %w", err)
	}
	return nil
}

// marks a pending offer expired after its deadline.
// if it held the ride, offered returns to matching and a retry event is written
func (s *Store) ExpireOffer(ctx context.Context, offerID string, now time.Time) error {
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin offer expiry: %w", err)
	}
	defer tx.Rollback(ctx)

	offer, ride, err := lockOfferAndRide(ctx, tx, offerID)
	if err != nil {
		return err
	}
	if offer.Status != domain.OfferStatusPending {
		return nil
	}
	if err := offer.Expire(now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE ride_offers SET status = $1, responded_at = $2 WHERE id = $3`, offer.Status, now, offer.ID); err != nil {
		return fmt.Errorf("expire ride offer: %w", err)
	}
	if ride.Status == domain.RideStatusOffered {
		if err := ride.TransitionTo(domain.RideStatusMatching); err != nil {
			return err
		}
		if err := updateRideStatus(ctx, tx, ride); err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]string{"ride_id": ride.ID})
		if err != nil {
			return fmt.Errorf("encode retry event: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id, ride_id, event_type, payload) VALUES ($1, $2, 'match_ride', $3)`, uuid.NewString(), ride.ID, payload); err != nil {
			return fmt.Errorf("create matching retry event: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit offer expiry: %w", err)
	}
	return nil
}

// returns non-expired pending offers belonging to a driver.
func (s *Store) PendingOffers(ctx context.Context, driverID string) ([]domain.RideOffer, error) {
	rows, err := s.database.Query(ctx, `
		SELECT id::text, ride_id::text, driver_id::text, status, expires_at, responded_at
		FROM ride_offers
		WHERE driver_id = $1 AND status = 'pending' AND expires_at > CURRENT_TIMESTAMP
		ORDER BY created_at
	`, driverID)
	if err != nil {
		return nil, fmt.Errorf("list pending offers: %w", err)
	}
	defer rows.Close()

	offers := make([]domain.RideOffer, 0)
	for rows.Next() {
		var offer domain.RideOffer
		if err := rows.Scan(&offer.ID, &offer.RideID, &offer.DriverID, &offer.Status, &offer.ExpiresAt, &offer.RespondedAt); err != nil {
			return nil, fmt.Errorf("scan pending offer: %w", err)
		}
		offers = append(offers, offer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending offers: %w", err)
	}
	return offers, nil
}

// loads and row locks only the ride inside an existing transaction.
func lockRide(ctx context.Context, tx pgx.Tx, rideID string) (domain.Ride, error) {
	var ride domain.Ride

	// row level lock on the ride id
	err := tx.QueryRow(ctx, `
		SELECT id::text, rider_id::text, COALESCE(driver_id::text, ''), status,
		pickup_latitude, pickup_longitude, destination_latitude, destination_longitude,
		fare_cents, idempotency_key, matching_deadline, created_at, updated_at
		FROM rides WHERE id = $1 FOR UPDATE
	`, rideID).Scan(
		&ride.ID, &ride.RiderID, &ride.DriverID, &ride.Status,
		&ride.Pickup.Latitude, &ride.Pickup.Longitude,
		&ride.Destination.Latitude, &ride.Destination.Longitude,
		&ride.FareCents, &ride.IdempotencyKey, &ride.MatchingDeadline,
		&ride.CreatedAt, &ride.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ride{}, ErrRideNotMatchable
	}
	if err != nil {
		return domain.Ride{}, fmt.Errorf("lock ride: %w", err)
	}
	return ride, nil
}

// locks the offer, reads its ride id then locks that ride too.
// response and timeout flows need both rows protected in one tx
func lockOfferAndRide(ctx context.Context, tx pgx.Tx, offerID string) (domain.RideOffer, domain.Ride, error) {
	var offer domain.RideOffer
	var rideID string
	err := tx.QueryRow(ctx, `SELECT id::text, ride_id::text, driver_id::text, status, expires_at, responded_at FROM ride_offers WHERE id = $1 FOR UPDATE`, offerID).Scan(
		&offer.ID, &rideID, &offer.DriverID, &offer.Status, &offer.ExpiresAt, &offer.RespondedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RideOffer{}, domain.Ride{}, ErrOfferNotFound
	}
	if err != nil {
		return domain.RideOffer{}, domain.Ride{}, fmt.Errorf("lock ride offer: %w", err)
	}
	ride, err := lockRide(ctx, tx, rideID)
	return offer, ride, err
}

// persists a status that has already passed domain validation.
func updateRideStatus(ctx context.Context, tx pgx.Tx, ride domain.Ride) error {
	if _, err := tx.Exec(ctx, `UPDATE rides SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, ride.Status, ride.ID); err != nil {
		return fmt.Errorf("update ride status: %w", err)
	}
	return nil
}

// identifies a uniqueness conflict caused by an existing offer.
func isOfferConflict(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
