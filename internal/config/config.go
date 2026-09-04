// Package config centralizes all environment-driven settings so the rest of
// the app never calls os.Getenv directly. This is the "12-factor config"
// pattern: behavior changes between dev/staging/prod via environment, not code.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env          string
	ServerPort   string
	LogLevel     string
	DB           DBConfig
	JWTSecret    string
	JWTExpiresIn time.Duration

	// RefreshTokenTTL controls how long a refresh token stays valid.
	// Access tokens should be short-lived (minutes) precisely because
	// refresh tokens exist to renew them without re-authenticating.
	RefreshTokenTTL time.Duration

	AllowedOrigins []string
	RateLimitRPS   float64
	RateLimitBurst int
	RequestTimeout time.Duration
	MaxBodyBytes   int64

	// RedisAddr, if set, backs distributed rate limiting so multiple
	// replicas share one accurate counter. Empty means "no Redis" — the
	// service falls back to an in-memory, single-instance-only limiter,
	// which is fine for local dev or a single-replica deployment.
	RedisAddr string

	// TracingEndpoint is the OTLP/HTTP collector to export spans to (e.g.
	// Jaeger's OTLP receiver). Empty disables export — spans are still
	// created (so instrumentation code never has to branch on this) but
	// dropped instead of sent anywhere.
	TracingEndpoint string

	// ShutdownDrainDelay is how long the process waits, after flipping
	// /readyz to "draining" but before actually calling srv.Shutdown, to
	// give a load balancer time to notice and stop sending new traffic.
	ShutdownDrainDelay time.Duration
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// defaultJWTSecret is the value shipped in .env.example for local dev. It
// must never be used in production — Load rejects it there.
const defaultJWTSecret = "dev-secret-change-me"

// Load reads configuration from environment variables, falling back to
// sane local-dev defaults so `go run` works without a .env file, then
// validates the result. Validation failures should stop the process from
// starting rather than let it run with settings that would be unsafe
// (a well-known JWT secret in production) or nonsensical (a zero or
// negative rate limit) in production.
func Load() (Config, error) {
	cfg := Config{
		Env:        getEnv("ENV", "development"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "taskflow"),
			Password: getEnvOrFile("DB_PASSWORD", "taskflow"),
			Name:     getEnv("DB_NAME", "taskflow"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWTSecret:       getEnvOrFile("JWT_SECRET", defaultJWTSecret),
		JWTExpiresIn:    getEnvDuration("JWT_EXPIRES_IN", 15*time.Minute),
		RefreshTokenTTL: getEnvDuration("REFRESH_TOKEN_EXPIRES_IN", 720*time.Hour),

		AllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS", []string{"*"}),
		RateLimitRPS:   getEnvFloat("RATE_LIMIT_RPS", 5),
		RateLimitBurst: int(getEnvFloat("RATE_LIMIT_BURST", 10)),
		RequestTimeout: getEnvDuration("REQUEST_TIMEOUT", 15*time.Second),
		MaxBodyBytes:   int64(getEnvFloat("MAX_BODY_BYTES", 1<<20)), // 1MB

		RedisAddr:          getEnv("REDIS_ADDR", ""),
		TracingEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		ShutdownDrainDelay: getEnvDuration("SHUTDOWN_DRAIN_DELAY", 5*time.Second),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	var errs []error

	if c.ServerPort == "" {
		errs = append(errs, errors.New("SERVER_PORT must not be empty"))
	}
	if c.DB.Host == "" || c.DB.Port == "" || c.DB.User == "" || c.DB.Name == "" {
		errs = append(errs, errors.New("DB_HOST, DB_PORT, DB_USER, and DB_NAME must not be empty"))
	}
	if c.JWTSecret == "" {
		errs = append(errs, errors.New("JWT_SECRET must not be empty"))
	}
	if c.Env == "production" {
		if c.JWTSecret == defaultJWTSecret {
			errs = append(errs, errors.New("JWT_SECRET must be overridden from its default value in production"))
		}
		if len(c.JWTSecret) < 32 {
			errs = append(errs, errors.New("JWT_SECRET must be at least 32 characters in production"))
		}
	}
	if c.RateLimitRPS <= 0 {
		errs = append(errs, errors.New("RATE_LIMIT_RPS must be positive"))
	}
	if c.RateLimitBurst <= 0 {
		errs = append(errs, errors.New("RATE_LIMIT_BURST must be positive"))
	}
	if c.RequestTimeout <= 0 {
		errs = append(errs, errors.New("REQUEST_TIMEOUT must be positive"))
	}
	if c.MaxBodyBytes <= 0 {
		errs = append(errs, errors.New("MAX_BODY_BYTES must be positive"))
	}
	if c.JWTExpiresIn <= 0 {
		errs = append(errs, errors.New("JWT_EXPIRES_IN must be positive"))
	}
	if c.RefreshTokenTTL <= 0 {
		errs = append(errs, errors.New("REFRESH_TOKEN_EXPIRES_IN must be positive"))
	}
	if c.RefreshTokenTTL <= c.JWTExpiresIn {
		errs = append(errs, errors.New("REFRESH_TOKEN_EXPIRES_IN must be longer than JWT_EXPIRES_IN"))
	}
	if c.ShutdownDrainDelay < 0 {
		errs = append(errs, errors.New("SHUTDOWN_DRAIN_DELAY must not be negative"))
	}

	return errors.Join(errs...)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvOrFile reads KEY, but prefers the contents of the file named by
// KEY_FILE when that's set — the same convention the official
// postgres/mysql Docker images use, so a Docker secret or a Kubernetes
// Secret volume mount can supply this value as a file instead of a plain
// environment variable (which shows up in `docker inspect`, `/proc/.../environ`,
// and most process-listing tools).
func getEnvOrFile(key, fallback string) string {
	if path := os.Getenv(key + "_FILE"); path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return fallback
		}
		return strings.TrimSpace(string(contents))
	}
	return getEnv(key, fallback)
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

// getEnvList reads a comma-separated env var (e.g. "https://a.com,https://b.com").
func getEnvList(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
