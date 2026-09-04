package service

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"taskflow/internal/domain"
	"taskflow/internal/repository"
)

type AuthService struct {
	users  repository.UserRepository
	tokens *TokenManager
}

func NewAuthService(users repository.UserRepository, tokens *TokenManager) *AuthService {
	return &AuthService{users: users, tokens: tokens}
}

// Signup hashes the password with bcrypt before it ever touches the
// repository/DB layer — plaintext passwords should never be persisted or
// logged. bcrypt also bakes in a random salt per call, so two identical
// passwords produce different hashes.
func (s *AuthService) Signup(ctx context.Context, email, password string) (*domain.User, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{Email: email, PasswordHash: string(hash)}
	user, err = s.users.Create(ctx, user)
	if err != nil {
		return nil, "", err
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	return user, token, nil
}

// Login verifies credentials with a constant-time bcrypt comparison and, on
// success, issues a fresh JWT. Note the deliberately generic error: we never
// reveal whether it was the email or the password that was wrong, which
// prevents attackers from enumerating valid accounts.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, "", domain.ErrInvalidCredentials
		}
		return nil, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", domain.ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	return user, token, nil
}
