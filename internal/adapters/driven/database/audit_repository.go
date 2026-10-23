package database

import (
	"context"
	"fmt"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"gorm.io/gorm"
)

type auditRepository struct {
	db *gorm.DB
}

// NewAuditRepository creates a new audit log repository adapter
func NewAuditRepository(db *gorm.DB) ports.AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Create(ctx context.Context, log *domain.AuditLog) (*domain.AuditLog, error) {
	model := auditLogModelFrom(log)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	created := model.toDomain()
	return &created, nil
}

func (r *auditRepository) FindAll(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
	var models []auditLogModel
	query := r.db.WithContext(ctx).Order("created_at DESC")

	if filter.Entity != "" {
		query = query.Where("entity = ?", filter.Entity)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.EntityID != "" {
		query = query.Where("entity_id = ?", filter.EntityID)
	}
	if filter.ActorID != "" {
		query = query.Where("actor_id = ?", filter.ActorID)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query = query.Limit(limit).Offset(filter.Offset)

	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	logs := make([]domain.AuditLog, len(models))
	for i, m := range models {
		logs[i] = m.toDomain()
	}
	return logs, nil
}
