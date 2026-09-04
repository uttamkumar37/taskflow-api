package middleware

import (
	"net/http"
	"time"
)

// Timeout bounds how long any single request is allowed to run. Without
// this, one slow downstream call (a stuck DB query, a hanging network call)
// can hold a goroutine open indefinitely and degrade the whole service.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":"request timed out"}`)
	}
}
