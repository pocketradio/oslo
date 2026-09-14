package domain

import (
	"fmt"
	"math"
)

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

func (c Coordinates) Validate() error {
	if math.IsNaN(c.Latitude) || math.IsInf(c.Latitude, 0) {
		return fmt.Errorf("%w: latitude must be finite", ErrInvalidCoordinates)
	}
	if math.IsNaN(c.Longitude) || math.IsInf(c.Longitude, 0) {
		return fmt.Errorf("%w: longitude must be finite", ErrInvalidCoordinates)
	}
	if c.Latitude < -90 || c.Latitude > 90 {
		return fmt.Errorf("%w: latitude must be between -90 and 90", ErrInvalidCoordinates)
	}
	if c.Longitude < -180 || c.Longitude > 180 {
		return fmt.Errorf("%w: longitude must be between -180 and 180", ErrInvalidCoordinates)
	}

	return nil
}
