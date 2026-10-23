package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/validator"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock services ──────────────────────────────────────────────────

type mockAuthService struct {
	RegisterFn      func(ctx context.Context, data *domain.User, file *domain.File) (*domain.User, error)
	ValidateUserFn  func(ctx context.Context, username string, password string) (*domain.User, error)
	LoginFn         func(ctx context.Context, user *domain.User) (*ports.LoginResult, error)
	RefreshTokensFn func(ctx context.Context, token string) (*ports.RefreshResult, error)
	ResetPasswordFn func(ctx context.Context, username string, password string, oldPassword string, caller *domain.Actor) (string, error)
	LogoutFn        func(ctx context.Context, refreshToken string, caller *domain.Actor) error
}

func (m *mockAuthService) Logout(ctx context.Context, refreshToken string, caller *domain.Actor) error {
	return m.LogoutFn(ctx, refreshToken, caller)
}

func (m *mockAuthService) Register(ctx context.Context, data *domain.User, file *domain.File) (*domain.User, error) {
	return m.RegisterFn(ctx, data, file)
}
func (m *mockAuthService) ValidateUser(ctx context.Context, username string, password string) (*domain.User, error) {
	return m.ValidateUserFn(ctx, username, password)
}
func (m *mockAuthService) Login(ctx context.Context, user *domain.User) (*ports.LoginResult, error) {
	return m.LoginFn(ctx, user)
}
func (m *mockAuthService) RefreshTokens(ctx context.Context, token string) (*ports.RefreshResult, error) {
	return m.RefreshTokensFn(ctx, token)
}
func (m *mockAuthService) ResetPassword(ctx context.Context, username string, password string, oldPassword string, caller *domain.Actor) (string, error) {
	return m.ResetPasswordFn(ctx, username, password, oldPassword, caller)
}

type mockUserService struct {
	CreateFn         func(ctx context.Context, user *domain.User, caller *domain.Actor) (*domain.User, error)
	FindAllFn        func(ctx context.Context, caller *domain.Actor) ([]domain.User, error)
	FindOneFn        func(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
	FindByIDFn       func(ctx context.Context, id string) (*domain.User, error)
	FindByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
	UpdateFn         func(ctx context.Context, id string, data domain.UserPatch, file *domain.File, caller *domain.Actor) (*domain.User, error)
	RemoveFn         func(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
	RestoreFn        func(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
	AccessReportFn   func(ctx context.Context, caller *domain.Actor) ([]domain.User, error)
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
func (m *mockUserService) UsernameTaken(context.Context, string) (bool, error) { return false, nil }
func (m *mockUserService) RecordLogin(context.Context, string) error           { return nil }
func (m *mockUserService) AccessReport(ctx context.Context, caller *domain.Actor) ([]domain.User, error) {
	return m.AccessReportFn(ctx, caller)
}

type mockTenantService struct {
	CreateFn       func(ctx context.Context, tenant *domain.Tenant, file *domain.File, caller *domain.Actor) (*domain.Tenant, error)
	FindAllFn      func(ctx context.Context, caller *domain.Actor) ([]domain.Tenant, error)
	FindOneFn      func(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error)
	UpdateFn       func(ctx context.Context, id string, data domain.TenantPatch, file *domain.File, caller *domain.Actor) (*domain.Tenant, error)
	UpdateLabelsFn func(ctx context.Context, id string, labels domain.Labels, caller *domain.Actor) (*domain.Tenant, error)
	RemoveFn       func(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error)
	RestoreFn      func(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error)
}

func (m *mockTenantService) Create(ctx context.Context, tenant *domain.Tenant, file *domain.File, caller *domain.Actor) (*domain.Tenant, error) {
	return m.CreateFn(ctx, tenant, file, caller)
}
func (m *mockTenantService) FindAll(ctx context.Context, caller *domain.Actor) ([]domain.Tenant, error) {
	return m.FindAllFn(ctx, caller)
}
func (m *mockTenantService) FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error) {
	return m.FindOneFn(ctx, id, caller)
}
func (m *mockTenantService) Update(ctx context.Context, id string, data domain.TenantPatch, file *domain.File, caller *domain.Actor) (*domain.Tenant, error) {
	return m.UpdateFn(ctx, id, data, file, caller)
}
func (m *mockTenantService) UpdateLabels(ctx context.Context, id string, labels domain.Labels, caller *domain.Actor) (*domain.Tenant, error) {
	return m.UpdateLabelsFn(ctx, id, labels, caller)
}
func (m *mockTenantService) Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error) {
	return m.RemoveFn(ctx, id, caller)
}
func (m *mockTenantService) Restore(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error) {
	return m.RestoreFn(ctx, id, caller)
}

type mockDepartmentService struct {
	CreateFn  func(ctx context.Context, department *domain.Department, caller *domain.Actor) (*domain.Department, error)
	FindAllFn func(ctx context.Context, caller *domain.Actor) ([]domain.Department, error)
	FindOneFn func(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error)
	UpdateFn  func(ctx context.Context, id string, data domain.DepartmentPatch, caller *domain.Actor) (*domain.Department, error)
	RemoveFn  func(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error)
	RestoreFn func(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error)
}

func (m *mockDepartmentService) Create(ctx context.Context, department *domain.Department, caller *domain.Actor) (*domain.Department, error) {
	return m.CreateFn(ctx, department, caller)
}
func (m *mockDepartmentService) FindAll(ctx context.Context, caller *domain.Actor) ([]domain.Department, error) {
	return m.FindAllFn(ctx, caller)
}
func (m *mockDepartmentService) FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error) {
	return m.FindOneFn(ctx, id, caller)
}
func (m *mockDepartmentService) Update(ctx context.Context, id string, data domain.DepartmentPatch, caller *domain.Actor) (*domain.Department, error) {
	return m.UpdateFn(ctx, id, data, caller)
}
func (m *mockDepartmentService) Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error) {
	return m.RemoveFn(ctx, id, caller)
}
func (m *mockDepartmentService) Restore(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error) {
	return m.RestoreFn(ctx, id, caller)
}

