//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"

	"taskflow/internal/domain"
	"taskflow/internal/repository"
)

func TestUserRepository_CreateAndFind(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	created, err := repo.Create(ctx, &domain.User{Email: "int-user@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected a non-zero ID after insert")
	}

	byEmail, err := repo.FindByEmail(ctx, "int-user@example.com")
	if err != nil {
		t.Fatalf("find by email: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Fatalf("expected FindByEmail to return the same row, got ID %d want %d", byEmail.ID, created.ID)
	}

	byID, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if byID.Email != "int-user@example.com" {
		t.Fatalf("unexpected email: %s", byID.Email)
	}
}

func TestUserRepository_CreateRejectsDuplicateEmail(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, &domain.User{Email: "dup@example.com", PasswordHash: "hash1"}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// This is the whole point of an integration test here: the unique
	// constraint (and the driver-error-to-domain-error translation in
	// isUniqueViolation) can only be exercised against a real Postgres —
	// an in-memory fake can't accidentally forget to enforce uniqueness
	// the way a hand-copied schema divergence could.
	_, err := repo.Create(ctx, &domain.User{Email: "dup@example.com", PasswordHash: "hash2"})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists for a duplicate email, got %v", err)
	}
}

func TestUserRepository_FindByEmailNotFound(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewUserRepository(db)

	_, err := repo.FindByEmail(context.Background(), "nobody@example.com")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
