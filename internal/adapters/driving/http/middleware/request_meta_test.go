package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestMeta_StoresMetaAndEchoesRequestID(t *testing.T) {
	var got domain.RequestMeta
	app := fiber.New()
	app.Use(RequestMeta(nil))
	app.Get("/", func(c fiber.Ctx) error {
		got = domain.RequestMetaFrom(c.Context())
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Request-ID", "from-gateway")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, "from-gateway", got.RequestID)
	assert.Equal(t, "from-gateway", resp.Header.Get("X-Request-ID"))
	assert.Equal(t, "test-agent", got.UserAgent)
	assert.NotEmpty(t, got.IP)
}

func TestRequestMeta_GeneratesIDWhenMissingOrOversized(t *testing.T) {
	var got domain.RequestMeta
	app := fiber.New()
	app.Use(RequestMeta(nil))
	app.Get("/", func(c fiber.Ctx) error {
		got = domain.RequestMetaFrom(c.Context())
		return nil
	})

	for _, header := range []string{"", strings.Repeat("x", 101)} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", header)
		_, err := app.Test(req)
		require.NoError(t, err)
		assert.NotEmpty(t, got.RequestID)
		assert.LessOrEqual(t, len(got.RequestID), 100)
		assert.NotEqual(t, header, got.RequestID)
	}
}

type captureAudit struct {
	actions []domain.AuditAction
	ids     []string
}

func (c *captureAudit) Record(_ context.Context, _ domain.AuditEntity, a domain.AuditAction, id string, _ *domain.Actor, _ any) {
	c.actions = append(c.actions, a)
	c.ids = append(c.ids, id)
}

func (c *captureAudit) FindAll(context.Context, ports.AuditFilter) ([]domain.AuditLog, error) {
	return nil, nil
}

func TestAuditDenied_RecordsOnlyForbiddenResponsesOfAuthenticatedCallers(t *testing.T) {
	audit := &captureAudit{}
	app := fiber.New()
	app.Use(AuditDenied(audit))
	as := func(c fiber.Ctx) error { c.Locals("user", &domain.Actor{ID: "U1"}); return c.Next() }
	app.Get("/denied", as, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusForbidden) })
	app.Get("/ok", as, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/anon", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusForbidden) })

	for _, path := range []string{"/ok", "/anon", "/denied"} {
		_, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
	}
	assert.Equal(t, []domain.AuditAction{domain.AuditActionAccessDenied}, audit.actions)
	assert.Equal(t, []string{"U1"}, audit.ids)
}
