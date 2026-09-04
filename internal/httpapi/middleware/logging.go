package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"taskflow/internal/httpapi/reqctx"
)

// statusRecorder wraps http.ResponseWriter to capture the status code and
// response size, since the standard interface has no way to read either
// back from a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Logging logs one line per request: method, path, status, latency, and
// response size. This is the "access log" every HTTP service needs for
// observability. The line is leveled by outcome — a 5xx is a server-side
// problem worth an Error, a 4xx is a client mistake worth a Warn, and
// everything else is routine Info — so log-based alerting can filter on
// level alone instead of parsing status codes out of every line.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		logger := reqctx.Logger(r.Context())
		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		}

		logger.Log(context.Background(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes_written", rec.bytes,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}
