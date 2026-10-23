package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
)

// --- Mock UserRepository ---
// Fn fields a test does not set fall back to a harmless default.
type mockUserRepo struct {
	CreateFn         func(ctx context.Context, user *domain.User) (*domain.User, error)
	FindAllFn        func(ctx context.Context, filter ports.UserFilter) ([]domain.User, error)
	FindByIDFn       func(ctx context.Context, id string) (*domain.User, error)
	FindByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
	FindAnyByIDFn    func(ctx context.Context, id string) (*domain.User, error)
	UsernameTakenFn  func(ctx context.Context, username string) (bool, error)
	UpdateFn         func(ctx context.Context, id string, data domain.UserPatch) (*domain.User, error)
	DeleteFn         func(ctx context.Context, id string, deletedBy string) (*domain.User, error)
	RestoreFn        func(ctx context.Context, id string) (*domain.User, error)
	RecordLoginFn    func(ctx context.Context, id string, at time.Time) error
}

func (m *mockUserRepo) RecordLogin(ctx context.Context, id string, at time.Time) error {
	if m.RecordLoginFn == nil {
		return nil
	}
	return m.RecordLoginFn(ctx, id, at)
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
func (m *mockUserRepo) FindAnyByID(ctx context.Context, id string) (*domain.User, error) {
	return m.FindAnyByIDFn(ctx, id)
}
func (m *mockUserRepo) UsernameTaken(ctx context.Context, username string) (bool, error) {
	if m.UsernameTakenFn == nil {
		return false, nil
	}
	return m.UsernameTakenFn(ctx, username)
}
func (m *mockUserRepo) Update(ctx context.Context, id string, data domain.UserPatch) (*domain.User, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockUserRepo) Delete(ctx context.Context, id string, deletedBy string) (*domain.User, error) {
	return m.DeleteFn(ctx, id, deletedBy)
}
func (m *mockUserRepo) Restore(ctx context.Context, id string) (*domain.User, error) {
	return m.RestoreFn(ctx, id)
}

// --- Mock DepartmentRepository ---
type mockDepartmentRepo struct {
	CreateFn      func(ctx context.Context, department *domain.Department) (*domain.Department, error)
	FindAllFn     func(ctx context.Context, filter ports.DepartmentFilter) ([]domain.Department, error)
	FindByIDFn    func(ctx context.Context, id string) (*domain.Department, error)
	FindByIDsFn   func(ctx context.Context, ids []string) ([]domain.Department, error)
	FindAnyByIDFn func(ctx context.Context, id string) (*domain.Department, error)
	NameTakenFn   func(ctx context.Context, tenantID, name, exceptID string) (bool, error)
	UpdateFn      func(ctx context.Context, id string, data domain.DepartmentPatch) (*domain.Department, error)
	DeleteFn      func(ctx context.Context, id string, deletedBy string) (*domain.Department, error)
	RestoreFn     func(ctx context.Context, id string) (*domain.Department, error)
}

func (m *mockDepartmentRepo) Create(ctx context.Context, d *domain.Department) (*domain.Department, error) {
	return m.CreateFn(ctx, d)
}
func (m *mockDepartmentRepo) FindAll(ctx context.Context, filter ports.DepartmentFilter) ([]domain.Department, error) {
	return m.FindAllFn(ctx, filter)
}
func (m *mockDepartmentRepo) FindByID(ctx context.Context, id string) (*domain.Department, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockDepartmentRepo) FindByIDs(ctx context.Context, ids []string) ([]domain.Department, error) {
	return m.FindByIDsFn(ctx, ids)
}
func (m *mockDepartmentRepo) FindAnyByID(ctx context.Context, id string) (*domain.Department, error) {
	return m.FindAnyByIDFn(ctx, id)
}
func (m *mockDepartmentRepo) NameTaken(ctx context.Context, tenantID, name, exceptID string) (bool, error) {
	if m.NameTakenFn == nil {
		return false, nil
	}
	return m.NameTakenFn(ctx, tenantID, name, exceptID)
}
func (m *mockDepartmentRepo) Update(ctx context.Context, id string, data domain.DepartmentPatch) (*domain.Department, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockDepartmentRepo) Delete(ctx context.Context, id string, deletedBy string) (*domain.Department, error) {
	return m.DeleteFn(ctx, id, deletedBy)
}
func (m *mockDepartmentRepo) Restore(ctx context.Context, id string) (*domain.Department, error) {
	return m.RestoreFn(ctx, id)
}

// --- Mock TenantRepository ---
type mockTenantRepo struct {
	CreateFn      func(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error)
	FindAllFn     func(ctx context.Context, filter ports.TenantFilter) ([]domain.Tenant, error)
	FindByIDFn    func(ctx context.Context, id string) (*domain.Tenant, error)
	FindAnyByIDFn func(ctx context.Context, id string) (*domain.Tenant, error)
	HasMembersFn  func(ctx context.Context, id string) (bool, error)
	UpdateFn      func(ctx context.Context, id string, data domain.TenantPatch) (*domain.Tenant, error)
	SaveLabelsFn  func(ctx context.Context, id string, labels domain.Labels) error
	DeleteFn      func(ctx context.Context, id string, deletedBy string) (*domain.Tenant, error)
	RestoreFn     func(ctx context.Context, id string) (*domain.Tenant, error)
}

func (m *mockTenantRepo) Create(ctx context.Context, t *domain.Tenant) (*domain.Tenant, error) {
	return m.CreateFn(ctx, t)
}
func (m *mockTenantRepo) FindAll(ctx context.Context, filter ports.TenantFilter) ([]domain.Tenant, error) {
	return m.FindAllFn(ctx, filter)
}
func (m *mockTenantRepo) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockTenantRepo) FindAnyByID(ctx context.Context, id string) (*domain.Tenant, error) {
	return m.FindAnyByIDFn(ctx, id)
}
func (m *mockTenantRepo) HasMembers(ctx context.Context, id string) (bool, error) {
	if m.HasMembersFn == nil {
		return false, nil
	}
	return m.HasMembersFn(ctx, id)
}
func (m *mockTenantRepo) Update(ctx context.Context, id string, data domain.TenantPatch) (*domain.Tenant, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockTenantRepo) SaveLabels(ctx context.Context, id string, labels domain.Labels) error {
	return m.SaveLabelsFn(ctx, id, labels)
}
func (m *mockTenantRepo) Delete(ctx context.Context, id string, deletedBy string) (*domain.Tenant, error) {
	return m.DeleteFn(ctx, id, deletedBy)
}
func (m *mockTenantRepo) Restore(ctx context.Context, id string) (*domain.Tenant, error) {
	return m.RestoreFn(ctx, id)
}

