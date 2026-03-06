package database

import (
	"context"

	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
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
		return nil, err
	}
	return user, nil
}

func (r *userRepository) FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.User, error) {
	var users []domain.User
	query := r.db.WithContext(ctx).
		Preload("Ward").
		Order(`"role" ASC`)

	if conditions != nil {
		// Handle hosId condition via join
		if hosID, ok := conditions["hosId"]; ok {
			query = query.Joins("JOIN \"Wards\" ON \"Wards\".\"id\" = \"Users\".\"wardId\"").
				Where("\"Wards\".\"hosId\" = ?", hosID)
		}
	}

	for _, nc := range notConditions {
		if hosID, ok := nc["hosId"]; ok {
			query = query.Where("\"Wards\".\"hosId\" != ?", hosID)
		}
		if role, ok := nc["role"]; ok {
			query = query.Where("\"Users\".\"role\" != ?", role)
		}
	}

	if err := query.Find(&users).Error; err != nil {
		return nil, err
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
		return nil, err
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
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, id string, data map[string]any) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Model(&user).Clauses(clause.Returning{}).Where("\"id\" = ?", id).Updates(data).Error; err != nil {
		return nil, err
	}
	// Reload with relations
	return r.FindByID(ctx, id)
}

func (r *userRepository) Delete(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Where("\"id\" = ?", id).Delete(&domain.User{}).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
