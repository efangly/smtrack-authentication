package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/adapters/driven/security"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/handlers"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/validator"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-jwt-secret"

// recorder notes whether any service call got past the router.
type recorder struct{ called bool }

type stubUserService struct{ rec *recorder }

func (s stubUserService) Create(context.Context, *domain.User, *domain.Actor) (*domain.User, error) {
	s.rec.called = true
	return &domain.User{}, nil
}
func (s stubUserService) FindAll(context.Context, *domain.Actor) ([]domain.User, error) {
	s.rec.called = true
	return nil, nil
}
func (s stubUserService) FindOne(context.Context, string, *domain.Actor) (*domain.User, error) {
	s.rec.called = true
	return &domain.User{}, nil
}
func (s stubUserService) FindByID(context.Context, string) (*domain.User, error) { return nil, nil }
func (s stubUserService) FindByUsername(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (s stubUserService) UsernameTaken(context.Context, string) (bool, error) { return false, nil }
func (s stubUserService) RecordLogin(context.Context, string) error           { return nil }
func (s stubUserService) AccessReport(context.Context, *domain.Actor) ([]domain.User, error) {
	s.rec.called = true
	return nil, nil
}
func (s stubUserService) Update(context.Context, string, domain.UserPatch, *domain.File, *domain.Actor) (*domain.User, error) {
	s.rec.called = true
	return &domain.User{}, nil
}
func (s stubUserService) Remove(context.Context, string, *domain.Actor) (*domain.User, error) {
	s.rec.called = true
	return &domain.User{}, nil
}
func (s stubUserService) Restore(context.Context, string, *domain.Actor) (*domain.User, error) {
	s.rec.called = true
	return &domain.User{}, nil
}

type stubTenantService struct{ rec *recorder }

func (s stubTenantService) Create(context.Context, *domain.Tenant, *domain.File, *domain.Actor) (*domain.Tenant, error) {
	s.rec.called = true
	return &domain.Tenant{}, nil
}
func (s stubTenantService) FindAll(context.Context, *domain.Actor) ([]domain.Tenant, error) {
	s.rec.called = true
	return nil, nil
}
func (s stubTenantService) FindOne(context.Context, string, *domain.Actor) (*domain.Tenant, error) {
	s.rec.called = true
	return &domain.Tenant{}, nil
}
func (s stubTenantService) Update(context.Context, string, domain.TenantPatch, *domain.File, *domain.Actor) (*domain.Tenant, error) {
	s.rec.called = true
	return &domain.Tenant{}, nil
}
func (s stubTenantService) UpdateLabels(context.Context, string, domain.Labels, *domain.Actor) (*domain.Tenant, error) {
	s.rec.called = true
	return &domain.Tenant{}, nil
}
func (s stubTenantService) Remove(context.Context, string, *domain.Actor) (*domain.Tenant, error) {
	s.rec.called = true
	return &domain.Tenant{}, nil
}
func (s stubTenantService) Restore(context.Context, string, *domain.Actor) (*domain.Tenant, error) {
	s.rec.called = true
	return &domain.Tenant{}, nil
}

type stubDepartmentService struct{ rec *recorder }

func (s stubDepartmentService) Create(context.Context, *domain.Department, *domain.Actor) (*domain.Department, error) {
	s.rec.called = true
	return &domain.Department{}, nil
}
func (s stubDepartmentService) FindAll(context.Context, *domain.Actor) ([]domain.Department, error) {
	s.rec.called = true
	return nil, nil
}
func (s stubDepartmentService) FindOne(context.Context, string, *domain.Actor) (*domain.Department, error) {
	s.rec.called = true
	return &domain.Department{}, nil
}
func (s stubDepartmentService) Update(context.Context, string, domain.DepartmentPatch, *domain.Actor) (*domain.Department, error) {
	s.rec.called = true
	return &domain.Department{}, nil
}
func (s stubDepartmentService) Remove(context.Context, string, *domain.Actor) (*domain.Department, error) {
	s.rec.called = true
	return &domain.Department{}, nil
}
func (s stubDepartmentService) Restore(context.Context, string, *domain.Actor) (*domain.Department, error) {
	s.rec.called = true
	return &domain.Department{}, nil
}

// stubAuditService records whether a listing got past the router.
type stubAuditService struct{ listed bool }

func (s *stubAuditService) Record(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *domain.Actor, changes any) {
}
func (s *stubAuditService) FindAll(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
	s.listed = true
	return nil, nil
}

// newTestRouter wires the real route table with stub services that record
// whether a request got past the router.
func newTestRouter(rec *recorder) *fiber.App {
	return newTestRouterCfg(testConfig(), rec, &stubAuditService{})
}

func newTestRouterWith(rec *recorder, auditSvc ports.AuditService) *fiber.App {
	return newTestRouterCfg(testConfig(), rec, auditSvc)
}

func testConfig() *config.Config {
	return &config.Config{JWTSecret: testJWTSecret, JWTRefreshSecret: "test-refresh-secret"}
}

func newTestRouterCfg(cfg *config.Config, rec *recorder, auditSvc ports.AuditService) *fiber.App {
	// Same validator as main.go, so invalid bodies stop at the handler.
	app := fiber.New(fiber.Config{StructValidator: validator.Validator{}})
	NewRouter(app, cfg, security.NewJWTIssuer(cfg),
		handlers.NewAuthHandler(nil),
		handlers.NewUserHandler(stubUserService{rec}),
		handlers.NewTenantHandler(stubTenantService{rec}),
		handlers.NewDepartmentHandler(stubDepartmentService{rec}),
		handlers.NewAuditHandler(auditSvc),
		handlers.NewHealthHandler(),
		handlers.NewImageHandler(nil),
	).Setup()
	return app
}

func tokenFor(t *testing.T, role domain.Role) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":            "U1",
		"name":          "Test User",
		"role":          string(role),
		"tenantId":      "T1",
		"departmentIds": []string{"D1"},
		"typ":           "access",
		"exp":           time.Now().Add(time.Hour).Unix(),
		"iat":           time.Now().Unix(),
	})
	signed, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return signed
}

