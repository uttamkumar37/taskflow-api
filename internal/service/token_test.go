package service

import (
	"testing"
	"time"
)

func TestTokenManager_GenerateAndValidate(t *testing.T) {
	tm := NewTokenManager("secret", time.Hour)

	token, err := tm.Generate(42)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	userID, err := tm.Validate(token)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if userID != 42 {
		t.Fatalf("expected user ID 42, got %d", userID)
	}
}

func TestTokenManager_RejectsExpiredToken(t *testing.T) {
	tm := NewTokenManager("secret", -time.Hour) // already expired

	token, err := tm.Generate(1)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if _, err := tm.Validate(token); err == nil {
		t.Fatal("expected expired token to fail validation")
	}
}

func TestTokenManager_RejectsWrongSecret(t *testing.T) {
	issuer := NewTokenManager("secret-a", time.Hour)
	verifier := NewTokenManager("secret-b", time.Hour)

	token, err := issuer.Generate(1)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if _, err := verifier.Validate(token); err == nil {
		t.Fatal("expected token signed with a different secret to fail validation")
	}
}
