package service

import (
	"context"
	"fmt"
	"time"

	"taskflow/internal/domain"
	"taskflow/internal/repository"
)

type TaskService struct {
	tasks repository.TaskRepository
}

func NewTaskService(tasks repository.TaskRepository) *TaskService {
	return &TaskService{tasks: tasks}
}

func (s *TaskService) Create(ctx context.Context, userID int64, title, description string, dueDate *time.Time) (*domain.Task, error) {
	task := &domain.Task{
		UserID:      userID,
		Title:       title,
		Description: description,
		Status:      domain.TaskStatusPending,
		DueDate:     dueDate,
	}
	return s.tasks.Create(ctx, task)
}

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

// List applies pagination defaults/caps here in the service layer (not the
// handler) so every caller of this method — HTTP today, a future gRPC or
// CLI entrypoint tomorrow — gets the same guardrails against a client
// asking for an unbounded number of rows.
// List returns the page of tasks along with the effective limit/offset it
// applied (after clamping), so callers can report accurate pagination
// metadata back to the client.
func (s *TaskService) List(ctx context.Context, userID int64, status domain.TaskStatus, limit, offset int) (tasks []domain.Task, total, effectiveLimit, effectiveOffset int, err error) {
	effectiveLimit = limit
	if effectiveLimit <= 0 {
		effectiveLimit = defaultPageLimit
	}
	if effectiveLimit > maxPageLimit {
		effectiveLimit = maxPageLimit
	}
	effectiveOffset = offset
	if effectiveOffset < 0 {
		effectiveOffset = 0
	}

	tasks, total, err = s.tasks.ListByUser(ctx, userID, repository.ListParams{
		Status: status,
		Limit:  effectiveLimit,
		Offset: effectiveOffset,
	})
	return tasks, total, effectiveLimit, effectiveOffset, err
}

// Get enforces ownership: fetching by ID alone isn't enough, since a
// malicious caller could pass any task ID to try to read another user's
// data. This kind of check belongs in the service layer, not the handler,
// so it can never be bypassed by a route that forgets to call it.
func (s *TaskService) Get(ctx context.Context, userID, taskID int64) (*domain.Task, error) {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.UserID != userID {
		return nil, domain.ErrForbidden
	}
	return task, nil
}

func (s *TaskService) Update(ctx context.Context, userID, taskID int64, title, description string, status domain.TaskStatus, dueDate *time.Time) (*domain.Task, error) {
	task, err := s.Get(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}

	if !status.Valid() {
		return nil, fmt.Errorf("%w: invalid status %q", domain.ErrValidation, status)
	}

	task.Title = title
	task.Description = description
	task.Status = status
	task.DueDate = dueDate

	return s.tasks.Update(ctx, task)
}

func (s *TaskService) Delete(ctx context.Context, userID, taskID int64) error {
	if _, err := s.Get(ctx, userID, taskID); err != nil {
		return err
	}
	return s.tasks.Delete(ctx, taskID)
}
