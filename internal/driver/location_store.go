package driver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/pocketradio/oslo/internal/domain"
)

const driverLocationKey = "drivers:locations"
const reservationTTL = 10 * time.Second

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
	if strings.TrimSpace(driverID) == "" {
		return fmt.Errorf("driver id is required")
	}
	if err := location.Validate(); err != nil {
		return err
	}

	// single tx to update loc and availability tog. hset just tracks each driver's redis metadata ( state )

	_, err := s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, driverKey(driverID), map[string]any{
			"available":    available,
			"latitude":     location.Latitude,
			"longitude":    location.Longitude,
			"last_seen_ms": time.Now().UnixMilli(),
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

func (s *LocationStore) RemoveStale(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, fmt.Errorf("max age must be positive")
	}

	// index 0 to -1 , geoindex is a sorted set
	// returns all drivers in the SS ( so driverIDs -> []string type )
	driverIDs, err := s.redis.ZRange(ctx, driverLocationKey, 0, -1).Result()
	if err != nil {
		return 0, fmt.Errorf("list driver locations: %w", err)
	}

	cutoff := time.Now().Add(-maxAge).UnixMilli() // -maxAge to get timestamp from 30s ago

	removed := 0

	for _, driverID := range driverIDs {
		result, err := removeStaleDriverScript.Run(
			ctx,
			s.redis,
			[]string{driverKey(driverID), driverLocationKey}, //keys 1,2
			cutoff,   //argv1
			driverID, //argv2
		).Int() // to convt to Go int,err

		if err != nil {
			return removed, fmt.Errorf("remove stale driver %s: %w", driverID, err)
		}
		removed += result
	}

	return removed, nil
}

func (s *LocationStore) ReserveDriver(ctx context.Context, driverID string, maxAge time.Duration) (string, bool, error) {
	if strings.TrimSpace(driverID) == "" {
		return "", false, fmt.Errorf("driver id is required")
	}
	if maxAge <= 0 {
		return "", false, fmt.Errorf("max age must be positive")
	}

	token := uuid.NewString()
	reservationKey := reservationKey(driverID)
	reserved, err := reserveDriverScript.Run(
		ctx,
		s.redis,
		[]string{driverKey(driverID), reservationKey},
		token,
		reservationTTL.Milliseconds(),
		time.Now().Add(-maxAge).UnixMilli(),
	).Bool()
	if err != nil {
		return "", false, fmt.Errorf("reserve driver: %w", err)
	}
	if !reserved {
		return "", false, nil
	}

	return token, true, nil
}

func (s *LocationStore) ReleaseDriver(ctx context.Context, driverID, token string) error {
	if strings.TrimSpace(driverID) == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("driver id and reservation token are required")
	}

	if err := releaseReservationScript.Run(
		ctx,
		s.redis,
		[]string{reservationKey(driverID)},
		token,
	).Err(); err != nil {
		return fmt.Errorf("release driver reservation: %w", err)
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

func reservationKey(driverID string) string {
	return "driver:reservation:" + driverID
}
