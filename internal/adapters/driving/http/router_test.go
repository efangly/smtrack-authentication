package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/config"
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

// stubWardService records whether a mutating call got past the router.
type stubWardService struct {
	updated bool
	removed bool
}

func (s *stubWardService) Create(ctx context.Context, ward *domain.Ward, caller *ports.JwtPayload) (*domain.Ward, error) {
	return ward, nil
}
func (s *stubWardService) FindAll(ctx context.Context, caller *ports.JwtPayload) ([]domain.Ward, error) {
	return nil, nil
}
func (s *stubWardService) FindOne(ctx context.Context, id string, caller *ports.JwtPayload) (*domain.Ward, error) {
	return &domain.Ward{ID: id}, nil
}
func (s *stubWardService) Update(ctx context.Context, id string, data map[string]any, caller *ports.JwtPayload) (*domain.Ward, error) {
	s.updated = true
	return &domain.Ward{ID: id}, nil
}
func (s *stubWardService) Remove(ctx context.Context, id string, caller *ports.JwtPayload) (string, error) {
	s.removed = true
	return "Ward deleted successfully", nil
}

// stubAuditService records whether a listing got past the router.
type stubAuditService struct{ listed bool }

func (s *stubAuditService) Record(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *ports.JwtPayload, changes any) {
}
func (s *stubAuditService) FindAll(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
	s.listed = true
	return nil, nil
}

// newTestRouter wires the real route table with only the ward service backed;
// the other handlers are never reached by these tests.
func newTestRouter(wardSvc ports.WardService) *fiber.App {
	return newTestRouterWith(wardSvc, &stubAuditService{})
}

func newTestRouterWith(wardSvc ports.WardService, auditSvc ports.AuditService) *fiber.App {
	return newTestRouterCfg(testConfig(), wardSvc, auditSvc)
}

func testConfig() *config.Config {
	return &config.Config{JWTSecret: testJWTSecret, JWTRefreshSecret: "test-refresh-secret"}
}

func newTestRouterCfg(cfg *config.Config, wardSvc ports.WardService, auditSvc ports.AuditService) *fiber.App {
	// Same validator as main.go, so invalid bodies stop at the handler.
	app := fiber.New(fiber.Config{StructValidator: validator.Validator{}})
	NewRouter(app, cfg,
		handlers.NewAuthHandler(nil),
		handlers.NewUserHandler(nil),
		handlers.NewHospitalHandler(nil),
		handlers.NewWardHandler(wardSvc),
		handlers.NewAuditHandler(auditSvc),
		handlers.NewHealthHandler(),
	).Setup()
	return app
}

func tokenFor(t *testing.T, role domain.Role) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":     "U1",
		"name":   "Test User",
		"role":   string(role),
		"hosId":  "H1",
		"wardId": "W1",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
	})
	signed, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return signed
}

var allRoles = []domain.Role{
	domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin,
	domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest,
}

func TestRouter_WardMutations_RequireSuperOrService(t *testing.T) {
	allowed := map[domain.Role]bool{
		domain.RoleSuper:   true,
		domain.RoleService: true,
	}
	requests := []struct {
		method string
		body   string
	}{
		{http.MethodPut, `{"wardName":"renamed"}`},
		{http.MethodDelete, ""},
	}

	for _, rq := range requests {
		for _, role := range allRoles {
			t.Run(rq.method+"/"+string(role), func(t *testing.T) {
				svc := &stubWardService{}
				app := newTestRouter(svc)

				req := httptest.NewRequest(rq.method, "/auth/ward/W1", strings.NewReader(rq.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+tokenFor(t, role))
				resp, err := app.Test(req)
				require.NoError(t, err)

				reached := svc.updated || svc.removed
				if allowed[role] {
					assert.Equal(t, fiber.StatusOK, resp.StatusCode)
					assert.True(t, reached, "service should be called")
				} else {
					assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
					assert.False(t, reached, "service must not be called")
				}
			})
		}
	}
}

// Audit logs span every hospital, so hospital-scoped admins must not read them.
func TestRouter_AuditLogs_RequireSuperOrService(t *testing.T) {
	for _, role := range allRoles {
		t.Run(string(role), func(t *testing.T) {
			audit := &stubAuditService{}
			app := newTestRouterWith(&stubWardService{}, audit)

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
	app := newTestRouterCfg(cfg, &stubWardService{}, &stubAuditService{})

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
	app := newTestRouterCfg(cfg, &stubWardService{}, &stubAuditService{})

	// The client varies the part of the header it controls; the gateway
	// appends the real address on the right.
	statuses := loginStatuses(t, app, 11, func(i int) string {
		return fmt.Sprintf("10.9.9.%d, 198.51.100.7", i+1)
	})
	assert.Equal(t, fiber.StatusTooManyRequests, statuses[10])
}

func TestRouter_LoginLimiter_IgnoresHeaderWithoutTrustedProxies(t *testing.T) {
	app := newTestRouterCfg(testConfig(), &stubWardService{}, &stubAuditService{})

	statuses := loginStatuses(t, app, 11, func(i int) string { return fmt.Sprintf("203.0.113.%d", i+1) })
	assert.Equal(t, fiber.StatusTooManyRequests, statuses[10])
}
