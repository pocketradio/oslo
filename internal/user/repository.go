package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pocketradio/oslo/internal/domain"
)

type Repository struct {
	database *pgxpool.Pool
}

func NewRepository(database *pgxpool.Pool) *Repository {
	return &Repository{database: database}
}

func (r *Repository) Create(ctx context.Context, user *domain.User) error {
	err := r.database.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`, user.ID, user.Email, user.PasswordHash, user.Role).Scan(&user.CreatedAt)
	if err == nil {
		return nil
	}

	// the query returns only created_at. pgx scan copies columns returned by pg into go vars 

	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.ConstraintName == "users_email_unique" {
		return domain.ErrEmailTaken
	}

	return fmt.Errorf("create user: %w", err)
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User

	// postgres stores ID as uuid, struct uses string. id::text convts it
	err := r.database.QueryRow(ctx, `
		SELECT id::text, email, password_hash, role, created_at
		FROM users
		WHERE lower(email) = lower($1)
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}
