package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(database *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(database))

	return mux
}
