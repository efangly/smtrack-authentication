package middleware

import (
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	headerRequestID = "X-Request-ID"
	maxRequestIDLen = 100
	maxUserAgentLen = 255
)

// RequestMeta puts who is calling (IP via the trusted-proxy rules, user agent) and a request
// id into the request context, where the audit service picks it up. A request id from the
// gateway is kept so one request can be followed across services; otherwise one is made.
func RequestMeta(proxies TrustedProxies) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Get(headerRequestID)
		if id == "" || len(id) > maxRequestIDLen {
			id = uuid.NewString()
		}
		ua := c.Get(fiber.HeaderUserAgent)
		if len(ua) > maxUserAgentLen {
			ua = ua[:maxUserAgentLen]
		}
		c.Set(headerRequestID, id)
		c.SetContext(domain.WithRequestMeta(c.Context(), domain.RequestMeta{
			IP: proxies.ClientIP(c), UserAgent: ua, RequestID: id,
		}))
		return c.Next()
	}
}

// AuditDenied records an ACCESS_DENIED event for every 403 given to an authenticated caller.
// It wraps the handlers, so it needs no change to the guards that refuse the request.
func AuditDenied(audit ports.AuditService) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := c.Next()
		if c.Response().StatusCode() != fiber.StatusForbidden {
			return err
		}
		if actor, ok := c.Locals("user").(*domain.Actor); ok && actor != nil {
			audit.Record(c.Context(), domain.AuditEntityUser, domain.AuditActionAccessDenied, actor.ID, actor,
				map[string]any{"method": c.Method(), "path": c.Path()})
		}
		return err
	}
}
