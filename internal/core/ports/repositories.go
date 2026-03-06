package ports

import (
	"context"

	"github.com/tng-coop/auth-service/internal/core/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	Update(ctx context.Context, id string, data map[string]any) (*domain.User, error)
	Delete(ctx context.Context, id string) (*domain.User, error)
}

type WardRepository interface {
	Create(ctx context.Context, ward *domain.Ward) (*domain.Ward, error)
	FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.Ward, error)
	FindByID(ctx context.Context, id string) (*domain.Ward, error)
	Update(ctx context.Context, id string, data map[string]any) (*domain.Ward, error)
	Delete(ctx context.Context, id string) error
}

type HospitalRepository interface {
	Create(ctx context.Context, hospital *domain.Hospital) (*domain.Hospital, error)
	FindAll(ctx context.Context, conditions map[string]any, notConditions []map[string]any) ([]domain.Hospital, error)
	FindByID(ctx context.Context, id string) (*domain.Hospital, error)
	Update(ctx context.Context, id string, data map[string]any) (*domain.Hospital, error)
	Delete(ctx context.Context, id string) (*domain.Hospital, error)
}
