package handler

import (
	"errors"
	"net/http"

	"taskflow/internal/domain"
	"taskflow/internal/httpapi/reqctx"
	"taskflow/internal/httpapi/response"
)

// writeDomainError centralizes the mapping from sentinel domain errors to
// HTTP status codes. Handlers never invent status codes themselves — this
// is the single place that decision is made, which keeps the API consistent
// as more endpoints are added.
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		response.Error(w, r, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		response.Error(w, r, http.StatusConflict, response.CodeAlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		response.Error(w, r, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		response.Error(w, r, http.StatusForbidden, response.CodeForbidden, err.Error())
	case errors.Is(err, domain.ErrValidation):
		response.Error(w, r, http.StatusBadRequest, response.CodeValidation, err.Error())
	default:
		// request_id is already baked into this logger by middleware.RequestID,
		// so this line correlates directly with the access-log line for the
		// same request even though this package never sees that middleware.
		reqctx.Logger(r.Context()).Error("unhandled error", "error", err)
		response.Error(w, r, http.StatusInternalServerError, response.CodeInternal, "internal server error")
	}
}
