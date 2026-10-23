package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
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
	RedisHost         string
	RedisPassword     string
	RabbitMQURL       string
	NodeEnv           string
	LogLevel          string
	// TrustedProxies lists the CIDR ranges or addresses of the proxies in
	// front of this service (KrakenD), from comma-separated TRUSTED_PROXIES.
	// Empty means the TCP peer is the client.
	TrustedProxies []string
	// Picture storage in MinIO (S3_ENDPOINT unset = uploads unavailable).
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	// ImageEncryptionKey is the base64 AES-256 key pictures are sealed with.
	ImageEncryptionKey string
	// ImageBaseURL is the public prefix of GET /auth/image, stored in front of
	// each picture's name (e.g. https://host/auth/image).
	ImageBaseURL string
	// BcryptCost is the work factor for new password hashes (BCRYPT_COST, at least 12).
	BcryptCost int
}

const (
	// MinSecretLength is the shortest JWT secret accepted (ISO/IEC 27001 A.8.24).
	MinSecretLength = 32
	// MinBcryptCost is the lowest work factor accepted.
	MinBcryptCost = 12
	// MinImageKeyBytes is the AES-256 key length IMAGE_ENCRYPTION_KEY must decode to.
	MinImageKeyBytes = 32
)

// Validate refuses configuration that weakens the service's cryptography. A secret that is
// short, or shared between access and refresh tokens, would make the token-type separation
// pointless.
func (c *Config) Validate() error {
	if len(c.JWTSecret) < MinSecretLength {
		return fmt.Errorf("JWT_SECRET must be at least %d characters", MinSecretLength)
	}
	if len(c.JWTRefreshSecret) < MinSecretLength {
		return fmt.Errorf("JWT_REFRESH_SECRET must be at least %d characters", MinSecretLength)
	}
	if c.JWTSecret == c.JWTRefreshSecret {
		return errors.New("JWT_SECRET and JWT_REFRESH_SECRET must be different")
	}
	if c.BcryptCost < MinBcryptCost || c.BcryptCost > 31 {
		return fmt.Errorf("BCRYPT_COST must be between %d and 31", MinBcryptCost)
	}
	if c.S3Endpoint != "" {
		if c.S3AccessKey == "" || c.S3SecretKey == "" || c.S3Bucket == "" {
			return errors.New("S3_ACCESS_KEY, S3_SECRET_KEY and S3_BUCKET are required with S3_ENDPOINT")
		}
		if c.ImageBaseURL == "" {
			return errors.New("IMAGE_BASE_URL is required with S3_ENDPOINT")
		}
		if _, err := base64.StdEncoding.DecodeString(c.ImageEncryptionKey); err != nil || len(c.ImageEncryptionKey) == 0 {
			return errors.New("IMAGE_ENCRYPTION_KEY must be base64")
		}
		if raw, _ := base64.StdEncoding.DecodeString(c.ImageEncryptionKey); len(raw) != MinImageKeyBytes {
			return fmt.Errorf("IMAGE_ENCRYPTION_KEY must decode to %d bytes", MinImageKeyBytes)
		}
	}
	return nil
}

// LoadDotEnv reads the .env file into the process environment. It must run
// before the logger is initialised so that LOG_LEVEL from .env takes effect.
func LoadDotEnv() {
	_ = godotenv.Load()
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		Port:               getEnv("PORT", "8080"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTRefreshSecret:   getEnv("JWT_REFRESH_SECRET", ""),
		ExpireTime:         getEnv("EXPIRE_TIME", "15m"),
		RefreshExpireTime:  getEnv("REFRESH_EXPIRE_TIME", "7d"),
		S3Endpoint:         getEnv("S3_ENDPOINT", ""),
		S3AccessKey:        getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:        getEnv("S3_SECRET_KEY", ""),
		S3Bucket:           getEnv("S3_BUCKET", ""),
		ImageEncryptionKey: getEnv("IMAGE_ENCRYPTION_KEY", ""),
		ImageBaseURL:       getEnv("IMAGE_BASE_URL", ""),
		RedisHost:          getEnv("REDIS_HOST", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RabbitMQURL:        getEnv("RABBITMQ", ""),
		NodeEnv:            getEnv("NODE_ENV", "production"),
		LogLevel:           getEnv("LOG_LEVEL", "warn"),
		TrustedProxies:     splitList(getEnv("TRUSTED_PROXIES", "")),
		BcryptCost:         getEnvInt("BCRYPT_COST", MinBcryptCost),
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
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid configuration", "error", err)
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

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
		// A malformed number must not silently fall back to a weaker default.
		return -1
	}
	return defaultValue
}
