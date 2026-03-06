package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
	"github.com/tng-coop/auth-service/pkg/logger"
)

type wardService struct {
	repo      ports.WardRepository
	cache     ports.CachePort
	messaging ports.MessagingPort
}

// NewWardService creates a new ward service
func NewWardService(
	repo ports.WardRepository,
	cache ports.CachePort,
	messaging ports.MessagingPort,
) ports.WardService {
	return &wardService{
		repo:      repo,
		cache:     cache,
		messaging: messaging,
	}
}

func (s *wardService) Create(ctx context.Context, ward *domain.Ward) (*domain.Ward, error) {
	if ward.ID == "" {
		ward.ID = uuid.New().String()
	}

	now := time.Now()
	ward.CreatedAt = now
	ward.UpdatedAt = now

	result, err := s.repo.Create(ctx, ward)
	if err != nil {
		return nil, err
	}

	// Clear relevant caches
	s.cache.Del(ctx, "hospital")
	s.cache.Del(ctx, "ward")

	return result, nil
}

func (s *wardService) FindAll(ctx context.Context, caller *ports.JwtPayload) ([]domain.Ward, error) {
	conditions, notConditions, key := buildWardConditions(caller)

	// Check cache
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var wards []domain.Ward
		if err := json.Unmarshal([]byte(cached), &wards); err == nil {
			return wards, nil
		}
	}

	wards, err := s.repo.FindAll(ctx, conditions, notConditions)
	if err != nil {
		return nil, err
	}

	if len(wards) > 0 {
		data, _ := json.Marshal(wards)
		s.cache.Set(ctx, key, string(data), 3600*10)
	}

	return wards, nil
}

func (s *wardService) FindOne(ctx context.Context, id string) (*domain.Ward, error) {
	ward, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ward == nil {
		return nil, errors.New("ward not found")
	}
	return ward, nil
}

func (s *wardService) Update(ctx context.Context, id string, data map[string]any) (*domain.Ward, error) {
	data["updateAt"] = time.Now()

	ward, err := s.repo.Update(ctx, id, data)
	if err != nil {
		return nil, err
	}

	// Send notifications to appropriate services
	if ward != nil {
		notification := map[string]string{
			"id":   ward.ID,
			"name": ward.WardName,
		}
		if ward.Type != nil && *ward.Type == domain.WardTypeLegacy {
			if err := s.messaging.SendToLegacy("update-ward", notification); err != nil {
				logger.Warn("Failed to send ward update to legacy", "error", err, "wardId", ward.ID)
			}
		} else {
			if err := s.messaging.SendToDevice("update-ward", notification); err != nil {
				logger.Warn("Failed to send ward update to device", "error", err, "wardId", ward.ID)
			}
		}
	}

	// Clear caches
	s.cache.Del(ctx, "hospital")
	s.cache.Del(ctx, "ward")

	return ward, nil
}

func (s *wardService) Remove(ctx context.Context, id string) (string, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if existing == nil {
		return "", errors.New("ward not found")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return "", err
	}

	// Clear caches
	s.cache.Del(ctx, "hospital")
	s.cache.Del(ctx, "ward")

	return "Ward deleted successfully", nil
}

func buildWardConditions(caller *ports.JwtPayload) (map[string]any, []map[string]any, string) {
	var conditions map[string]any
	var notConditions []map[string]any
	var key string

	switch caller.Role {
	case "ADMIN", "LEGACY_ADMIN":
		conditions = map[string]any{"hosId": caller.HosID}
		notConditions = []map[string]any{
			{"hosId": "HID-DEVELOPMENT"},
		}
		key = fmt.Sprintf("ward:%s", caller.HosID)
	case "SERVICE":
		notConditions = []map[string]any{
			{"hosId": "HID-DEVELOPMENT"},
		}
		key = "ward:HID-DEVELOPMENT"
	case "SUPER":
		key = "ward"
	default:
		key = "ward"
	}

	return conditions, notConditions, key
}
