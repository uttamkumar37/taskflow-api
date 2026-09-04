package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"taskflow/internal/domain"
	"taskflow/internal/httpapi/dto"
	"taskflow/internal/httpapi/middleware"
	"taskflow/internal/httpapi/response"
	"taskflow/internal/service"
)

type TaskHandler struct {
	tasks *service.TaskService
}

func NewTaskHandler(tasks *service.TaskService) *TaskHandler {
	return &TaskHandler{tasks: tasks}
}

// currentUserID pulls the authenticated user ID set by middleware.Auth. The
// "not ok" branch should be unreachable in practice (it would mean this
// handler was mounted without the Auth middleware), so it's treated as a
// server bug rather than a client error.
func currentUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, r, http.StatusInternalServerError, response.CodeInternal, "internal server error")
		return 0, false
	}
	return userID, true
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	var req dto.TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeValidation, err.Error())
		return
	}

	task, err := h.tasks.Create(r.Context(), userID, req.Title, req.Description, req.DueDate)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, dto.NewTaskResponse(task))
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	status := domain.TaskStatus(r.URL.Query().Get("status"))
	if status != "" && !status.Valid() {
		response.Error(w, r, http.StatusBadRequest, response.CodeValidation, "invalid status filter")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	tasks, total, effLimit, effOffset, err := h.tasks.List(r.Context(), userID, status, limit, offset)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.NewTaskListResponse(tasks, total, effLimit, effOffset))
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	taskID, err := parseTaskID(r)
	if err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid task id")
		return
	}

	task, err := h.tasks.Get(r.Context(), userID, taskID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.NewTaskResponse(task))
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	taskID, err := parseTaskID(r)
	if err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid task id")
		return
	}

	var req dto.TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeValidation, err.Error())
		return
	}
	status := domain.TaskStatus(req.Status)
	if status == "" {
		status = domain.TaskStatusPending
	}

	task, err := h.tasks.Update(r.Context(), userID, taskID, req.Title, req.Description, status, req.DueDate)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.NewTaskResponse(task))
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(w, r)
	if !ok {
		return
	}

	taskID, err := parseTaskID(r)
	if err != nil {
		response.Error(w, r, http.StatusBadRequest, response.CodeBadRequest, "invalid task id")
		return
	}

	if err := h.tasks.Delete(r.Context(), userID, taskID); err != nil {
		writeDomainError(w, r, err)
		return
	}

	response.JSON(w, http.StatusNoContent, nil)
}

func parseTaskID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
