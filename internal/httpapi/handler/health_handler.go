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
// to restart the container. It reports the running build's version, which
// is the first thing worth checking during an incident ("is the fix we
// shipped actually deployed here?").
func Health(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	}
}

// Ready is the readiness probe: "is this instance able to serve real
// traffic right now". It checks the database connection, since a request
// would fail anyway without it. Orchestrators use readiness (separately
// from liveness) to decide whether to route traffic to this instance —
// e.g. during startup, before migrations finish, or during a brief DB
// failover, the process is alive but not yet ready.
func Ready(db *sql.DB, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			response.JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":  "unavailable",
				"error":   "database unreachable",
				"version": version,
			})
			return
		}

		response.JSON(w, http.StatusOK, map[string]string{"status": "ready", "version": version})
	}
}
