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

// creates the ride request service around the ride store.
// ride workflows use this boundary instead of issuing database queries directly.
func NewRequestService(store *RideStore) *RequestService {
	return &RequestService{store: store}
}

// validates a rider request and creates or reuses an idempotent ride.
// the resulting ride is persisted with its initial matching event.
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

// loads a ride while applying the caller's visibility rules.
// rider and driver access are constrained by identity in the store query.
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

// cancels a rider-owned ride when its current status permits cancellation.
// persistence and transition validation happen inside the ride store.
func (s *RequestService) Cancel(ctx context.Context, riderID, rideID string) (domain.Ride, error) {
	if uuid.Validate(rideID) != nil {
		return domain.Ride{}, ErrRideNotFound
	}

	return s.store.CancelByRider(ctx, riderID, rideID)
}

// advances a driver-owned ride through its next lifecycle state.
// the store verifies ownership and rejects illegal transitions transactionally.
func (s *RequestService) Advance(ctx context.Context, driverID, rideID string, next domain.RideStatus) (domain.Ride, error) {
	if uuid.Validate(rideID) != nil {
		return domain.Ride{}, ErrRideNotFound
	}

	// preventing driver req from trying to directly set states like cancelled, failed etc
	switch next {
	case domain.RideStatusDriverArriving,
		domain.RideStatusInProgress,
		domain.RideStatusCompleted:
		return s.store.AdvanceByDriver(ctx, driverID, rideID, next)
	default:
		return domain.Ride{}, domain.ErrInvalidRideTransition
	}
}
