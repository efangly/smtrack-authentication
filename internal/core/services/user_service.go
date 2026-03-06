package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
	"github.com/tng-coop/auth-service/pkg/logger"
)

type userService struct {
	repo   ports.UserRepository
	cache  ports.CachePort
	upload ports.FileUploadPort
}

// NewUserService creates a new user service
func NewUserService(repo ports.UserRepository, cache ports.CachePort, upload ports.FileUploadPort) ports.UserService {
	return &userService{repo: repo, cache: cache, upload: upload}
}

func (s *userService) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.Username = strings.ToLower(user.Username)
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	result, err := s.repo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// Clear cache
	s.cache.Del(ctx, "user")
	return result, nil
}

func (s *userService) FindAll(ctx context.Context, caller *ports.JwtPayload) ([]domain.User, error) {
	conditions, notConditions, key := buildUserConditions(caller)

	// Check cache
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var users []domain.User
		if err := json.Unmarshal([]byte(cached), &users); err == nil {
			return users, nil
		}
	}

	users, err := s.repo.FindAll(ctx, conditions, notConditions)
	if err != nil {
		return nil, err
	}

	if len(users) > 0 {
		data, _ := json.Marshal(users)
		s.cache.Set(ctx, key, string(data), 3600*10)
	}

	return users, nil
}

func (s *userService) FindOne(ctx context.Context, id string) (*domain.User, error) {
	cacheKey := fmt.Sprintf("user:%s", id)

	cached, err := s.cache.Get(ctx, cacheKey)
	if err == nil && cached != "" {
		var user domain.User
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			return &user, nil
		}
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	data, _ := json.Marshal(user)
	s.cache.Set(ctx, cacheKey, string(data), 3600*24)

	return user, nil
}

func (s *userService) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	// Do not use cache here: domain.User.Password has json:"-" so it would be
	// lost on marshal/unmarshal, causing bcrypt comparison to always fail on
	// cached reads.
	user, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader) (*domain.User, error) {
	if file != nil {
		picURL, err := s.upload.Upload(file, "user")
		if err != nil {
			return nil, fmt.Errorf("image upload failed: %w", err)
		}
		data["pic"] = picURL

		// Delete old image
		user, err := s.FindOne(ctx, id)
		if err == nil && user != nil && user.Pic != nil {
			parts := strings.Split(*user.Pic, "/")
			fileName := parts[len(parts)-1]
			if err := s.upload.Delete("user", fileName); err != nil {
				logger.Warn("Failed to delete old user image", "error", err, "file", fileName)
			}
		}
	}

	data["updateAt"] = time.Now()

	updatedUser, err := s.repo.Update(ctx, id, data)
	if err != nil {
		return nil, err
	}

	// Clear caches
	s.cache.Del(ctx, "user")
	s.cache.Del(ctx, fmt.Sprintf("user:%s", id))
	if updatedUser != nil && updatedUser.Username != "" {
		s.cache.Del(ctx, fmt.Sprintf("user:%s", updatedUser.Username))
	}

	return updatedUser, nil
}

func (s *userService) Remove(ctx context.Context, id string) (*domain.User, error) {
	// Check existence
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("user not found")
	}

	user, err := s.repo.Delete(ctx, id)
	if err != nil {
		return nil, err
	}

	// Delete associated image
	if user.Pic != nil && *user.Pic != "" {
		parts := strings.Split(*user.Pic, "/")
		fileName := parts[len(parts)-1]
		if err := s.upload.Delete("user", fileName); err != nil {
			logger.Warn("Error deleting user image", "error", err, "file", fileName)
		}
	}

	// Clear caches
	s.cache.Del(ctx, "user")
	s.cache.Del(ctx, fmt.Sprintf("user:%s", id))
	if user.Username != "" {
		s.cache.Del(ctx, fmt.Sprintf("user:%s", user.Username))
	}

	return user, nil
}

func buildUserConditions(caller *ports.JwtPayload) (map[string]any, []map[string]any, string) {
	var conditions map[string]any
	var notConditions []map[string]any
	var key string

	switch caller.Role {
	case "ADMIN":
		conditions = map[string]any{"hosId": caller.HosID}
		notConditions = []map[string]any{
			{"hosId": "HID-DEVELOPMENT"},
			{"role": "SERVICE"},
		}
		key = fmt.Sprintf("user:%s", caller.HosID)
	case "LEGACY_ADMIN":
		conditions = map[string]any{"hosId": caller.HosID}
		notConditions = []map[string]any{
			{"hosId": "HID-DEVELOPMENT"},
		}
		key = fmt.Sprintf("user:%s", caller.HosID)
	case "SERVICE":
		notConditions = []map[string]any{
			{"hosId": "HID-DEVELOPMENT"},
		}
		key = "user:HID-DEVELOPMENT"
	case "SUPER":
		key = "user"
	default:
		key = "user"
	}

	return conditions, notConditions, key
}
