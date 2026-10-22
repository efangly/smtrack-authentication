package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func valid() Config {
	return Config{JWTSecret: strings.Repeat("a", 32), JWTRefreshSecret: strings.Repeat("b", 32), BcryptCost: 12}
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.mutate(&c)
			assert.ErrorContains(t, c.Validate(), tt.want)
		})
	}
}
