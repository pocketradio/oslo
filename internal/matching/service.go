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
func NewService(store *Store, drivers *driver.Service) *Service {
	return &Service{store: store, drivers: drivers}
}

// routes queue messages to matching or offer-timeout handling.
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
func (s *Service) AcceptOffer(ctx context.Context, offerID, driverID string) error {
	return s.store.RespondToOffer(ctx, offerID, driverID, true, time.Now())
}

// records rejection and queues another matching attempt.
func (s *Service) RejectOffer(ctx context.Context, offerID, driverID string) error {
	return s.store.RespondToOffer(ctx, offerID, driverID, false, time.Now())
}

// handles a timeout message and safely ignores already-resolved offers.
func (s *Service) ExpireOffer(ctx context.Context, offerID string) error {
	err := s.store.ExpireOffer(ctx, offerID, time.Now())
	if err == ErrOfferNotFound || err == ErrOfferNotAvailable {
		return nil
	}
	return err
}

// returns the driver's currently visible offers for polling.
func (s *Service) PendingOffers(ctx context.Context, driverID string) ([]domain.RideOffer, error) {
	return s.store.PendingOffers(ctx, driverID)
}