type mockAuditService struct {
	RecordFn  func(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *domain.Actor, changes any)
	FindAllFn func(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error)
}

func (m *mockAuditService) Record(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *domain.Actor, changes any) {
	if m.RecordFn != nil {
		m.RecordFn(ctx, entity, action, entityID, caller, changes)
	}
}
func (m *mockAuditService) FindAll(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
	return m.FindAllFn(ctx, filter)
}

// ── Helpers ────────────────────────────────────────────────────────

func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

func parseResponse(t *testing.T, resp *http.Response) dto.StandardResponse {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var sr dto.StandardResponse
	require.NoError(t, json.Unmarshal(body, &sr))
	return sr
}

func parseErrorResponse(t *testing.T, resp *http.Response) dto.ErrorResponse {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var er dto.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &er))
	return er
}

// injectCaller puts the authenticated actor in Locals, as JWTAuth does
func injectCaller(payload *domain.Actor) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("user", payload)
		return c.Next()
	}
}

// ── Health Handler Tests ───────────────────────────────────────────

func TestHealthHandler_Check(t *testing.T) {
	app := fiber.New()
	h := NewHealthHandler()
	app.Get("/health", h.Check)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "ok", result["status"])
}

// ── Auth Handler Tests ─────────────────────────────────────────────

func TestAuthHandler_Login_Success(t *testing.T) {
	svc := &mockAuthService{
		ValidateUserFn: func(_ context.Context, username, password string) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: username}, nil
		},
		LoginFn: func(_ context.Context, user *domain.User) (*ports.LoginResult, error) {
			return &ports.LoginResult{Token: "tok", RefreshToken: "ref", ID: user.ID}, nil
		},
	}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Post("/login", h.Login)

	req := httptest.NewRequest(http.MethodPost, "/login",
		jsonBody(t, map[string]string{"username": "admin", "password": "1234"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	sr := parseResponse(t, resp)
	assert.True(t, sr.Success)
}

func TestAuthHandler_Login_InvalidBody(t *testing.T) {
	svc := &mockAuthService{}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Post("/login", h.Login)

	req := httptest.NewRequest(http.MethodPost, "/login",
		bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_Login_WrongCredentials(t *testing.T) {
	svc := &mockAuthService{
		ValidateUserFn: func(_ context.Context, _, _ string) (*domain.User, error) {
			return nil, errors.New("invalid credentials")
		},
	}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Post("/login", h.Login)

	req := httptest.NewRequest(http.MethodPost, "/login",
		jsonBody(t, map[string]string{"username": "bad", "password": "wrong"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	er := parseErrorResponse(t, resp)
	assert.Equal(t, "invalid credentials", er.Message)
}

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	svc := &mockAuthService{
		RefreshTokensFn: func(_ context.Context, token string) (*ports.RefreshResult, error) {
			return &ports.RefreshResult{Token: "new-tok", RefreshToken: "new-ref"}, nil
		},
	}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Post("/refresh", h.RefreshToken)

	req := httptest.NewRequest(http.MethodPost, "/refresh",
		jsonBody(t, map[string]string{"token": "old-ref"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthHandler_RefreshToken_MissingToken(t *testing.T) {
	svc := &mockAuthService{}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Post("/refresh", h.RefreshToken)

	req := httptest.NewRequest(http.MethodPost, "/refresh",
		jsonBody(t, map[string]string{}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_RefreshToken_Invalid(t *testing.T) {
	svc := &mockAuthService{
		RefreshTokensFn: func(_ context.Context, _ string) (*ports.RefreshResult, error) {
			return nil, errors.New("invalid token")
		},
	}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Post("/refresh", h.RefreshToken)

	req := httptest.NewRequest(http.MethodPost, "/refresh",
		jsonBody(t, map[string]string{"token": "bad"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_ResetPassword_Success(t *testing.T) {
	svc := &mockAuthService{
		ResetPasswordFn: func(_ context.Context, _, _, _ string, _ *domain.Actor) (string, error) {
			return "password reset", nil
		},
	}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Patch("/reset/:id", injectCaller(&domain.Actor{ID: "U1", Role: "SUPER"}), h.ResetPassword)

	req := httptest.NewRequest(http.MethodPatch, "/reset/U2",
		jsonBody(t, map[string]string{"password": "newpw"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthHandler_ResetPassword_Error(t *testing.T) {
	svc := &mockAuthService{
		ResetPasswordFn: func(_ context.Context, _, _, _ string, _ *domain.Actor) (string, error) {
			return "", errors.New("old password wrong")
		},
	}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Patch("/reset/:id", injectCaller(&domain.Actor{ID: "U1", Role: "USER"}), h.ResetPassword)

	req := httptest.NewRequest(http.MethodPatch, "/reset/U1",
		jsonBody(t, map[string]string{"password": "new", "oldPassword": "wrong"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── User Handler Tests ─────────────────────────────────────────────

func TestUserHandler_Create_Success(t *testing.T) {
	svc := &mockUserService{
		CreateFn: func(_ context.Context, u *domain.User, _ *domain.Actor) (*domain.User, error) {
			u.ID = "U-NEW"
			return u, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Post("/users", injectCaller(&domain.Actor{ID: "U1", Role: "SUPER"}), h.Create)

	req := httptest.NewRequest(http.MethodPost, "/users",
		jsonBody(t, map[string]string{"username": "alice", "password": "pass123"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestUserHandler_Create_Error(t *testing.T) {
	svc := &mockUserService{
		CreateFn: func(_ context.Context, _ *domain.User, _ *domain.Actor) (*domain.User, error) {
			return nil, errors.New("duplicate username")
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Post("/users", injectCaller(&domain.Actor{ID: "U1", Role: "SUPER"}), h.Create)

	req := httptest.NewRequest(http.MethodPost, "/users",
		jsonBody(t, map[string]string{"username": "alice", "password": "pass123"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUserHandler_FindAll_Success(t *testing.T) {
	svc := &mockUserService{
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.User, error) {
			return []domain.User{{ID: "U1"}, {ID: "U2"}}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Get("/users", injectCaller(&domain.Actor{ID: "U1", Role: "ADMIN"}), h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUserHandler_FindAll_NoCaller(t *testing.T) {
	svc := &mockUserService{}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Get("/users", h.FindAll) // no injectCaller middleware

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUserHandler_FindOne_Success(t *testing.T) {
	svc := &mockUserService{
		FindOneFn: func(_ context.Context, id string, _ *domain.Actor) (*domain.User, error) {
			return &domain.User{ID: id, Username: "alice"}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Get("/users/:id", injectCaller(&domain.Actor{ID: "U1", Role: "GUEST"}), h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/users/U1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUserHandler_FindOne_NotFound(t *testing.T) {
	svc := &mockUserService{
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.User, error) {
			return nil, errors.New("not found")
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Get("/users/:id", injectCaller(&domain.Actor{ID: "S1", Role: "SUPER"}), h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/users/NOPE", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUserHandler_Remove_Success(t *testing.T) {
	svc := &mockUserService{
		RemoveFn: func(_ context.Context, id string, _ *domain.Actor) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Delete("/users/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/users/U1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ── Tenant Handler Tests ─────────────────────────────────────────

func TestTenantHandler_FindAll_Success(t *testing.T) {
	svc := &mockTenantService{
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.Tenant, error) {
			return []domain.Tenant{{ID: "T1"}}, nil
		},
	}
	app := fiber.New()
	h := NewTenantHandler(svc)
	app.Get("/tenants", injectCaller(&domain.Actor{ID: "U1", Role: "ADMIN"}), h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTenantHandler_FindAll_NoCaller(t *testing.T) {
	svc := &mockTenantService{}
	app := fiber.New()
	h := NewTenantHandler(svc)
	app.Get("/tenants", h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTenantHandler_FindOne_Success(t *testing.T) {
	svc := &mockTenantService{
		FindOneFn: func(_ context.Context, id string, _ *domain.Actor) (*domain.Tenant, error) {
			return &domain.Tenant{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewTenantHandler(svc)
	app.Get("/tenants/:id", injectCaller(&domain.Actor{ID: "S1", Role: "SUPER"}), h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/tenants/T1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTenantHandler_FindOne_NotFound(t *testing.T) {
	svc := &mockTenantService{
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Tenant, error) {
			return nil, errors.New("not found")
		},
	}
	app := fiber.New()
	h := NewTenantHandler(svc)
	app.Get("/tenants/:id", injectCaller(&domain.Actor{ID: "S1", Role: "SUPER"}), h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/tenants/NOPE", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTenantHandler_Remove_Success(t *testing.T) {
	svc := &mockTenantService{
		RemoveFn: func(_ context.Context, id string, _ *domain.Actor) (*domain.Tenant, error) {
			return &domain.Tenant{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewTenantHandler(svc)
	app.Delete("/tenants/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/tenants/T1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTenantHandler_Remove_Error(t *testing.T) {
	svc := &mockTenantService{
		RemoveFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Tenant, error) {
			return nil, errors.New("remove failed")
		},
	}
	app := fiber.New()
	h := NewTenantHandler(svc)
	app.Delete("/tenants/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/tenants/T1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── Department Handler Tests ─────────────────────────────────────────────

func TestDepartmentHandler_Create_Success(t *testing.T) {
	svc := &mockDepartmentService{
		CreateFn: func(_ context.Context, w *domain.Department, _ *domain.Actor) (*domain.Department, error) {
			w.ID = "W-NEW"
			return w, nil
		},
	}
	app := fiber.New()
	h := NewDepartmentHandler(svc)
	app.Post("/departments", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/departments",
		jsonBody(t, map[string]string{"name": "ICU", "tenantId": "T1"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestDepartmentHandler_Create_Error(t *testing.T) {
	svc := &mockDepartmentService{
		CreateFn: func(_ context.Context, _ *domain.Department, _ *domain.Actor) (*domain.Department, error) {
			return nil, errors.New("duplicate department")
		},
	}
	app := fiber.New()
	h := NewDepartmentHandler(svc)
	app.Post("/departments", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/departments",
		jsonBody(t, map[string]string{"name": "ICU", "tenantId": "T1"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestDepartmentHandler_FindAll_Success(t *testing.T) {
	svc := &mockDepartmentService{
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.Department, error) {
			return []domain.Department{{ID: "D1"}, {ID: "D2"}}, nil
		},
	}
	app := fiber.New()
	h := NewDepartmentHandler(svc)
	app.Get("/departments", injectCaller(&domain.Actor{ID: "U1", Role: "ADMIN", TenantID: "T1"}), h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/departments", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDepartmentHandler_FindAll_NoCaller(t *testing.T) {
	svc := &mockDepartmentService{}
	app := fiber.New()
	h := NewDepartmentHandler(svc)
	app.Get("/departments", h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/departments", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestDepartmentHandler_FindOne_Success(t *testing.T) {
	svc := &mockDepartmentService{
		FindOneFn: func(_ context.Context, id string, _ *domain.Actor) (*domain.Department, error) {
			return &domain.Department{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewDepartmentHandler(svc)
	app.Get("/departments/:id", injectCaller(&domain.Actor{ID: "S1", Role: "SUPER"}), h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/departments/D1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDepartmentHandler_FindOne_NotFound(t *testing.T) {
	svc := &mockDepartmentService{
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Department, error) {
			return nil, errors.New("not found")
		},
	}
	app := fiber.New()
	h := NewDepartmentHandler(svc)
	app.Get("/departments/:id", injectCaller(&domain.Actor{ID: "S1", Role: "SUPER"}), h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/departments/NOPE", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDepartmentHandler_Remove_Success(t *testing.T) {
	svc := &mockDepartmentService{
		RemoveFn: func(_ context.Context, id string, _ *domain.Actor) (*domain.Department, error) {
			return &domain.Department{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewDepartmentHandler(svc)
	app.Delete("/departments/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/departments/D1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDepartmentHandler_Remove_Error(t *testing.T) {
	svc := &mockDepartmentService{
		RemoveFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Department, error) {
			return nil, errors.New("cannot delete")
		},
	}
	app := fiber.New()
	h := NewDepartmentHandler(svc)
	app.Delete("/departments/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/departments/D1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── getCaller helper Tests ─────────────────────────────────────────

func TestGetCaller_Valid(t *testing.T) {
	app := fiber.New()
	var result *domain.Actor
	app.Get("/test", injectCaller(&domain.Actor{ID: "U1", Role: "ADMIN"}), func(c fiber.Ctx) error {
		result = getCaller(c)
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "U1", result.ID)
}

func TestGetCaller_NilLocals(t *testing.T) {
	app := fiber.New()
	var result *domain.Actor
	app.Get("/test", func(c fiber.Ctx) error {
		result = getCaller(c)
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestGetCaller_WrongType(t *testing.T) {
	app := fiber.New()
	var result *domain.Actor
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user", "not-a-payload")
		result = getCaller(c)
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	assert.Nil(t, result)
}

// --- Authorization on user routes ---

func TestUserHandler_FindOne_ForbiddenForOtherUser(t *testing.T) {
	svc := &mockUserService{
		FindOneFn: func(_ context.Context, id string, _ *domain.Actor) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Get("/users/:id", injectCaller(&domain.Actor{ID: "GUEST1", Role: "GUEST"}), h.FindOne)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/users/SOMEONE_ELSE", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUserHandler_Update_ForbiddenForOtherUser(t *testing.T) {
	called := false
	svc := &mockUserService{
		UpdateFn: func(_ context.Context, _ string, _ domain.UserPatch, _ *domain.File, _ *domain.Actor) (*domain.User, error) {
			called = true
			return &domain.User{}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Put("/users/:id", injectCaller(&domain.Actor{ID: "GUEST1", Role: "GUEST"}), h.Update)

	req := httptest.NewRequest(http.MethodPut, "/users/VICTIM", strings.NewReader(`{"display":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.False(t, called, "service must not be reached")
}

func TestUserHandler_Update_NonAdminCannotEscalateRole(t *testing.T) {
	var received domain.UserPatch
	svc := &mockUserService{
		UpdateFn: func(_ context.Context, _ string, data domain.UserPatch, _ *domain.File, _ *domain.Actor) (*domain.User, error) {
			received = data
			return &domain.User{ID: "U1"}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Put("/users/:id", injectCaller(&domain.Actor{ID: "U1", Role: "GUEST"}), h.Update)

	body := `{"role":"SUPER","status":true,"tenantId":"T9","departmentIds":["D9"],"display":"New Name"}`
	req := httptest.NewRequest(http.MethodPut, "/users/U1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Nil(t, received.Role)
	assert.Nil(t, received.Status)
	assert.Nil(t, received.TenantID)
	assert.Nil(t, received.DepartmentIDs)
	require.NotNil(t, received.Display, "non-privileged fields still apply")
	assert.Equal(t, "New Name", *received.Display)
}

func TestUserHandler_Update_AdminKeepsPrivilegedFields(t *testing.T) {
	var received domain.UserPatch
	svc := &mockUserService{
		UpdateFn: func(_ context.Context, _ string, data domain.UserPatch, _ *domain.File, _ *domain.Actor) (*domain.User, error) {
			received = data
			return &domain.User{ID: "U2"}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Put("/users/:id", injectCaller(&domain.Actor{ID: "A1", Role: "ADMIN"}), h.Update)

	req := httptest.NewRequest(http.MethodPut, "/users/U2", strings.NewReader(`{"role":"USER"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, received.Role)
	assert.Equal(t, domain.RoleUser, *received.Role)
}

// ── Audit Handler Tests ────────────────────────────────────────────

func TestAuditHandler_FindAll_Success(t *testing.T) {
	svc := &mockAuditService{
		FindAllFn: func(_ context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
			assert.Equal(t, "USER", filter.Entity)
			return []domain.AuditLog{{ID: "A1", Entity: domain.AuditEntityUser}}, nil
		},
	}
	app := fiber.New()
	h := NewAuditHandler(svc)
	app.Get("/audit", injectCaller(&domain.Actor{ID: "U1", Role: "SUPER"}), h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/audit?entity=USER", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuditHandler_FindAll_ServiceError(t *testing.T) {
	svc := &mockAuditService{
		FindAllFn: func(_ context.Context, _ ports.AuditFilter) ([]domain.AuditLog, error) {
			return nil, errors.New("query failed")
		},
	}
	app := fiber.New()
	h := NewAuditHandler(svc)
	app.Get("/audit", h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── Logout and access report ───────────────────────────────────────

func TestAuthHandler_Logout(t *testing.T) {
	var gotToken string
	var gotCaller *domain.Actor
	svc := &mockAuthService{LogoutFn: func(_ context.Context, token string, caller *domain.Actor) error {
		gotToken, gotCaller = token, caller
		return nil
	}}
	app := fiber.New(fiber.Config{StructValidator: validator.Validator{}})
	app.Post("/logout", injectCaller(&domain.Actor{ID: "U1"}), NewAuthHandler(svc).Logout)

	req := httptest.NewRequest(http.MethodPost, "/logout", jsonBody(t, map[string]string{"token": "refresh-1"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "refresh-1", gotToken)
	assert.Equal(t, "U1", gotCaller.ID)
}

func TestAuthHandler_Logout_MissingTokenOrCaller(t *testing.T) {
	svc := &mockAuthService{}
	app := fiber.New(fiber.Config{StructValidator: validator.Validator{}})
	app.Post("/logout", NewAuthHandler(svc).Logout)

	req := httptest.NewRequest(http.MethodPost, "/logout", jsonBody(t, map[string]string{}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	req = httptest.NewRequest(http.MethodPost, "/logout", jsonBody(t, map[string]string{"token": "t"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUserHandler_AccessReport(t *testing.T) {
	svc := &mockUserService{AccessReportFn: func(_ context.Context, caller *domain.Actor) ([]domain.User, error) {
		return []domain.User{{ID: "U1", Username: "alice", TenantID: caller.TenantID, Role: domain.RoleUser}}, nil
	}}
	app := fiber.New()
	app.Get("/report", injectCaller(&domain.Actor{ID: "A1", Role: domain.RoleAdmin, TenantID: "T1"}), NewUserHandler(svc).AccessReport)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/report", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `"lastLoginAt"`)
	assert.NotContains(t, string(body), "password")
}
