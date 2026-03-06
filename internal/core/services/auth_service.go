package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tng-coop/auth-service/config"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
	"github.com/tng-coop/auth-service/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	userService ports.UserService
	cache       ports.CachePort
	upload      ports.FileUploadPort
	cfg         *config.Config
}

func NewAuthService(
	userService ports.UserService,
	cache ports.CachePort,
	upload ports.FileUploadPort,
	cfg *config.Config,
) ports.AuthService {
	return &authService{
		userService: userService,
		cache:       cache,
		upload:      upload,
		cfg:         cfg,
	}
}

func (s *authService) Register(ctx context.Context, data *domain.User, file *multipart.FileHeader) (*domain.User, error) {
	existing, err := s.userService.FindByUsername(ctx, strings.ToLower(data.Username))
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("username already exists")
	}

	if file != nil {
		picURL, err := s.upload.Upload(file, "user")
		if err != nil {
			logger.Error("Failed to upload user image", "error", err)
			return nil, fmt.Errorf("image upload failed: %w", err)
		}
		data.Pic = &picURL
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), 10)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	data.Password = string(hashedPassword)

	return s.userService.Create(ctx, data)
}

func (s *authService) ValidateUser(ctx context.Context, username string, password string) (*domain.User, error) {
	user, err := s.userService.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, nil
	}

	return user, nil
}

func (s *authService) Login(ctx context.Context, user *domain.User) (*ports.LoginResult, error) {
	if s.cfg.JWTSecret == "" || s.cfg.JWTRefreshSecret == "" {
		return nil, errors.New("authentication configuration error")
	}

	var hosID string
	if user.Ward != nil {
		hosID = user.Ward.HosID
	}

	displayName := ""
	if user.Display != nil {
		displayName = *user.Display
	}

	payload := jwt.MapClaims{
		"id":     user.ID,
		"name":   displayName,
		"role":   string(user.Role),
		"hosId":  hosID,
		"wardId": user.WardID,
		"exp":    time.Now().Add(parseDuration(s.cfg.ExpireTime, time.Hour)).Unix(),
		"iat":    time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	tokenStr, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	refreshPayload := jwt.MapClaims{
		"id":     user.ID,
		"name":   displayName,
		"role":   string(user.Role),
		"hosId":  hosID,
		"wardId": user.WardID,
		"exp":    time.Now().Add(parseDuration(s.cfg.RefreshExpireTime, 7*24*time.Hour)).Unix(),
		"iat":    time.Now().Unix(),
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshPayload)
	refreshTokenStr, err := refreshToken.SignedString([]byte(s.cfg.JWTRefreshSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &ports.LoginResult{
		Token:        tokenStr,
		RefreshToken: refreshTokenStr,
		ID:           user.ID,
		Name:         user.Display,
		HosID:        hosID,
		WardID:       user.WardID,
		Role:         string(user.Role),
		Pic:          user.Pic,
	}, nil
}

func (s *authService) RefreshTokens(tokenStr string) (*ports.RefreshResult, error) {
	if s.cfg.JWTSecret == "" || s.cfg.JWTRefreshSecret == "" {
		return nil, errors.New("authentication configuration error")
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTRefreshSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	newPayload := jwt.MapClaims{
		"id":     claims["id"],
		"name":   claims["name"],
		"role":   claims["role"],
		"hosId":  claims["hosId"],
		"wardId": claims["wardId"],
		"exp":    time.Now().Add(parseDuration(s.cfg.ExpireTime, time.Hour)).Unix(),
		"iat":    time.Now().Unix(),
	}

	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newPayload)
	newTokenStr, err := newToken.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, err
	}

	refreshPayload := jwt.MapClaims{
		"id":     claims["id"],
		"name":   claims["name"],
		"role":   claims["role"],
		"hosId":  claims["hosId"],
		"wardId": claims["wardId"],
		"exp":    time.Now().Add(parseDuration(s.cfg.RefreshExpireTime, 7*24*time.Hour)).Unix(),
		"iat":    time.Now().Unix(),
	}

	newRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshPayload)
	newRefreshTokenStr, err := newRefreshToken.SignedString([]byte(s.cfg.JWTRefreshSecret))
	if err != nil {
		return nil, err
	}

	return &ports.RefreshResult{
		Token:        newTokenStr,
		RefreshToken: newRefreshTokenStr,
	}, nil
}

func (s *authService) ResetPassword(ctx context.Context, username string, password string, oldPassword string, caller *ports.JwtPayload) (string, error) {
	user, err := s.userService.FindByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("user not found")
	}

	if caller.Role != "SUPER" {
		if oldPassword == "" {
			return "", errors.New("old password is required")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
			return "", errors.New("old password is incorrect")
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = s.userService.Update(ctx, user.ID, map[string]any{
		"password": string(hashedPassword),
	}, nil)
	if err != nil {
		return "", err
	}

	s.cache.Del(ctx, fmt.Sprintf("user:%s", username))
	s.cache.Del(ctx, fmt.Sprintf("user:%s", user.ID))

	return "Password reset successfully", nil
}

func parseDuration(s string, defaultDuration time.Duration) time.Duration {
	if s == "" {
		return defaultDuration
	}
	if strings.HasSuffix(s, "d") {
		var days int
		fmt.Sscanf(s, "%dd", &days)
		if days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDuration
	}
	return d
}
