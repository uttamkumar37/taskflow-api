package dto

import (
	"fmt"
	"strings"

	"taskflow/internal/domain"
)

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r SignupRequest) Validate() error {
	if !strings.Contains(r.Email, "@") {
		return fmt.Errorf("%w: email must be valid", domain.ErrValidation)
	}
	if len(r.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", domain.ErrValidation)
	}
	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r LoginRequest) Validate() error {
	if r.Email == "" || r.Password == "" {
		return fmt.Errorf("%w: email and password are required", domain.ErrValidation)
	}
	return nil
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (r RefreshRequest) Validate() error {
	if r.RefreshToken == "" {
		return fmt.Errorf("%w: refresh_token is required", domain.ErrValidation)
	}
	return nil
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (r LogoutRequest) Validate() error {
	if r.RefreshToken == "" {
		return fmt.Errorf("%w: refresh_token is required", domain.ErrValidation)
	}
	return nil
}

type AuthResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

type TokenResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type UserResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func NewUserResponse(u *domain.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email}
}
