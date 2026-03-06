package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tng-coop/auth-service/config"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/handlers"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/middleware"
	"github.com/tng-coop/auth-service/internal/core/domain"
)

type Router struct {
	app             *fiber.App
	cfg             *config.Config
	authHandler     *handlers.AuthHandler
	userHandler     *handlers.UserHandler
	hospitalHandler *handlers.HospitalHandler
	wardHandler     *handlers.WardHandler
	healthHandler   *handlers.HealthHandler
}

func NewRouter(
	app *fiber.App,
	cfg *config.Config,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	hospitalHandler *handlers.HospitalHandler,
	wardHandler *handlers.WardHandler,
	healthHandler *handlers.HealthHandler,
) *Router {
	return &Router{
		app:             app,
		cfg:             cfg,
		authHandler:     authHandler,
		userHandler:     userHandler,
		hospitalHandler: hospitalHandler,
		wardHandler:     wardHandler,
		healthHandler:   healthHandler,
	}
}

func (r *Router) Setup() {
	auth := r.app.Group("/auth")

	// Public routes (no auth required)
	auth.Post("/register", r.authHandler.Register)
	auth.Post("/login", r.authHandler.Login)

	// Refresh token route (requires refresh JWT)
	auth.Post("/refresh", middleware.RefreshJWTAuth(r.cfg.JWTRefreshSecret), r.authHandler.RefreshToken)

	// Reset password (requires JWT)
	auth.Patch("/reset/:id", middleware.JWTAuth(r.cfg.JWTSecret), r.authHandler.ResetPassword)

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
	userGroup.Get("/:id", r.userHandler.FindOne)
	userGroup.Put("/:id", r.userHandler.Update)
	userGroup.Delete("/:id", r.userHandler.Remove)

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
	wardGroup.Put("/:id", r.wardHandler.Update)
	wardGroup.Delete("/:id", r.wardHandler.Remove)
}
