package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"taskflow/internal/domain"
)

func newTestAuthService() (*AuthService, *fakeRefreshTokenRepository) {
	userRepo := newFakeUserRepository()
	refreshRepo := newFakeRefreshTokenRepository()
	tokens := NewTokenManager("test-secret", time.Hour)
	return NewAuthService(userRepo, refreshRepo, tokens, 24*time.Hour), refreshRepo
}

func TestAuthService_SignupAndLogin(t *testing.T) {
	auth, _ := newTestAuthService()
	ctx := context.Background()

	user, accessToken, refreshToken, err := auth.Signup(ctx, "alice@example.com", "hunter22")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}
	if accessToken == "" || refreshToken == "" {
		t.Fatal("expected non-empty access and refresh tokens from signup")
	}
	if user.PasswordHash == "hunter22" {
		t.Fatal("password was stored in plaintext, expected a bcrypt hash")
	}

	_, loginAccess, loginRefresh, err := auth.Login(ctx, "alice@example.com", "hunter22")
	if err != nil {
		t.Fatalf("login with correct password failed: %v", err)
	}
	if loginAccess == "" || loginRefresh == "" {
		t.Fatal("expected non-empty access and refresh tokens from login")
	}
	if loginRefresh == refreshToken {
		t.Fatal("expected login to issue a distinct refresh token from signup")
	}
}

func TestAuthService_SignupDuplicateEmail(t *testing.T) {
	auth, _ := newTestAuthService()
	ctx := context.Background()

	if _, _, _, err := auth.Signup(ctx, "bob@example.com", "password1"); err != nil {
		t.Fatalf("first signup failed: %v", err)
	}

	_, _, _, err := auth.Signup(ctx, "bob@example.com", "password2")
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestAuthService_LoginWrongPassword(t *testing.T) {
	auth, _ := newTestAuthService()
	ctx := context.Background()

	if _, _, _, err := auth.Signup(ctx, "carol@example.com", "correct-password"); err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	_, _, _, err := auth.Login(ctx, "carol@example.com", "wrong-password")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_LoginUnknownEmail(t *testing.T) {
	auth, _ := newTestAuthService()

	_, _, _, err := auth.Login(context.Background(), "nobody@example.com", "whatever1")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown email (not ErrNotFound, to avoid leaking account existence), got %v", err)
	}
}

func TestAuthService_RefreshRotatesToken(t *testing.T) {
	auth, _ := newTestAuthService()
	ctx := context.Background()

	_, _, refreshToken, err := auth.Signup(ctx, "dave@example.com", "password123")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	newAccess, newRefresh, err := auth.Refresh(ctx, refreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if newAccess == "" || newRefresh == "" {
		t.Fatal("expected non-empty access and refresh tokens from refresh")
	}
	if newRefresh == refreshToken {
		t.Fatal("expected refresh to rotate to a new refresh token, got the same one back")
	}

	// the new refresh token should itself work for a further rotation
	if _, _, err := auth.Refresh(ctx, newRefresh); err != nil {
		t.Fatalf("expected the newly-issued refresh token to work, got: %v", err)
	}
}

func TestAuthService_RefreshRejectsUnknownToken(t *testing.T) {
	auth, _ := newTestAuthService()

	_, _, err := auth.Refresh(context.Background(), "this-token-was-never-issued")
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestAuthService_RefreshRejectsExpiredToken(t *testing.T) {
	auth, refreshRepo := newTestAuthService()
	ctx := context.Background()

	_, _, refreshToken, err := auth.Signup(ctx, "erin@example.com", "password123")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	// simulate time passing past the token's expiry
	for _, stored := range refreshRepo.byHash {
		stored.ExpiresAt = time.Now().Add(-time.Minute)
	}

	if _, _, err := auth.Refresh(ctx, refreshToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid for an expired token, got %v", err)
	}
}

func TestAuthService_RefreshReuseDetectionRevokesAllSessions(t *testing.T) {
	auth, _ := newTestAuthService()
	ctx := context.Background()

	_, _, original, err := auth.Signup(ctx, "frank@example.com", "password123")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	// legitimate rotation: original -> rotated
	_, rotated, err := auth.Refresh(ctx, original)
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}

	// an attacker (or a retried request) replays the now-revoked original
	// token — this must be treated as a compromise signal.
	if _, _, err := auth.Refresh(ctx, original); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("expected reusing a revoked token to fail with ErrTokenInvalid, got %v", err)
	}

	// the legitimate client's own rotated token must now be dead too —
	// reuse detection revokes the whole session, not just the replayed token.
	if _, _, err := auth.Refresh(ctx, rotated); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("expected reuse detection to revoke the rotated token as well, got %v", err)
	}
}

func TestAuthService_LogoutRevokesToken(t *testing.T) {
	auth, _ := newTestAuthService()
	ctx := context.Background()

	_, _, refreshToken, err := auth.Signup(ctx, "grace@example.com", "password123")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	if err := auth.Logout(ctx, refreshToken); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	if _, _, err := auth.Refresh(ctx, refreshToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("expected a logged-out refresh token to be rejected, got %v", err)
	}
}

func TestAuthService_LogoutIsIdempotent(t *testing.T) {
	auth, _ := newTestAuthService()
	ctx := context.Background()

	_, _, refreshToken, err := auth.Signup(ctx, "heidi@example.com", "password123")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	if err := auth.Logout(ctx, refreshToken); err != nil {
		t.Fatalf("first logout failed: %v", err)
	}
	if err := auth.Logout(ctx, refreshToken); err != nil {
		t.Fatalf("second logout on an already-revoked token should be a no-op, got: %v", err)
	}
}
