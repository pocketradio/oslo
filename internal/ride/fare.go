package ride

import (
	"fmt"
	"math"

	"github.com/pocketradio/oslo/internal/domain"
)

const (
	earthRadiusKilometers = 6371.0
	baseFareCents         = 300
	perKilometerCents     = 150
)

func EstimateFare(pickup, destination domain.Coordinates) (int64, error) {
	if err := pickup.Validate(); err != nil {
		return 0, fmt.Errorf("pickup: %w", err)
	}
	if err := destination.Validate(); err != nil {
		return 0, fmt.Errorf("destination: %w", err)
	}

	distance := distanceKilometers(pickup, destination)
	fare := baseFareCents + int64(math.Round(distance*perKilometerCents))

	return fare, nil
}

func distanceKilometers(from, to domain.Coordinates) float64 {
	latitudeDelta := degreesToRadians(to.Latitude - from.Latitude)
	longitudeDelta := degreesToRadians(to.Longitude - from.Longitude)
	fromLatitude := degreesToRadians(from.Latitude)
	toLatitude := degreesToRadians(to.Latitude)

	// haversine to account for curvature
	a := math.Sin(latitudeDelta/2)*math.Sin(latitudeDelta/2) +
		math.Cos(fromLatitude)*math.Cos(toLatitude)*
			math.Sin(longitudeDelta/2)*math.Sin(longitudeDelta/2)

	return earthRadiusKilometers * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func degreesToRadians(value float64) float64 {
	return value * math.Pi / 180
}
