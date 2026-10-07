package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/pocketradio/oslo/internal/domain"
)

var ErrInvalidToken = errors.New("invalid token")
var ErrWeakTokenSecret = errors.New("token secret must be at least 32 bytes")
var ErrInvalidTokenLifetime = errors.New("token lifetime must be positive")

type TokenPayload struct {
	Role domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret   []byte
	lifetime time.Duration
}

// validates signing configuration and prepares the token manager.
// all issued tokens use the configured secret and lifetime.
func NewTokenManager(secret string, lifetime time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, ErrWeakTokenSecret
	}
	if lifetime <= 0 {
		return nil, ErrInvalidTokenLifetime
	}

	return &TokenManager{
		secret:   []byte(secret),
		lifetime: lifetime,
	}, nil
}

// creates a signed token containing the user's identity and role.
// callers send the token back as a bearer credential on protected requests.
func (m *TokenManager) Issue(user domain.User) (string, error) {
	now := time.Now()
	payload := TokenPayload{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "oslo",
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.lifetime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	return token.SignedString(m.secret)
}

// verifies a token signature and validity window before returning its claims.
// invalid, expired, or malformed tokens are rejected as authentication failures.
func (m *TokenManager) Verify(raw string) (TokenPayload, error) {
	payload := TokenPayload{}
	token, err := jwt.ParseWithClaims(raw, &payload, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}

		return m.secret, nil
	}, jwt.WithIssuer("oslo"), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return TokenPayload{}, ErrInvalidToken
	}

	if payload.Subject == "" || (payload.Role != domain.UserRoleRider && payload.Role != domain.UserRoleDriver) {
		return TokenPayload{}, ErrInvalidToken
	}

	return payload, nil
}
