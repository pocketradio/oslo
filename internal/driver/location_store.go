package driver

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/pocketradio/oslo/internal/domain"
)

const driverLocationKey = "drivers:locations"

type NearbyDriver struct {
	ID         string
	DistanceKm float64
}

type LocationStore struct {
	redis *redis.Client
}

func NewLocationStore(client *redis.Client) *LocationStore {
	return &LocationStore{redis: client}
}

func (s *LocationStore) UpdateLocation(
	ctx context.Context,
	driverID string,
	location domain.Coordinates,
	available bool,
) error {
	if err := location.Validate(); err != nil {
		return err
	}

	// single tx to update loc and availability tog. hset just tracks each driver's redis metadata ( state )

	_, err := s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, driverKey(driverID), map[string]any{
			"available": available,
			"latitude":  location.Latitude,
			"longitude": location.Longitude,
		})

		if available {
			pipe.GeoAdd(ctx, driverLocationKey, &redis.GeoLocation{
				Name:      driverID,
				Longitude: location.Longitude,
				Latitude:  location.Latitude,
			})
		} else {
			pipe.ZRem(ctx, driverLocationKey, driverID) // removes from geo sorted set
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update driver location: %w", err)
	}

	return nil
}

func (s *LocationStore) FindNearby(
	ctx context.Context,
	location domain.Coordinates,
	radiusKm float64,
	limit int, // at most no of drivers returned
) ([]NearbyDriver, error) {
	if err := location.Validate(); err != nil {
		return nil, err
	}
	if radiusKm <= 0 || limit <= 0 {
		return nil, fmt.Errorf("radius and limit must be positive")
	}

	locations, err := s.redis.GeoSearchLocation(ctx, driverLocationKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  location.Longitude,
			Latitude:   location.Latitude,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
			Count:      limit,
		},
		WithDist: true, // include each driver's distance
	}).Result() // this gives a []redis.geolocation, err
	if err != nil {
		return nil, fmt.Errorf("find nearby drivers: %w", err)
	}

	// converting to nearbydriver type :

	nearby := make([]NearbyDriver, 0, len(locations))
	for _, location := range locations {
		nearby = append(nearby, NearbyDriver{
			ID:         location.Name,
			DistanceKm: location.Dist,
		})
	}

	return nearby, nil
}

func driverKey(driverID string) string {
	return "driver:" + driverID
}
