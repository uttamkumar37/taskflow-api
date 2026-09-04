//go:build integration

// These tests run against a real Postgres (via testcontainers-go) using
// the exact database.Migrate function production uses — not a hand-copied
// schema — so they catch bugs the in-memory fakes used everywhere else
// structurally cannot: real constraint violations, real cascade deletes,
// real SQL syntax errors. Run with `make test-integration` (requires Docker).
package repository_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"taskflow/internal/database"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("taskflow_test"),
		postgres.WithUsername("taskflow"),
		postgres.WithPassword("taskflow"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	db, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return db
}
