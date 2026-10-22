package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type departmentRepository struct {
	db *gorm.DB
}

// NewDepartmentRepository creates a new department repository adapter
func NewDepartmentRepository(db *gorm.DB) ports.DepartmentRepository {
	return &departmentRepository{db: db}
}

func (r *departmentRepository) Create(ctx context.Context, department *domain.Department) (*domain.Department, error) {
	model := departmentModelFrom(department)
	if err := r.db.WithContext(ctx).Clauses(clause.Returning{}).Create(&model).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return r.FindByID(ctx, model.ID)
}

func (r *departmentRepository) FindAll(ctx context.Context, filter ports.DepartmentFilter) ([]domain.Department, error) {
	var models []departmentModel
	query := r.db.WithContext(ctx).Preload("Tenant").Order("departments.seq ASC")

	if filter.TenantID != "" {
		query = query.Where("departments.tenant_id = ?", filter.TenantID)
	}
	if filter.LimitToIDs {
		if len(filter.IDs) == 0 {
			return []domain.Department{}, nil
		}
		query = query.Where("departments.id IN ?", filter.IDs)
	}
	if filter.ExcludeHiddenTenants {
		query = query.Where("departments.tenant_id NOT IN (?)",
			r.db.Model(&tenantModel{}).Unscoped().Select("id").Where("hidden = ?", true))
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	out := make([]domain.Department, len(models))
	for i, m := range models {
		out[i] = m.toDomain()
	}
	return out, nil
}

func (r *departmentRepository) FindByID(ctx context.Context, id string) (*domain.Department, error) {
	return r.findOne(r.db.WithContext(ctx).Preload("Tenant").Where("departments.id = ?", id))
}

func (r *departmentRepository) FindAnyByID(ctx context.Context, id string) (*domain.Department, error) {
	return r.findOne(r.db.WithContext(ctx).Unscoped().Preload("Tenant").Where("departments.id = ?", id))
}

func (r *departmentRepository) findOne(query *gorm.DB) (*domain.Department, error) {
	var model departmentModel
	if err := query.First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	department := model.toDomain()
	return &department, nil
}

func (r *departmentRepository) FindByIDs(ctx context.Context, ids []string) ([]domain.Department, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var models []departmentModel
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	out := make([]domain.Department, len(models))
	for i, m := range models {
		out[i] = m.toDomain()
	}
	return out, nil
}

func (r *departmentRepository) NameTaken(ctx context.Context, tenantID, name, exceptID string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&departmentModel{}).
		Where("tenant_id = ? AND name = ?", tenantID, name)
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("database error: %w", err)
	}
	return count > 0, nil
}

func (r *departmentRepository) Update(ctx context.Context, id string, patch domain.DepartmentPatch) (*domain.Department, error) {
	if err := r.db.WithContext(ctx).Model(&departmentModel{}).Where("id = ?", id).Updates(departmentColumns(patch)).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return r.FindByID(ctx, id)
}

func (r *departmentRepository) Delete(ctx context.Context, id string, deletedBy string) (*domain.Department, error) {
	department, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if department == nil {
		return nil, errors.New("department not found")
	}
	if err := softDelete(ctx, r.db, &departmentModel{}, id, deletedBy); err != nil {
		return nil, err
	}
	return department, nil
}

func (r *departmentRepository) Restore(ctx context.Context, id string) (*domain.Department, error) {
	if err := restoreRow(ctx, r.db, &departmentModel{}, id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
