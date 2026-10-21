package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/google/uuid"
)

type hospitalService struct {
	repo      ports.HospitalRepository
	cache     ports.CachePort
	upload    ports.FileUploadPort
	messaging ports.MessagingPort
	audit     ports.AuditService
}

// NewHospitalService creates a new hospital service
func NewHospitalService(
	repo ports.HospitalRepository,
	cache ports.CachePort,
	upload ports.FileUploadPort,
	messaging ports.MessagingPort,
	audit ports.AuditService,
) ports.HospitalService {
	return &hospitalService{
		repo:      repo,
		cache:     cache,
		upload:    upload,
		messaging: messaging,
		audit:     audit,
	}
}

func (s *hospitalService) Create(ctx context.Context, hospital *domain.Hospital, file *domain.File, caller *domain.Actor) (*domain.Hospital, error) {
	if hospital.ID == "" {
		hospital.ID = uuid.New().String()
	}

	if file != nil {
		picURL, err := s.upload.Upload(ctx, file, "hospital")
		if err != nil {
			return nil, fmt.Errorf("image upload failed: %w", err)
		}
		hospital.HosPic = &picURL
	}

	now := time.Now()
	hospital.CreatedAt = now
	hospital.UpdatedAt = now

	result, err := s.repo.Create(ctx, hospital)
	if err != nil {
		return nil, err
	}

	s.cache.Del(ctx, "hospital")
	s.audit.Record(ctx, domain.AuditEntityHospital, domain.AuditActionCreate, result.ID, caller, result)
	return result, nil
}

func (s *hospitalService) FindAll(ctx context.Context, caller *domain.Actor) ([]domain.Hospital, error) {
	conditions, notConditions, key := buildHospitalConditions(caller)

	// Check cache
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var hospitals []domain.Hospital
		if err := json.Unmarshal([]byte(cached), &hospitals); err == nil {
			return hospitals, nil
		}
	}

	hospitals, err := s.repo.FindAll(ctx, conditions, notConditions)
	if err != nil {
		return nil, err
	}

	if len(hospitals) > 0 {
		data, _ := json.Marshal(hospitals)
		s.cache.Set(ctx, key, string(data), 3600*10)
	}

	return hospitals, nil
}

func (s *hospitalService) FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.Hospital, error) {
	if caller == nil {
		return nil, errors.New("unauthorized")
	}
	hospital, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Out of scope reads as not found, so a lookup by id never confirms a
	// hospital the caller's listing would hide.
	if hospital == nil || !canSeeHospital(caller, hospital) {
		return nil, errors.New("hospital not found")
	}
	return hospital, nil
}

func (s *hospitalService) Update(ctx context.Context, id string, data map[string]any, file *domain.File, caller *domain.Actor) (*domain.Hospital, error) {
	if file != nil {
		picURL, err := s.upload.Upload(ctx, file, "hospital")
		if err != nil {
			return nil, fmt.Errorf("image upload failed: %w", err)
		}
		data["hosPic"] = picURL

		// Delete old image
		existing, err := s.repo.FindByID(ctx, id)
		if err == nil && existing != nil && existing.HosPic != nil {
			parts := strings.Split(*existing.HosPic, "/")
			fileName := parts[len(parts)-1]
			if err := s.upload.Delete("hospital", fileName); err != nil {
				logger.Warn("Failed to delete old hospital image", "error", err, "file", fileName)
			}
		}
	}

	data["updateAt"] = time.Now()

	hospital, err := s.repo.Update(ctx, id, data)
	if err != nil {
		return nil, err
	}

	// Send notifications to other services
	if hospital != nil {
		notification := map[string]string{
			"id":   hospital.ID,
			"name": hospital.HosName,
		}
		if err := s.messaging.SendToDevice("update-hospital", notification); err != nil {
			logger.Warn("Failed to send hospital update to device", "error", err, "hospitalId", hospital.ID)
		}
		if err := s.messaging.SendToLegacy("update-hospital", notification); err != nil {
			logger.Warn("Failed to send hospital update to legacy", "error", err, "hospitalId", hospital.ID)
		}
	}

	s.cache.Del(ctx, "hospital")
	// User and ward listings embed hospital data, so they go stale too.
	s.cache.Del(ctx, "ward")
	s.cache.Del(ctx, "user")
	s.audit.Record(ctx, domain.AuditEntityHospital, domain.AuditActionUpdate, id, caller, data)
	return hospital, nil
}

func (s *hospitalService) Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.Hospital, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("hospital not found")
	}

	hospital, err := s.repo.Delete(ctx, id)
	if err != nil {
		return nil, err
	}

	// Delete associated image
	if hospital.HosPic != nil && *hospital.HosPic != "" {
		parts := strings.Split(*hospital.HosPic, "/")
		fileName := parts[len(parts)-1]
		if err := s.upload.Delete("hospital", fileName); err != nil {
			logger.Warn("Error deleting hospital image", "error", err, "file", fileName)
		}
	}

	s.cache.Del(ctx, "hospital")
	// User and ward listings embed hospital data, so they go stale too.
	s.cache.Del(ctx, "ward")
	s.cache.Del(ctx, "user")
	s.audit.Record(ctx, domain.AuditEntityHospital, domain.AuditActionDelete, id, caller, hospital)
	return hospital, nil
}

func buildHospitalConditions(caller *domain.Actor) (map[string]any, []map[string]any, string) {
	var conditions map[string]any
	var notConditions []map[string]any
	var key string

	switch caller.Role {
	case domain.RoleAdmin, domain.RoleLegacyAdmin:
		conditions = map[string]any{"id": caller.HosID}
		notConditions = []map[string]any{
			{"id": "HID-DEVELOPMENT"},
		}
		key = fmt.Sprintf("hospital:%s", caller.HosID)
	case domain.RoleService:
		notConditions = []map[string]any{
			{"id": "HID-DEVELOPMENT"},
		}
		key = "hospital:HID-DEVELOPMENT"
	case domain.RoleSuper:
		key = "hospital"
	case domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest:
		// An empty wardId would filter on "" and, worse, make every ward-less
		// caller share the cache key "hospital:".
		if caller.WardID == "" {
			key = "hospital"
			break
		}
		conditions = map[string]any{"wardId": caller.WardID}
		key = fmt.Sprintf("hospital:%s", caller.WardID)
	default:
		key = "hospital"
	}

	return conditions, notConditions, key
}
