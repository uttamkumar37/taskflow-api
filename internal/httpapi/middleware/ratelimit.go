package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"taskflow/internal/httpapi/reqctx"
	"taskflow/internal/httpapi/response"
)

// limiterBackend is the strategy RateLimiter delegates to. Having two
// implementations (in-memory, Redis) behind one interface means the
// middleware and its call sites don't need to know or care which is
// active — that decision is made once, at startup, based on whether a
// Redis address was configured.
type limiterBackend interface {
	allow(ctx context.Context, ip string) (bool, error)
}

// RateLimiter enforces a per-client-IP request budget. In-process
// (in-memory) rate limiting only sees traffic hitting *this* instance, so
// it silently allows N times the intended rate once you run N replicas
// behind a load balancer. The Redis backend fixes that by sharing counters
// across every instance; in-memory remains the zero-dependency fallback for
// local dev or a single-instance deployment.
type RateLimiter struct {
	backend limiterBackend
}

// NewRateLimiter picks the Redis-backed limiter when redisClient is
// non-nil and reachable, otherwise falls back to the in-memory limiter —
// logging a warning rather than failing startup, since a rate limiter
// degrading to "less accurate under scale" is a far better outcome than
// the whole API refusing to start because a cache happened to be down.
func NewRateLimiter(rps float64, burst int, redisClient *redis.Client) *RateLimiter {
	if redisClient != nil {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := redisClient.Ping(pingCtx).Err(); err != nil {
			slog.Warn("redis unreachable, falling back to in-memory rate limiting", "error", err)
		} else {
			slog.Info("rate limiting backed by redis")
			return &RateLimiter{backend: newRedisBackend(redisClient, burst)}
		}
	}

	slog.Info("rate limiting backed by in-memory token bucket (single-instance only)")
	return &RateLimiter{backend: newInMemoryBackend(rps, burst)}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		allowed, err := rl.backend.allow(r.Context(), ip)
		if err != nil {
			// Fail open: a rate-limiter backend hiccup should degrade
			// protection, not take the whole API down with it.
			reqctx.Logger(r.Context()).Error("rate limiter backend error, allowing request", "error", err)
			allowed = true
		}

		if !allowed {
			response.Error(w, r, http.StatusTooManyRequests, response.CodeRateLimited, "rate limit exceeded, slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
