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
	DriverID         string             `json:"driver_id,omitempty"`
	Status           domain.RideStatus  `json:"status"`
	Pickup           domain.Coordinates `json:"pickup"`
	Destination      domain.Coordinates `json:"destination"`
	FareCents        int64              `json:"fare_cents"`
	MatchingDeadline time.Time          `json:"matching_deadline"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
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

	authenticated, ok := authenticatedUserFrom(r)
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

	writeJSON(w, http.StatusCreated, newRideResponse(created))
}

func (h *rideRequestHandler) get(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := authenticatedUserFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	found, err := h.rides.Get(r.Context(), authenticated.ID, authenticated.Role, r.PathValue("rideID"))
	if errors.Is(err, ride.ErrRideNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, newRideResponse(found))
}

func (h *rideRequestHandler) cancel(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := authenticatedUserFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	cancelled, err := h.rides.Cancel(r.Context(), authenticated.ID, r.PathValue("rideID"))
	switch {
	case errors.Is(err, ride.ErrRideNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	case errors.Is(err, domain.ErrInvalidRideTransition):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "ride cannot be cancelled"})
		return
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, newRideResponse(cancelled))
}

func newRideResponse(ride domain.Ride) rideResponse {
	return rideResponse{
		ID:               ride.ID,
		DriverID:         ride.DriverID,
		Status:           ride.Status,
		Pickup:           ride.Pickup,
		Destination:      ride.Destination,
		FareCents:        ride.FareCents,
		MatchingDeadline: ride.MatchingDeadline,
		CreatedAt:        ride.CreatedAt,
		UpdatedAt:        ride.UpdatedAt,
	}
}
