package config

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func valid() Config {
	return Config{JWTSecret: strings.Repeat("a", 32), JWTRefreshSecret: strings.Repeat("b", 32), BcryptCost: 12}
}

func s3(c *Config) {
	c.S3Endpoint, c.S3AccessKey, c.S3SecretKey, c.S3Bucket = "http://m:9000", "a", "b", "bucket"
	c.ImageBaseURL = "https://h/auth/image"
	c.ImageEncryptionKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestConfig_ValidateS3(t *testing.T) {
	c := valid()
	s3(&c)
	assert.NoError(t, c.Validate())
}

func TestConfig_Validate(t *testing.T) {
	ok := valid()
	assert.NoError(t, ok.Validate())

	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"short access secret", func(c *Config) { c.JWTSecret = "short" }, "JWT_SECRET must be at least 32"},
		{"short refresh secret", func(c *Config) { c.JWTRefreshSecret = "short" }, "JWT_REFRESH_SECRET must be at least 32"},
		{"same secret", func(c *Config) { c.JWTRefreshSecret = c.JWTSecret }, "must be different"},
		{"weak bcrypt", func(c *Config) { c.BcryptCost = 10 }, "BCRYPT_COST"},
		{"malformed bcrypt", func(c *Config) { c.BcryptCost = -1 }, "BCRYPT_COST"},
		{"absurd bcrypt", func(c *Config) { c.BcryptCost = 32 }, "BCRYPT_COST"},
		{"s3 without bucket", func(c *Config) { c.S3Endpoint = "http://m:9000" }, "S3_BUCKET"},
		{"s3 without base url", func(c *Config) { s3(c); c.ImageBaseURL = "" }, "IMAGE_BASE_URL"},
		{"s3 key not base64", func(c *Config) { s3(c); c.ImageEncryptionKey = "%%%" }, "base64"},
		{"s3 short key", func(c *Config) { s3(c); c.ImageEncryptionKey = base64.StdEncoding.EncodeToString(make([]byte, 16)) }, "32 bytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.mutate(&c)
			assert.ErrorContains(t, c.Validate(), tt.want)
		})
	}
}
