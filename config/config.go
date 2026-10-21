package config

import (
	"os"
	"strings"

	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/joho/godotenv"
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
	LogLevel          string
	// TrustedProxies lists the CIDR ranges or addresses of the proxies in
	// front of this service (KrakenD), from comma-separated TRUSTED_PROXIES.
	// Empty means the TCP peer is the client.
	TrustedProxies []string
}

// LoadDotEnv reads the .env file into the process environment. It must run
// before the logger is initialised so that LOG_LEVEL from .env takes effect.
func LoadDotEnv() {
	_ = godotenv.Load()
}

func Load() *Config {
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
		LogLevel:          getEnv("LOG_LEVEL", "warn"),
		TrustedProxies:    splitList(getEnv("TRUSTED_PROXIES", "")),
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

// splitList splits a comma-separated value, dropping blank entries.
func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
