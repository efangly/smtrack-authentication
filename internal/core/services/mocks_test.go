package services

import (
	"context"
	"errors"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
)

// --- Mock UserRepository ---
type mockUserRepo struct {
	CreateFn         func(ctx context.Context, user *domain.User) (*domain.User, error)
	FindAllFn        func(ctx context.Context, filter ports.UserFilter) ([]domain.User, error)
	FindByIDFn       func(ctx context.Context, id string) (*domain.User, error)
	FindByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
	UpdateFn         func(ctx context.Context, id string, data domain.UserPatch) (*domain.User, error)
	DeleteFn         func(ctx context.Context, id string) (*domain.User, error)
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	return m.CreateFn(ctx, user)
}
func (m *mockUserRepo) FindAll(ctx context.Context, filter ports.UserFilter) ([]domain.User, error) {
	return m.FindAllFn(ctx, filter)
}
func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockUserRepo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	return m.FindByUsernameFn(ctx, username)
}
func (m *mockUserRepo) Update(ctx context.Context, id string, data domain.UserPatch) (*domain.User, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockUserRepo) Delete(ctx context.Context, id string) (*domain.User, error) {
	return m.DeleteFn(ctx, id)
}

// --- Mock WardRepository ---
type mockWardRepo struct {
	CreateFn   func(ctx context.Context, ward *domain.Ward) (*domain.Ward, error)
	FindAllFn  func(ctx context.Context, filter ports.WardFilter) ([]domain.Ward, error)
	FindByIDFn func(ctx context.Context, id string) (*domain.Ward, error)
	UpdateFn   func(ctx context.Context, id string, data domain.WardPatch) (*domain.Ward, error)
	DeleteFn   func(ctx context.Context, id string) error
}

func (m *mockWardRepo) Create(ctx context.Context, ward *domain.Ward) (*domain.Ward, error) {
	return m.CreateFn(ctx, ward)
}
func (m *mockWardRepo) FindAll(ctx context.Context, filter ports.WardFilter) ([]domain.Ward, error) {
	return m.FindAllFn(ctx, filter)
}
func (m *mockWardRepo) FindByID(ctx context.Context, id string) (*domain.Ward, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockWardRepo) Update(ctx context.Context, id string, data domain.WardPatch) (*domain.Ward, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockWardRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFn(ctx, id)
}

// --- Mock HospitalRepository ---
type mockHospitalRepo struct {
	CreateFn   func(ctx context.Context, hospital *domain.Hospital) (*domain.Hospital, error)
	FindAllFn  func(ctx context.Context, filter ports.HospitalFilter) ([]domain.Hospital, error)
	FindByIDFn func(ctx context.Context, id string) (*domain.Hospital, error)
	UpdateFn   func(ctx context.Context, id string, data domain.HospitalPatch) (*domain.Hospital, error)
	DeleteFn   func(ctx context.Context, id string) (*domain.Hospital, error)
}

func (m *mockHospitalRepo) Create(ctx context.Context, hospital *domain.Hospital) (*domain.Hospital, error) {
	return m.CreateFn(ctx, hospital)
}
func (m *mockHospitalRepo) FindAll(ctx context.Context, filter ports.HospitalFilter) ([]domain.Hospital, error) {
	return m.FindAllFn(ctx, filter)
}
func (m *mockHospitalRepo) FindByID(ctx context.Context, id string) (*domain.Hospital, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockHospitalRepo) Update(ctx context.Context, id string, data domain.HospitalPatch) (*domain.Hospital, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockHospitalRepo) Delete(ctx context.Context, id string) (*domain.Hospital, error) {
	return m.DeleteFn(ctx, id)
}

// --- Mock CachePort ---
type mockCache struct {
	store map[string]string
	SetFn func(ctx context.Context, key string, value string, expireSeconds int) error
	GetFn func(ctx context.Context, key string) (string, error)
	DelFn func(ctx context.Context, pattern string) error
}

func newMockCache() *mockCache {
	m := &mockCache{store: make(map[string]string)}
	m.SetFn = func(_ context.Context, key, value string, _ int) error {
		m.store[key] = value
		return nil
	}
	m.GetFn = func(_ context.Context, key string) (string, error) {
		v, ok := m.store[key]
		if !ok {
			return "", context.Canceled // simulate cache miss
		}
		return v, nil
	}
	m.DelFn = func(_ context.Context, _ string) error { return nil }
	return m
}

func (m *mockCache) Set(ctx context.Context, key string, value string, expireSeconds int) error {
	return m.SetFn(ctx, key, value, expireSeconds)
}
func (m *mockCache) Get(ctx context.Context, key string) (string, error) {
	return m.GetFn(ctx, key)
}
func (m *mockCache) Del(ctx context.Context, pattern string) error {
	return m.DelFn(ctx, pattern)
}

// --- Mock MessagingPort ---
type mockMessaging struct {
	SendToDeviceFn func(queue string, payload any) error
	SendToLegacyFn func(queue string, payload any) error
}

