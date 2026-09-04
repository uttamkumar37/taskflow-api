// Package logging builds the process-wide slog.Logger: JSON output in
// production (what log aggregators like Loki/CloudWatch/Datadog expect),
// human-readable text in development, with a configurable level so noisy
// debug output can be enabled without a code change or silenced in prod.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New builds the base logger. Callers typically chain
// .With("service", ..., "version", ...) on the result to tag every line.
func New(env, level string) *slog.Logger {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: lvl == slog.LevelDebug,
	}

	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
