package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"taskflow/internal/domain"
)

type ListParams struct {
	Status domain.TaskStatus
	Limit  int
	Offset int
}

type TaskRepository interface {
	Create(ctx context.Context, t *domain.Task) (*domain.Task, error)
	FindByID(ctx context.Context, id int64) (*domain.Task, error)
	// ListByUser returns a page of tasks plus the total count matching the
	// filter (ignoring Limit/Offset), so callers can compute total pages
	// without a second round trip.
	ListByUser(ctx context.Context, userID int64, params ListParams) ([]domain.Task, int, error)
	Update(ctx context.Context, t *domain.Task) (*domain.Task, error)
	Delete(ctx context.Context, id int64) error
}

type taskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) Create(ctx context.Context, t *domain.Task) (*domain.Task, error) {
	const q = `
		INSERT INTO tasks (user_id, title, description, status, due_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, q, t.UserID, t.Title, t.Description, t.Status, t.DueDate).
		Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	return t, nil
}

func (r *taskRepository) FindByID(ctx context.Context, id int64) (*domain.Task, error) {
	const q = `
		SELECT id, user_id, title, description, status, due_date, created_at, updated_at
		FROM tasks WHERE id = $1`

	var t domain.Task
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find task by id: %w", err)
	}
	return &t, nil
}

func (r *taskRepository) ListByUser(ctx context.Context, userID int64, params ListParams) ([]domain.Task, int, error) {
	where := `WHERE user_id = $1`
	args := []any{userID}

	if params.Status != "" {
		where += ` AND status = $2`
		args = append(args, params.Status)
	}

	var total int
	countQ := `SELECT count(*) FROM tasks ` + where
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	listArgs := append(append([]any{}, args...), params.Limit, params.Offset)
	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	listQ := fmt.Sprintf(`
		SELECT id, user_id, title, description, status, due_date, created_at, updated_at
		FROM tasks %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, limitPos, offsetPos)

	rows, err := r.db.QueryContext(ctx, listQ, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tasks []domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan task row: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate task rows: %w", err)
	}

	return tasks, total, nil
}

func (r *taskRepository) Update(ctx context.Context, t *domain.Task) (*domain.Task, error) {
	const q = `
		UPDATE tasks
		SET title = $1, description = $2, status = $3, due_date = $4, updated_at = now()
		WHERE id = $5
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, q, t.Title, t.Description, t.Status, t.DueDate, t.ID).Scan(&t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	return t, nil
}

func (r *taskRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete task rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
