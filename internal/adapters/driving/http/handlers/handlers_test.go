package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/dto"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

// ── Mock services ──────────────────────────────────────────────────

type mockAuthService struct {
	RegisterFn      func(ctx context.Context, data *domain.User, file *multipart.FileHeader) (*domain.User, error)
	ValidateUserFn  func(ctx context.Context, username string, password string) (*domain.User, error)
	LoginFn         func(ctx context.Context, user *domain.User) (*ports.LoginResult, error)
	RefreshTokensFn func(token string) (*ports.RefreshResult, error)
	ResetPasswordFn func(ctx context.Context, username string, password string, oldPassword string, caller *ports.JwtPayload) (string, error)
}

func (m *mockAuthService) Register(ctx context.Context, data *domain.User, file *multipart.FileHeader) (*domain.User, error) {
	return m.RegisterFn(ctx, data, file)
}
func (m *mockAuthService) ValidateUser(ctx context.Context, username string, password string) (*domain.User, error) {
	return m.ValidateUserFn(ctx, username, password)
}
func (m *mockAuthService) Login(ctx context.Context, user *domain.User) (*ports.LoginResult, error) {
	return m.LoginFn(ctx, user)
}
func (m *mockAuthService) RefreshTokens(token string) (*ports.RefreshResult, error) {
	return m.RefreshTokensFn(token)
}
func (m *mockAuthService) ResetPassword(ctx context.Context, username string, password string, oldPassword string, caller *ports.JwtPayload) (string, error) {
	return m.ResetPasswordFn(ctx, username, password, oldPassword, caller)
}

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

type mockHospitalService struct {
	CreateFn  func(ctx context.Context, hospital *domain.Hospital, file *multipart.FileHeader) (*domain.Hospital, error)
	FindAllFn func(ctx context.Context, caller *ports.JwtPayload) ([]domain.Hospital, error)
	FindOneFn func(ctx context.Context, id string) (*domain.Hospital, error)
	UpdateFn  func(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader) (*domain.Hospital, error)
	RemoveFn  func(ctx context.Context, id string) (*domain.Hospital, error)
}

func (m *mockHospitalService) Create(ctx context.Context, hospital *domain.Hospital, file *multipart.FileHeader) (*domain.Hospital, error) {
	return m.CreateFn(ctx, hospital, file)
}
func (m *mockHospitalService) FindAll(ctx context.Context, caller *ports.JwtPayload) ([]domain.Hospital, error) {
	return m.FindAllFn(ctx, caller)
}
func (m *mockHospitalService) FindOne(ctx context.Context, id string) (*domain.Hospital, error) {
	return m.FindOneFn(ctx, id)
}
func (m *mockHospitalService) Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader) (*domain.Hospital, error) {
	return m.UpdateFn(ctx, id, data, file)
}
func (m *mockHospitalService) Remove(ctx context.Context, id string) (*domain.Hospital, error) {
	return m.RemoveFn(ctx, id)
}

type mockWardService struct {
	CreateFn  func(ctx context.Context, ward *domain.Ward) (*domain.Ward, error)
	FindAllFn func(ctx context.Context, caller *ports.JwtPayload) ([]domain.Ward, error)
	FindOneFn func(ctx context.Context, id string) (*domain.Ward, error)
	UpdateFn  func(ctx context.Context, id string, data map[string]any) (*domain.Ward, error)
	RemoveFn  func(ctx context.Context, id string) (string, error)
}

