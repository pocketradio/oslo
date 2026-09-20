package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/ride"
)

type createRideRequest struct {
	Pickup      domain.Coordinates `json:"pickup"`
	Destination domain.Coordinates `json:"destination"`
}

type rideResponse struct {
	ID               string             `json:"id"`
	Status           domain.RideStatus  `json:"status"`
	Pickup           domain.Coordinates `json:"pickup"`
	Destination      domain.Coordinates `json:"destination"`
	FareCents        int64              `json:"fare_cents"`
	MatchingDeadline time.Time          `json:"matching_deadline"`
	CreatedAt        time.Time          `json:"created_at"`
}

type rideRequestHandler struct {
	rides *ride.RequestService
}

func newRideRequestHandler(rides *ride.RequestService) *rideRequestHandler {
	return &rideRequestHandler{rides: rides}
}

func (h *rideRequestHandler) create(w http.ResponseWriter, r *http.Request) {
	var request createRideRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	authenticated, ok := r.Context().Value(userContextKey{}).(authenticatedUser) // type assertion
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	created, err := h.rides.Request(
		r.Context(),
		authenticated.ID,
		r.Header.Get("Idempotency-Key"),
		request.Pickup,
		request.Destination,
	)
	switch {
	case errors.Is(err, ride.ErrInvalidIdempotencyKey),
		errors.Is(err, domain.ErrInvalidCoordinates):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	case errors.Is(err, ride.ErrActiveRideExists):
		writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
		return
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, rideResponse{
		ID:               created.ID,
		Status:           created.Status,
		Pickup:           created.Pickup,
		Destination:      created.Destination,
		FareCents:        created.FareCents,
		MatchingDeadline: created.MatchingDeadline,
		CreatedAt:        created.CreatedAt,
	})
}
