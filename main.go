package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"gorm.io/gorm"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/adapters/driven/cache"
	"github.com/efangly/smtrack-authentication/internal/adapters/driven/database"
	"github.com/efangly/smtrack-authentication/internal/adapters/driven/messaging"
	"github.com/efangly/smtrack-authentication/internal/adapters/driven/upload"
	httpAdapter "github.com/efangly/smtrack-authentication/internal/adapters/driving/http"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/handlers"
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/middleware"
	"github.com/efangly/smtrack-authentication/internal/core/services"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/efangly/smtrack-authentication/pkg/validator"
)

func main() {
	// .env must be loaded before the logger reads LOG_LEVEL from the environment.
	config.LoadDotEnv()

	// Initialize structured JSON logger for Loki
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "warn"
	}
	logger.Init(logLevel, "auth-service")

	// Load configuration
	cfg := config.Load()

	// Initialize driven adapters (infrastructure)
	db := database.NewPostgresDB(cfg)
	redisCache := cache.NewRedisAdapter(cfg)
	msgAdapter := messaging.NewRabbitMQAdapter(cfg)
	fileUpload := upload.NewFileUploadAdapter(cfg)

	// Initialize repositories
	userRepo := database.NewUserRepository(db)
	wardRepo := database.NewWardRepository(db)
	hospitalRepo := database.NewHospitalRepository(db)
	auditRepo := database.NewAuditRepository(db)

	// Initialize core services (use cases)
	auditService := services.NewAuditService(auditRepo)
	userService := services.NewUserService(userRepo, wardRepo, redisCache, fileUpload, auditService)
	wardService := services.NewWardService(wardRepo, redisCache, msgAdapter, auditService)
	hospitalService := services.NewHospitalService(hospitalRepo, redisCache, fileUpload, msgAdapter, auditService)
	authService := services.NewAuthService(userService, redisCache, fileUpload, cfg)

	// Initialize driving adapters (HTTP handlers)
	authHandler := handlers.NewAuthHandler(authService)
	userHandler := handlers.NewUserHandler(userService)
	hospitalHandler := handlers.NewHospitalHandler(hospitalService)
	wardHandler := handlers.NewWardHandler(wardService)
	auditHandler := handlers.NewAuditHandler(auditService)
	healthHandler := handlers.NewHealthHandler()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "auth-service",
		ErrorHandler: middleware.ErrorHandler,
		// Applies the DTO `validate` tags on every c.Bind().Body() call.
		StructValidator: validator.Validator{},
	})

	// Global middleware
	app.Use(middleware.Recover())
	app.Use(middleware.RequestLogger())
	app.Use(cors.New())

	// Setup routes
	router := httpAdapter.NewRouter(app, cfg, authHandler, userHandler, hospitalHandler, wardHandler, auditHandler, healthHandler)
	router.Setup()

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("Shutting down server...")
		if err := app.Shutdown(); err != nil {
			logger.Error("Error shutting down server", "error", err)
		}

		// Release infrastructure only once no request can still be using it.
		closeAll(db, redisCache, msgAdapter)
	}()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Info("Auth service starting", "port", cfg.Port)
	if err := app.Listen(addr, fiber.ListenConfig{
		DisableStartupMessage: true,
	}); err != nil {
		logger.Fatal("Failed to start server", "error", err)
	}
}

// closeAll releases every driven adapter that holds a connection. Adapters that
// do not expose Close (e.g. when RabbitMQ was never configured) are skipped.
func closeAll(db *gorm.DB, adapters ...any) {
	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			logger.Error("Error closing database", "error", err)
		}
	}
	for _, a := range adapters {
		closer, ok := a.(interface{ Close() error })
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			logger.Error("Error closing adapter", "error", err)
		}
	}
}
