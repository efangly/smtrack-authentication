package services

import (
	"context"
	"errors"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWardService(repo *mockWardRepo) (ports.WardService, *mockCache, *mockMessaging) {
	cache := newMockCache()
	msg := newMockMessaging()
	svc := NewWardService(repo, cache, msg, newMockAuditService())
	return svc, cache, msg
}

// --- Create ---

func TestWardService_Create_Success(t *testing.T) {
	repo := &mockWardRepo{
		CreateFn: func(_ context.Context, w *domain.Ward) (*domain.Ward, error) {
			return w, nil
		},
	}
	svc, _, _ := newTestWardService(repo)

	w, err := svc.Create(context.Background(), &domain.Ward{WardName: "Ward A", HosID: "H1"}, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, w.ID)
	assert.Equal(t, "Ward A", w.WardName)
	assert.False(t, w.CreatedAt.IsZero())
}

func TestWardService_Create_PreservesID(t *testing.T) {
	repo := &mockWardRepo{
		CreateFn: func(_ context.Context, w *domain.Ward) (*domain.Ward, error) {
			return w, nil
		},
	}
	svc, _, _ := newTestWardService(repo)

	w, err := svc.Create(context.Background(), &domain.Ward{ID: "CUSTOM", WardName: "W1", HosID: "H1"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM", w.ID)
}

func TestWardService_Create_RepoError(t *testing.T) {
	repo := &mockWardRepo{
		CreateFn: func(_ context.Context, _ *domain.Ward) (*domain.Ward, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _ := newTestWardService(repo)

	_, err := svc.Create(context.Background(), &domain.Ward{WardName: "W1"}, nil)
	assert.EqualError(t, err, "db error")
}

// --- FindAll ---

func TestWardService_FindAll_Success(t *testing.T) {
	repo := &mockWardRepo{
		FindAllFn: func(_ context.Context, _ ports.WardFilter) ([]domain.Ward, error) {
			return []domain.Ward{{ID: "W1"}, {ID: "W2"}}, nil
		},
	}
	svc, _, _ := newTestWardService(repo)

	wards, err := svc.FindAll(context.Background(), &domain.Actor{Role: "SUPER"})
	require.NoError(t, err)
	assert.Len(t, wards, 2)
}

func TestWardService_FindAll_RepoError(t *testing.T) {
	repo := &mockWardRepo{
		FindAllFn: func(_ context.Context, _ ports.WardFilter) ([]domain.Ward, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _ := newTestWardService(repo)

	_, err := svc.FindAll(context.Background(), &domain.Actor{Role: "SUPER"})
	assert.EqualError(t, err, "db error")
}

// --- FindOne ---

func TestWardService_FindOne_Success(t *testing.T) {
	repo := &mockWardRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Ward, error) {
			return &domain.Ward{ID: id, WardName: "Test Ward"}, nil
		},
	}
	svc, _, _ := newTestWardService(repo)

	w, err := svc.FindOne(context.Background(), "W1", superCaller)
	require.NoError(t, err)
	assert.Equal(t, "W1", w.ID)
}

func TestWardService_FindOne_NotFound(t *testing.T) {
	repo := &mockWardRepo{
		FindByIDFn: func(_ context.Context, _ string) (*domain.Ward, error) {
			return nil, nil
		},
	}
	svc, _, _ := newTestWardService(repo)

	_, err := svc.FindOne(context.Background(), "nonexistent", superCaller)
	assert.EqualError(t, err, "ward not found")
}

// --- Update ---

func TestWardService_Update_SendsToDeviceForNewType(t *testing.T) {
	deviceCalled := false
	newType := domain.WardTypeNew
	repo := &mockWardRepo{
		UpdateFn: func(_ context.Context, _ string, _ domain.WardPatch) (*domain.Ward, error) {
			return &domain.Ward{ID: "W1", WardName: "Updated", Type: &newType}, nil
		},
	}
	svc, _, msg := newTestWardService(repo)
	msg.SendToDeviceFn = func(string, any) error { deviceCalled = true; return nil }

	w, err := svc.Update(context.Background(), "W1", domain.WardPatch{WardName: ptr("Updated")}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Updated", w.WardName)
	assert.True(t, deviceCalled)
}

func TestWardService_Update_SendsToLegacyForLegacyType(t *testing.T) {
	legacyCalled := false
	legacyType := domain.WardTypeLegacy
	repo := &mockWardRepo{
		UpdateFn: func(_ context.Context, _ string, _ domain.WardPatch) (*domain.Ward, error) {
			return &domain.Ward{ID: "W1", WardName: "Legacy", Type: &legacyType}, nil
		},
	}
	svc, _, msg := newTestWardService(repo)
	msg.SendToLegacyFn = func(string, any) error { legacyCalled = true; return nil }

	_, err := svc.Update(context.Background(), "W1", domain.WardPatch{WardName: ptr("Legacy")}, nil)
	require.NoError(t, err)
	assert.True(t, legacyCalled)
}

func TestWardService_Update_RepoError(t *testing.T) {
	repo := &mockWardRepo{
		UpdateFn: func(_ context.Context, _ string, _ domain.WardPatch) (*domain.Ward, error) {
			return nil, errors.New("update failed")
		},
	}
	svc, _, _ := newTestWardService(repo)

	_, err := svc.Update(context.Background(), "W1", domain.WardPatch{}, nil)
	assert.EqualError(t, err, "update failed")
}

// --- Remove ---

func TestWardService_Remove_Success(t *testing.T) {
	repo := &mockWardRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Ward, error) {
			return &domain.Ward{ID: id}, nil
		},
		DeleteFn: func(_ context.Context, _ string) error {
			return nil
		},
	}
	svc, _, _ := newTestWardService(repo)

	msg, err := svc.Remove(context.Background(), "W1", nil)
	require.NoError(t, err)
	assert.Equal(t, "Ward deleted successfully", msg)
}

func TestWardService_Remove_NotFound(t *testing.T) {
	repo := &mockWardRepo{
		FindByIDFn: func(_ context.Context, _ string) (*domain.Ward, error) {
			return nil, nil
		},
	}
	svc, _, _ := newTestWardService(repo)

	_, err := svc.Remove(context.Background(), "nonexistent", nil)
	assert.EqualError(t, err, "ward not found")
}

func TestWardService_Remove_DeleteError(t *testing.T) {
	repo := &mockWardRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Ward, error) {
			return &domain.Ward{ID: id}, nil
		},
		DeleteFn: func(_ context.Context, _ string) error {
			return errors.New("delete failed")
		},
	}
	svc, _, _ := newTestWardService(repo)

	_, err := svc.Remove(context.Background(), "W1", nil)
	assert.EqualError(t, err, "delete failed")
}

// --- buildWardConditions ---

func TestBuildWardFilter(t *testing.T) {
	dev := []string{developmentHosID}
	tests := []struct {
		role       domain.Role
		hosID      string
		wantFilter ports.WardFilter
		wantKey    string
	}{
		{domain.RoleSuper, "", ports.WardFilter{}, "ward"},
		{domain.RoleAdmin, "H1", ports.WardFilter{HosID: "H1", ExcludeHosIDs: dev}, "ward:H1"},
		{domain.RoleLegacyAdmin, "H2", ports.WardFilter{HosID: "H2", ExcludeHosIDs: dev}, "ward:H2"},
		{domain.RoleService, "", ports.WardFilter{ExcludeHosIDs: dev}, "ward:HID-DEVELOPMENT"},
		{domain.RoleUser, "", ports.WardFilter{}, "ward"},
	}
	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			filter, key := buildWardFilter(&domain.Actor{Role: tt.role, HosID: tt.hosID})
			assert.Equal(t, tt.wantFilter, filter)
			assert.Equal(t, tt.wantKey, key)
		})
	}
}
