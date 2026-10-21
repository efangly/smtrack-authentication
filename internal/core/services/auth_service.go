package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor used for every password hash in this service.
const bcryptCost = 10

// dummyHash is compared against when no user matches, so that login takes the
// same time whether or not the username exists.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcryptCost)

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

	// userService.Create hashes the password before it reaches the database.
	return s.userService.Create(ctx, data, nil)
}

func (s *authService) ValidateUser(ctx context.Context, username string, password string) (*domain.User, error) {
	user, err := s.userService.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		// Hash anyway so an unknown username costs the same as a wrong
		// password — otherwise response time reveals which accounts exist.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, nil
	}

	// A deactivated account fails exactly like a wrong password, and only
	// after the hash comparison, so neither the message nor the timing tells
	// a caller that the account exists but is disabled.
	if !user.Status {
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

func (s *authService) RefreshTokens(ctx context.Context, tokenStr string) (*ports.RefreshResult, error) {
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

	userID, _ := claims["id"].(string)
	if userID == "" {
		return nil, errors.New("invalid token claims")
	}

	// Rebuild the claims from the current record rather than copying the old
	// ones: a user who was demoted or deleted must not keep their old rights
	// for the remaining lifetime of the refresh token.
	// FindByID bypasses the cache, so a deactivation takes effect at the very
	// next refresh.
	user, err := s.userService.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.Status {
		return nil, errors.New("user is no longer active")
	}

	var hosID string
	if user.Ward != nil {
		hosID = user.Ward.HosID
	}
	displayName := ""
	if user.Display != nil {
		displayName = *user.Display
	}

	base := func(ttl time.Duration) jwt.MapClaims {
		return jwt.MapClaims{
			"id":     user.ID,
			"name":   displayName,
			"role":   string(user.Role),
			"hosId":  hosID,
			"wardId": user.WardID,
			"exp":    time.Now().Add(ttl).Unix(),
			"iat":    time.Now().Unix(),
		}
	}

	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, base(parseDuration(s.cfg.ExpireTime, time.Hour)))
	newTokenStr, err := newToken.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	newRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, base(parseDuration(s.cfg.RefreshExpireTime, 7*24*time.Hour)))
	newRefreshTokenStr, err := newRefreshToken.SignedString([]byte(s.cfg.JWTRefreshSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
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

	// Only an admin-tier caller may target somebody else's account.
	if !caller.IsAdminTier() && caller.ID != user.ID {
		return "", errors.New("you can only reset your own password")
	}

	// Everyone but SUPER must prove the current password.
	if caller.Role != string(domain.RoleSuper) {
		if oldPassword == "" {
			return "", errors.New("old password is required")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
			return "", errors.New("old password is incorrect")
		}
	}

	// userService.Update hashes the password before it reaches the database.
	_, err = s.userService.Update(ctx, user.ID, map[string]any{
		"password": password,
	}, nil, caller)
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
		// time.ParseDuration has no day unit, so "7d" is handled here.
		if n, err := fmt.Sscanf(s, "%dd", &days); err != nil || n != 1 || days <= 0 {
			return defaultDuration
		}
		return time.Duration(days) * 24 * time.Hour
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDuration
	}
	return d
}
