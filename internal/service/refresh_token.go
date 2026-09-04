package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// generateRefreshToken returns a high-entropy opaque token (32 random
// bytes, base64url-encoded for safe transport in JSON/URLs) and the hex
// SHA-256 hash of it. Only the hash is ever persisted — a refresh token is
// already maximally random (unlike a user-chosen password), so a fast hash
// is appropriate here; bcrypt's deliberate slowness buys nothing and would
// just cost CPU on every refresh.
func generateRefreshToken() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, hashRefreshToken(plaintext), nil
}

func hashRefreshToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
