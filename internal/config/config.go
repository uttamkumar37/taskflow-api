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

	AllowedOrigins []string
	RateLimitRPS   float64
	RateLimitBurst int
	RequestTimeout time.Duration
	MaxBodyBytes   int64
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
			Password: getEnv("DB_PASSWORD", "taskflow"),
			Name:     getEnv("DB_NAME", "taskflow"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWTSecret:    getEnv("JWT_SECRET", defaultJWTSecret),
		JWTExpiresIn: getEnvDuration("JWT_EXPIRES_IN", 24*time.Hour),

		AllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS", []string{"*"}),
		RateLimitRPS:   getEnvFloat("RATE_LIMIT_RPS", 5),
		RateLimitBurst: int(getEnvFloat("RATE_LIMIT_BURST", 10)),
		RequestTimeout: getEnvDuration("REQUEST_TIMEOUT", 15*time.Second),
		MaxBodyBytes:   int64(getEnvFloat("MAX_BODY_BYTES", 1<<20)), // 1MB
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

	return errors.Join(errs...)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
