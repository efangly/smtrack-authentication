package http

import (
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/handlers"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/middleware"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

type Router struct {
	app             *fiber.App
	cfg             *config.Config
	authHandler     *handlers.AuthHandler
	userHandler     *handlers.UserHandler
	hospitalHandler *handlers.HospitalHandler
	wardHandler     *handlers.WardHandler
	auditHandler    *handlers.AuditHandler
	healthHandler   *handlers.HealthHandler
}

func NewRouter(
	app *fiber.App,
	cfg *config.Config,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	hospitalHandler *handlers.HospitalHandler,
	wardHandler *handlers.WardHandler,
	auditHandler *handlers.AuditHandler,
	healthHandler *handlers.HealthHandler,
) *Router {
	return &Router{
		app:             app,
		cfg:             cfg,
		authHandler:     authHandler,
		userHandler:     userHandler,
		hospitalHandler: hospitalHandler,
		wardHandler:     wardHandler,
		auditHandler:    auditHandler,
		healthHandler:   healthHandler,
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
	auth.Post("/refresh", middleware.RefreshJWTAuth(r.cfg.JWTRefreshSecret), r.authHandler.RefreshToken)

	// Reset password (requires JWT). The path parameter is a username, not an id.
	auth.Patch("/reset/:username", middleware.JWTAuth(r.cfg.JWTSecret), r.authHandler.ResetPassword)

	// Health check
	auth.Get("/health", r.healthHandler.Check)

	// JWT-protected middleware
	jwtAuth := middleware.JWTAuth(r.cfg.JWTSecret)

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

	// Hospital routes
	hospitalGroup := auth.Group("/hospital", jwtAuth)
	hospitalGroup.Post("/",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.hospitalHandler.Create,
	)
	hospitalGroup.Get("/", r.hospitalHandler.FindAll)
	hospitalGroup.Get("/:id", r.hospitalHandler.FindOne)
	hospitalGroup.Put("/:id",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.hospitalHandler.Update,
	)
	hospitalGroup.Delete("/:id",
		middleware.RolesGuard(domain.RoleSuper),
		r.hospitalHandler.Remove,
	)

	// Ward routes
	wardGroup := auth.Group("/ward", jwtAuth)
	wardGroup.Post("/",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.wardHandler.Create,
	)
	wardGroup.Get("/", r.wardHandler.FindAll)
	wardGroup.Get("/:id", r.wardHandler.FindOne)
	// Update and Remove carry the same guard as Create: without it any
	// authenticated caller — including a self-registered GUEST — could
	// rename or delete any ward.
	wardGroup.Put("/:id",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.wardHandler.Update,
	)
	wardGroup.Delete("/:id",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.wardHandler.Remove,
	)

	// Audit log routes (read-only). Logs are not tenant-scoped, so only the
	// system-wide roles may read them.
	auditGroup := auth.Group("/audit", jwtAuth)
	auditGroup.Get("/",
		middleware.RolesGuard(domain.RoleSuper, domain.RoleService),
		r.auditHandler.FindAll,
	)
}
