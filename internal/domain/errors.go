package domain

import "errors"

// Sentinel errors shared across service/repository layers. Handlers map
// these to HTTP status codes without the layers needing to know about HTTP.
var (
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrForbidden          = errors.New("you do not have access to this resource")
	ErrValidation         = errors.New("validation failed")
	// ErrTokenInvalid covers a refresh token that's unknown, expired, or
	// already revoked. It's deliberately as generic as ErrInvalidCredentials
	// — the client shouldn't be able to distinguish "never existed" from
	// "expired" from "was revoked" from the error alone.
	ErrTokenInvalid = errors.New("invalid or expired refresh token")
)
