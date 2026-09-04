package service

import (
	"context"
	"time"

	"taskflow/internal/domain"
	"taskflow/internal/repository"
)

// fakeUserRepository is an in-memory stand-in for repository.UserRepository.
// Because the repository is an interface, tests never need a real Postgres
// connection to exercise service-layer logic — this is the whole point of
// depending on interfaces rather than concrete types.
type fakeUserRepository struct {
	byEmail map[string]*domain.User
	nextID  int64
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{byEmail: make(map[string]*domain.User)}
}

func (f *fakeUserRepository) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	if _, exists := f.byEmail[u.Email]; exists {
		return nil, domain.ErrAlreadyExists
	}
	f.nextID++
	u.ID = f.nextID
	f.byEmail[u.Email] = u
	return u, nil
}

func (f *fakeUserRepository) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepository) FindByID(_ context.Context, id int64) (*domain.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

// fakeTaskRepository is the same idea for repository.TaskRepository.
type fakeTaskRepository struct {
	byID   map[int64]*domain.Task
	nextID int64
}

func newFakeTaskRepository() *fakeTaskRepository {
	return &fakeTaskRepository{byID: make(map[int64]*domain.Task)}
}

func (f *fakeTaskRepository) Create(_ context.Context, t *domain.Task) (*domain.Task, error) {
	f.nextID++
	t.ID = f.nextID
	f.byID[t.ID] = t
	return t, nil
}

func (f *fakeTaskRepository) FindByID(_ context.Context, id int64) (*domain.Task, error) {
	t, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeTaskRepository) ListByUser(_ context.Context, userID int64, params repository.ListParams) ([]domain.Task, int, error) {
	var out []domain.Task
	for _, t := range f.byID {
		if t.UserID != userID {
			continue
		}
		if params.Status != "" && t.Status != params.Status {
			continue
		}
		out = append(out, *t)
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

func (f *fakeTaskRepository) Update(_ context.Context, t *domain.Task) (*domain.Task, error) {
	if _, ok := f.byID[t.ID]; !ok {
		return nil, domain.ErrNotFound
	}
	f.byID[t.ID] = t
	return t, nil
}

func (f *fakeTaskRepository) Delete(_ context.Context, id int64) error {
	if _, ok := f.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

// fakeRefreshTokenRepository is the in-memory stand-in for
// repository.RefreshTokenRepository, keyed by hash the same way the real
// Postgres table is (a unique index on token_hash).
type fakeRefreshTokenRepository struct {
	byHash map[string]*domain.RefreshToken
	nextID int64
}

func newFakeRefreshTokenRepository() *fakeRefreshTokenRepository {
	return &fakeRefreshTokenRepository{byHash: make(map[string]*domain.RefreshToken)}
}

func (f *fakeRefreshTokenRepository) Create(_ context.Context, t *domain.RefreshToken) (*domain.RefreshToken, error) {
	f.nextID++
	t.ID = f.nextID
	t.CreatedAt = time.Now()
	stored := *t
	f.byHash[t.TokenHash] = &stored
	return &stored, nil
}

func (f *fakeRefreshTokenRepository) FindByHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	t, ok := f.byHash[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	found := *t
	return &found, nil
}

func (f *fakeRefreshTokenRepository) Revoke(_ context.Context, id int64) error {
	for _, t := range f.byHash {
		if t.ID == id {
			now := time.Now()
			t.RevokedAt = &now
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeRefreshTokenRepository) RevokeAllForUser(_ context.Context, userID int64) error {
	now := time.Now()
	for _, t := range f.byHash {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}
