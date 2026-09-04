package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// These are the standard "RED" metrics (Rate, Errors, Duration) that let an
// operator (or an alert) answer "is this service healthy?" without reading
// logs — the bread and butter of any production Go service's /metrics
// endpoint, scraped by Prometheus and graphed in Grafana.
var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed, labeled by method, route, and status.",
		},
		[]string{"method", "route", "status"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration)
}

// Metrics records request count and latency per (method, route, status).
// It uses r.Pattern (the matched ServeMux pattern, e.g. "GET /api/v1/tasks/{id}")
// rather than r.URL.Path, so metrics stay low-cardinality even as real task
// IDs vary — a critical detail, since Prometheus performance degrades badly
// with high-cardinality labels.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}

		requestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		requestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}
