package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"taskflow/internal/domain"
)

// UserRepository is an interface, not a struct, so the service layer depends
// on a contract rather than a concrete Postgres implementation. In tests you
// can swap in an in-memory fake that satisfies this same interface.
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id int64) (*domain.User, error)
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) (*domain.User, error) {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, q, u.Email, u.PasswordHash).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `SELECT id, email, password_hash, created_at FROM users WHERE email = $1`

	var u domain.User
	err := r.db.QueryRowContext(ctx, q, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &u, nil
}

func (r *userRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	const q = `SELECT id, email, password_hash, created_at FROM users WHERE id = $1`

	var u domain.User
	err := r.db.QueryRowContext(ctx, q, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &u, nil
}
