package domain

import "time"

// RefreshToken is stored as a hash, never the plaintext value handed to the
// client — a DB leak alone should never be enough to impersonate a user.
type RefreshToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (t RefreshToken) Expired() bool {
	return time.Now().After(t.ExpiresAt)
}

func (t RefreshToken) Revoked() bool {
	return t.RevokedAt != nil
}
