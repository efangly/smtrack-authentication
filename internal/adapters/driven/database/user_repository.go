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

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository adapter
func NewUserRepository(db *gorm.DB) ports.UserRepository {
	return &userRepository{db: db}
}

// withRelations loads what a user read returns: its tenant (with the label
// profile) and its live departments.
func withRelations(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Tenant.Profile").
		Preload("Departments", func(db *gorm.DB) *gorm.DB { return db.Order("departments.seq ASC") })
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	model := userModelFrom(user)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		return linkDepartments(tx, model.ID, user.DepartmentIDs)
	})
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return r.FindByID(ctx, model.ID)
}

func (r *userRepository) FindAll(ctx context.Context, filter ports.UserFilter) ([]domain.User, error) {
	var models []userModel
	query := withRelations(r.db.WithContext(ctx)).Order("users.role ASC")

	if filter.TenantID != "" {
		query = query.Where("users.tenant_id = ?", filter.TenantID)
	}
	if filter.ExcludeHiddenTenants {
		query = query.Where("users.tenant_id NOT IN (?)",
			r.db.Model(&tenantModel{}).Unscoped().Select("id").Where("hidden = ?", true))
	}
	if len(filter.ExcludeRoles) > 0 {
		roles := make([]string, len(filter.ExcludeRoles))
		for i, role := range filter.ExcludeRoles {
			roles[i] = string(role)
		}
		query = query.Where("users.role NOT IN ?", roles)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	users := make([]domain.User, len(models))
	for i, m := range models {
		users[i] = m.toDomain()
	}
	return users, nil
}

func (r *userRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return r.findOne(withRelations(r.db.WithContext(ctx)).Where("users.id = ?", id))
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	return r.findOne(withRelations(r.db.WithContext(ctx)).Where("users.username = ?", username))
}

func (r *userRepository) FindAnyByID(ctx context.Context, id string) (*domain.User, error) {
	return r.findOne(withRelations(r.db.WithContext(ctx).Unscoped()).Where("users.id = ?", id))
}

func (r *userRepository) findOne(query *gorm.DB) (*domain.User, error) {
	var model userModel
	if err := query.First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	user := model.toDomain()
	return &user, nil
}

func (r *userRepository) UsernameTaken(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Unscoped().Model(&userModel{}).
		Where("username = ?", username).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("database error: %w", err)
	}
	return count > 0, nil
}

func (r *userRepository) Update(ctx context.Context, id string, patch domain.UserPatch) (*domain.User, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&userModel{}).Where("id = ?", id).Updates(userColumns(patch))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 || patch.DepartmentIDs == nil {
			return nil
		}
		if err := tx.Where("user_id = ?", id).Delete(&userDepartmentModel{}).Error; err != nil {
			return err
		}
		return linkDepartments(tx, id, *patch.DepartmentIDs)
	})
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return r.FindByID(ctx, id)
}

func (r *userRepository) Delete(ctx context.Context, id string, deletedBy string) (*domain.User, error) {
	user, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if err := softDelete(ctx, r.db, &userModel{}, id, deletedBy); err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepository) Restore(ctx context.Context, id string) (*domain.User, error) {
	if err := restoreRow(ctx, r.db, &userModel{}, id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *userRepository) RecordLogin(ctx context.Context, id string, at time.Time) error {
	// UpdateColumn skips the updated_at stamp: a login is not an edit of the record.
	if err := r.db.WithContext(ctx).Model(&userModel{}).Where("id = ?", id).UpdateColumn("last_login_at", at).Error; err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	return nil
}

// linkDepartments links the user to each department; a link that exists is left alone.
func linkDepartments(tx *gorm.DB, userID string, departmentIDs []string) error {
	if len(departmentIDs) == 0 {
		return nil
	}
	rows := make([]userDepartmentModel, 0, len(departmentIDs))
	for _, id := range departmentIDs {
		rows = append(rows, userDepartmentModel{UserID: userID, DepartmentID: id})
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}
