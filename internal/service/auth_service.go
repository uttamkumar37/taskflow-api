package service

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"taskflow/internal/domain"
	"taskflow/internal/repository"
)

type AuthService struct {
	users         repository.UserRepository
	refreshTokens repository.RefreshTokenRepository
	tokens        *TokenManager
	refreshTTL    time.Duration
}

func NewAuthService(users repository.UserRepository, refreshTokens repository.RefreshTokenRepository, tokens *TokenManager, refreshTTL time.Duration) *AuthService {
	return &AuthService{users: users, refreshTokens: refreshTokens, tokens: tokens, refreshTTL: refreshTTL}
}

// issueTokenPair generates an access JWT and persists a fresh refresh
// token for userID, returning the access token and the refresh token's
// plaintext (the only time it's ever available — only its hash is stored).
func (s *AuthService) issueTokenPair(ctx context.Context, userID int64) (accessToken, refreshToken string, err error) {
	accessToken, err = s.tokens.Generate(userID)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	plaintext, hash, err := generateRefreshToken()
	if err != nil {
		return "", "", err
	}

	_, err = s.refreshTokens.Create(ctx, &domain.RefreshToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		return "", "", fmt.Errorf("persist refresh token: %w", err)
	}

	return accessToken, plaintext, nil
}

// Signup hashes the password with bcrypt before it ever touches the
// repository/DB layer — plaintext passwords should never be persisted or
// logged. bcrypt also bakes in a random salt per call, so two identical
// passwords produce different hashes.
func (s *AuthService) Signup(ctx context.Context, email, password string) (*domain.User, string, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{Email: email, PasswordHash: string(hash)}
	user, err = s.users.Create(ctx, user)
	if err != nil {
		return nil, "", "", err
	}

	accessToken, refreshToken, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

// Login verifies credentials with a constant-time bcrypt comparison and, on
// success, issues a fresh access+refresh token pair. Note the deliberately
// generic error: we never reveal whether it was the email or the password
// that was wrong, which prevents attackers from enumerating valid accounts.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, string, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, "", "", domain.ErrInvalidCredentials
		}
		return nil, "", "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", "", domain.ErrInvalidCredentials
	}

	accessToken, refreshToken, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

// Refresh implements OAuth2-style refresh token rotation with reuse
// detection. Each refresh token is single-use: presenting it exchanges it
// for a brand new access+refresh pair and revokes the one just used. If a
// token that's *already revoked* is presented again, that's a strong
// signal it was stolen and used by an attacker after the legitimate client
// already rotated past it — the response is to revoke every refresh token
// for that user, forcing re-authentication everywhere, rather than trust
// either party's copy going forward.
func (s *AuthService) Refresh(ctx context.Context, plaintext string) (accessToken, refreshToken string, err error) {
	existing, err := s.refreshTokens.FindByHash(ctx, hashRefreshToken(plaintext))
	if err != nil {
		if err == domain.ErrNotFound {
			return "", "", domain.ErrTokenInvalid
		}
		return "", "", err
	}

	if existing.Revoked() {
		if revokeErr := s.refreshTokens.RevokeAllForUser(ctx, existing.UserID); revokeErr != nil {
			return "", "", fmt.Errorf("revoke all refresh tokens after reuse detected: %w", revokeErr)
		}
		return "", "", domain.ErrTokenInvalid
	}
	if existing.Expired() {
		return "", "", domain.ErrTokenInvalid
	}

	if err := s.refreshTokens.Revoke(ctx, existing.ID); err != nil {
		return "", "", fmt.Errorf("revoke used refresh token: %w", err)
	}

	return s.issueTokenPair(ctx, existing.UserID)
}

// Logout revokes a single refresh token (the "current device" only — use
// Refresh's reuse-detection path, or a future admin action, to revoke
// every session for a user).
func (s *AuthService) Logout(ctx context.Context, plaintext string) error {
	existing, err := s.refreshTokens.FindByHash(ctx, hashRefreshToken(plaintext))
	if err != nil {
		if err == domain.ErrNotFound {
			return domain.ErrTokenInvalid
		}
		return err
	}
	if existing.Revoked() {
		return nil // already logged out; logout is idempotent
	}
	return s.refreshTokens.Revoke(ctx, existing.ID)
}
