package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"taskflow/internal/domain"
	"taskflow/internal/httpapi/response"
)

// writeDomainError centralizes the mapping from sentinel domain errors to
// HTTP status codes. Handlers never invent status codes themselves — this
// is the single place that decision is made, which keeps the API consistent
// as more endpoints are added.
func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		response.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		response.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		response.Error(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		response.Error(w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrValidation):
		response.Error(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("unhandled error", "error", err)
		response.Error(w, http.StatusInternalServerError, "internal server error")
	}
}
