package domain

import "time"

type UserRole string

const (
	UserRoleRider  UserRole = "rider"
	UserRoleDriver UserRole = "driver"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         UserRole
	CreatedAt    time.Time
}
