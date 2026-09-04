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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"taskflow/internal/config"
	"taskflow/internal/database"
	"taskflow/internal/httpapi"
	"taskflow/internal/logging"
	"taskflow/internal/repository"
	"taskflow/internal/service"
	"taskflow/internal/tracing"
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

	shutdownTracing, err := tracing.Init(ctx, "taskflow-api", version.Version, cfg.TracingEndpoint)
	if err != nil {
		slog.Error("failed to initialize tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdownCtx); err != nil {
			slog.Error("failed to flush trace exporter", "error", err)
		}
	}()

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

	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
		defer func() { _ = redisClient.Close() }()
	}

	// Dependency injection by hand: each layer only knows about the layer
	// directly below it (handlers -> services -> repositories -> db), wired
	// together once here at startup rather than via a DI framework.
	userRepo := repository.NewUserRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)

	tokens := service.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiresIn)
	authService := service.NewAuthService(userRepo, refreshTokenRepo, tokens, cfg.RefreshTokenTTL)
	taskService := service.NewTaskService(taskRepo)

	var draining atomic.Bool

	router := httpapi.NewRouter(httpapi.Deps{
		Handlers:     httpapi.Handlers{Auth: authService, Task: taskService},
		Tokens:       tokens,
		DB:           db,
		Config:       cfg,
		BuildVersion: version.Version,
		RedisClient:  redisClient,
		Draining:     &draining,
	})

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

	// Flip readiness to "draining" immediately, then wait a beat before
	// actually stopping — a load balancer or k8s Service needs to notice
	// via /readyz (or the pod's own deregistration) and stop sending new
	// traffic here before connections start getting cut.
	draining.Store(true)
	if cfg.ShutdownDrainDelay > 0 {
		time.Sleep(cfg.ShutdownDrainDelay)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped cleanly")
}
