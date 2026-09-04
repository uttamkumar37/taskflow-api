package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"taskflow/internal/httpapi/reqctx"
)

const requestIDHeader = "X-Request-ID"

// RequestID assigns every request a unique ID (or trusts an upstream
// load balancer / gateway's ID if one is already present), echoes it back
// in the response header, and stashes both the ID and a logger pre-tagged
// with it in the context — so every log line for this request, at any
// layer, can be correlated without threading request_id through every
// function signature by hand.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = generateRequestID()
		}

		w.Header().Set(requestIDHeader, id)

		ctx := reqctx.WithRequestID(r.Context(), id)

		logger := slog.Default().With("request_id", id)
		// otelhttp wraps the whole handler chain one layer further out, so
		// a span already exists here — tagging every log line with its
		// trace ID is what makes "grep this log line, then open the
		// matching trace in Jaeger" possible.
		if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.HasTraceID() {
			logger = logger.With("trace_id", spanCtx.TraceID().String())
		}
		ctx = reqctx.WithLogger(ctx, logger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext reads the request ID set by RequestID.
func RequestIDFromContext(ctx context.Context) string {
	return reqctx.RequestID(ctx)
}

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
