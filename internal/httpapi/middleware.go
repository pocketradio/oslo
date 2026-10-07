package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/pocketradio/oslo/internal/auth"
	"github.com/pocketradio/oslo/internal/domain"
)

type userContextKey struct{}

type authenticatedUser struct {
	ID   string
	Role domain.UserRole
}

// retrieves the verified identity attached by authentication middleware.
// the boolean reports whether authentication populated the request context.
func authenticatedUserFrom(r *http.Request) (authenticatedUser, bool) {
	user, ok := r.Context().Value(userContextKey{}).(authenticatedUser)
	return user, ok
}

// validates a bearer token and places its identity into request context.
// missing or invalid credentials stop the request with an unauthorized response.
func authenticate(tokens *auth.TokenManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}

		payload, err := tokens.Verify(parts[1])
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid access token"})
			return
		}

		user := authenticatedUser{ID: payload.Subject, Role: payload.Role}
		// net/http handlers only receive a response writer and request.
		// so the context attaches the verified identity to this request so role checks and
		// later handlers can read it without extra function parameters

		ctx := context.WithValue(r.Context(), userContextKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx)) // to pass to next middleware in chain
	})
}

// checks that the authenticated identity has the role required by an endpoint.
// a valid token with another role is rejected before business logic runs.
func requireRole(role domain.UserRole, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUserFrom(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}
		if user.Role != role {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
