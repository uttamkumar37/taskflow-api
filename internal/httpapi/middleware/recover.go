package middleware

import (
	"net/http"

	"taskflow/internal/httpapi/reqctx"
	"taskflow/internal/httpapi/response"
)

// Recover turns a panic anywhere in the handler chain into a 500 response
// instead of crashing the whole server process. Without this, a single nil
// pointer dereference in one request would take down every in-flight request.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqctx.Logger(r.Context()).Error("panic recovered", "error", rec, "path", r.URL.Path)
				response.Error(w, r, http.StatusInternalServerError, response.CodeInternal, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
