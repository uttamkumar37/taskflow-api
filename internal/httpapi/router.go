// Package httpapi wires the HTTP layer: routes, middleware chains, and
// handler construction. It's the "composition root" for everything web-facing.
package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"taskflow/internal/config"
	"taskflow/internal/httpapi/handler"
	"taskflow/internal/httpapi/middleware"
	"taskflow/internal/service"
)

type Handlers struct {
	Auth *service.AuthService
	Task *service.TaskService
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
// accounted for exactly once.
func NewRouter(h Handlers, tokens *service.TokenManager, db *sql.DB, cfg config.Config) http.Handler {
	root := http.NewServeMux()

	root.HandleFunc("GET /healthz", handler.Health)
	root.Handle("GET /readyz", handler.Ready(db))
	root.Handle("GET /metrics", promhttp.Handler())

	authHandler := handler.NewAuthHandler(h.Auth)
	taskHandler := handler.NewTaskHandler(h.Task)

	api := http.NewServeMux()
	api.HandleFunc("POST /api/v1/auth/signup", authHandler.Signup)
	api.HandleFunc("POST /api/v1/auth/login", authHandler.Login)

	// Protected routes get their own sub-mux wrapped in the Auth middleware,
	// so the requirement "must be logged in" is enforced in one place rather
	// than repeated in every handler.
	protected := http.NewServeMux()
	protected.HandleFunc("POST /api/v1/tasks", taskHandler.Create)
	protected.HandleFunc("GET /api/v1/tasks", taskHandler.List)
	protected.HandleFunc("GET /api/v1/tasks/{id}", taskHandler.Get)
	protected.HandleFunc("PUT /api/v1/tasks/{id}", taskHandler.Update)
	protected.HandleFunc("DELETE /api/v1/tasks/{id}", taskHandler.Delete)

	api.Handle("/api/v1/tasks", middleware.Auth(tokens)(protected))
	api.Handle("/api/v1/tasks/", middleware.Auth(tokens)(protected))

	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	apiHandler := middleware.Chain(
		middleware.CORS(cfg.AllowedOrigins),
		middleware.MaxBodyBytes(cfg.MaxBodyBytes),
		rateLimiter.Middleware,
		middleware.Timeout(cfg.RequestTimeout),
	)(api)

	root.Handle("/api/v1/", apiHandler)

	return middleware.Chain(
		middleware.RequestID,
		middleware.Logging,
		middleware.Recover,
		middleware.Metrics,
	)(root)
}
