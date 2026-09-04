package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"taskflow/internal/config"
	"taskflow/internal/domain"
	"taskflow/internal/repository"
	"taskflow/internal/service"
)

// In-memory repositories satisfying the repository interfaces, so this test
// exercises the real router/middleware/handler/service wiring end-to-end
// over HTTP without needing a live Postgres instance.

type memUserRepo struct {
	byEmail map[string]*domain.User
	nextID  int64
}

func newMemUserRepo() *memUserRepo { return &memUserRepo{byEmail: map[string]*domain.User{}} }

func (r *memUserRepo) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	if _, ok := r.byEmail[u.Email]; ok {
		return nil, domain.ErrAlreadyExists
	}
	r.nextID++
	u.ID = r.nextID
	r.byEmail[u.Email] = u
	return u, nil
}

func (r *memUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := r.byEmail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (r *memUserRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	for _, u := range r.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

type memTaskRepo struct {
	byID   map[int64]*domain.Task
	nextID int64
}

func newMemTaskRepo() *memTaskRepo { return &memTaskRepo{byID: map[int64]*domain.Task{}} }

func (r *memTaskRepo) Create(_ context.Context, t *domain.Task) (*domain.Task, error) {
	r.nextID++
	t.ID = r.nextID
	r.byID[t.ID] = t
	return t, nil
}

func (r *memTaskRepo) FindByID(_ context.Context, id int64) (*domain.Task, error) {
	t, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (r *memTaskRepo) ListByUser(_ context.Context, userID int64, params repository.ListParams) ([]domain.Task, int, error) {
	var out []domain.Task
	for _, t := range r.byID {
		if t.UserID == userID && (params.Status == "" || t.Status == params.Status) {
			out = append(out, *t)
		}
	}
	total := len(out)

	start := params.Offset
	if start > len(out) {
		start = len(out)
	}
	end := start + params.Limit
	if params.Limit <= 0 || end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}

func (r *memTaskRepo) Update(_ context.Context, t *domain.Task) (*domain.Task, error) {
	if _, ok := r.byID[t.ID]; !ok {
		return nil, domain.ErrNotFound
	}
	r.byID[t.ID] = t
	return t, nil
}

func (r *memTaskRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

type memRefreshTokenRepo struct {
	byHash map[string]*domain.RefreshToken
	nextID int64
}

func newMemRefreshTokenRepo() *memRefreshTokenRepo {
	return &memRefreshTokenRepo{byHash: map[string]*domain.RefreshToken{}}
}

func (r *memRefreshTokenRepo) Create(_ context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error) {
	r.nextID++
	t.ID = r.nextID
	t.CreatedAt = time.Now()
	stored := *t
	r.byHash[t.TokenHash] = &stored
	return &stored, nil
}

func (r *memRefreshTokenRepo) FindByHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	t, ok := r.byHash[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	found := *t
	return &found, nil
}

func (r *memRefreshTokenRepo) Revoke(_ context.Context, id int64) error {
	for _, t := range r.byHash {
		if t.ID == id {
			now := time.Now()
			t.RevokedAt = &now
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *memRefreshTokenRepo) RevokeAllForUser(_ context.Context, userID int64) error {
	now := time.Now()
	for _, t := range r.byHash {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}

// testConfig returns permissive operational settings (generous rate limit,
// wide CORS) so tests exercise routing/business logic without tripping the
// protective middleware — except TestRouter_RateLimiting, which deliberately
// builds its own tight-limit router.
func testConfig() config.Config {
	return config.Config{
		AllowedOrigins: []string{"*"},
		RateLimitRPS:   1000,
		RateLimitBurst: 1000,
		RequestTimeout: 5 * time.Second,
		MaxBodyBytes:   1 << 20,
	}
}

// newTestRouter wires the router against in-memory fakes instead of a real
// Postgres connection. Passing a nil *sql.DB is safe here because none of
// these tests hit /readyz (the only handler that dereferences it).
func newTestRouter() http.Handler {
	return newTestRouterWithConfig(testConfig())
}

func newTestRouterWithConfig(cfg config.Config) http.Handler {
	tokens := service.NewTokenManager("test-secret", time.Hour)
	authSvc := service.NewAuthService(
		repository.UserRepository(newMemUserRepo()),
		repository.RefreshTokenRepository(newMemRefreshTokenRepo()),
		tokens,
		24*time.Hour,
	)
	taskSvc := service.NewTaskService(repository.TaskRepository(newMemTaskRepo()))

	return NewRouter(Deps{
		Handlers:     Handlers{Auth: authSvc, Task: taskSvc},
		Tokens:       tokens,
		DB:           nil,
		Config:       cfg,
		BuildVersion: "test",
	})
}

func doJSON(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = strings.NewReader(string(b))
	} else {
		reader = strings.NewReader("")
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRouter_TaskLifecycle(t *testing.T) {
	router := newTestRouter()

	// liveness check needs no auth
	if rec := doJSON(t, router, "GET", "/healthz", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("healthz: expected 200, got %d", rec.Code)
	}

	// tasks endpoint requires auth
	if rec := doJSON(t, router, "GET", "/api/v1/tasks", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list: expected 401, got %d", rec.Code)
	}

	// sign up to get a token
	signupRec := doJSON(t, router, "POST", "/api/v1/auth/signup", "", map[string]string{
		"email": "router-test@example.com", "password": "password123",
	})
	if signupRec.Code != http.StatusCreated {
		t.Fatalf("signup: expected 201, got %d: %s", signupRec.Code, signupRec.Body.String())
	}
	var signupResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(signupRec.Body.Bytes(), &signupResp); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}
	token := signupResp.Token

	// create a task
	createRec := doJSON(t, router, "POST", "/api/v1/tasks", token, map[string]string{"title": "Ship it"})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create task: expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var task struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	// list tasks, should contain the one we made, wrapped in the paginated envelope
	listRec := doJSON(t, router, "GET", "/api/v1/tasks", token, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list tasks: expected 200, got %d", listRec.Code)
	}
	var listResp struct {
		Data  []struct{ Title string } `json:"data"`
		Total int                      `json:"total"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Data) != 1 || listResp.Data[0].Title != "Ship it" {
		t.Fatalf("expected one task titled 'Ship it', got %+v", listResp)
	}

	// a second user cannot access the first user's task
	signup2Rec := doJSON(t, router, "POST", "/api/v1/auth/signup", "", map[string]string{
		"email": "other-user@example.com", "password": "password123",
	})
	var signup2Resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(signup2Rec.Body.Bytes(), &signup2Resp); err != nil {
		t.Fatalf("decode second signup response: %v", err)
	}

	forbiddenPath := "/api/v1/tasks/" + strconv.FormatInt(task.ID, 10)
	if rec := doJSON(t, router, "GET", forbiddenPath, signup2Resp.Token, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-user get: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	// owner can delete it
	if rec := doJSON(t, router, "DELETE", forbiddenPath, token, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete task: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRouter_RateLimiting(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimitRPS = 1
	cfg.RateLimitBurst = 2
	router := newTestRouterWithConfig(cfg)

	var lastCode int
	for i := 0; i < 5; i++ {
		lastCode = doJSON(t, router, "GET", "/api/v1/tasks", "", nil).Code
		if lastCode == http.StatusTooManyRequests {
			break
		}
	}

	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected rate limiting to eventually return 429, got %d", lastCode)
	}
}

func TestRouter_ErrorEnvelope(t *testing.T) {
	router := newTestRouter()

	signupRec := doJSON(t, router, "POST", "/api/v1/auth/signup", "", map[string]string{
		"email": "envelope-test@example.com", "password": "password123",
	})
	var signupResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(signupRec.Body.Bytes(), &signupResp); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}

	rec := doJSON(t, router, "GET", "/api/v1/tasks/999999", signupResp.Token, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Error     string `json:"error"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if body.Code != "NOT_FOUND" {
		t.Fatalf("expected code NOT_FOUND, got %q", body.Code)
	}
	if body.RequestID == "" {
		t.Fatalf("expected a non-empty request_id in the error body")
	}
	if headerID := rec.Header().Get("X-Request-ID"); headerID != body.RequestID {
		t.Fatalf("expected body request_id %q to match X-Request-ID header %q", body.RequestID, headerID)
	}
}

func TestRouter_AuthLifecycle_RefreshAndLogout(t *testing.T) {
	router := newTestRouter()

	signupRec := doJSON(t, router, "POST", "/api/v1/auth/signup", "", map[string]string{
		"email": "lifecycle@example.com", "password": "password123",
	})
	if signupRec.Code != http.StatusCreated {
		t.Fatalf("signup: expected 201, got %d: %s", signupRec.Code, signupRec.Body.String())
	}
	var tokens struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(signupRec.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}
	if tokens.RefreshToken == "" {
		t.Fatal("expected signup to return a refresh_token")
	}

	refreshRec := doJSON(t, router, "POST", "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": tokens.RefreshToken,
	})
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d: %s", refreshRec.Code, refreshRec.Body.String())
	}
	var rotated struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(refreshRec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if rotated.RefreshToken == tokens.RefreshToken {
		t.Fatal("expected refresh to rotate to a new refresh token")
	}

	// reusing the original (now-revoked) refresh token must fail...
	reuseRec := doJSON(t, router, "POST", "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": tokens.RefreshToken,
	})
	if reuseRec.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh token: expected 401, got %d: %s", reuseRec.Code, reuseRec.Body.String())
	}

	// ...and reuse detection must have revoked the rotated token too.
	afterReuseRec := doJSON(t, router, "POST", "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": rotated.RefreshToken,
	})
	if afterReuseRec.Code != http.StatusUnauthorized {
		t.Fatalf("rotated token after reuse detection: expected 401, got %d: %s", afterReuseRec.Code, afterReuseRec.Body.String())
	}

	// a fresh login + logout round trip should work cleanly
	loginRec := doJSON(t, router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": "lifecycle@example.com", "password": "password123",
	})
	var loginResp struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	logoutRec := doJSON(t, router, "POST", "/api/v1/auth/logout", "", map[string]string{
		"refresh_token": loginResp.RefreshToken,
	})
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout: expected 204, got %d: %s", logoutRec.Code, logoutRec.Body.String())
	}

	postLogoutRec := doJSON(t, router, "POST", "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": loginResp.RefreshToken,
	})
	if postLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: expected 401, got %d: %s", postLogoutRec.Code, postLogoutRec.Body.String())
	}
}

func TestRouter_MetricsEndpoint(t *testing.T) {
	router := newTestRouter()

	rec := doJSON(t, router, "GET", "/metrics", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics: expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "http_requests_total") {
		t.Fatalf("expected /metrics to expose http_requests_total, got body starting %q", rec.Body.String()[:min(200, rec.Body.Len())])
	}
}
