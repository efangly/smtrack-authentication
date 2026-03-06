package config

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/tng-coop/auth-service/pkg/logger"
)

type Config struct {
	DatabaseURL       string
	Port              string
	JWTSecret         string
	JWTRefreshSecret  string
	ExpireTime        string
	RefreshExpireTime string
	UploadPath        string
	RedisHost         string
	RedisPassword     string
	RabbitMQURL       string
	NodeEnv           string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		logger.Info("No .env file found, using environment variables")
	}

	cfg := &Config{
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		Port:              getEnv("PORT", "8080"),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		JWTRefreshSecret:  getEnv("JWT_REFRESH_SECRET", ""),
		ExpireTime:        getEnv("EXPIRE_TIME", "1h"),
		RefreshExpireTime: getEnv("REFRESH_EXPIRE_TIME", "7d"),
		UploadPath:        getEnv("UPLOAD_PATH", ""),
		RedisHost:         getEnv("REDIS_HOST", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RabbitMQURL:       getEnv("RABBITMQ", ""),
		NodeEnv:           getEnv("NODE_ENV", "production"),
	}

	if cfg.DatabaseURL == "" {
		logger.Fatal("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		logger.Fatal("JWT_SECRET is required")
	}
	if cfg.JWTRefreshSecret == "" {
		logger.Fatal("JWT_REFRESH_SECRET is required")
	}
	if cfg.UploadPath == "" {
		logger.Warn("UPLOAD_PATH is not set - file upload will be unavailable")
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
