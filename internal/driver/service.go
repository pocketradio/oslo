package driver

import (
	"context"
	"time"

	"github.com/pocketradio/oslo/internal/domain"
)

const (
	staleDriverAfter     = 30 * time.Second
	staleCleanupInterval = 10 * time.Second
)

type Service struct {
	locations *LocationStore
}

// creates the driver service around the location and reservation store.
// business operations use this boundary instead of accessing redis directly.
func NewService(locations *LocationStore) *Service {
	return &Service{locations: locations}
}

// validates and persists a driver's reported location.
// the service keeps transport handlers independent from the redis store.
func (s *Service) UpdateLocation(
	ctx context.Context,
	driverID string,
	location domain.Coordinates,
	available bool,
) error {
	return s.locations.UpdateLocation(ctx, driverID, location, available)
}

// finds candidate drivers that are fresh and geographically suitable.
// matching receives only the service-level result and not redis details.
func (s *Service) FindNearby(
	ctx context.Context,
	location domain.Coordinates,
	radiusKm float64,
	limit int,
) ([]NearbyDriver, error) {
	return s.locations.FindNearby(ctx, location, radiusKm, limit)
}

// requests an atomic temporary reservation for one driver.
// the token identifies the caller that may later release it.
func (s *Service) Reserve(ctx context.Context, driverID string) (string, bool, error) {
	return s.locations.ReserveDriver(ctx, driverID, staleDriverAfter)
}

// releases a previously acquired driver reservation.
// an incorrect token cannot release another matching attempt's lease.
func (s *Service) Release(ctx context.Context, driverID, token string) error {
	return s.locations.ReleaseDriver(ctx, driverID, token)
}

// periodically removes driver locations that are no longer fresh.
// the loop stops with the application context and reports recoverable failures.
func (s *Service) RunStaleCleanup(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(staleCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C: // receives a time.Time value but ignored since unblocking the channel is the goal
			_, err := s.locations.RemoveStale(ctx, staleDriverAfter)
			if err != nil && ctx.Err() == nil && onError != nil {
				onError(err)
			}
		case <-ctx.Done():
			return
		}
	}
}
