package services

import (
	"context"
	"errors"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/google/uuid"
)

type departmentService struct {
	repo       ports.DepartmentRepository
	tenantRepo ports.TenantRepository
	cache      ports.CachePort
	messaging  ports.MessagingPort
	audit      ports.AuditService
}

// NewDepartmentService creates a new department service
func NewDepartmentService(
	repo ports.DepartmentRepository,
	tenantRepo ports.TenantRepository,
	cache ports.CachePort,
	messaging ports.MessagingPort,
	audit ports.AuditService,
) ports.DepartmentService {
	return &departmentService{
		repo:       repo,
		tenantRepo: tenantRepo,
		cache:      cache,
		messaging:  messaging,
		audit:      audit,
	}
}

func (s *departmentService) Create(ctx context.Context, department *domain.Department, caller *domain.Actor) (*domain.Department, error) {
	if caller == nil {
		return nil, errors.New("unauthorized")
	}
	if err := s.checkTenant(ctx, caller, department.TenantID); err != nil {
		return nil, err
	}
	if err := s.checkName(ctx, department.TenantID, department.Name, ""); err != nil {
		return nil, err
	}
	if department.ID == "" {
		department.ID = uuid.New().String()
	}

	now := time.Now()
	department.CreatedAt = now
	department.UpdatedAt = now

	result, err := s.repo.Create(ctx, department)
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx)
	s.audit.Record(ctx, domain.AuditEntityDepartment, domain.AuditActionCreate, result.ID, caller, departmentSnapshot(result))
	return result, nil
}

func (s *departmentService) FindAll(ctx context.Context, caller *domain.Actor) ([]domain.Department, error) {
	filter, key, ok := buildDepartmentFilter(caller)
	if !ok {
		return []domain.Department{}, nil
	}

	var cached []domain.Department
	if hit, err := s.cache.Get(ctx, key, &cached); err == nil && hit {
		return cached, nil
	}

	departments, err := s.repo.FindAll(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(departments) > 0 {
		s.cache.Set(ctx, key, departments, listTTL)
	}

	return departments, nil
}

func (s *departmentService) FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error) {
	if caller == nil {
		return nil, errors.New("unauthorized")
	}
	department, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Out of scope reads as not found, as in tenantService.FindOne.
	if department == nil || !canSeeDepartment(caller, department) {
		return nil, errors.New("department not found")
	}
	return department, nil
}

func (s *departmentService) Update(ctx context.Context, id string, patch domain.DepartmentPatch, caller *domain.Actor) (*domain.Department, error) {
	existing, err := s.FindOne(ctx, id, caller)
	if err != nil {
		return nil, err
	}
	if patch.Name != nil && *patch.Name != existing.Name {
		if err := s.checkName(ctx, existing.TenantID, *patch.Name, id); err != nil {
			return nil, err
		}
	}

	department, err := s.repo.Update(ctx, id, patch)
	if err != nil {
		return nil, err
	}

	// Send notifications to appropriate services
	if department != nil {
		notification := map[string]string{
			"id":   department.ID,
			"name": department.Name,
		}
		if department.Legacy {
			if err := s.messaging.SendToLegacy("update-department", notification); err != nil {
				logger.Warn("Failed to send department update to legacy", "error", err, "departmentId", department.ID)
			}
		} else {
			if err := s.messaging.SendToDevice("update-department", notification); err != nil {
				logger.Warn("Failed to send department update to device", "error", err, "departmentId", department.ID)
			}
		}
	}

	s.invalidate(ctx)
	s.audit.Record(ctx, domain.AuditEntityDepartment, domain.AuditActionUpdate, id, caller, departmentPatchChanges(patch))
	return department, nil
}

// Remove soft-deletes a department. The users linked to it keep their links so
// a restore brings the membership back; reads simply skip a deleted department.
func (s *departmentService) Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error) {
	if _, err := s.FindOne(ctx, id, caller); err != nil {
		return nil, err
	}

	department, err := s.repo.Delete(ctx, id, caller.ID)
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx)
	s.audit.Record(ctx, domain.AuditEntityDepartment, domain.AuditActionDelete, id, caller, departmentSnapshot(department))
	return department, nil
}

func (s *departmentService) Restore(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error) {
	if caller == nil {
		return nil, errors.New("unauthorized")
	}
	existing, err := s.repo.FindAnyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.DeletedAt == nil || !canSeeDepartment(caller, existing) {
		return nil, errors.New("department not found")
	}
	// A department cannot come back under a deleted tenant, or under a name
	// another live department of its tenant has taken since.
	if err := s.checkTenant(ctx, caller, existing.TenantID); err != nil {
		return nil, err
	}
	if err := s.checkName(ctx, existing.TenantID, existing.Name, id); err != nil {
		return nil, err
	}

	department, err := s.repo.Restore(ctx, id)
	if err != nil {
		return nil, err
	}

	s.invalidate(ctx)
	s.audit.Record(ctx, domain.AuditEntityDepartment, domain.AuditActionRestore, id, caller, departmentSnapshot(department))
	return department, nil
}

// checkTenant requires a live tenant the caller may see.
func (s *departmentService) checkTenant(ctx context.Context, caller *domain.Actor, tenantID string) error {
	tenant, err := s.tenantRepo.FindByID(ctx, tenantID)
	if err != nil {
		return err
	}
	if tenant == nil || !canSeeTenant(caller, tenant) {
		return errors.New("tenant not found")
	}
	return nil
}

func (s *departmentService) checkName(ctx context.Context, tenantID, name, exceptID string) error {
	taken, err := s.repo.NameTaken(ctx, tenantID, name, exceptID)
	if err != nil {
		return err
	}
	if taken {
		return errors.New("department name already exists")
	}
	return nil
}

// invalidate clears every cache that embeds department data: tenant and user
// listings carry their departments (ADR 0002).
func (s *departmentService) invalidate(ctx context.Context) {
	s.cache.Del(ctx, "tenant")
	s.cache.Del(ctx, "department")
	s.cache.Del(ctx, "user")
}

// buildDepartmentFilter is the department listing each role may see, and its
// cache key. ok is false when the caller's scope is empty.
func buildDepartmentFilter(caller *domain.Actor) (filter ports.DepartmentFilter, key string, ok bool) {
	switch caller.Role {
	case domain.RoleSuper:
		return ports.DepartmentFilter{}, "department", true
	case domain.RoleService:
		return ports.DepartmentFilter{ExcludeHiddenTenants: true}, "department:service", true
	case domain.RoleAdmin, domain.RoleLegacyAdmin:
		if caller.TenantID == "" {
			return filter, "", false
		}
		return ports.DepartmentFilter{TenantID: caller.TenantID, ExcludeHiddenTenants: true}, "department:tenant:" + caller.TenantID, true
	}
	if caller.TenantID == "" || len(caller.DepartmentIDs) == 0 {
		return filter, "", false
	}
	return ports.DepartmentFilter{
		TenantID:   caller.TenantID,
		IDs:        caller.DepartmentIDs,
		LimitToIDs: true,
	}, "department:user:" + caller.ID, true
}
