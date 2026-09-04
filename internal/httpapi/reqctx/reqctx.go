// Package reqctx holds request-scoped context helpers (request ID, a
// pre-tagged logger) that both the middleware and response packages need.
// It exists as its own leaf package — with no imports of its own — so that
// dependency stays acyclic: middleware already imports response, so
// response can't import middleware back to read the request ID.
package reqctx

import (
	"context"
	"log/slog"
)

type requestIDKeyType struct{}
type loggerKeyType struct{}

var requestIDKey requestIDKeyType
var loggerKey loggerKeyType

// WithRequestID stores the request ID for this request in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID reads back the request ID stored by WithRequestID, or "" if none.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithLogger stores a logger (normally pre-tagged with request_id) in the context.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// Logger returns the request-scoped logger, falling back to slog.Default()
// so callers never need a nil check.
func Logger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}
