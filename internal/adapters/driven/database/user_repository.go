package database

import (
	"context"
	"fmt"

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

func (r *userRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return user, nil
}

func (r *userRepository) FindAll(ctx context.Context, filter ports.UserFilter) ([]domain.User, error) {
	var users []domain.User
	query := r.db.WithContext(ctx).
		Preload("Ward").
		Order(`"Users"."role" ASC`)

	// Hospital conditions apply through the user's ward, so any of them needs
	// the join. (Excluding hospitals without it used to fail with a missing
	// FROM-clause entry for "Wards".)
	if filter.HosID != "" || len(filter.ExcludeHosIDs) > 0 {
		query = query.Joins(`JOIN "Wards" ON "Wards"."id" = "Users"."wardId"`)
	}
	if filter.HosID != "" {
		query = query.Where(`"Wards"."hosId" = ?`, filter.HosID)
	}
	if len(filter.ExcludeHosIDs) > 0 {
		query = query.Where(`"Wards"."hosId" NOT IN ?`, filter.ExcludeHosIDs)
	}
	if len(filter.ExcludeRoles) > 0 {
		roles := make([]string, len(filter.ExcludeRoles))
		for i, role := range filter.ExcludeRoles {
			roles[i] = string(role)
		}
		query = query.Where(`"Users"."role" NOT IN ?`, roles)
	}

	if err := query.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return users, nil
}

func (r *userRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Preload("Ward.Hospital").
		Where("\"Users\".\"id\" = ?", id).
		First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &user, nil
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Preload("Ward").
		Where("\"Users\".\"username\" = ?", username).
		First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, id string, patch domain.UserPatch) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Model(&user).Clauses(clause.Returning{}).Where("\"id\" = ?", id).Updates(userColumns(patch)).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	// Reload with relations
	return r.FindByID(ctx, id)
}

func (r *userRepository) Delete(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).First(&user).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).Delete(&domain.User{}).Error; err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &user, nil
}
