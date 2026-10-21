package database

import (
	"context"
	"fmt"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
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
		return nil, fmt.Errorf("database error: %w", err)
	}
	return hospital, nil
}

func (r *hospitalRepository) FindAll(ctx context.Context, filter ports.HospitalFilter) ([]domain.Hospital, error) {
	var hospitals []domain.Hospital
	query := r.db.WithContext(ctx).
		Preload("Wards", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"wardSeq\" ASC")
		}).
		Order("\"hosSeq\" ASC")

	if filter.ID != "" {
		query = query.Where("\"id\" = ?", filter.ID)
	}
	if filter.WardID != "" {
		query = query.Where(`EXISTS (SELECT 1 FROM "Wards" WHERE "Wards"."hosId" = "Hospitals".id AND "Wards".id = ?)`, filter.WardID)
	}
	if len(filter.ExcludeIDs) > 0 {
		query = query.Where("\"id\" NOT IN ?", filter.ExcludeIDs)
	}

	if err := query.Find(&hospitals).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
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
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &hospital, nil
}

func (r *hospitalRepository) Update(ctx context.Context, id string, patch domain.HospitalPatch) (*domain.Hospital, error) {
	var hospital domain.Hospital
	if err := r.db.WithContext(ctx).Model(&hospital).Clauses(clause.Returning{}).Where("\"id\" = ?", id).Updates(hospitalColumns(patch)).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return r.FindByID(ctx, id)
}

func (r *hospitalRepository) Delete(ctx context.Context, id string) (*domain.Hospital, error) {
	var hospital domain.Hospital
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).First(&hospital).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).Delete(&domain.Hospital{}).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &hospital, nil
}
