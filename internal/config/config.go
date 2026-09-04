// Package config centralizes all environment-driven settings so the rest of
// the app never calls os.Getenv directly. This is the "12-factor config"
// pattern: behavior changes between dev/staging/prod via environment, not code.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env          string
	ServerPort   string
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

// Load reads configuration from environment variables, falling back to
// sane local-dev defaults so `go run` works without a .env file.
func Load() Config {
	return Config{
		Env:        getEnv("ENV", "development"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "taskflow"),
			Password: getEnv("DB_PASSWORD", "taskflow"),
			Name:     getEnv("DB_NAME", "taskflow"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWTSecret:    getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTExpiresIn: getEnvDuration("JWT_EXPIRES_IN", 24*time.Hour),

		AllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS", []string{"*"}),
		RateLimitRPS:   getEnvFloat("RATE_LIMIT_RPS", 5),
		RateLimitBurst: int(getEnvFloat("RATE_LIMIT_BURST", 10)),
		RequestTimeout: getEnvDuration("REQUEST_TIMEOUT", 15*time.Second),
		MaxBodyBytes:   int64(getEnvFloat("MAX_BODY_BYTES", 1<<20)), // 1MB
	}
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
