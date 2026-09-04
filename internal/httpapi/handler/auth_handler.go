package handler

import (
	"encoding/json"
	"net/http"

	"taskflow/internal/httpapi/dto"
	"taskflow/internal/httpapi/response"
	"taskflow/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req dto.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeValidation, err.Error())
		return
	}

	user, accessToken, refreshToken, err := h.auth.Signup(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, dto.AuthResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         dto.NewUserResponse(user),
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeValidation, err.Error())
		return
	}

	user, accessToken, refreshToken, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.AuthResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         dto.NewUserResponse(user),
	})
}

// Refresh exchanges a refresh token for a new access+refresh pair,
// rotating the used token (single-use — see AuthService.Refresh for the
// reuse-detection behavior when a token is presented twice).
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeValidation, err.Error())
		return
	}

	accessToken, refreshToken, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.TokenResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
	})
}

// Logout revokes the given refresh token. It doesn't require a bearer
// access token — the refresh token itself is the credential being acted on.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeValidation, err.Error())
		return
	}

	if err := h.auth.Logout(r.Context(), req.RefreshToken); err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusNoContent, nil)
}
