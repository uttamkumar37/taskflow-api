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

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type UserResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func NewUserResponse(u *domain.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email}
}
