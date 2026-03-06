package database

import (
	"context"

	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type hospitalRepository struct {
	db *gorm.DB
}

// NewHospitalRepository creates a new hospital repository adapter
func NewHospitalRepository(db *gorm.DB) ports.HospitalRepository {
	return &hospitalRepository{db: db}
}

func (r *hospitalRepository) Create(ctx context.Context, hospital *domain.Hospital) (*domain.Hospital, error) {
	if err := r.db.WithContext(ctx).Create(hospital).Error; err != nil {
		return nil, err
	}
	return hospital, nil
}

func (r *hospitalRepository) FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.Hospital, error) {
	var hospitals []domain.Hospital
	query := r.db.WithContext(ctx).
		Preload("Wards", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"wardSeq\" ASC")
		}).
		Order("\"hosSeq\" ASC")

	if conditions != nil {
		if id, ok := conditions["id"]; ok {
			query = query.Where("\"id\" = ?", id)
		}
	}

	for _, nc := range notConditions {
		if id, ok := nc["id"]; ok {
			query = query.Where("\"id\" != ?", id)
		}
	}

	if err := query.Find(&hospitals).Error; err != nil {
		return nil, err
	}
	return hospitals, nil
}

func (r *hospitalRepository) FindByID(ctx context.Context, id string) (*domain.Hospital, error) {
	var hospital domain.Hospital
	err := r.db.WithContext(ctx).
		Preload("Wards", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"wardSeq\" ASC")
		}).
		Where("\"id\" = ?", id).
		First(&hospital).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &hospital, nil
}

func (r *hospitalRepository) Update(ctx context.Context, id string, data map[string]any) (*domain.Hospital, error) {
	var hospital domain.Hospital
	if err := r.db.WithContext(ctx).Model(&hospital).Clauses(clause.Returning{}).Where("\"id\" = ?", id).Updates(data).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *hospitalRepository) Delete(ctx context.Context, id string) (*domain.Hospital, error) {
	var hospital domain.Hospital
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).First(&hospital).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).Delete(&domain.Hospital{}).Error; err != nil {
		return nil, err
	}
	return &hospital, nil
}
