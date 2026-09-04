package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"taskflow/internal/domain"
)

func newTestAuthService() *AuthService {
	repo := newFakeUserRepository()
	tokens := NewTokenManager("test-secret", time.Hour)
	return NewAuthService(repo, tokens)
}

func TestAuthService_SignupAndLogin(t *testing.T) {
	auth := newTestAuthService()
	ctx := context.Background()

	user, token, err := auth.Signup(ctx, "alice@example.com", "hunter22")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token from signup")
	}
	if user.PasswordHash == "hunter22" {
		t.Fatal("password was stored in plaintext, expected a bcrypt hash")
	}

	_, loginToken, err := auth.Login(ctx, "alice@example.com", "hunter22")
	if err != nil {
		t.Fatalf("login with correct password failed: %v", err)
	}
	if loginToken == "" {
		t.Fatal("expected non-empty token from login")
	}
}

func TestAuthService_SignupDuplicateEmail(t *testing.T) {
	auth := newTestAuthService()
	ctx := context.Background()

	if _, _, err := auth.Signup(ctx, "bob@example.com", "password1"); err != nil {
		t.Fatalf("first signup failed: %v", err)
	}

	_, _, err := auth.Signup(ctx, "bob@example.com", "password2")
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestAuthService_LoginWrongPassword(t *testing.T) {
	auth := newTestAuthService()
	ctx := context.Background()

	if _, _, err := auth.Signup(ctx, "carol@example.com", "correct-password"); err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	_, _, err := auth.Login(ctx, "carol@example.com", "wrong-password")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_LoginUnknownEmail(t *testing.T) {
	auth := newTestAuthService()

	_, _, err := auth.Login(context.Background(), "nobody@example.com", "whatever1")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown email (not ErrNotFound, to avoid leaking account existence), got %v", err)
	}
}
