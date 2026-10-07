package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pocketradio/oslo/internal/auth"
	"github.com/pocketradio/oslo/internal/domain"
	"github.com/pocketradio/oslo/internal/driver"
	"github.com/pocketradio/oslo/internal/matching"
	"github.com/pocketradio/oslo/internal/ride"
	"github.com/pocketradio/oslo/internal/user"
)

// creates the route table and binds endpoints to service-backed handlers.
// authentication and role middleware are attached to protected routes here.
func NewRouter(
	database *pgxpool.Pool,
	users *user.Service,
	tokens *auth.TokenManager,
	rideRequests *ride.RequestService,
	drivers *driver.Service,
	matchingService *matching.Service,
) http.Handler {
	mux := http.NewServeMux()
	authHandler := newAuthHandler(users, tokens)
	rideHandler := newRideRequestHandler(rideRequests)
	driverHandler := newDriverLocationHandler(drivers)
	offerHandler := newOfferHandler(matchingService)
	tripHandler := newTripHandler(rideRequests)

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
	mux.Handle(
		"GET /drivers/offers",
		authenticate(tokens, requireRole(domain.UserRoleDriver, http.HandlerFunc(offerHandler.list))),
	)
	mux.Handle(
		"POST /drivers/offers/{offerID}/accept",
		authenticate(tokens, requireRole(domain.UserRoleDriver, http.HandlerFunc(offerHandler.accept))),
	)
	mux.Handle(
		"POST /drivers/offers/{offerID}/reject",
		authenticate(tokens, requireRole(domain.UserRoleDriver, http.HandlerFunc(offerHandler.reject))),
	)
	mux.Handle(
		"POST /drivers/rides/{rideID}/arrive",
		authenticate(tokens, requireRole(domain.UserRoleDriver, http.HandlerFunc(tripHandler.arrive))),
	)
	mux.Handle(
		"POST /drivers/rides/{rideID}/start",
		authenticate(tokens, requireRole(domain.UserRoleDriver, http.HandlerFunc(tripHandler.start))),
	)
	mux.Handle(
		"POST /drivers/rides/{rideID}/complete",
		authenticate(tokens, requireRole(domain.UserRoleDriver, http.HandlerFunc(tripHandler.complete))),
	)

	return mux
}
