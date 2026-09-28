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

func NewService(locations *LocationStore) *Service {
	return &Service{locations: locations}
}

func (s *Service) UpdateLocation(
	ctx context.Context,
	driverID string,
	location domain.Coordinates,
	available bool,
) error {
	return s.locations.UpdateLocation(ctx, driverID, location, available)
}

func (s *Service) FindNearby(
	ctx context.Context,
	location domain.Coordinates,
	radiusKm float64,
	limit int,
) ([]NearbyDriver, error) {
	return s.locations.FindNearby(ctx, location, radiusKm, limit)
}

func (s *Service) Reserve(ctx context.Context, driverID string) (string, bool, error) {
	return s.locations.ReserveDriver(ctx, driverID, staleDriverAfter)
}

func (s *Service) Release(ctx context.Context, driverID, token string) error {
	return s.locations.ReleaseDriver(ctx, driverID, token)
}

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
