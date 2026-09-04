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
)
