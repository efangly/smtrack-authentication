package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/handlers"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
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
	app := fiber.New()
	cfg := &config.Config{JWTSecret: testJWTSecret, JWTRefreshSecret: "test-refresh-secret"}
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