var allRoles = []domain.Role{
	domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin,
	domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest,
}

// Every route that changes state carries a RolesGuard. A mutating route added
// without one let any token — even a self-registered GUEST — rename or delete
// other people's records once; list each new route here.
func TestRouter_MutatingRoutes_RequireTheirRoles(t *testing.T) {
	system := []domain.Role{domain.RoleSuper, domain.RoleService}
	tenantAdmin := []domain.Role{domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin}
	routes := []struct {
		method, path, body string
		allowed            []domain.Role
	}{
		{http.MethodPost, "/auth/user", `{"tenantId":"T1","username":"u","password":"pass1"}`, tenantAdmin},
		{http.MethodDelete, "/auth/user/U2", "", tenantAdmin},
		{http.MethodPost, "/auth/user/U2/restore", "", tenantAdmin},
		{http.MethodGet, "/auth/report/access", "", tenantAdmin},
		{http.MethodPost, "/auth/tenant", `{"name":"T"}`, system},
		{http.MethodPut, "/auth/tenant/T1", `{"name":"renamed"}`, system},
		{http.MethodPut, "/auth/tenant/T1/labels", `{"labels":{"tenant":{"en":"Plant"}}}`, tenantAdmin},
		{http.MethodDelete, "/auth/tenant/T1", "", []domain.Role{domain.RoleSuper}},
		{http.MethodPost, "/auth/tenant/T1/restore", "", []domain.Role{domain.RoleSuper}},
		{http.MethodPost, "/auth/department", `{"name":"D","tenantId":"T1"}`, system},
		{http.MethodPut, "/auth/department/D1", `{"name":"renamed"}`, system},
		{http.MethodDelete, "/auth/department/D1", "", system},
		{http.MethodPost, "/auth/department/D1/restore", "", system},
	}

	for _, rt := range routes {
		for _, role := range allRoles {
			t.Run(rt.method+" "+rt.path+"/"+string(role), func(t *testing.T) {
				rec := &recorder{}
				app := newTestRouter(rec)

				req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+tokenFor(t, role))
				resp, err := app.Test(req)
				require.NoError(t, err)

				if slices.Contains(rt.allowed, role) {
					assert.NotEqual(t, fiber.StatusForbidden, resp.StatusCode)
					assert.True(t, rec.called, "service should be called")
				} else {
					assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
					assert.False(t, rec.called, "service must not be called")
				}
			})
		}
	}
}

