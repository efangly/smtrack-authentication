package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type tenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository creates a new tenant repository adapter
func NewTenantRepository(db *gorm.DB) ports.TenantRepository {
	return &tenantRepository{db: db}
}

// withTenantRelations loads the label profile and the live departments.
func withTenantRelations(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Profile").
		Preload("Departments", func(db *gorm.DB) *gorm.DB { return db.Order("departments.seq ASC") })
}

func (r *tenantRepository) Create(ctx context.Context, tenant *domain.Tenant) (*domain.Tenant, error) {
	model := tenantModelFrom(tenant)
	if err := r.db.WithContext(ctx).Clauses(clause.Returning{}).Create(&model).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return r.FindByID(ctx, model.ID)
}

func (r *tenantRepository) FindAll(ctx context.Context, filter ports.TenantFilter) ([]domain.Tenant, error) {
	var models []tenantModel
	query := withTenantRelations(r.db.WithContext(ctx)).Order("tenants.seq ASC")

	if filter.ID != "" {
		query = query.Where("tenants.id = ?", filter.ID)
	}
	if filter.ExcludeHidden {
		query = query.Where("tenants.hidden = ?", false)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	out := make([]domain.Tenant, len(models))
	for i, m := range models {
		out[i] = m.toDomain()
	}
	return out, nil
}

func (r *tenantRepository) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	return r.findOne(withTenantRelations(r.db.WithContext(ctx)).Where("tenants.id = ?", id))
}

func (r *tenantRepository) FindAnyByID(ctx context.Context, id string) (*domain.Tenant, error) {
	return r.findOne(withTenantRelations(r.db.WithContext(ctx).Unscoped()).Where("tenants.id = ?", id))
}

func (r *tenantRepository) findOne(query *gorm.DB) (*domain.Tenant, error) {
	var model tenantModel
	if err := query.First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	tenant := model.toDomain()
	return &tenant, nil
}

func (r *tenantRepository) HasMembers(ctx context.Context, id string) (bool, error) {
	for _, model := range []any{&userModel{}, &departmentModel{}} {
		var count int64
		if err := r.db.WithContext(ctx).Model(model).Where("tenant_id = ?", id).Count(&count).Error; err != nil {
			return false, fmt.Errorf("database error: %w", err)
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (r *tenantRepository) Update(ctx context.Context, id string, patch domain.TenantPatch) (*domain.Tenant, error) {
	if err := r.db.WithContext(ctx).Model(&tenantModel{}).Where("id = ?", id).Updates(tenantColumns(patch)).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return r.FindByID(ctx, id)
}

func (r *tenantRepository) SaveLabels(ctx context.Context, id string, labels domain.Labels) error {
	now := time.Now()
	row := tenantProfileModel{TenantID: id, Labels: labelsJSON(labels), CreatedAt: now, UpdatedAt: now}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"labels", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	return nil
}

func (r *tenantRepository) Delete(ctx context.Context, id string, deletedBy string) (*domain.Tenant, error) {
	tenant, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, errors.New("tenant not found")
	}
	if err := softDelete(ctx, r.db, &tenantModel{}, id, deletedBy); err != nil {
		return nil, err
	}
	return tenant, nil
}

func (r *tenantRepository) Restore(ctx context.Context, id string) (*domain.Tenant, error) {
	if err := restoreRow(ctx, r.db, &tenantModel{}, id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}
