package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status string `json:"status"`
}

// reports that the process is alive without checking external dependencies.
// orchestration can use this endpoint to decide whether to restart the process.
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// creates a readiness handler that checks database connectivity per request.
// failure means the process should not receive traffic yet.
func readyz(database *pgxpool.Pool) http.HandlerFunc {

	// the returned handler will run once/readyz request

	return func(w http.ResponseWriter, r *http.Request) {
		if err := database.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not ready"})
			return
		}

		writeJSON(w, http.StatusOK, healthResponse{Status: "ready"})
	}
}

// serializes a value as json with the supplied http status.
// api handlers use this helper for consistent response headers and encoding.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
