package services

import (
	"context"
	"encoding/json"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/google/uuid"
)

type auditService struct {
	repo ports.AuditRepository
}

// NewAuditService creates a new audit log service
func NewAuditService(repo ports.AuditRepository) ports.AuditService {
	return &auditService{repo: repo}
}

func (s *auditService) Record(
	ctx context.Context,
	entity domain.AuditEntity,
	action domain.AuditAction,
	entityID string,
	caller *domain.Actor,
	changes any,
) {
	data, _ := json.Marshal(sanitizeAuditChanges(changes))

	log := &domain.AuditLog{
		ID:       uuid.New().String(),
		Entity:   entity,
		EntityID: entityID,
		Action:   action,
		Changes:  string(data),
	}
	if caller != nil {
		log.ActorID = caller.ID
		log.ActorName = caller.Name
	} else {
		log.ActorName = "self"
	}

	if _, err := s.repo.Create(ctx, log); err != nil {
		logger.Warn("Failed to write audit log",
			"error", err, "entity", entity, "entityId", entityID, "action", action)
	}
}

func (s *auditService) FindAll(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
	return s.repo.FindAll(ctx, filter)
}

// sanitizeAuditChanges strips a "password" key before changes is persisted.
// A domain entity's Password field already carries json:"-" so a struct
// (the Create/Delete case) is safe as-is; a raw partial-update map (the
// Update case) is not guaranteed to be, so it is scrubbed explicitly.
func sanitizeAuditChanges(changes any) any {
	m, ok := changes.(map[string]any)
	if !ok {
		return changes
	}
	sanitized := make(map[string]any, len(m))
	for k, v := range m {
		if k == "password" {
			continue
		}
		sanitized[k] = v
	}
	return sanitized
}
