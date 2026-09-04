package service

import (
	"context"
	"errors"
	"testing"

	"taskflow/internal/domain"
)

func newTestTaskService() *TaskService {
	return NewTaskService(newFakeTaskRepository())
}

func TestTaskService_CreateAndGet(t *testing.T) {
	svc := newTestTaskService()
	ctx := context.Background()

	created, err := svc.Create(ctx, 1, "Write tests", "cover the service layer", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.Status != domain.TaskStatusPending {
		t.Fatalf("expected new task to default to pending, got %s", created.Status)
	}

	got, err := svc.Get(ctx, 1, created.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Title != "Write tests" {
		t.Fatalf("expected title %q, got %q", "Write tests", got.Title)
	}
}

func TestTaskService_GetEnforcesOwnership(t *testing.T) {
	svc := newTestTaskService()
	ctx := context.Background()

	task, err := svc.Create(ctx, 1, "Alice's task", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// User 2 tries to read user 1's task by guessing/incrementing the ID.
	_, err = svc.Get(ctx, 2, task.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden when accessing another user's task, got %v", err)
	}
}

func TestTaskService_UpdateRejectsInvalidStatus(t *testing.T) {
	svc := newTestTaskService()
	ctx := context.Background()

	task, err := svc.Create(ctx, 1, "Task", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = svc.Update(ctx, 1, task.ID, "Task", "", domain.TaskStatus("not_a_real_status"), nil)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation for invalid status, got %v", err)
	}
}

func TestTaskService_DeleteEnforcesOwnership(t *testing.T) {
	svc := newTestTaskService()
	ctx := context.Background()

	task, err := svc.Create(ctx, 1, "Task", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := svc.Delete(ctx, 2, task.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden when deleting another user's task, got %v", err)
	}

	// Owner can still delete it afterwards.
	if err := svc.Delete(ctx, 1, task.ID); err != nil {
		t.Fatalf("owner delete failed: %v", err)
	}
}
