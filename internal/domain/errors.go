package domain

import "errors"

var ErrInvalidRideTransition = errors.New("invalid ride transition")
var ErrInvalidOfferTransition = errors.New("invalid offer transition")
var ErrOfferExpired = errors.New("offer expired")
var ErrOfferNotExpired = errors.New("offer has not expired")
var ErrInvalidCoordinates = errors.New("invalid coordinates")
var ErrUserNotFound = errors.New("user not found")
var ErrEmailTaken = errors.New("email already in use")
var ErrInvalidEmail = errors.New("invalid email")
var ErrInvalidUserRole = errors.New("invalid user role")
var ErrInvalidCredentials = errors.New("invalid credentials")
