package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/driver"
	"github.com/pocketradio/oslo/internal/queue"
)

const (
	searchRadiusKm = 10
	candidateLimit = 20
	offerDuration  = 10 * time.Second
)

type Service struct {
	store   *Store
	drivers *driver.Service
}

// wires the matching store to the driver service.
// creates the matching service with transactional storage and driver discovery.
// matching decisions stay in this layer while persistence remains in the store.
func NewService(store *Store, drivers *driver.Service) *Service {
	return &Service{store: store, drivers: drivers}
}

// routes queue messages to matching or offer-timeout handling.
// decodes a queue message and dispatches it to the matching operation it names.
// unknown message types are rejected so they are not acknowledged accidentally.
func (s *Service) HandleMessage(ctx context.Context, message queue.Message) error {
	switch message.Type {
	case queue.MessageTypeMatchRide:
		var payload queue.MatchRideMessage
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return fmt.Errorf("decode match ride message: %w", err)
		}
		return s.MatchRide(ctx, payload.RideID)
	case queue.MessageTypeOfferTimeout:
		var payload queue.OfferTimeoutMessage
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return fmt.Errorf("decode offer timeout message: %w", err)
		}
		return s.ExpireOffer(ctx, payload.OfferID)
	default:
		return fmt.Errorf("unsupported queue message type %q", message.Type)
	}
}

// prepares a ride, searches nearby drivers, reserves one, and creates an offer.
// failed reservations try the next driver; infrastructure errors are returned for retry.
// finds a driver, reserves it, and creates a time-limited offer for a ride.
// no candidate or a failed offer leaves the ride available for later handling.
func (s *Service) MatchRide(ctx context.Context, rideID string) error {
	ride, err := s.store.PrepareRide(ctx, rideID, time.Now())
	if err != nil {
		if err == ErrRideNotMatchable {
			return nil
		}
		return err
	}

	candidates, err := s.drivers.FindNearby(ctx, ride.Pickup, searchRadiusKm, candidateLimit)
	if err != nil {
		return err
	}

	for _, candidate := range candidates {
		token, reserved, err := s.drivers.Reserve(ctx, candidate.ID) // token = fresh uuid
		if err != nil {
			return err
		}
		if !reserved {
			continue
		}

		offered, err := s.store.CreateOffer(ctx, ride.ID, candidate.ID, uuid.NewString(), time.Now().Add(offerDuration))

		// offer creation failed , redis reservation is released and SQS will retry.
		if err != nil {
			_ = s.drivers.Release(ctx, candidate.ID, token)
			return err
		}
		if offered {
			return nil
		}
		if err := s.drivers.Release(ctx, candidate.ID, token); err != nil {
			return err
		}
	}

	return s.store.FailRide(ctx, ride.ID)
}

// records a driver's acceptance and assigns the ride atomically.
// accepts a driver's offer through the store's transactional state transition.
// the driver identity is passed through so ownership is checked in the database.
func (s *Service) AcceptOffer(ctx context.Context, offerID, driverID string) error {
	return s.store.RespondToOffer(ctx, offerID, driverID, true, time.Now())
}

// records rejection and queues another matching attempt.
// rejects a driver's offer and allows matching to continue for the ride.
// ownership and current offer state are validated transactionally.
func (s *Service) RejectOffer(ctx context.Context, offerID, driverID string) error {
	return s.store.RespondToOffer(ctx, offerID, driverID, false, time.Now())
}

// handles a timeout message and safely ignores already-resolved offers.
// expires an offer and schedules another matching attempt when appropriate.
// the same transaction updates offer and ride state with its outbox event.
func (s *Service) ExpireOffer(ctx context.Context, offerID string) error {
	err := s.store.ExpireOffer(ctx, offerID, time.Now())
	if err == ErrOfferNotFound || err == ErrOfferNotAvailable {
		return nil
	}
	return err
}

// returns the driver's currently visible offers for polling.
// returns currently pending offers assigned to one driver.
// the store applies the driver filter so callers cannot see another driver's offers.
func (s *Service) PendingOffers(ctx context.Context, driverID string) ([]domain.RideOffer, error) {
	return s.store.PendingOffers(ctx, driverID)
}

// finds expired pending offers and applies their expiration transitions.
// this repairs work whose delayed queue message was lost or delayed.
func (s *Service) RecoverExpiredOffers(ctx context.Context) error {
	offerIDs, err := s.store.FindExpiredOfferIDs(ctx, time.Now(), 100)
	if err != nil {
		return err
	}
	for _, offerID := range offerIDs {
		if err := s.ExpireOffer(ctx, offerID); err != nil {
			return fmt.Errorf("recover expired offer %s: %w", offerID, err)
		}
	}
	return nil
}

// periodically runs expired-offer recovery until application shutdown.
// failures are reported while the next interval keeps the repair loop alive.
func (s *Service) RunExpiredOfferRecovery(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		if err := s.RecoverExpiredOffers(ctx); err != nil && ctx.Err() == nil && onError != nil {
			onError(err)
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}
