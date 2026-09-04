//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"

	"taskflow/internal/domain"
	"taskflow/internal/repository"
)

func TestTaskRepository_CreateFindUpdateDelete(t *testing.T) {
	db := newTestDB(t)
	users := repository.NewUserRepository(db)
	tasks := repository.NewTaskRepository(db)
	ctx := context.Background()

	user, err := users.Create(ctx, &domain.User{Email: "task-owner@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	created, err := tasks.Create(ctx, &domain.Task{
		UserID:      user.ID,
		Title:       "Write integration tests",
		Description: "against a real Postgres",
		Status:      domain.TaskStatusPending,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected a non-zero task ID")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("expected CreatedAt/UpdatedAt to be populated by the DB default")
	}

	found, err := tasks.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.Title != "Write integration tests" {
		t.Fatalf("unexpected title: %s", found.Title)
	}

	found.Status = domain.TaskStatusDone
	updated, err := tasks.Update(ctx, found)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Fatal("expected updated_at to advance after an update")
	}

	if err := tasks.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := tasks.FindByID(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTaskRepository_ListByUser_PaginationAndStatusFilter(t *testing.T) {
	db := newTestDB(t)
	users := repository.NewUserRepository(db)
	tasks := repository.NewTaskRepository(db)
	ctx := context.Background()

	user, err := users.Create(ctx, &domain.User{Email: "pagination@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	statuses := []domain.TaskStatus{
		domain.TaskStatusPending, domain.TaskStatusPending, domain.TaskStatusPending,
		domain.TaskStatusDone, domain.TaskStatusDone,
	}
	for i, status := range statuses {
		if _, err := tasks.Create(ctx, &domain.Task{
			UserID: user.ID,
			Title:  "task",
			Status: status,
		}); err != nil {
			t.Fatalf("create task %d: %v", i, err)
		}
	}

	all, total, err := tasks.ListByUser(ctx, user.ID, repository.ListParams{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 5 || len(all) != 5 {
		t.Fatalf("expected 5 total tasks, got total=%d len=%d", total, len(all))
	}

	done, total, err := tasks.ListByUser(ctx, user.ID, repository.ListParams{Status: domain.TaskStatusDone, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("list done: %v", err)
	}
	if total != 2 || len(done) != 2 {
		t.Fatalf("expected 2 done tasks, got total=%d len=%d", total, len(done))
	}

	page, total, err := tasks.ListByUser(ctx, user.ID, repository.ListParams{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total to reflect the full set (5) regardless of paging, got %d", total)
	}
	if len(page) != 2 {
		t.Fatalf("expected a page of 2, got %d", len(page))
	}
}

func TestTaskRepository_DeletedWhenOwningUserIsDeleted(t *testing.T) {
	db := newTestDB(t)
	users := repository.NewUserRepository(db)
	tasks := repository.NewTaskRepository(db)
	ctx := context.Background()

	user, err := users.Create(ctx, &domain.User{Email: "cascade@example.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	task, err := tasks.Create(ctx, &domain.Task{UserID: user.ID, Title: "orphan-to-be", Status: domain.TaskStatusPending})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// This is exactly the kind of behavior an in-memory fake repository
	// can't verify: `ON DELETE CASCADE` is a property of the real schema,
	// asserted here by deleting the user directly at the SQL level and
	// confirming the task disappears with it.
	if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if _, err := tasks.FindByID(ctx, task.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected the task to be cascade-deleted with its owner, got %v", err)
	}
}
