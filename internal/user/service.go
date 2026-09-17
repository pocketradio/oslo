package user

import (
	"context"
	"net/mail"
	"strings"

	"github.com/google/uuid"

	"github.com/pocketradio/oslo/internal/auth"
	"github.com/pocketradio/oslo/internal/domain"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) Register(ctx context.Context, email, password string, role domain.UserRole) (domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return domain.User{}, domain.ErrInvalidEmail
	}

	if role != domain.UserRoleRider && role != domain.UserRoleDriver {
		return domain.User{}, domain.ErrInvalidUserRole
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return domain.User{}, err
	}

	user := domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
	}
	if err := s.store.Create(ctx, &user); err != nil {
		return domain.User{}, err
	}

	return user, nil
}