func newMockMessaging() *mockMessaging {
	return &mockMessaging{
		SendToDeviceFn: func(string, any) error { return nil },
		SendToLegacyFn: func(string, any) error { return nil },
	}
}

func (m *mockMessaging) SendToDevice(queue string, payload any) error {
	return m.SendToDeviceFn(queue, payload)
}
func (m *mockMessaging) SendToLegacy(queue string, payload any) error {
	return m.SendToLegacyFn(queue, payload)
}

// --- Mock FileUploadPort ---
type mockUpload struct {
	UploadFn func(file *domain.File, path string) (string, error)
	DeleteFn func(path string, filename string) error
}

func newMockUpload() *mockUpload {
	return &mockUpload{
		UploadFn: func(_ *domain.File, _ string) (string, error) {
			return "http://example.com/pic.jpg", nil
		},
		DeleteFn: func(string, string) error { return nil },
	}
}

func (m *mockUpload) Upload(_ context.Context, file *domain.File, path string) (string, error) {
	return m.UploadFn(file, path)
}
func (m *mockUpload) Delete(path string, filename string) error {
	return m.DeleteFn(path, filename)
}

// --- Mock UserService (for auth_service tests) ---
type mockUserService struct {
	CreateFn         func(ctx context.Context, user *domain.User, caller *domain.Actor) (*domain.User, error)
	FindAllFn        func(ctx context.Context, caller *domain.Actor) ([]domain.User, error)
	FindOneFn        func(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
	FindByIDFn       func(ctx context.Context, id string) (*domain.User, error)
	FindByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
	UpdateFn         func(ctx context.Context, id string, data domain.UserPatch, file *domain.File, caller *domain.Actor) (*domain.User, error)
	RemoveFn         func(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
}

func (m *mockUserService) Create(ctx context.Context, user *domain.User, caller *domain.Actor) (*domain.User, error) {
	return m.CreateFn(ctx, user, caller)
}
func (m *mockUserService) FindAll(ctx context.Context, caller *domain.Actor) ([]domain.User, error) {
	return m.FindAllFn(ctx, caller)
}
func (m *mockUserService) FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error) {
	return m.FindOneFn(ctx, id, caller)
}
func (m *mockUserService) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockUserService) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	return m.FindByUsernameFn(ctx, username)
}
func (m *mockUserService) Update(ctx context.Context, id string, data domain.UserPatch, file *domain.File, caller *domain.Actor) (*domain.User, error) {
	return m.UpdateFn(ctx, id, data, file, caller)
}
func (m *mockUserService) Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error) {
	return m.RemoveFn(ctx, id, caller)
}

// --- Mock AuditRepository ---
type mockAuditRepo struct {
	CreateFn  func(ctx context.Context, log *domain.AuditLog) (*domain.AuditLog, error)
	FindAllFn func(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error)
}

func (m *mockAuditRepo) Create(ctx context.Context, log *domain.AuditLog) (*domain.AuditLog, error) {
	return m.CreateFn(ctx, log)
}
func (m *mockAuditRepo) FindAll(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
	return m.FindAllFn(ctx, filter)
}

// --- Mock AuditService ---
type mockAuditService struct {
	RecordFn  func(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *domain.Actor, changes any)
	FindAllFn func(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error)
}

func newMockAuditService() *mockAuditService {
	return &mockAuditService{
		RecordFn: func(context.Context, domain.AuditEntity, domain.AuditAction, string, *domain.Actor, any) {},
		FindAllFn: func(context.Context, ports.AuditFilter) ([]domain.AuditLog, error) {
			return nil, nil
		},
	}
}

func (m *mockAuditService) Record(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *domain.Actor, changes any) {
	m.RecordFn(ctx, entity, action, entityID, caller, changes)
}
func (m *mockAuditService) FindAll(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
	return m.FindAllFn(ctx, filter)
}

// --- Fake PasswordHasher ---
// Deterministic and instant, so tests can assert on the stored value.

func fakeHash(plain string) string { return "hashed:" + plain }

type fakeHasher struct{ dummyCalls int }

func (h *fakeHasher) Hash(plain string) (string, error) { return fakeHash(plain), nil }
func (h *fakeHasher) Compare(hash, plain string) bool   { return hash == fakeHash(plain) }
func (h *fakeHasher) CompareDummy(string)               { h.dummyCalls++ }

// --- Fake TokenIssuer ---

type fakeTokens struct {
	issued        []domain.Actor
	refreshUserID string
	refreshErr    error
}

func (f *fakeTokens) IssuePair(actor domain.Actor) (ports.TokenPair, error) {
	f.issued = append(f.issued, actor)
	return ports.TokenPair{
		Access:  "access:" + actor.ID + ":" + string(actor.Role),
		Refresh: "refresh:" + actor.ID,
	}, nil
}
func (f *fakeTokens) VerifyAccess(string) (*domain.Actor, error) {
	return nil, errors.New("not used by the services")
}
func (f *fakeTokens) VerifyRefresh(string) (string, error) {
	return f.refreshUserID, f.refreshErr
}

func ptr[T any](v T) *T { return &v }
