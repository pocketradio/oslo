package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/ride"
)

type fareEstimateRequest struct {
	Pickup      domain.Coordinates `json:"pickup"`
	Destination domain.Coordinates `json:"destination"`
}

type fareEstimateResponse struct {
	FareCents int64 `json:"fare_cents"`
}

func handleFareEstimate(w http.ResponseWriter, r *http.Request) {
	var request fareEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	fare, err := ride.EstimateFare(request.Pickup, request.Destination)
	if errors.Is(err, domain.ErrInvalidCoordinates) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, fareEstimateResponse{FareCents: fare})
}