func TestRouter_MutatingRoutes_RequireAToken(t *testing.T) {
	rec := &recorder{}
	app := newTestRouter(rec)
	for _, path := range []string{"/auth/tenant/T1/labels", "/auth/department/D1", "/auth/user/U2/restore"} {
		req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, path)
	}
	assert.False(t, rec.called)
}

// Audit logs span every tenant, so tenant-scoped admins must not read them.
func TestRouter_AuditLogs_RequireSuperOrService(t *testing.T) {
	for _, role := range allRoles {
		t.Run(string(role), func(t *testing.T) {
			audit := &stubAuditService{}
			app := newTestRouterWith(&recorder{}, audit)

			req := httptest.NewRequest(http.MethodGet, "/auth/audit", nil)
			req.Header.Set("Authorization", "Bearer "+tokenFor(t, role))
			resp, err := app.Test(req)
			require.NoError(t, err)

			if role == domain.RoleSuper || role == domain.RoleService {
				assert.Equal(t, fiber.StatusOK, resp.StatusCode)
				assert.True(t, audit.listed)
			} else {
				assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
				assert.False(t, audit.listed, "service must not be called")
			}
		})
	}
}

// loginStatuses sends n login requests (an empty body fails validation before
// any service is reached, but still counts against the limiter) and returns
// the status codes. xff(i) supplies each request's X-Forwarded-For.
func loginStatuses(t *testing.T, app *fiber.App, n int, xff func(i int) string) []int {
	t.Helper()
	statuses := make([]int, n)
	for i := range n {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", xff(i))
		resp, err := app.Test(req)
		require.NoError(t, err)
		statuses[i] = resp.StatusCode
	}
	return statuses
}

// app.Test always reports the peer as 0.0.0.0, which stands in for KrakenD.
func TestRouter_LoginLimiter_KeysOnClientBehindTrustedProxy(t *testing.T) {
	cfg := testConfig()
	cfg.TrustedProxies = []string{"0.0.0.0/32"}
	app := newTestRouterCfg(cfg, &recorder{}, &stubAuditService{})

	// Eleven different clients behind the gateway each get their own bucket.
	statuses := loginStatuses(t, app, 11, func(i int) string { return fmt.Sprintf("203.0.113.%d", i+1) })
	assert.NotContains(t, statuses, fiber.StatusTooManyRequests)

	// One client is still limited to 10 per minute.
	statuses = loginStatuses(t, app, 11, func(int) string { return "198.51.100.7" })
	assert.Equal(t, fiber.StatusTooManyRequests, statuses[10])
	assert.NotContains(t, statuses[:10], fiber.StatusTooManyRequests)
}

func TestRouter_LoginLimiter_SpoofedLeftmostHopDoesNotEvade(t *testing.T) {
	cfg := testConfig()
	cfg.TrustedProxies = []string{"0.0.0.0/32"}
	app := newTestRouterCfg(cfg, &recorder{}, &stubAuditService{})

	// The client varies the part of the header it controls; the gateway
	// appends the real address on the right.
	statuses := loginStatuses(t, app, 11, func(i int) string {
		return fmt.Sprintf("10.9.9.%d, 198.51.100.7", i+1)
	})
	assert.Equal(t, fiber.StatusTooManyRequests, statuses[10])
}

func TestRouter_LoginLimiter_IgnoresHeaderWithoutTrustedProxies(t *testing.T) {
	app := newTestRouterCfg(testConfig(), &recorder{}, &stubAuditService{})

	statuses := loginStatuses(t, app, 11, func(i int) string { return fmt.Sprintf("203.0.113.%d", i+1) })
	assert.Equal(t, fiber.StatusTooManyRequests, statuses[10])
}
