package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type requestIDContextKey struct{}

type RateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

// RL in redis
// all api tasks use the same counters when they share redis.
func NewRateLimiter(client *redis.Client, limit int, window time.Duration) (*RateLimiter, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("rate limit must be positive")
	}
	if window <= 0 {
		return nil, fmt.Errorf("rate limit window must be positive")
	}
	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
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
		allowed, err := limiter.Allow(r.Context(), client, time.Now())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "rate limiter unavailable"})
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", strconv.FormatInt(int64(limiter.window.Seconds()), 10))
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// applies one atomic fixed window counter for a client address. counter lives in redis

func (RL *RateLimiter) Allow(ctx context.Context, client string, now time.Time) (bool, error) {

	bucket := now.UnixNano() / RL.window.Nanoseconds() // divides time into fixed windows. requests in the same window share a bucket ID

	key := fmt.Sprintf("oslo:rate-limit:%s:%d", client, bucket)

	// increments
	count, err := rateLimitScript.Run(ctx, RL.client, []string{key}, RL.window.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("increment rate limit counter: %w", err)
	}
	return count <= int64(RL.limit), nil
}

var rateLimitScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`) // pexpire sets TTL for key in ms instead of sec

// extracts the host portion used as the rate-limit client key.
// malformed remote addresses fall back to the raw value.
func requestClientAddress(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.SplitN(r.Header.Get("X-Forwarded-For"), ",", 2)[0]); forwarded != "" {
		return forwarded
	}
	address, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return address
	}
	return r.RemoteAddr
}
