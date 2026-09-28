package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pocketradio/oslo/internal/auth"
	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/driver"
	"github.com/pocketradio/oslo/internal/ride"
	"github.com/pocketradio/oslo/internal/user"
)

func NewRouter(
	database *pgxpool.Pool,
	users *user.Service,
	tokens *auth.TokenManager,
	rideRequests *ride.RequestService,
	drivers *driver.Service,
) http.Handler {
	mux := http.NewServeMux()
	authHandler := newAuthHandler(users, tokens)
	rideHandler := newRideRequestHandler(rideRequests)
	driverHandler := newDriverLocationHandler(drivers)

	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(database))
	mux.HandleFunc("POST /auth/register", authHandler.register)
	mux.HandleFunc("POST /auth/login", authHandler.login)
	mux.Handle(
		"POST /rides/fare-estimate",
		authenticate(tokens, requireRole(domain.UserRoleRider, http.HandlerFunc(handleFareEstimate))),
	)
	mux.Handle(
		"POST /rides",
		authenticate(tokens, requireRole(domain.UserRoleRider, http.HandlerFunc(rideHandler.create))),
	)
	mux.Handle(
		"GET /rides/{rideID}",
		authenticate(tokens, http.HandlerFunc(rideHandler.get)),
	)
	mux.Handle(
		"POST /rides/{rideID}/cancel",
		authenticate(tokens, requireRole(domain.UserRoleRider, http.HandlerFunc(rideHandler.cancel))),
	)
	mux.Handle(
		"PUT /drivers/location",
		authenticate(tokens, requireRole(domain.UserRoleDriver, http.HandlerFunc(driverHandler.update))),
	)

	return mux
}
