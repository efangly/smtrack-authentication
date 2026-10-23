package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/google/uuid"
)

type userService struct {
	repo           ports.UserRepository
	tenantRepo     ports.TenantRepository
	departmentRepo ports.DepartmentRepository
	cache          ports.CachePort
	upload         ports.FileUploadPort
	hasher         ports.PasswordHasher
	audit          ports.AuditService
}

// NewUserService creates a new user service
func NewUserService(
	repo ports.UserRepository,
	tenantRepo ports.TenantRepository,
	departmentRepo ports.DepartmentRepository,
	cache ports.CachePort,
	upload ports.FileUploadPort,
	hasher ports.PasswordHasher,
	audit ports.AuditService,
) ports.UserService {
	return &userService{
		repo:           repo,
		tenantRepo:     tenantRepo,
		departmentRepo: departmentRepo,
		cache:          cache,
		upload:         upload,
		hasher:         hasher,
		audit:          audit,
	}
}

func (s *userService) Create(ctx context.Context, user *domain.User, caller *domain.Actor) (*domain.User, error) {
	// An unset role falls back to the least-privileged one; a set role must be
	// one we actually know about.
	if user.Role == "" {
		user.Role = domain.RoleGuest
	} else if !user.Role.IsValid() {
		return nil, fmt.Errorf("invalid role: %s", user.Role)
	}
	// Public registration (no caller) can only ever produce a GUEST without
	// departments; an authenticated caller is held to its role ceiling.
	if caller == nil {
		if user.Role != domain.RoleGuest {
			return nil, fmt.Errorf("you cannot assign role %s", user.Role)
		}
		if len(user.DepartmentIDs) > 0 {
			return nil, errors.New("you cannot assign departments")
		}
	} else if !caller.CanAssign(user.Role) {
		return nil, fmt.Errorf("you cannot assign role %s", user.Role)
	}
	if err := s.checkTenantInScope(ctx, caller, user.TenantID); err != nil {
		return nil, err
	}
	if err := s.checkDepartments(ctx, user.TenantID, user.DepartmentIDs); err != nil {
		return nil, err
	}

	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.Username = strings.ToLower(user.Username)
	if taken, err := s.repo.UsernameTaken(ctx, user.Username); err != nil {
		return nil, err
	} else if taken {
		return nil, errors.New("username already exists")
	}

	// Every path into Create carries a plain-text password — both the public
	// register flow (authService.Register passes it straight through) and the
	// authenticated POST /auth/user — so it must be hashed here, mirroring the
	// password branch in Update.
	hashedPassword, err := s.hasher.Hash(user.Password)
	if err != nil {
		return nil, err
	}
	user.Password = hashedPassword

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	result, err := s.repo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// Clear cache
	s.cache.Del(ctx, "user")
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionCreate, result.ID, caller, userSnapshot(result))
	return result, nil
}

func (s *userService) FindAll(ctx context.Context, caller *domain.Actor) ([]domain.User, error) {
	filter, key, ok := buildUserFilter(caller)
	if !ok {
		return []domain.User{}, nil
	}

	var cached []domain.User
	if hit, err := s.cache.Get(ctx, key, &cached); err == nil && hit {
		return cached, nil
	}

	users, err := s.repo.FindAll(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(users) > 0 {
		s.cache.Set(ctx, key, domain.UsersWithoutSecrets(users), listTTL)
	}

	return users, nil
}

func (s *userService) RecordLogin(ctx context.Context, id string) error {
	if err := s.repo.RecordLogin(ctx, id, time.Now()); err != nil {
		return err
	}
	// Listings embed lastLoginAt; let them refresh rather than serve a stale value for long.
	s.cache.Del(ctx, fmt.Sprintf("user:%s", id))
	return nil
}

func (s *userService) AccessReport(ctx context.Context, caller *domain.Actor) ([]domain.User, error) {
	if caller == nil || !caller.IsAdminTier() {
		return nil, errors.New("unauthorized")
	}
	filter, _, ok := buildUserFilter(caller)
	if !ok {
		return []domain.User{}, nil
	}
	// Straight from the database: a review must not see a cached copy.
	users, err := s.repo.FindAll(ctx, filter)
	if err != nil {
		return nil, err
	}
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionAccessReport, caller.ID, caller,
		map[string]any{"users": len(users)})
	return domain.UsersWithoutSecrets(users), nil
}

