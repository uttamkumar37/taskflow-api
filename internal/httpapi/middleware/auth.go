package middleware

import (
	"context"
	"net/http"
	"strings"

	"taskflow/internal/httpapi/response"
	"taskflow/internal/service"
)

// unexported type prevents context key collisions with other packages that
// might also store a value under a plain string key like "userID".
type contextKey int

const userIDKey contextKey = iota

// Auth validates the "Authorization: Bearer <token>" header and, on success,
// stashes the authenticated user's ID in the request context for downstream
// handlers to read via UserIDFromContext. This is where authentication
// (who are you) is enforced; authorization (what are you allowed to touch)
// still happens in the service layer per-resource.
func Auth(tokens *service.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				response.Error(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}

			userID, err := tokens.Validate(parts[1])
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext reads the user ID set by Auth. It's only ever called on
// routes wrapped by Auth, so the "ok" case failing indicates a wiring bug,
// not a runtime condition callers need to handle gracefully.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}
