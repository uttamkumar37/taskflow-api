package dto

import (
	"fmt"
	"time"

	"taskflow/internal/domain"
)

type TaskRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	DueDate     *time.Time `json:"due_date"`
}

// Validate performs field-level checks before the request ever reaches the
// service layer. Keeping this on the DTO (not the domain model) means HTTP
// input rules stay separate from core business rules.
func (r TaskRequest) Validate() error {
	if r.Title == "" {
		return fmt.Errorf("%w: title is required", domain.ErrValidation)
	}
	if r.Status != "" && !domain.TaskStatus(r.Status).Valid() {
		return fmt.Errorf("%w: status must be one of pending, in_progress, done", domain.ErrValidation)
	}
	return nil
}

type TaskResponse struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func NewTaskResponse(t *domain.Task) TaskResponse {
	return TaskResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Status:      string(t.Status),
		DueDate:     t.DueDate,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

type TaskListResponse struct {
	Data   []TaskResponse `json:"data"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func NewTaskListResponse(tasks []domain.Task, total, limit, offset int) TaskListResponse {
	out := make([]TaskResponse, 0, len(tasks))
	for i := range tasks {
		out = append(out, NewTaskResponse(&tasks[i]))
	}
	return TaskListResponse{Data: out, Total: total, Limit: limit, Offset: offset}
}
