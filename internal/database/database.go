// Package database owns the Postgres connection lifecycle: connecting with
// retry (containers rarely start in dependency order, even with
// docker-compose `depends_on`), and applying schema migrations on boot.
package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Connect opens a pooled connection to Postgres, retrying for a bounded
// window since the DB container may still be starting up.
func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	const (
		maxAttempts = 10
		delay       = 2 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = db.PingContext(pingCtx)
		cancel()

		if lastErr == nil {
			return db, nil
		}

		slog.Warn("database not ready, retrying", "attempt", attempt, "max_attempts", maxAttempts, "error", lastErr)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("could not connect to database after %d attempts: %w", maxAttempts, lastErr)
}

// Migrate applies every .sql file under migrations/ in filename order.
// Each file is expected to be idempotent (CREATE TABLE IF NOT EXISTS, etc.)
// which keeps this simple runner safe to execute on every startup.
func Migrate(ctx context.Context, db *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		contents, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := db.ExecContext(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		slog.Info("applied migration", "file", name)
	}

	return nil
}
