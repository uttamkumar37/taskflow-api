package config

import "testing"

func TestLoad_RejectsDefaultSecretInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to reject the default JWT secret in production")
	}
}

func TestLoad_RejectsShortSecretInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "too-short")

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to reject a short JWT secret in production")
	}
}

func TestLoad_AcceptsStrongSecretInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "this-is-a-sufficiently-long-production-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected Load to succeed with a strong secret, got: %v", err)
	}
	if cfg.Env != "production" {
		t.Fatalf("expected Env to be production, got %q", cfg.Env)
	}
}

func TestLoad_AcceptsDefaultsInDevelopment(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("JWT_SECRET", "")

	if _, err := Load(); err != nil {
		t.Fatalf("expected Load to succeed in development with default secret, got: %v", err)
	}
}

func TestLoad_RejectsNonPositiveRateLimit(t *testing.T) {
	t.Setenv("RATE_LIMIT_RPS", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to reject a non-positive RATE_LIMIT_RPS")
	}
}
