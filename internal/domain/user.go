package domain

import "time"

// User is the core entity. Note it never carries the plaintext password —
// only PasswordHash — so it's always safe to serialize (though handlers
// still use a separate DTO to control the JSON shape precisely).
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