// --- Mock CachePort ---
type mockCache struct {
	// stored keeps exactly what each Set received, so tests can inspect it.
	stored  map[string]any
	ttls    map[string]time.Duration
	deleted []string
}

func newMockCache() *mockCache {
	return &mockCache{stored: map[string]any{}, ttls: map[string]time.Duration{}}
}

func (m *mockCache) Set(_ context.Context, key string, v any, ttl time.Duration) error {
	m.stored[key] = v
	m.ttls[key] = ttl
	return nil
}

// Get round-trips through JSON, as the Redis adapter does, so a hit returns a
// copy rather than the stored value itself.
func (m *mockCache) Get(_ context.Context, key string, dest any) (bool, error) {
	v, ok := m.stored[key]
	if !ok {
		return false, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(data, dest)
}

func (m *mockCache) Del(_ context.Context, prefix string) error {
	m.deleted = append(m.deleted, prefix)
	for key := range m.stored {
		if strings.HasPrefix(key, prefix) {
			delete(m.stored, key)
		}
	}
	return nil
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

func (m *mockUpload) Download(context.Context, string, string) (*domain.File, error) {
	return nil, nil
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
	RestoreFn        func(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
	UsernameTakenFn  func(ctx context.Context, username string) (bool, error)
	RecordLoginFn    func(ctx context.Context, id string) error
	AccessReportFn   func(ctx context.Context, caller *domain.Actor) ([]domain.User, error)
}

func (m *mockUserService) RecordLogin(ctx context.Context, id string) error {
	if m.RecordLoginFn == nil {
		return nil
	}
	return m.RecordLoginFn(ctx, id)
}
func (m *mockUserService) AccessReport(ctx context.Context, caller *domain.Actor) ([]domain.User, error) {
	return m.AccessReportFn(ctx, caller)
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
func (m *mockUserService) Restore(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error) {
	return m.RestoreFn(ctx, id, caller)
}
func (m *mockUserService) UsernameTaken(ctx context.Context, username string) (bool, error) {
	if m.UsernameTakenFn == nil {
		return false, nil
	}
	return m.UsernameTakenFn(ctx, username)
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
	// refreshClaims, when set, is returned as-is instead of one built from refreshUserID.
	refreshClaims *ports.RefreshClaims
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
func (f *fakeTokens) VerifyRefresh(string) (*ports.RefreshClaims, error) {
	if f.refreshErr != nil {
		return nil, f.refreshErr
	}
	if f.refreshClaims != nil {
		return f.refreshClaims, nil
	}
	return &ports.RefreshClaims{UserID: f.refreshUserID, TokenID: "jti-1", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}, nil
}

// --- Fake SessionStore ---

type fakeSessions struct {
	failures   map[string]int
	revoked    map[string]bool
	validAfter map[string]time.Time
	err        error
}

func newFakeSessions() *fakeSessions {
	return &fakeSessions{failures: map[string]int{}, revoked: map[string]bool{}, validAfter: map[string]time.Time{}}
}

func (f *fakeSessions) RecordLoginFailure(_ context.Context, key string, _ time.Duration) (int, error) {
	f.failures[key]++
	return f.failures[key], f.err
}
func (f *fakeSessions) LoginFailures(_ context.Context, key string) (int, error) {
	return f.failures[key], f.err
}
func (f *fakeSessions) ClearLoginFailures(_ context.Context, key string) error {
	delete(f.failures, key)
	return f.err
}
func (f *fakeSessions) RevokeToken(_ context.Context, id string, _ time.Duration) error {
	f.revoked[id] = true
	return f.err
}
func (f *fakeSessions) IsTokenRevoked(_ context.Context, id string) (bool, error) {
	return f.revoked[id], f.err
}
func (f *fakeSessions) SetTokensValidAfter(_ context.Context, id string, at time.Time, _ time.Duration) error {
	f.validAfter[id] = at
	return f.err
}
func (f *fakeSessions) TokensValidAfter(_ context.Context, id string) (time.Time, error) {
	return f.validAfter[id], f.err
}

// recordedAudit collects the events an audit mock was given.
type recordedAudit struct {
	*mockAuditService
	actions []domain.AuditAction
}

func newRecordedAudit() *recordedAudit {
	r := &recordedAudit{}
	r.mockAuditService = &mockAuditService{RecordFn: func(_ context.Context, _ domain.AuditEntity, a domain.AuditAction, _ string, _ *domain.Actor, _ any) {
		r.actions = append(r.actions, a)
	}}
	return r
}

func ptr[T any](v T) *T { return &v }
