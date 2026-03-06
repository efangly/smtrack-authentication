package database

import (
	"context"

	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
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
		return nil, err
	}
	// Reload with hospital relation
	return r.FindByID(ctx, ward.ID)
}

func (r *wardRepository) FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.Ward, error) {
	var wards []domain.Ward
	query := r.db.WithContext(ctx).
		Preload("Hospital").
		Order("\"wardSeq\" ASC")

	if conditions != nil {
		if hosID, ok := conditions["hosId"]; ok {
			query = query.Where("\"hosId\" = ?", hosID)
		}
	}

	for _, nc := range notConditions {
		if hosID, ok := nc["hosId"]; ok {
			query = query.Where("\"hosId\" != ?", hosID)
		}
	}

	if err := query.Find(&wards).Error; err != nil {
		return nil, err
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
		return nil, err
	}
	return &ward, nil
}

func (r *wardRepository) Update(ctx context.Context, id string, data map[string]any) (*domain.Ward, error) {
	var ward domain.Ward
	if err := r.db.WithContext(ctx).Model(&ward).Clauses(clause.Returning{}).Where("\"id\" = ?", id).Updates(data).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *wardRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).Delete(&domain.Ward{}).Error; err != nil {
		return err
	}
	return nil
}
