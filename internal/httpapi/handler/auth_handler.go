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

	user, token, err := h.auth.Signup(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, dto.AuthResponse{
		Token: token,
		User:  dto.NewUserResponse(user),
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

	user, token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.AuthResponse{
		Token: token,
		User:  dto.NewUserResponse(user),
	})
}
