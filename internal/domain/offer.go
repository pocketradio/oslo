package domain

import (
	"fmt"
	"time"
)

type OfferStatus string

const (
	OfferStatusPending  OfferStatus = "pending"
	OfferStatusAccepted OfferStatus = "accepted"
	OfferStatusRejected OfferStatus = "rejected"
	OfferStatusExpired  OfferStatus = "expired"
)

type RideOffer struct {
	ID          string
	RideID      string
	DriverID    string
	Status      OfferStatus
	ExpiresAt   time.Time
	RespondedAt *time.Time
}

func (o *RideOffer) Accept(now time.Time) error {
	return o.respond(OfferStatusAccepted, now)
}

func (o *RideOffer) Reject(now time.Time) error {
	return o.respond(OfferStatusRejected, now)
}

func (o *RideOffer) Expire(now time.Time) error {
	if o.Status != OfferStatusPending {
		return fmt.Errorf("%w: %s to %s", ErrInvalidOfferTransition, o.Status, OfferStatusExpired)
	}
	if now.Before(o.ExpiresAt) {
		return ErrOfferNotExpired
	}

	o.Status = OfferStatusExpired
	return nil
}

func (o *RideOffer) respond(next OfferStatus, now time.Time) error {
	if o.Status != OfferStatusPending { // only pending offers can receive a response
		return fmt.Errorf("%w: %s to %s", ErrInvalidOfferTransition, o.Status, next)
	}
	if !now.Before(o.ExpiresAt) { // offer expired
		return ErrOfferExpired
	}

	o.Status = next
	o.RespondedAt = &now
	return nil
}
