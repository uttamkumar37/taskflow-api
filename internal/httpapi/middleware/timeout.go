package middleware

import (
	"net/http"
	"time"
)

// Timeout bounds how long any single request is allowed to run. Without
// this, one slow downstream call (a stuck DB query, a hanging network call)
// can hold a goroutine open indefinitely and degrade the whole service.
//
// http.TimeoutHandler's body is a fixed string decided up front, so unlike
// every other error path in this service it can't carry a per-request
// request_id — the access log (which does have it, via middleware.Logging)
// is the source of truth for correlating a specific timeout.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":"request timed out","code":"TIMEOUT"}`)
	}
}
