package ride

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pocketradio/oslo/internal/domain"
)

var ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")

const matchingWindow = 60 * time.Second

type RequestService struct {
	store *RideStore
}

func NewRequestService(store *RideStore) *RequestService {
	return &RequestService{store: store}
}

func (s *RequestService) Request(
	ctx context.Context,
	riderID string,
	idempotencyKey string,
	pickup domain.Coordinates,
	destination domain.Coordinates,
) (domain.Ride, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.Ride{}, ErrInvalidIdempotencyKey
	}

	fare, err := EstimateFare(pickup, destination)
	if err != nil {
		return domain.Ride{}, err
	}

	ride := domain.Ride{
		ID:               uuid.NewString(),
		RiderID:          riderID,
		Status:           domain.RideStatusRequested,
		Pickup:           pickup,
		Destination:      destination,
		FareCents:        fare,
		IdempotencyKey:   idempotencyKey,
		MatchingDeadline: time.Now().Add(matchingWindow),
	}

	return s.store.Create(ctx, ride)
}

func (s *RequestService) Get(
	ctx context.Context,
	userID string,
	role domain.UserRole,
	rideID string,
) (domain.Ride, error) {
	if uuid.Validate(rideID) != nil {
		return domain.Ride{}, ErrRideNotFound
	}

	ride, err := s.store.FindByID(ctx, rideID)
	if err != nil {
		return domain.Ride{}, err
	}

	allowed := role == domain.UserRoleRider && ride.RiderID == userID ||
		role == domain.UserRoleDriver && ride.DriverID == userID
	if !allowed {
		return domain.Ride{}, ErrRideNotFound
	}

	return ride, nil
}
