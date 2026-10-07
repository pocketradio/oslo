package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/driver"
)

type driverLocationRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Available bool    `json:"available"`
}

type driverLocationHandler struct {
	drivers *driver.Service
}

// connects the driver location endpoint to the driver service.
// construction keeps routing independent of service internals.
func newDriverLocationHandler(drivers *driver.Service) *driverLocationHandler {
	return &driverLocationHandler{drivers: drivers}
}

// decodes and stores the authenticated driver's latest coordinates.
// malformed input and service failures are translated into http responses.
func (h *driverLocationHandler) update(w http.ResponseWriter, r *http.Request) {
	var request driverLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	authenticated, ok := authenticatedUserFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	err := h.drivers.UpdateLocation(
		r.Context(),
		authenticated.ID,
		domain.Coordinates{Latitude: request.Latitude, Longitude: request.Longitude},
		request.Available,
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
