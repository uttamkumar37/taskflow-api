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
	"taskflow/internal/repository"
	"taskflow/internal/service"
)

func main() {
	cfg := config.Load()

	// JSON logs in production are what log aggregators (Loki, CloudWatch,
	// Datadog) expect; a human-readable text handler is friendlier for local
	// development, where a person is reading the terminal directly.
	if cfg.Env == "production" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	}

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
	}, tokens, db, cfg)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
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
