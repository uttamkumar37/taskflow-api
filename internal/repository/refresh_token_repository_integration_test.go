//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"taskflow/internal/domain"
	"taskflow/internal/repository"
)

func TestRefreshTokenRepository_CreateFindRevoke(t *testing.T) {
	db := newTestDB(t)
	users := repository.NewUserRepository(db)
	refreshTokens := repository.NewRefreshTokenRepository(db)
	ctx := context.Background()

	user, err := users.Create(ctx, &domain.User{Email: "refresh-int@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	created, err := refreshTokens.Create(ctx, &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: "int-test-hash-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected a non-zero ID")
	}

	found, err := refreshTokens.FindByHash(ctx, "int-test-hash-1")
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if found.Revoked() {
		t.Fatal("a freshly created token should not be revoked")
	}

	if err := refreshTokens.Revoke(ctx, found.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	afterRevoke, err := refreshTokens.FindByHash(ctx, "int-test-hash-1")
	if err != nil {
		t.Fatalf("find after revoke: %v", err)
	}
	if !afterRevoke.Revoked() {
		t.Fatal("expected the token to be revoked")
	}
}

func TestRefreshTokenRepository_RevokeAllForUser(t *testing.T) {
	db := newTestDB(t)
	users := repository.NewUserRepository(db)
	refreshTokens := repository.NewRefreshTokenRepository(db)
	ctx := context.Background()

	user, err := users.Create(ctx, &domain.User{Email: "revoke-all@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := refreshTokens.Create(ctx, &domain.RefreshToken{
			UserID:    user.ID,
			TokenHash: "hash-" + string(rune('a'+i)),
			ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("create token %d: %v", i, err)
		}
	}

	if err := refreshTokens.RevokeAllForUser(ctx, user.ID); err != nil {
		t.Fatalf("revoke all: %v", err)
	}

	for i := 0; i < 3; i++ {
		hash := "hash-" + string(rune('a'+i))
		found, err := refreshTokens.FindByHash(ctx, hash)
		if err != nil {
			t.Fatalf("find %s: %v", hash, err)
		}
		if !found.Revoked() {
			t.Fatalf("expected token %s to be revoked by RevokeAllForUser", hash)
		}
	}
}

func TestRefreshTokenRepository_FindByHashNotFound(t *testing.T) {
	db := newTestDB(t)
	refreshTokens := repository.NewRefreshTokenRepository(db)

	_, err := refreshTokens.FindByHash(context.Background(), "does-not-exist")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// CascadeDeleteRemovesRefreshTokens confirms refresh_tokens.user_id's
// ON DELETE CASCADE, the same schema property that matters for the
// "revoke everything and force re-login" story if a user is ever deleted.
func TestRefreshTokenRepository_CascadeDeleteRemovesRefreshTokens(t *testing.T) {
	db := newTestDB(t)
	users := repository.NewUserRepository(db)
	refreshTokens := repository.NewRefreshTokenRepository(db)
	ctx := context.Background()

	user, err := users.Create(ctx, &domain.User{Email: "cascade-refresh@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := refreshTokens.Create(ctx, &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: "cascade-hash",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if _, err := refreshTokens.FindByHash(ctx, "cascade-hash"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected the refresh token to be cascade-deleted with its owner, got %v", err)
	}
}
