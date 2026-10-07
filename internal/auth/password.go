package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var ErrPasswordTooShort = errors.New("password must be at least 8 bytes")
var ErrPasswordTooLong = errors.New("password must not exceed 72 bytes")

// hashes a password with the configured password-hashing algorithm.
// the returned value is safe to persist instead of the plaintext password.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrPasswordTooShort
	}
	if len(password) > 72 {
		return "", ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost) // default is 10. more cost = more comp. expensive hashing
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// compares a plaintext password against a stored password hash.
// a boolean result avoids exposing comparison details to callers.
func PasswordMatches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
