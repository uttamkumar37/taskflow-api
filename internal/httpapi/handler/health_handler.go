package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"taskflow/internal/httpapi/response"
)

// Health is the liveness probe: "is the process up and able to respond at
// all". It deliberately checks nothing else — a container orchestrator
// (Kubernetes, ECS, Docker's own healthcheck) uses this to decide whether
// to restart the container.
func Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready is the readiness probe: "is this instance able to serve real
// traffic right now". It checks the database connection, since a request
// would fail anyway without it. Orchestrators use readiness (separately
// from liveness) to decide whether to route traffic to this instance —
// e.g. during startup, before migrations finish, or during a brief DB
// failover, the process is alive but not yet ready.
func Ready(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			response.JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  "database unreachable",
			})
			return
		}

		response.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
