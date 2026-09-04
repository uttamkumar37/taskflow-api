// Command seed populates the database with realistic dummy data for local
// development/demoing: a handful of users, each with a set of tasks across
// every status. It goes through the same service layer cmd/api/main.go
// uses (AuthService.Signup, TaskService.Create) rather than raw SQL, so
// passwords come out properly bcrypt-hashed and every DB constraint the
// real app relies on is exercised exactly the same way.
//
// Safe to run more than once: a user that already exists is looked up and
// re-seeded with tasks (skipping ones with the same title) instead of
// failing.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"taskflow/internal/config"
	"taskflow/internal/database"
	"taskflow/internal/domain"
	"taskflow/internal/repository"
	"taskflow/internal/service"
)

type seedUser struct {
	Email    string
	Password string
}

var seedUsers = []seedUser{
	{Email: "alice@example.com", Password: "password123"},
	{Email: "bob@example.com", Password: "password123"},
	{Email: "carol@example.com", Password: "password123"},
}

type seedTask struct {
	Title       string
	Description string
	Status      domain.TaskStatus
	DueInDays   *int // nil = no due date
}

func days(n int) *int { return &n }

// taskTemplates is a shared pool; each seed user gets a rotating slice of
// it so the data looks varied without needing real randomness (keeps this
// command deterministic and its output diffable/reproducible).
var taskTemplates = []seedTask{
	{"Set up CI pipeline", "Configure GitHub Actions for lint/test/build", domain.TaskStatusDone, nil},
	{"Write API documentation", "Document all endpoints in the OpenAPI spec", domain.TaskStatusDone, nil},
	{"Add refresh token rotation", "Implement OAuth2-style rotation with reuse detection", domain.TaskStatusInProgress, days(2)},
	{"Review pull request #42", "Review the distributed tracing PR", domain.TaskStatusPending, days(1)},
	{"Fix flaky integration test", "TestTaskRepository_ListByUser occasionally times out under load", domain.TaskStatusPending, days(5)},
	{"Plan Q3 roadmap", "Draft the roadmap doc for next quarter", domain.TaskStatusPending, days(14)},
	{"Upgrade Postgres to v16", "Test compatibility and plan a migration window", domain.TaskStatusInProgress, days(7)},
	{"Onboard new team member", "Walk through architecture and local dev setup", domain.TaskStatusDone, nil},
	{"Investigate 500s on /api/v1/tasks", "Spiked briefly during last night's deploy", domain.TaskStatusDone, nil},
	{"Buy birthday gift", "Something not work-related, for once", domain.TaskStatusPending, days(3)},
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	db, err := database.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if err := database.Migrate(ctx, db); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	tokens := service.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiresIn)
	authService := service.NewAuthService(userRepo, refreshTokenRepo, tokens, cfg.RefreshTokenTTL)
	taskService := service.NewTaskService(taskRepo)

	for i, su := range seedUsers {
		user, err := getOrCreateUser(ctx, authService, userRepo, su)
		if err != nil {
			slog.Error("failed to seed user", "email", su.Email, "error", err)
			os.Exit(1)
		}

		created, skipped := seedTasksForUser(ctx, taskService, user.ID, i)
		slog.Info("seeded user",
			"email", user.Email,
			"user_id", user.ID,
			"tasks_created", created,
			"tasks_already_present", skipped,
		)
	}

	fmt.Println()
	fmt.Println("Done. Log in as any of:")
	for _, su := range seedUsers {
		fmt.Printf("  %-22s / %s\n", su.Email, su.Password)
	}
}

func getOrCreateUser(ctx context.Context, authService *service.AuthService, userRepo repository.UserRepository, su seedUser) (*domain.User, error) {
	user, _, _, err := authService.Signup(ctx, su.Email, su.Password)
	if err == nil {
		return user, nil
	}
	if errors.Is(err, domain.ErrAlreadyExists) {
		return userRepo.FindByEmail(ctx, su.Email)
	}
	return nil, err
}

// seedTasksForUser assigns a rotating window of taskTemplates to a user so
// each seeded account has a plausible, slightly different task list.
// Re-running the seed skips titles that user already has, so it's safe to
// run repeatedly against the same database.
func seedTasksForUser(ctx context.Context, taskService *service.TaskService, userID int64, offset int) (created, skipped int) {
	existing, _, _, _, err := taskService.List(ctx, userID, "", 100, 0)
	if err != nil {
		slog.Warn("could not list existing tasks, will attempt to create all", "user_id", userID, "error", err)
	}
	existingTitles := make(map[string]bool, len(existing))
	for _, t := range existing {
		existingTitles[t.Title] = true
	}

	const perUser = 5
	for i := 0; i < perUser; i++ {
		tmpl := taskTemplates[(offset+i)%len(taskTemplates)]
		if existingTitles[tmpl.Title] {
			skipped++
			continue
		}

		var dueDate *time.Time
		if tmpl.DueInDays != nil {
			d := time.Now().AddDate(0, 0, *tmpl.DueInDays)
			dueDate = &d
		}

		task, err := taskService.Create(ctx, userID, tmpl.Title, tmpl.Description, dueDate)
		if err != nil {
			slog.Warn("failed to create seed task", "title", tmpl.Title, "error", err)
			continue
		}

		// Create always starts a task as "pending" (matching real API
		// behavior); update it to the template's intended status so the
		// seeded data actually spans pending/in_progress/done.
		if tmpl.Status != domain.TaskStatusPending {
			if _, err := taskService.Update(ctx, userID, task.ID, task.Title, task.Description, tmpl.Status, task.DueDate); err != nil {
				slog.Warn("failed to set seed task status", "title", tmpl.Title, "error", err)
			}
		}

		created++
	}

	return created, skipped
}
