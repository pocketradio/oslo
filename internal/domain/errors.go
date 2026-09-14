package domain

import "errors"

var ErrInvalidRideTransition = errors.New("invalid ride transition")
var ErrInvalidOfferTransition = errors.New("invalid offer transition")
var ErrOfferExpired = errors.New("offer expired")
var ErrOfferNotExpired = errors.New("offer has not expired")
var ErrInvalidCoordinates = errors.New("invalid coordinates")
