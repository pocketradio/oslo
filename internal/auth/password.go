package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var ErrPasswordTooShort = errors.New("password must be at least 8 bytes")
var ErrPasswordTooLong = errors.New("password must not exceed 72 bytes")

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

func PasswordMatches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
