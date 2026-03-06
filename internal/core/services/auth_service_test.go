package services

import (
	"context"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tng-coop/auth-service/config"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
	"golang.org/x/crypto/bcrypt"
)

func newTestAuthService(userSvc *mockUserService) (ports.AuthService, *mockCache, *mockUpload) {
	cache := newMockCache()
	upload := newMockUpload()
	cfg := &config.Config{
		JWTSecret:         "test-secret",
		JWTRefreshSecret:  "test-refresh-secret",
		ExpireTime:        "1h",
		RefreshExpireTime: "7d",
	}
	svc := NewAuthService(userSvc, cache, upload, cfg)
	return svc, cache, upload
}

// --- Register ---

func TestAuthService_Register_Success(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil // no existing user
		},
		CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) {
			return user, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	user, err := svc.Register(context.Background(), &domain.User{
		Username: "TestUser",
		Password: "pass1234",
		WardID:   "W1",
	}, nil)

	require.NoError(t, err)
	assert.NotNil(t, user)
	// password should be hashed
	assert.NotEqual(t, "pass1234", user.Password)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("pass1234")))
}

func TestAuthService_Register_DuplicateUsername(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "existing"}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.Register(context.Background(), &domain.User{
		Username: "existinguser",
		Password: "pass1234",
	}, nil)

	assert.EqualError(t, err, "username already exists")
}

func TestAuthService_Register_WithFile(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
		CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) {
			return user, nil
		},
	}
	svc, _, upload := newTestAuthService(userSvc)
	upload.UploadFn = func(_ *multipart.FileHeader, _ string) (string, error) {
		return "http://cdn.example.com/user/pic.jpg", nil
	}

	user, err := svc.Register(context.Background(), &domain.User{
		Username: "newuser",
		Password: "pass1234",
	}, &multipart.FileHeader{Filename: "pic.jpg"})

	require.NoError(t, err)
	assert.NotNil(t, user.Pic)
	assert.Equal(t, "http://cdn.example.com/user/pic.jpg", *user.Pic)
}

// --- ValidateUser ---

func TestAuthService_ValidateUser_Success(t *testing.T) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("correct"), 10)
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: string(hashed)}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	user, err := svc.ValidateUser(context.Background(), "admin", "correct")
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "U1", user.ID)
}

func TestAuthService_ValidateUser_WrongPassword(t *testing.T) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("correct"), 10)
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: string(hashed)}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	user, err := svc.ValidateUser(context.Background(), "admin", "wrong")
	require.NoError(t, err)
	assert.Nil(t, user)
}

func TestAuthService_ValidateUser_UserNotFound(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	user, err := svc.ValidateUser(context.Background(), "nobody", "pass")
	require.NoError(t, err)
	assert.Nil(t, user)
}

// --- Login ---

func TestAuthService_Login_Success(t *testing.T) {
	userSvc := &mockUserService{}
	svc, _, _ := newTestAuthService(userSvc)

	display := "Admin"
	user := &domain.User{
		ID:      "U1",
		Role:    domain.RoleAdmin,
		WardID:  "W1",
		Ward:    &domain.Ward{HosID: "H1"},
		Display: &display,
	}

	result, err := svc.Login(context.Background(), user)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, "U1", result.ID)
	assert.Equal(t, "ADMIN", result.Role)
	assert.Equal(t, "H1", result.HosID)
	assert.Equal(t, "W1", result.WardID)

	// Validate token can be parsed
	token, err := jwt.Parse(result.Token, func(t *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	})
	require.NoError(t, err)
	assert.True(t, token.Valid)
}

func TestAuthService_Login_MissingSecret(t *testing.T) {
	cache := newMockCache()
	upload := newMockUpload()
	cfg := &config.Config{JWTSecret: "", JWTRefreshSecret: ""}
	svc := NewAuthService(&mockUserService{}, cache, upload, cfg)

	_, err := svc.Login(context.Background(), &domain.User{ID: "U1"})
	assert.EqualError(t, err, "authentication configuration error")
}

// --- RefreshTokens ---

func TestAuthService_RefreshTokens_Success(t *testing.T) {
	userSvc := &mockUserService{}
	svc, _, _ := newTestAuthService(userSvc)

	// Create a valid refresh token first
	claims := jwt.MapClaims{
		"id":     "U1",
		"name":   "Admin",
		"role":   "ADMIN",
		"hosId":  "H1",
		"wardId": "W1",
		"exp":    float64(9999999999),
		"iat":    float64(1000000000),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := refreshToken.SignedString([]byte("test-refresh-secret"))

	result, err := svc.RefreshTokens(tokenStr)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	assert.NotEmpty(t, result.RefreshToken)
}

func TestAuthService_RefreshTokens_InvalidToken(t *testing.T) {
	userSvc := &mockUserService{}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.RefreshTokens("invalid-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid refresh token")
}

// --- ResetPassword ---

func TestAuthService_ResetPassword_SuperRole(t *testing.T) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), 10)
	updated := false
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: "user1", Password: string(hashed)}, nil
		},
		UpdateFn: func(_ context.Context, _ string, _ map[string]any, _ *multipart.FileHeader) (*domain.User, error) {
			updated = true
			return &domain.User{ID: "U1"}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	msg, err := svc.ResetPassword(context.Background(), "user1", "newpass", "", &ports.JwtPayload{Role: "SUPER"})
	require.NoError(t, err)
	assert.Equal(t, "Password reset successfully", msg)
	assert.True(t, updated)
}

func TestAuthService_ResetPassword_NonSuperRequiresOldPassword(t *testing.T) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), 10)
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: string(hashed)}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.ResetPassword(context.Background(), "user1", "newpass", "", &ports.JwtPayload{Role: "ADMIN"})
	assert.EqualError(t, err, "old password is required")
}

func TestAuthService_ResetPassword_WrongOldPassword(t *testing.T) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), 10)
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: string(hashed)}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.ResetPassword(context.Background(), "user1", "newpass", "wrongold", &ports.JwtPayload{Role: "ADMIN"})
	assert.EqualError(t, err, "old password is incorrect")
}

func TestAuthService_ResetPassword_UserNotFound(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.ResetPassword(context.Background(), "nobody", "newpass", "", &ports.JwtPayload{Role: "SUPER"})
	assert.EqualError(t, err, "user not found")
}

// --- parseDuration ---

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1h", "1h0m0s"},
		{"30m", "30m0s"},
		{"7d", "168h0m0s"},
		{"", "1h0m0s"}, // default
		{"invalid", "1h0m0s"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d := parseDuration(tt.input, 1*3600*1e9) // 1h default as time.Duration
			assert.Equal(t, tt.expected, d.String())
		})
	}
}

// --- Register error from FindByUsername ---

func TestAuthService_Register_FindByUsernameError(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.Register(context.Background(), &domain.User{Username: "u1", Password: "pass"}, nil)
	assert.EqualError(t, err, "db error")
}
