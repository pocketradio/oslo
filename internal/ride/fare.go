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

// validates coordinates, estimates route distance, and calculates fare cents.
// the result is deterministic for the same pickup and destination.
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

// calculates great-circle distance between two coordinate pairs.
// the value is used as the distance input to fare calculation.
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

// converts an angular coordinate from degrees into radians.
// trigonometric distance functions use radians as their input unit.
func degreesToRadians(value float64) float64 {
	return value * math.Pi / 180
}