func (s *userService) FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error) {
	if caller == nil {
		return nil, errors.New("unauthorized")
	}
	if caller.ID != id && !caller.IsAdminTier() {
		return nil, errors.New("user not found")
	}

	user, err := s.findOneCached(ctx, id)
	if err != nil {
		return nil, err
	}
	if caller.ID != id && !canAdminTenant(caller, user.TenantID) {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *userService) findOneCached(ctx context.Context, id string) (*domain.User, error) {
	cacheKey := fmt.Sprintf("user:%s", id)

	var cached domain.User
	if hit, err := s.cache.Get(ctx, cacheKey, &cached); err == nil && hit {
		return &cached, nil
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	s.cache.Set(ctx, cacheKey, user.WithoutSecrets(), itemTTL)

	return user, nil
}

func (s *userService) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *userService) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	// Uncached on purpose: cached users have their password hash stripped, and
	// the auth flows need it.
	user, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) UsernameTaken(ctx context.Context, username string) (bool, error) {
	return s.repo.UsernameTaken(ctx, username)
}

func (s *userService) Update(ctx context.Context, id string, patch domain.UserPatch, file *domain.File, caller *domain.Actor) (*domain.User, error) {
	target, err := s.authorizeChange(ctx, id, caller)
	if err != nil {
		return nil, err
	}

	tenantID := target.TenantID
	if patch.TenantID != nil && *patch.TenantID != target.TenantID {
		if err := s.checkTenantInScope(ctx, caller, *patch.TenantID); err != nil {
			return nil, err
		}
		// Department links belong to one tenant, so moving a user means saying
		// which departments it has in the new one.
		if patch.DepartmentIDs == nil {
			return nil, errors.New("departmentIds is required when changing tenant")
		}
		tenantID = *patch.TenantID
	}
	if patch.DepartmentIDs != nil {
		if err := s.checkDepartments(ctx, tenantID, *patch.DepartmentIDs); err != nil {
			return nil, err
		}
	}

	if patch.Username != nil && *patch.Username != target.Username {
		if taken, err := s.repo.UsernameTaken(ctx, *patch.Username); err != nil {
			return nil, err
		} else if taken {
			return nil, errors.New("username already exists")
		}
	}

	if patch.Role != nil {
		role := *patch.Role
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
		picURL, err := s.upload.Upload(ctx, file, "user")
		if err != nil {
			return nil, fmt.Errorf("image upload failed: %w", err)
		}
		patch.Pic = &picURL

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
	if patch.Password != nil {
		if *patch.Password == "" {
			return nil, errors.New("invalid password")
		}
		hashed, err := s.hasher.Hash(*patch.Password)
		if err != nil {
			return nil, err
		}
		patch.Password = &hashed
	}

	updatedUser, err := s.repo.Update(ctx, id, patch)
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx, id, target.Username)
	if updatedUser != nil && updatedUser.Username != "" {
		s.cache.Del(ctx, fmt.Sprintf("user:%s", updatedUser.Username))
	}

	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionUpdate, id, caller, userPatchChanges(patch))
	return updatedUser, nil
}

// Remove soft-deletes a user. Its image is kept so a restore brings the whole
// record back; it is deleted when the user is purged.
func (s *userService) Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error) {
	if _, err := s.authorizeChange(ctx, id, caller); err != nil {
		return nil, err
	}

	user, err := s.repo.Delete(ctx, id, caller.ID)
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx, id, user.Username)
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionDelete, id, caller, userSnapshot(user))
	return user, nil
}

