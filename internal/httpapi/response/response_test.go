package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"taskflow/internal/httpapi/reqctx"
)

func TestError_IncludesCodeAndRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(reqctx.WithRequestID(req.Context(), "req-123"))
	rec := httptest.NewRecorder()

	Error(rec, req, http.StatusNotFound, CodeNotFound, "resource not found")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	var body struct {
		Error     string `json:"error"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Error != "resource not found" {
		t.Errorf("Error = %q, want %q", body.Error, "resource not found")
	}
	if body.Code != CodeNotFound {
		t.Errorf("Code = %q, want %q", body.Code, CodeNotFound)
	}
	if body.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", body.RequestID, "req-123")
	}
}

func TestError_OmitsRequestIDWhenAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	Error(rec, req, http.StatusInternalServerError, CodeInternal, "internal server error")

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, present := raw["request_id"]; present {
		t.Errorf("expected request_id to be omitted when absent, got %v", raw["request_id"])
	}
}
