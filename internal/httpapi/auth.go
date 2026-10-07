package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	authn "github.com/pocketradio/oslo/internal/auth"
	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/user"
)

type authHandler struct {
	users  *user.Service
	tokens *authn.TokenManager
}

type registerRequest struct {
	Email    string          `json:"email"`
	Password string          `json:"password"`
	Role     domain.UserRole `json:"role"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        string          `json:"id"`
	Email     string          `json:"email"`
	Role      domain.UserRole `json:"role"`
	CreatedAt time.Time       `json:"created_at"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// connects authentication endpoints to the user service and token manager.
// the returned handler keeps transport wiring separate from authentication logic.
func newAuthHandler(users *user.Service, tokens *authn.TokenManager) *authHandler {
	return &authHandler{users: users, tokens: tokens}
}

// decodes a registration request and creates a user through the user service.
// validation failures become client errors while unexpected failures stay generic.
func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	created, err := h.users.Register(r.Context(), request.Email, request.Password, request.Role)
	switch {
	case errors.Is(err, domain.ErrEmailTaken):
		writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
		return
	case errors.Is(err, domain.ErrInvalidEmail),
		errors.Is(err, domain.ErrInvalidUserRole),
		errors.Is(err, authn.ErrPasswordTooShort),
		errors.Is(err, authn.ErrPasswordTooLong):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, userResponse{
		ID:        created.ID,
		Email:     created.Email,
		Role:      created.Role,
		CreatedAt: created.CreatedAt,
	})
}

// verifies credentials and returns a signed access token on success.
// invalid credentials are reported without revealing which field failed.
func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	authenticated, err := h.users.Authenticate(r.Context(), request.Email, request.Password)
	if errors.Is(err, domain.ErrInvalidCredentials) {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	jwtToken, err := h.tokens.Issue(authenticated)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{AccessToken: jwtToken})
}