// Restore brings a soft-deleted user back under the same scope and role
// ceiling that let the caller delete it.
func (s *userService) Restore(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error) {
	if caller == nil {
		return nil, errors.New("unauthorized")
	}
	target, err := s.repo.FindAnyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if target == nil || target.DeletedAt == nil || !canAdminTenant(caller, target.TenantID) {
		return nil, errors.New("user not found")
	}
	if !caller.CanAssign(target.Role) {
		return nil, errors.New("you do not have permission to modify this user")
	}
	tenant, err := s.tenantRepo.FindByID(ctx, target.TenantID)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}

	user, err := s.repo.Restore(ctx, id)
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx, id, user.Username)
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionRestore, id, caller, userSnapshot(user))
	return user, nil
}

func (s *userService) invalidate(ctx context.Context, id, username string) {
	s.cache.Del(ctx, "user")
	s.cache.Del(ctx, fmt.Sprintf("user:%s", id))
	if username != "" {
		s.cache.Del(ctx, fmt.Sprintf("user:%s", username))
	}
}

// authorizeChange loads the user about to be updated or removed and checks
// that caller may act on it: anyone may act on their own record, but acting on
// somebody else's requires that user to be in the caller's tenant scope and
// within its role ceiling. A user outside the scope is reported as not found so
// its existence is not revealed.
// It reads the repository directly so the decision is never made on a stale
// cached role.
func (s *userService) authorizeChange(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error) {
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
	if caller.ID != id {
		if !canAdminTenant(caller, target.TenantID) {
			return nil, errors.New("user not found")
		}
		if !caller.CanAssign(target.Role) {
			return nil, errors.New("you do not have permission to modify this user")
		}
	}
	return target, nil
}

// checkTenantInScope rejects placing a user into a tenant that does not exist
// or is outside the caller's scope. A nil caller is public registration, which
// may not join a hidden tenant. A tenant that cannot be used reads as not found.
func (s *userService) checkTenantInScope(ctx context.Context, caller *domain.Actor, tenantID string) error {
	tenant, err := s.tenantRepo.FindByID(ctx, tenantID)
	if err != nil {
		return err
	}
	if tenant == nil {
		return errors.New("tenant not found")
	}
	if caller == nil {
		if tenant.Hidden {
			return errors.New("tenant not found")
		}
		return nil
	}
	if !canAdminTenant(caller, tenant.ID) {
		return errors.New("tenant not found")
	}
	return nil
}

// checkDepartments requires every department to exist and belong to tenantID.
func (s *userService) checkDepartments(ctx context.Context, tenantID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	found, err := s.departmentRepo.FindByIDs(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[string]domain.Department, len(found))
	for _, d := range found {
		byID[d.ID] = d
	}
	for _, id := range ids {
		if d, ok := byID[id]; !ok || d.TenantID != tenantID {
			return errors.New("department not found")
		}
	}
	return nil
}

// buildUserFilter is the user listing each role may see, and its cache key. ok
// is false when the caller's scope is empty (a tenant-scoped role whose token
// names no tenant), which must list nothing rather than everything.
func buildUserFilter(caller *domain.Actor) (filter ports.UserFilter, key string, ok bool) {
	switch caller.Role {
	case domain.RoleSuper:
		return ports.UserFilter{}, "user", true
	case domain.RoleService:
		return ports.UserFilter{ExcludeHiddenTenants: true}, "user:service", true
	case domain.RoleAdmin:
		if caller.TenantID == "" {
			return filter, "", false
		}
		return ports.UserFilter{
			TenantID:             caller.TenantID,
			ExcludeHiddenTenants: true,
			ExcludeRoles:         []domain.Role{domain.RoleService},
		}, "user:tenant:" + caller.TenantID, true
	case domain.RoleLegacyAdmin:
		if caller.TenantID == "" {
			return filter, "", false
		}
		return ports.UserFilter{TenantID: caller.TenantID, ExcludeHiddenTenants: true}, "user:tenant:" + caller.TenantID + ":legacy", true
	}
	if caller.TenantID == "" {
		return filter, "", false
	}
	return ports.UserFilter{TenantID: caller.TenantID}, "user:tenant:" + caller.TenantID + ":member", true
}
