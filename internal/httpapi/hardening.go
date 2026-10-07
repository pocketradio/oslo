package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type requestIDContextKey struct{}

type RateLimiter struct {
	limit   int
	window  time.Duration
	mu      sync.Mutex
	clients map[string]rateWindow // IP : requests count
}

// each client gets its own fixed time window

type rateWindow struct {
	started time.Time
	count   int
}

// validates rate-limit settings and creates isolated client windows.
// each limiter instance belongs to one api process.
func NewRateLimiter(limit int, window time.Duration) (*RateLimiter, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("rate limit must be positive")
	}
	if window <= 0 {
		return nil, fmt.Errorf("rate limit window must be positive")
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]rateWindow),
	}, nil
}

// reads the correlation identifier placed into the request context.
// an empty result means no requestID middleware has populated the request
func RequestIDFrom(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDContextKey{}).(string)
	return requestID
}

// accepts a safe client request id or generates one when it is missing.
// the id is copied into context and the response header for tracing through MW chain
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = uuid.NewString()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bounds how long the wrapped handler may take to produce a response.
// the timeout context also propagates to downstream service and database calls.
func WithRequestDeadline(timeout time.Duration, next http.Handler) http.Handler {
	return http.TimeoutHandler(next, timeout, `{"error":"request timed out"}`)
}

// limits how many bytes handlers may read from an incoming request body.
// decoders receive the limiting reader and fail when the configured size is exceeded.
func WithBodyLimit(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// counts requests for the caller's address and rejects exhausted windows.
// accepted requests continue to the next MW in chain
func WithRateLimit(limiter *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := requestClientAddress(r) // returns IP as str
		if !limiter.Allow(client, time.Now()) {
			w.Header().Set("Retry-After", strconv.FormatInt(int64(limiter.window.Seconds()), 10))
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// applies a fixed-window counter to one client address.
// mutex makes window creation and count updates safe across GORs
// client parameter will be the IP as a string

func (RL *RateLimiter) Allow(client string, now time.Time) bool {
	RL.mu.Lock()
	defer RL.mu.Unlock()

	current, ok := RL.clients[client]
	if !ok || now.Sub(current.started) >= RL.window {
		RL.clients[client] = rateWindow{started: now, count: 1}
		return true
	}
	if current.count >= RL.limit {
		return false
	}
	current.count++
	RL.clients[client] = current
	return true
}

// extracts the host portion used as the rate-limit client key.
// malformed remote addresses fall back to the raw value.
func requestClientAddress(r *http.Request) string {
	address, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return address
	}
	return r.RemoteAddr
}
