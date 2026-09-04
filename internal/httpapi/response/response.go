// Package response gives every handler one consistent way to write JSON, so
// callers of the API always see the same envelope shape for data and errors.
package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"taskflow/internal/httpapi/reqctx"
)

// Machine-readable error codes. Clients should branch on these, not on the
// human-readable message, which may change wording over time.
const (
	CodeBadRequest    = "BAD_REQUEST"
	CodeValidation    = "VALIDATION_ERROR"
	CodeUnauthorized  = "UNAUTHORIZED"
	CodeForbidden     = "FORBIDDEN"
	CodeNotFound      = "NOT_FOUND"
	CodeAlreadyExists = "ALREADY_EXISTS"
	CodeRateLimited   = "RATE_LIMITED"
	CodeTimeout       = "TIMEOUT"
	CodeInternal      = "INTERNAL_ERROR"
)

type errorBody struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id,omitempty"`
}

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

// Error writes a consistent error envelope, tagging it with the request ID
// (from reqctx, set by middleware.RequestID) so a client reporting an error
// gives support a value that's grep-able straight into the matching log line.
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, status, errorBody{
		Error:     message,
		Code:      code,
		RequestID: reqctx.RequestID(r.Context()),
	})
}
