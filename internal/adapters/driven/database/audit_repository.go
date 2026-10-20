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
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return log, nil
}

func (r *auditRepository) FindAll(ctx context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
	var logs []domain.AuditLog
	query := r.db.WithContext(ctx).Order("\"createAt\" DESC")

	if filter.Entity != "" {
		query = query.Where("\"entity\" = ?", filter.Entity)
	}
	if filter.Action != "" {
		query = query.Where("\"action\" = ?", filter.Action)
	}
	if filter.EntityID != "" {
		query = query.Where("\"entityId\" = ?", filter.EntityID)
	}
	if filter.ActorID != "" {
		query = query.Where("\"actorId\" = ?", filter.ActorID)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query = query.Limit(limit).Offset(filter.Offset)

	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return logs, nil
}
