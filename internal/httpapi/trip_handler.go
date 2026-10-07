package httpapi

import (
	"errors"
	"net/http"

	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/ride"
)

type tripHandler struct {
	rides *ride.RequestService
}

// connects trip lifecycle endpoints to the ride request service.
// the handler translates driver actions into service transitions.
func newTripHandler(rides *ride.RequestService) *tripHandler {
	return &tripHandler{rides: rides}
}

// moves an assigned ride into the driver-arriving state.
// authorization and transition validation happen before persistence.
func (h *tripHandler) arrive(w http.ResponseWriter, r *http.Request) {
	h.advance(w, r, domain.RideStatusDriverArriving)
}

// moves an arriving ride into the in-progress state.
// the shared transition path returns the updated ride on success.
func (h *tripHandler) start(w http.ResponseWriter, r *http.Request) {
	h.advance(w, r, domain.RideStatusInProgress)
}

// moves an in-progress ride into the completed state.
// invalid lifecycle changes are reported as conflicts.
func (h *tripHandler) complete(w http.ResponseWriter, r *http.Request) {
	h.advance(w, r, domain.RideStatusCompleted)
}

// applies one requested driver lifecycle transition to a ride.
// authorization, ownership, and domain state validation determine the branch.
func (h *tripHandler) advance(w http.ResponseWriter, r *http.Request, next domain.RideStatus) {
	authenticated, ok := authenticatedUserFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	updated, err := h.rides.Advance(
		r.Context(),
		authenticated.ID,
		r.PathValue("rideID"),
		next,
	)
	switch {
	case errors.Is(err, ride.ErrRideNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "ride not found"})
	case errors.Is(err, domain.ErrInvalidRideTransition):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "ride cannot move to this status"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	default:
		writeJSON(w, http.StatusOK, newRideResponse(updated))
	}
}
