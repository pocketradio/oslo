package domain

import (
	"fmt"
	"time"
)

type RideStatus string

const ( // only allowed opts
	RideStatusRequested      RideStatus = "requested"
	RideStatusMatching       RideStatus = "matching"
	RideStatusOffered        RideStatus = "offered"
	RideStatusAssigned       RideStatus = "assigned"
	RideStatusDriverArriving RideStatus = "driver_arriving"
	RideStatusInProgress     RideStatus = "in_progress"
	RideStatusCompleted      RideStatus = "completed"
	RideStatusCancelled      RideStatus = "cancelled"
	RideStatusFailed         RideStatus = "failed"
)

type Ride struct {
	ID               string
	RiderID          string
	DriverID         string
	Status           RideStatus
	Pickup           Coordinates
	Destination      Coordinates
	FareCents        int64
	IdempotencyKey   string
	MatchingDeadline time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// this fn is called with a requested status.
// it checks if the current status ( rideStatus ) can transition to the reqd one.

func (r *Ride) TransitionTo(next RideStatus) error {
	if !r.Status.canTransitionTo(next) {
		return fmt.Errorf("%w from %s to %s", ErrInvalidRideTransition, r.Status, next)
	}

	r.Status = next
	return nil
}

// checks if current status -> next is legal.
func (s RideStatus) canTransitionTo(next RideStatus) bool {
	switch s {
	case RideStatusRequested:
		return next == RideStatusMatching || next == RideStatusCancelled
	case RideStatusMatching:
		return next == RideStatusOffered || next == RideStatusFailed || next == RideStatusCancelled
	case RideStatusOffered:
		return next == RideStatusAssigned || next == RideStatusMatching || next == RideStatusCancelled
	case RideStatusAssigned:
		return next == RideStatusDriverArriving || next == RideStatusCancelled
	case RideStatusDriverArriving:
		return next == RideStatusInProgress || next == RideStatusCancelled
	case RideStatusInProgress:
		return next == RideStatusCompleted
	default:
		return false
	}
}
