// Package httpapi wires the HTTP layer: routes, middleware chains, and
// handler construction. It's the "composition root" for everything web-facing.
package httpapi

import (
	"database/sql"
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"taskflow/internal/config"
	"taskflow/internal/httpapi/handler"
	"taskflow/internal/httpapi/middleware"
	"taskflow/internal/service"
)

type Handlers struct {
	Auth *service.AuthService
	Task *service.TaskService
}

// Deps bundles everything NewRouter needs. A struct (rather than a long
// positional parameter list) is used deliberately: this constructor's
// dependency count only grows as the service gains cross-cutting concerns
// (a cache, a draining flag, ...) — positional params become error-prone
// and hard to read past four or five arguments.
type Deps struct {
	Handlers     Handlers
	Tokens       *service.TokenManager
	DB           *sql.DB
	Config       config.Config
	BuildVersion string
	// RedisClient backs distributed rate limiting when non-nil; nil falls
	// back to an in-memory, single-instance-only limiter.
	RedisClient *redis.Client
	// Draining flips to true while the server is shutting down, so
	// /readyz can fail fast — without even touching the DB — the moment
	// shutdown begins, before in-flight requests finish draining.
	Draining *atomic.Bool
}

// NewRouter builds the full route table. Go 1.22+'s net/http ServeMux
// supports method-specific patterns ("POST /api/v1/tasks") and path
// parameters ("{id}") natively, so no third-party router is needed for a
// service this size.
//
// Routes are split into two tiers with different middleware:
//   - Operational endpoints (/healthz, /readyz, /metrics) are for
//     orchestrators and monitoring, not API clients — no CORS, rate
//     limiting, or request-size limits.
//   - The versioned API (/api/v1/...) gets the full protective stack.
//
// Both tiers share request ID tagging, access logging, panic recovery, and
// metrics, applied once at the outermost layer so every response is
// accounted for exactly once. otelhttp wraps everything one layer further
// out, so a trace span already exists by the time RequestID builds its
// request-scoped logger and can tag it with the trace ID.
func NewRouter(d Deps) http.Handler {
	root := http.NewServeMux()

	root.Handle("GET /healthz", handler.Health(d.BuildVersion))
	root.Handle("GET /readyz", handler.Ready(d.DB, d.BuildVersion, d.Draining))
	root.Handle("GET /metrics", promhttp.Handler())

	authHandler := handler.NewAuthHandler(d.Handlers.Auth)
	taskHandler := handler.NewTaskHandler(d.Handlers.Task)

	api := http.NewServeMux()
	api.HandleFunc("POST /api/v1/auth/signup", authHandler.Signup)
	api.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	// Refresh and logout authenticate via the refresh token in the request
	// body, not a bearer access token, so they sit at the same middleware
	// tier as signup/login rather than behind middleware.Auth.
	api.HandleFunc("POST /api/v1/auth/refresh", authHandler.Refresh)
	api.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)

	// Protected routes get their own sub-mux wrapped in the Auth middleware,
	// so the requirement "must be logged in" is enforced in one place rather
	// than repeated in every handler.
	protected := http.NewServeMux()
	protected.HandleFunc("POST /api/v1/tasks", taskHandler.Create)
	protected.HandleFunc("GET /api/v1/tasks", taskHandler.List)
	protected.HandleFunc("GET /api/v1/tasks/{id}", taskHandler.Get)
	protected.HandleFunc("PUT /api/v1/tasks/{id}", taskHandler.Update)
	protected.HandleFunc("DELETE /api/v1/tasks/{id}", taskHandler.Delete)

	api.Handle("/api/v1/tasks", middleware.Auth(d.Tokens)(protected))
	api.Handle("/api/v1/tasks/", middleware.Auth(d.Tokens)(protected))

	rateLimiter := middleware.NewRateLimiter(d.Config.RateLimitRPS, d.Config.RateLimitBurst, d.RedisClient)

	apiHandler := middleware.Chain(
		middleware.CORS(d.Config.AllowedOrigins),
		middleware.MaxBodyBytes(d.Config.MaxBodyBytes),
		rateLimiter.Middleware,
		middleware.Timeout(d.Config.RequestTimeout),
	)(api)

	root.Handle("/api/v1/", apiHandler)

	handlerChain := middleware.Chain(
		middleware.RequestID,
		middleware.Logging,
		middleware.Recover,
		middleware.Metrics,
		middleware.SecurityHeaders,
	)(root)

	return otelhttp.NewHandler(handlerChain, "taskflow-api")
}
