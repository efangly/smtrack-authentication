package services

import (
	"context"
	"mime/multipart"

	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

// --- Mock UserRepository ---
type mockUserRepo struct {
	CreateFn         func(ctx context.Context, user *domain.User) (*domain.User, error)
	FindAllFn        func(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.User, error)
	FindByIDFn       func(ctx context.Context, id string) (*domain.User, error)
	FindByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
	UpdateFn         func(ctx context.Context, id string, data map[string]any) (*domain.User, error)
	DeleteFn         func(ctx context.Context, id string) (*domain.User, error)
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	return m.CreateFn(ctx, user)
}
func (m *mockUserRepo) FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.User, error) {
	return m.FindAllFn(ctx, conditions, notConditions)
}
func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockUserRepo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	return m.FindByUsernameFn(ctx, username)
}
func (m *mockUserRepo) Update(ctx context.Context, id string, data map[string]any) (*domain.User, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockUserRepo) Delete(ctx context.Context, id string) (*domain.User, error) {
	return m.DeleteFn(ctx, id)
}

// --- Mock WardRepository ---
type mockWardRepo struct {
	CreateFn   func(ctx context.Context, ward *domain.Ward) (*domain.Ward, error)
	FindAllFn  func(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.Ward, error)
	FindByIDFn func(ctx context.Context, id string) (*domain.Ward, error)
	UpdateFn   func(ctx context.Context, id string, data map[string]any) (*domain.Ward, error)
	DeleteFn   func(ctx context.Context, id string) error
}

func (m *mockWardRepo) Create(ctx context.Context, ward *domain.Ward) (*domain.Ward, error) {
	return m.CreateFn(ctx, ward)
}
func (m *mockWardRepo) FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.Ward, error) {
	return m.FindAllFn(ctx, conditions, notConditions)
}
func (m *mockWardRepo) FindByID(ctx context.Context, id string) (*domain.Ward, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockWardRepo) Update(ctx context.Context, id string, data map[string]any) (*domain.Ward, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockWardRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFn(ctx, id)
}

// --- Mock HospitalRepository ---
type mockHospitalRepo struct {
	CreateFn   func(ctx context.Context, hospital *domain.Hospital) (*domain.Hospital, error)
	FindAllFn  func(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.Hospital, error)
	FindByIDFn func(ctx context.Context, id string) (*domain.Hospital, error)
	UpdateFn   func(ctx context.Context, id string, data map[string]any) (*domain.Hospital, error)
	DeleteFn   func(ctx context.Context, id string) (*domain.Hospital, error)
}

func (m *mockHospitalRepo) Create(ctx context.Context, hospital *domain.Hospital) (*domain.Hospital, error) {
	return m.CreateFn(ctx, hospital)
}
func (m *mockHospitalRepo) FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.Hospital, error) {
	return m.FindAllFn(ctx, conditions, notConditions)
}
func (m *mockHospitalRepo) FindByID(ctx context.Context, id string) (*domain.Hospital, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *mockHospitalRepo) Update(ctx context.Context, id string, data map[string]any) (*domain.Hospital, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockHospitalRepo) Delete(ctx context.Context, id string) (*domain.Hospital, error) {
	return m.DeleteFn(ctx, id)
}

// --- Mock CachePort ---
type mockCache struct {
	store  map[string]string
	SetFn  func(ctx context.Context, key string, value string, expireSeconds int) error
	GetFn  func(ctx context.Context, key string) (string, error)
	DelFn  func(ctx context.Context, pattern string) error
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
	UploadFn func(file *multipart.FileHeader, path string) (string, error)
	DeleteFn func(path string, filename string) error
}

func newMockUpload() *mockUpload {
	return &mockUpload{
		UploadFn: func(_ *multipart.FileHeader, _ string) (string, error) {
			return "http://example.com/pic.jpg", nil
		},
		DeleteFn: func(string, string) error { return nil },
	}
}

func (m *mockUpload) Upload(file *multipart.FileHeader, path string) (string, error) {
	return m.UploadFn(file, path)
}
func (m *mockUpload) Delete(path string, filename string) error {
	return m.DeleteFn(path, filename)
}

// --- Mock UserService (for auth_service tests) ---
type mockUserService struct {
	CreateFn         func(ctx context.Context, user *domain.User) (*domain.User, error)
	FindAllFn        func(ctx context.Context, caller *ports.JwtPayload) ([]domain.User, error)
	FindOneFn        func(ctx context.Context, id string) (*domain.User, error)
	FindByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
	UpdateFn         func(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader) (*domain.User, error)
	RemoveFn         func(ctx context.Context, id string) (*domain.User, error)
}

func (m *mockUserService) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	return m.CreateFn(ctx, user)
}
func (m *mockUserService) FindAll(ctx context.Context, caller *ports.JwtPayload) ([]domain.User, error) {
	return m.FindAllFn(ctx, caller)
}
func (m *mockUserService) FindOne(ctx context.Context, id string) (*domain.User, error) {
	return m.FindOneFn(ctx, id)
}
func (m *mockUserService) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	return m.FindByUsernameFn(ctx, username)
}
func (m *mockUserService) Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader) (*domain.User, error) {
	return m.UpdateFn(ctx, id, data, file)
}
func (m *mockUserService) Remove(ctx context.Context, id string) (*domain.User, error) {
	return m.RemoveFn(ctx, id)
}
