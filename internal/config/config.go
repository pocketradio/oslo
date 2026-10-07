package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL           string
	RedisURL              string
	SQSEndpoint           string
	SQSQueueURL           string
	AWSRegion             string
	JWTSecret             string
	JWTLifetime           time.Duration
	HTTPAddr              string
	HTTPReadTimeout       time.Duration
	HTTPReadHeaderTimeout time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	HTTPShutdownTimeout   time.Duration
	HTTPRequestTimeout    time.Duration
	HTTPMaxBodyBytes      int64
	HTTPRateLimitRequests int
	HTTPRateLimitWindow   time.Duration
}

// reads environment-backed settings and applies safe development defaults.
// invalid durations or numeric values stop startup instead of being ignored.
func Load() (Config, error) {
	readTimeout, err := duration("HTTP_READ_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	readHeaderTimeout, err := duration("HTTP_READ_HEADER_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	writeTimeout, err := duration("HTTP_WRITE_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	idleTimeout, err := duration("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := duration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	requestTimeout, err := duration("HTTP_REQUEST_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	rateLimitWindow, err := duration("HTTP_RATE_LIMIT_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}

	maxBodyBytes, err := integer64("HTTP_MAX_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	rateLimitRequests, err := integer("HTTP_RATE_LIMIT_REQUESTS", 60)
	if err != nil {
		return Config{}, err
	}

	jwtLifetime, err := duration("JWT_LIFETIME", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	return Config{
		DatabaseURL:           value("DATABASE_URL", "postgres://oslo:oslo@localhost:5433/oslo?sslmode=disable"),
		RedisURL:              value("REDIS_URL", "redis://localhost:6379"),
		SQSEndpoint:           value("SQS_ENDPOINT", "http://localhost:4566"),
		SQSQueueURL:           value("SQS_QUEUE_URL", "http://localhost:4566/000000000000/oslo-matching"),
		AWSRegion:             value("AWS_REGION", "ap-south-1"),
		JWTSecret:             value("JWT_SECRET", "oslo-local-jwt-signing-secret-key"),
		JWTLifetime:           jwtLifetime,
		HTTPAddr:              value("HTTP_ADDR", ":8080"),
		HTTPReadTimeout:       readTimeout,
		HTTPReadHeaderTimeout: readHeaderTimeout,
		HTTPWriteTimeout:      writeTimeout,
		HTTPIdleTimeout:       idleTimeout,
		HTTPShutdownTimeout:   shutdownTimeout,
		HTTPRequestTimeout:    requestTimeout,
		HTTPMaxBodyBytes:      maxBodyBytes,
		HTTPRateLimitRequests: rateLimitRequests,
		HTTPRateLimitWindow:   rateLimitWindow,
	}, nil
}

// returns an environment value or the supplied fallback when it is absent.
// this keeps default selection in one small configuration helper.
func value(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

// parses a duration setting while retaining the configured fallback when absent.
// malformed values are returned as startup errors.
func duration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}

	return value, nil
}

// parses a signed integer setting used by size-related configuration.
// malformed values are returned instead of silently changing behavior.
func integer64(key string, fallback int64) (int64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

// parses an integer setting used by count-based configuration.
// malformed values are returned as configuration errors.
func integer(key string, fallback int) (int, error) {
	value, err := integer64(key, int64(fallback))
	return int(value), err
}
