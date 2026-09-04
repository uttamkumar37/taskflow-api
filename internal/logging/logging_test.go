package logging

import (
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"bogus":   slog.LevelInfo,
	}

	for input, want := range cases {
		if got := parseLevel(input); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestNew_ReturnsUsableLogger(t *testing.T) {
	if logger := New("development", "debug"); logger == nil {
		t.Fatal("expected New to return a non-nil logger")
	}
	if logger := New("production", "info"); logger == nil {
		t.Fatal("expected New to return a non-nil logger")
	}
}
