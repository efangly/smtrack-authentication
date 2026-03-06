package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	"github.com/tng-coop/auth-service/config"
	"github.com/tng-coop/auth-service/internal/adapters/driven/cache"
	"github.com/tng-coop/auth-service/internal/adapters/driven/database"
	"github.com/tng-coop/auth-service/internal/adapters/driven/messaging"
	"github.com/tng-coop/auth-service/internal/adapters/driven/upload"
	httpAdapter "github.com/tng-coop/auth-service/internal/adapters/driving/http"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/handlers"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/middleware"
	"github.com/tng-coop/auth-service/internal/core/services"
	"github.com/tng-coop/auth-service/pkg/logger"
)

func main() {
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

	// Initialize core services (use cases)
	userService := services.NewUserService(userRepo, redisCache, fileUpload)
	wardService := services.NewWardService(wardRepo, redisCache, msgAdapter)
	hospitalService := services.NewHospitalService(hospitalRepo, redisCache, fileUpload, msgAdapter)
	authService := services.NewAuthService(userService, redisCache, fileUpload, cfg)

	// Initialize driving adapters (HTTP handlers)
	authHandler := handlers.NewAuthHandler(authService)
	userHandler := handlers.NewUserHandler(userService)
	hospitalHandler := handlers.NewHospitalHandler(hospitalService)
	wardHandler := handlers.NewWardHandler(wardService)
	healthHandler := handlers.NewHealthHandler()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "auth-service",
		ErrorHandler: middleware.ErrorHandler,
	})

	// Global middleware
	app.Use(middleware.Recover())
	app.Use(middleware.RequestLogger())
	app.Use(cors.New())

	// Setup routes
	router := httpAdapter.NewRouter(app, cfg, authHandler, userHandler, hospitalHandler, wardHandler, healthHandler)
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
