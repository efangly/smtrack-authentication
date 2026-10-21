package database

import (
	"context"
	"fmt"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type wardRepository struct {
	db *gorm.DB
}

// NewWardRepository creates a new ward repository adapter
func NewWardRepository(db *gorm.DB) ports.WardRepository {
	return &wardRepository{db: db}
}

func (r *wardRepository) Create(ctx context.Context, ward *domain.Ward) (*domain.Ward, error) {
	if err := r.db.WithContext(ctx).Create(ward).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	// Reload with hospital relation
	return r.FindByID(ctx, ward.ID)
}

func (r *wardRepository) FindAll(ctx context.Context, filter ports.WardFilter) ([]domain.Ward, error) {
	var wards []domain.Ward
	query := r.db.WithContext(ctx).
		Preload("Hospital").
		Order("\"wardSeq\" ASC")

	if filter.HosID != "" {
		query = query.Where("\"hosId\" = ?", filter.HosID)
	}
	if len(filter.ExcludeHosIDs) > 0 {
		query = query.Where("\"hosId\" NOT IN ?", filter.ExcludeHosIDs)
	}

	if err := query.Find(&wards).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return wards, nil
}

func (r *wardRepository) FindByID(ctx context.Context, id string) (*domain.Ward, error) {
	var ward domain.Ward
	err := r.db.WithContext(ctx).
		Preload("Hospital").
		Where("\"id\" = ?", id).
		First(&ward).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &ward, nil
}

func (r *wardRepository) Update(ctx context.Context, id string, patch domain.WardPatch) (*domain.Ward, error) {
	var ward domain.Ward
	if err := r.db.WithContext(ctx).Model(&ward).Clauses(clause.Returning{}).Where("\"id\" = ?", id).Updates(wardColumns(patch)).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return r.FindByID(ctx, id)
}

func (r *wardRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).Delete(&domain.Ward{}).Error; err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	return nil
}
