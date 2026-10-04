package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/matching"
)

type offerHandler struct {
	matching *matching.Service
}

type offerResponse struct {
	ID        string             `json:"id"`
	RideID    string             `json:"ride_id"`
	DriverID  string             `json:"driver_id"`
	Status    domain.OfferStatus `json:"status"`
	ExpiresAt time.Time          `json:"expires_at"`
}

func newOfferHandler(matchingService *matching.Service) *offerHandler {
	return &offerHandler{matching: matchingService}
}

func (h *offerHandler) list(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := authenticatedUserFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	offers, err := h.matching.PendingOffers(r.Context(), authenticated.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	response := make([]offerResponse, 0, len(offers))
	for _, offer := range offers {
		response = append(response, newOfferResponse(offer))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *offerHandler) accept(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, true)
}

func (h *offerHandler) reject(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, false)
}

func (h *offerHandler) respond(w http.ResponseWriter, r *http.Request, accepted bool) {
	authenticated, ok := authenticatedUserFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	offerID := r.PathValue("offerID")
	if uuid.Validate(offerID) != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "offer not found"})
		return
	}

	var err error
	if accepted {
		err = h.matching.AcceptOffer(r.Context(), offerID, authenticated.ID)
	} else {
		err = h.matching.RejectOffer(r.Context(), offerID, authenticated.ID)
	}

	switch {
	case errors.Is(err, matching.ErrOfferNotFound), errors.Is(err, matching.ErrOfferNotForDriver):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "offer not found"})
	case errors.Is(err, matching.ErrOfferNotAvailable), errors.Is(err, domain.ErrOfferExpired), errors.Is(err, domain.ErrInvalidOfferTransition):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "offer is no longer available"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	default:
		writeJSON(w, http.StatusNoContent, nil)
	}
}

func newOfferResponse(offer domain.RideOffer) offerResponse {
	return offerResponse{
		ID:        offer.ID,
		RideID:    offer.RideID,
		DriverID:  offer.DriverID,
		Status:    offer.Status,
		ExpiresAt: offer.ExpiresAt,
	}
}
