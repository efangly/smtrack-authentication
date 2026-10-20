package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	repo   ports.UserRepository
	cache  ports.CachePort
	upload ports.FileUploadPort
	audit  ports.AuditService
}

// NewUserService creates a new user service
func NewUserService(repo ports.UserRepository, cache ports.CachePort, upload ports.FileUploadPort, audit ports.AuditService) ports.UserService {
	return &userService{repo: repo, cache: cache, upload: upload, audit: audit}
}

func (s *userService) Create(ctx context.Context, user *domain.User, caller *ports.JwtPayload) (*domain.User, error) {
	// An unset role falls back to the least-privileged one; a set role must be
	// one we actually know about.
	if user.Role == "" {
		user.Role = domain.RoleGuest
	} else if !user.Role.IsValid() {
		return nil, fmt.Errorf("invalid role: %s", user.Role)
	}
	// Public registration (no caller) can only ever produce a GUEST; an
	// authenticated caller is held to its role ceiling.
	if caller == nil {
		if user.Role != domain.RoleGuest {
			return nil, fmt.Errorf("you cannot assign role %s", user.Role)
		}
	} else if !caller.CanAssign(user.Role) {
		return nil, fmt.Errorf("you cannot assign role %s", user.Role)
	}
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.Username = strings.ToLower(user.Username)

	// Every path into Create carries a plain-text password (authService.Register
	// hashes its own copy separately via ToPublicUser, but the authenticated
	// POST /auth/user path reaches here directly), so it must be hashed here —
	// mirroring the password branch in Update.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	user.Password = string(hashedPassword)

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	result, err := s.repo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// Clear cache
	s.cache.Del(ctx, "user")
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionCreate, result.ID, caller, result)
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

func (s *userService) Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader, caller *ports.JwtPayload) (*domain.User, error) {
	target, err := s.authorizeChange(ctx, id, caller)
	if err != nil {
		return nil, err
	}

	if raw, ok := data["role"]; ok {
		role := toRole(raw)
		if role != target.Role {
			if caller.ID == id {
				return nil, errors.New("you cannot change your own role")
			}
			if !caller.CanAssign(role) {
				return nil, fmt.Errorf("you cannot assign role %s", role)
			}
		}
	}

	if file != nil {
		picURL, err := s.upload.Upload(file, "user")
		if err != nil {
			return nil, fmt.Errorf("image upload failed: %w", err)
		}
		data["pic"] = picURL

		// Delete old image
		if target.Pic != nil {
			parts := strings.Split(*target.Pic, "/")
			fileName := parts[len(parts)-1]
			if err := s.upload.Delete("user", fileName); err != nil {
				logger.Warn("Failed to delete old user image", "error", err, "file", fileName)
			}
		}
	}

	// A password must never reach the database in plain text. This is the single
	// place that hashes on update, so callers pass the plain value.
	if raw, ok := data["password"]; ok {
		plain, ok := raw.(string)
		if !ok || plain == "" {
			return nil, errors.New("invalid password")
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		data["password"] = string(hashed)
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

	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionUpdate, id, caller, data)
	return updatedUser, nil
}

func (s *userService) Remove(ctx context.Context, id string, caller *ports.JwtPayload) (*domain.User, error) {
	if _, err := s.authorizeChange(ctx, id, caller); err != nil {
		return nil, err
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

	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionDelete, id, caller, user)
	return user, nil
}

// authorizeChange loads the user about to be updated or removed and checks
// that caller may act on it: anyone may act on their own record, but acting on
// somebody else's requires a role ceiling at or above that user's current role.
// It reads the repository directly so the decision is never made on a stale
// cached role.
func (s *userService) authorizeChange(ctx context.Context, id string, caller *ports.JwtPayload) (*domain.User, error) {
	if caller == nil {
		return nil, errors.New("unauthorized")
	}
	target, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, errors.New("user not found")
	}
	if caller.ID != id && !caller.CanAssign(target.Role) {
		return nil, errors.New("you do not have permission to modify this user")
	}
	return target, nil
}

// toRole reads a role out of a partial-update map, where it may arrive as a
// plain string (from the DTO) or already as a domain.Role.
func toRole(v any) domain.Role {
	switch r := v.(type) {
	case domain.Role:
		return r
	case string:
		return domain.Role(r)
	}
	return ""
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