func (m *mockWardService) Create(ctx context.Context, ward *domain.Ward) (*domain.Ward, error) {
	return m.CreateFn(ctx, ward)
}
func (m *mockWardService) FindAll(ctx context.Context, caller *ports.JwtPayload) ([]domain.Ward, error) {
	return m.FindAllFn(ctx, caller)
}
func (m *mockWardService) FindOne(ctx context.Context, id string) (*domain.Ward, error) {
	return m.FindOneFn(ctx, id)
}
func (m *mockWardService) Update(ctx context.Context, id string, data map[string]any) (*domain.Ward, error) {
	return m.UpdateFn(ctx, id, data)
}
func (m *mockWardService) Remove(ctx context.Context, id string) (string, error) {
	return m.RemoveFn(ctx, id)
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

// injectCaller adds a JwtPayload to Locals via middleware
func injectCaller(payload *ports.JwtPayload) fiber.Handler {
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
		RefreshTokensFn: func(token string) (*ports.RefreshResult, error) {
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
		RefreshTokensFn: func(_ string) (*ports.RefreshResult, error) {
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
		ResetPasswordFn: func(_ context.Context, _, _, _ string, _ *ports.JwtPayload) (string, error) {
			return "password reset", nil
		},
	}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Patch("/reset/:id", injectCaller(&ports.JwtPayload{ID: "U1", Role: "SUPER"}), h.ResetPassword)

	req := httptest.NewRequest(http.MethodPatch, "/reset/U2",
		jsonBody(t, map[string]string{"password": "newpw"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthHandler_ResetPassword_Error(t *testing.T) {
	svc := &mockAuthService{
		ResetPasswordFn: func(_ context.Context, _, _, _ string, _ *ports.JwtPayload) (string, error) {
			return "", errors.New("old password wrong")
		},
	}
	app := fiber.New()
	h := NewAuthHandler(svc)
	app.Patch("/reset/:id", injectCaller(&ports.JwtPayload{ID: "U1", Role: "USER"}), h.ResetPassword)

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
		CreateFn: func(_ context.Context, u *domain.User) (*domain.User, error) {
			u.ID = "U-NEW"
			return u, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Post("/users", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/users",
		jsonBody(t, map[string]string{"username": "alice", "password": "pass123"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestUserHandler_Create_Error(t *testing.T) {
	svc := &mockUserService{
		CreateFn: func(_ context.Context, _ *domain.User) (*domain.User, error) {
			return nil, errors.New("duplicate username")
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Post("/users", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/users",
		jsonBody(t, map[string]string{"username": "alice", "password": "pass123"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUserHandler_FindAll_Success(t *testing.T) {
	svc := &mockUserService{
		FindAllFn: func(_ context.Context, _ *ports.JwtPayload) ([]domain.User, error) {
			return []domain.User{{ID: "U1"}, {ID: "U2"}}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Get("/users", injectCaller(&ports.JwtPayload{ID: "U1", Role: "ADMIN"}), h.FindAll)

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
		FindOneFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "alice"}, nil
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Get("/users/:id", h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/users/U1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUserHandler_FindOne_NotFound(t *testing.T) {
	svc := &mockUserService{
		FindOneFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, errors.New("not found")
		},
	}
	app := fiber.New()
	h := NewUserHandler(svc)
	app.Get("/users/:id", h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/users/NOPE", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUserHandler_Remove_Success(t *testing.T) {
	svc := &mockUserService{
		RemoveFn: func(_ context.Context, id string) (*domain.User, error) {
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

// ── Hospital Handler Tests ─────────────────────────────────────────

func TestHospitalHandler_FindAll_Success(t *testing.T) {
	svc := &mockHospitalService{
		FindAllFn: func(_ context.Context, _ *ports.JwtPayload) ([]domain.Hospital, error) {
			return []domain.Hospital{{ID: "H1"}}, nil
		},
	}
	app := fiber.New()
	h := NewHospitalHandler(svc)
	app.Get("/hospitals", injectCaller(&ports.JwtPayload{ID: "U1", Role: "ADMIN"}), h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/hospitals", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHospitalHandler_FindAll_NoCaller(t *testing.T) {
	svc := &mockHospitalService{}
	app := fiber.New()
	h := NewHospitalHandler(svc)
	app.Get("/hospitals", h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/hospitals", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHospitalHandler_FindOne_Success(t *testing.T) {
	svc := &mockHospitalService{
		FindOneFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewHospitalHandler(svc)
	app.Get("/hospitals/:id", h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/hospitals/H1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHospitalHandler_FindOne_NotFound(t *testing.T) {
	svc := &mockHospitalService{
		FindOneFn: func(_ context.Context, _ string) (*domain.Hospital, error) {
			return nil, errors.New("not found")
		},
	}
	app := fiber.New()
	h := NewHospitalHandler(svc)
	app.Get("/hospitals/:id", h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/hospitals/NOPE", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHospitalHandler_Remove_Success(t *testing.T) {
	svc := &mockHospitalService{
		RemoveFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewHospitalHandler(svc)
	app.Delete("/hospitals/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/hospitals/H1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHospitalHandler_Remove_Error(t *testing.T) {
	svc := &mockHospitalService{
		RemoveFn: func(_ context.Context, _ string) (*domain.Hospital, error) {
			return nil, errors.New("remove failed")
		},
	}
	app := fiber.New()
	h := NewHospitalHandler(svc)
	app.Delete("/hospitals/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/hospitals/H1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── Ward Handler Tests ─────────────────────────────────────────────

func TestWardHandler_Create_Success(t *testing.T) {
	svc := &mockWardService{
		CreateFn: func(_ context.Context, w *domain.Ward) (*domain.Ward, error) {
			w.ID = "W-NEW"
			return w, nil
		},
	}
	app := fiber.New()
	h := NewWardHandler(svc)
	app.Post("/wards", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/wards",
		jsonBody(t, map[string]string{"wardName": "ICU", "hosId": "H1"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestWardHandler_Create_Error(t *testing.T) {
	svc := &mockWardService{
		CreateFn: func(_ context.Context, _ *domain.Ward) (*domain.Ward, error) {
			return nil, errors.New("duplicate ward")
		},
	}
	app := fiber.New()
	h := NewWardHandler(svc)
	app.Post("/wards", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/wards",
		jsonBody(t, map[string]string{"wardName": "ICU", "hosId": "H1"}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWardHandler_FindAll_Success(t *testing.T) {
	svc := &mockWardService{
		FindAllFn: func(_ context.Context, _ *ports.JwtPayload) ([]domain.Ward, error) {
			return []domain.Ward{{ID: "W1"}, {ID: "W2"}}, nil
		},
	}
	app := fiber.New()
	h := NewWardHandler(svc)
	app.Get("/wards", injectCaller(&ports.JwtPayload{ID: "U1", Role: "ADMIN", HosID: "H1"}), h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/wards", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWardHandler_FindAll_NoCaller(t *testing.T) {
	svc := &mockWardService{}
	app := fiber.New()
	h := NewWardHandler(svc)
	app.Get("/wards", h.FindAll)

	req := httptest.NewRequest(http.MethodGet, "/wards", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWardHandler_FindOne_Success(t *testing.T) {
	svc := &mockWardService{
		FindOneFn: func(_ context.Context, id string) (*domain.Ward, error) {
			return &domain.Ward{ID: id}, nil
		},
	}
	app := fiber.New()
	h := NewWardHandler(svc)
	app.Get("/wards/:id", h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/wards/W1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWardHandler_FindOne_NotFound(t *testing.T) {
	svc := &mockWardService{
		FindOneFn: func(_ context.Context, _ string) (*domain.Ward, error) {
			return nil, errors.New("not found")
		},
	}
	app := fiber.New()
	h := NewWardHandler(svc)
	app.Get("/wards/:id", h.FindOne)

	req := httptest.NewRequest(http.MethodGet, "/wards/NOPE", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestWardHandler_Remove_Success(t *testing.T) {
	svc := &mockWardService{
		RemoveFn: func(_ context.Context, id string) (string, error) {
			return "deleted " + id, nil
		},
	}
	app := fiber.New()
	h := NewWardHandler(svc)
	app.Delete("/wards/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/wards/W1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWardHandler_Remove_Error(t *testing.T) {
	svc := &mockWardService{
		RemoveFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("cannot delete")
		},
	}
	app := fiber.New()
	h := NewWardHandler(svc)
	app.Delete("/wards/:id", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/wards/W1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── getCaller helper Tests ─────────────────────────────────────────

func TestGetCaller_Valid(t *testing.T) {
	app := fiber.New()
	var result *ports.JwtPayload
	app.Get("/test", injectCaller(&ports.JwtPayload{ID: "U1", Role: "ADMIN"}), func(c fiber.Ctx) error {
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
	var result *ports.JwtPayload
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
	var result *ports.JwtPayload
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
