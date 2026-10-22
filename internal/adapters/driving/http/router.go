package http

import (
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/handlers"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/middleware"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

type Router struct {
	app               *fiber.App
	cfg               *config.Config
	tokens            ports.TokenIssuer
	authHandler       *handlers.AuthHandler
	userHandler       *handlers.UserHandler
	tenantHandler     *handlers.TenantHandler
	departmentHandler *handlers.DepartmentHandler
	auditHandler      *handlers.AuditHandler
	healthHandler     *handlers.HealthHandler
}

func NewRouter(
	app *fiber.App,
	cfg *config.Config,
	tokens ports.TokenIssuer,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	tenantHandler *handlers.TenantHandler,
	departmentHandler *handlers.DepartmentHandler,
	auditHandler *handlers.AuditHandler,
	healthHandler *handlers.HealthHandler,
) *Router {
	return &Router{
		app:               app,
		cfg:               cfg,
		tokens:            tokens,
		authHandler:       authHandler,
		userHandler:       userHandler,
		tenantHandler:     tenantHandler,
		departmentHandler: departmentHandler,
		auditHandler:      auditHandler,
		healthHandler:     healthHandler,
	}
}

func (r *Router) Setup() {
	auth := r.app.Group("/auth")

	// Behind KrakenD every connection comes from the gateway, so without this
	// the limiter below would put all clients into one shared bucket.
	proxies, err := middleware.ParseTrustedProxies(r.cfg.TrustedProxies)
	if err != nil {
		logger.Fatal("Invalid TRUSTED_PROXIES", "error", err)
	}

	// Public routes (no auth required). Rate limited per client IP so
	// credentials cannot be brute forced and accounts cannot be mass-created.
	publicLimiter := limiter.New(limiter.Config{
		Max:          10,
		Expiration:   time.Minute,
		KeyGenerator: proxies.ClientIP,
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).
				JSON(dto.FailResponse("Too many requests, please try again later"))
		},
	})
	auth.Post("/register", publicLimiter, r.authHandler.Register)
	auth.Post("/login", publicLimiter, r.authHandler.Login)

	// Refresh token route (requires refresh JWT)
	auth.Post("/refresh", middleware.RefreshJWTAuth(r.tokens), r.authHandler.RefreshToken)

	// Reset password (requires JWT). The path parameter is a username, not an id.
	auth.Patch("/reset/:username", middleware.JWTAuth(r.tokens), r.authHandler.ResetPassword)

	// Logout (requires the access token; the body carries the refresh token to revoke).
	auth.Post("/logout", middleware.JWTAuth(r.tokens), r.authHandler.Logout)

	// Health check
	auth.Get("/health", r.healthHandler.Check)

	// JWT-protected middleware
	jwtAuth := middleware.JWTAuth(r.tokens)

	// User routes
	userGroup := auth.Group("/user", jwtAuth)
	userGroup.Post("/",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin),
		r.userHandler.Create,
	)
	userGroup.Get("/",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin),
		r.userHandler.FindAll,
	)
	// FindOne and Update additionally enforce ownership inside the handler:
	// a non-admin caller may only touch its own record.
	userGroup.Get("/:id", r.userHandler.FindOne)
	userGroup.Put("/:id", r.userHandler.Update)
	userGroup.Delete("/:id",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin),
		r.userHandler.Remove,
	)
	userGroup.Post("/:id/restore",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin),
		r.userHandler.Restore,
	)

	// Access review: who holds which role where, and when they last signed in.
	auth.Get("/report/access", jwtAuth,
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin),
		r.userHandler.AccessReport,
	)

	// Tenant routes
	tenantGroup := auth.Group("/tenant", jwtAuth)
	tenantGroup.Post("/",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.tenantHandler.Create,
	)
	tenantGroup.Get("/", r.tenantHandler.FindAll)
	tenantGroup.Get("/:id", r.tenantHandler.FindOne)
	tenantGroup.Put("/:id",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.tenantHandler.Update,
	)
	// A tenant's own admin may rename its labels; the service confirms the
	// tenant is the caller's.
	tenantGroup.Put("/:id/labels",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin),
		r.tenantHandler.UpdateLabels,
	)
	tenantGroup.Delete("/:id",
		middleware.RolesGuard(domain.RoleSuper),
		r.tenantHandler.Remove,
	)
	tenantGroup.Post("/:id/restore",
		middleware.RolesGuard(domain.RoleSuper),
		r.tenantHandler.Restore,
	)

	// Department routes
	departmentGroup := auth.Group("/department", jwtAuth)
	departmentGroup.Post("/",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.departmentHandler.Create,
	)
	departmentGroup.Get("/", r.departmentHandler.FindAll)
	departmentGroup.Get("/:id", r.departmentHandler.FindOne)
	// Every mutating route carries a guard: without one any authenticated
	// caller — including a self-registered GUEST — could rename or delete any
	// department.
	departmentGroup.Put("/:id",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.departmentHandler.Update,
	)
	departmentGroup.Delete("/:id",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.departmentHandler.Remove,
	)
	departmentGroup.Post("/:id/restore",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.departmentHandler.Restore,
	)

	// Audit log routes (read-only). Logs are not tenant-scoped, so only the
	// system-wide roles may read them.
	auditGroup := auth.Group("/audit", jwtAuth)
	auditGroup.Get("/",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.auditHandler.FindAll,
	)
}
