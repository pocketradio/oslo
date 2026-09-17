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
