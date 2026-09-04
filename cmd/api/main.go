// Command api is the entrypoint: it loads config, connects to the database,
// wires up repositories -> services -> handlers, and starts the HTTP server
// with graceful shutdown on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"taskflow/internal/config"
	"taskflow/internal/database"
	"taskflow/internal/httpapi"
	"taskflow/internal/logging"
	"taskflow/internal/repository"
	"taskflow/internal/service"
	"taskflow/internal/version"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// The real logger isn't built yet (it depends on cfg), so fall back
		// to a minimal one just for this fatal message.
		slog.New(slog.NewTextHandler(os.Stderr, nil)).Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.Env, cfg.LogLevel).With(
		"service", "taskflow-api",
		"env", cfg.Env,
		"version", version.Version,
	)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.Migrate(ctx, db); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Dependency injection by hand: each layer only knows about the layer
	// directly below it (handlers -> services -> repositories -> db), wired
	// together once here at startup rather than via a DI framework.
	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	tokens := service.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiresIn)
	authService := service.NewAuthService(userRepo, tokens)
	taskService := service.NewTaskService(taskRepo)

	router := httpapi.NewRouter(httpapi.Handlers{
		Auth: authService,
		Task: taskService,
	}, tokens, db, cfg, version.Version)

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second, // mitigates slow-header (slowloris) attacks
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Block until we receive SIGINT/SIGTERM, then give in-flight requests a
	// bounded window to finish instead of dropping them mid-response — this
	// is what makes container restarts/deploys not lose requests.
	<-ctx.Done()
	slog.Info("shutdown signal received, draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped cleanly")
}
