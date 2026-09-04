package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenManager issues and validates JWTs. Keeping it as its own type (rather
// than free functions) means the signing secret and expiry are captured once
// at startup and threaded explicitly, instead of leaking into a global.
type TokenManager struct {
	secret    []byte
	expiresIn time.Duration
}

func NewTokenManager(secret string, expiresIn time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), expiresIn: expiresIn}
}

// Generate creates a signed JWT whose subject is the user's ID. We use the
// HMAC-SHA256 (HS256) algorithm, which is appropriate here because the same
// service both signs and verifies tokens (no third party needs a public key).
func (m *TokenManager) Generate(userID int64) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.expiresIn)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Validate parses and verifies a token, returning the embedded user ID.
func (m *TokenManager) Validate(tokenString string) (int64, error) {
	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return 0, fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid token subject: %w", err)
	}

	return userID, nil
}
