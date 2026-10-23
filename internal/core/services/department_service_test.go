package services

import (
	"context"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDepartmentService(repo *mockDepartmentRepo) (ports.DepartmentService, *mockCache, *mockMessaging) {
	cache := newMockCache()
	messaging := newMockMessaging()
	tenants := &mockTenantRepo{FindByIDFn: tenantsExist}
	svc := NewDepartmentService(repo, tenants, cache, messaging, newMockAuditService())
	return svc, cache, messaging
}

func TestDepartmentService_Create(t *testing.T) {
	repo := &mockDepartmentRepo{CreateFn: func(_ context.Context, d *domain.Department) (*domain.Department, error) { return d, nil }}
	svc, cache, _ := newTestDepartmentService(repo)

	d, err := svc.Create(context.Background(), &domain.Department{Name: "Line 1", TenantID: "T1"}, superCaller)
	require.NoError(t, err)
	assert.NotEmpty(t, d.ID)
	assert.False(t, d.CreatedAt.IsZero())
	assert.Subset(t, cache.deleted, []string{"department", "tenant", "user"})

	_, err = svc.Create(context.Background(), &domain.Department{Name: "Line 1", TenantID: "T9"}, superCaller)
	assert.EqualError(t, err, "tenant not found")

	_, err = svc.Create(context.Background(), &domain.Department{Name: "Line 1", TenantID: "T2"}, actor(domain.RoleAdmin, "T1"))
	assert.EqualError(t, err, "tenant not found")

	repo.NameTakenFn = func(_ context.Context, tenantID, name, _ string) (bool, error) { return name == "Line 1", nil }
	_, err = svc.Create(context.Background(), &domain.Department{Name: "Line 1", TenantID: "T1"}, superCaller)
	assert.EqualError(t, err, "department name already exists")
}

func TestDepartmentService_Update_NotifiesTheRightService(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		repo := &mockDepartmentRepo{
			FindByIDFn: func(_ context.Context, id string) (*domain.Department, error) {
				return &domain.Department{ID: id, TenantID: "T1", Name: "old"}, nil
			},
			UpdateFn: func(_ context.Context, id string, _ domain.DepartmentPatch) (*domain.Department, error) {
				return &domain.Department{ID: id, Name: "new", Legacy: legacy}, nil
			},
		}
		svc, _, messaging := newTestDepartmentService(repo)
		var sentTo string
		messaging.SendToDeviceFn = func(queue string, _ any) error { sentTo = "device:" + queue; return nil }
		messaging.SendToLegacyFn = func(queue string, _ any) error { sentTo = "legacy:" + queue; return nil }

		_, err := svc.Update(context.Background(), "D1", domain.DepartmentPatch{Name: ptr("new")}, superCaller)
		require.NoError(t, err)
		if legacy {
			assert.Equal(t, "legacy:update-department", sentTo)
		} else {
			assert.Equal(t, "device:update-department", sentTo)
		}
	}
}

func TestDepartmentService_Update_RenameChecksTheName(t *testing.T) {
	repo := &mockDepartmentRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Department, error) {
			return &domain.Department{ID: id, TenantID: "T1", Name: "old"}, nil
		},
		NameTakenFn: func(_ context.Context, _, _, exceptID string) (bool, error) {
			assert.Equal(t, "D1", exceptID, "a department does not clash with itself")
			return true, nil
		},
		UpdateFn: func(_ context.Context, id string, _ domain.DepartmentPatch) (*domain.Department, error) {
			return &domain.Department{ID: id}, nil
		},
	}
	svc, _, _ := newTestDepartmentService(repo)

	_, err := svc.Update(context.Background(), "D1", domain.DepartmentPatch{Name: ptr("taken")}, superCaller)
	assert.EqualError(t, err, "department name already exists")

	// Resubmitting the current name is not a clash.
	_, err = svc.Update(context.Background(), "D1", domain.DepartmentPatch{Name: ptr("old")}, superCaller)
	assert.NoError(t, err)
}

func TestDepartmentService_Remove(t *testing.T) {
	var by string
	repo := &mockDepartmentRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Department, error) {
			return &domain.Department{ID: id, TenantID: "T1"}, nil
		},
		DeleteFn: func(_ context.Context, id, deletedBy string) (*domain.Department, error) {
			by = deletedBy
			return &domain.Department{ID: id}, nil
		},
	}
	svc, cache, _ := newTestDepartmentService(repo)

	_, err := svc.Remove(context.Background(), "D1", superCaller)
	require.NoError(t, err)
	assert.Equal(t, "SUPER-1", by)
	assert.Subset(t, cache.deleted, []string{"department", "tenant", "user"})

	// Outside the caller's scope reads as not found.
	_, err = svc.Remove(context.Background(), "D1", actor(domain.RoleAdmin, "T2"))
	assert.EqualError(t, err, "department not found")
}

func TestDepartmentService_Restore(t *testing.T) {
	now := time.Now()
	nameClash := false
	repo := &mockDepartmentRepo{
		FindAnyByIDFn: func(_ context.Context, id string) (*domain.Department, error) {
			switch id {
			case "GONE":
				return &domain.Department{ID: id, TenantID: "T1", Name: "Line", DeletedAt: &now}, nil
			case "GONE-T9":
				return &domain.Department{ID: id, TenantID: "T9", Name: "Line", DeletedAt: &now}, nil
			case "LIVE":
				return &domain.Department{ID: id, TenantID: "T1"}, nil
			}
			return nil, nil
		},
		NameTakenFn: func(context.Context, string, string, string) (bool, error) { return nameClash, nil },
		RestoreFn:   func(_ context.Context, id string) (*domain.Department, error) { return &domain.Department{ID: id}, nil },
	}
	svc, _, _ := newTestDepartmentService(repo)

	_, err := svc.Restore(context.Background(), "GONE", superCaller)
	assert.NoError(t, err)

	for _, id := range []string{"LIVE", "MISSING"} {
		_, err = svc.Restore(context.Background(), id, superCaller)
		assert.EqualError(t, err, "department not found", id)
	}

	// Its tenant is gone.
	_, err = svc.Restore(context.Background(), "GONE-T9", superCaller)
	assert.EqualError(t, err, "tenant not found")

	// Its name has been taken by another live department since.
	nameClash = true
	_, err = svc.Restore(context.Background(), "GONE", superCaller)
	assert.EqualError(t, err, "department name already exists")
}

func TestBuildDepartmentFilter(t *testing.T) {
	tests := []struct {
		name       string
		caller     *domain.Actor
		wantFilter ports.DepartmentFilter
		wantKey    string
		wantOK     bool
	}{
		{"SUPER", actor(domain.RoleSuper, ""), ports.DepartmentFilter{}, "department", true},
		{"SERVICE", actor(domain.RoleService, ""), ports.DepartmentFilter{ExcludeHiddenTenants: true}, "department:service", true},
		{"ADMIN", actor(domain.RoleAdmin, "T1"), ports.DepartmentFilter{TenantID: "T1", ExcludeHiddenTenants: true}, "department:tenant:T1", true},
		{"ADMIN without tenant", actor(domain.RoleAdmin, ""), ports.DepartmentFilter{}, "", false},
		{"USER limited to its departments", actorIn(domain.RoleUser, "T1", "D1", "D2"),
			ports.DepartmentFilter{TenantID: "T1", IDs: []string{"D1", "D2"}, LimitToIDs: true}, "department:user:C1", true},
		{"USER without departments sees none", actorIn(domain.RoleUser, "T1"), ports.DepartmentFilter{}, "", false},
		{"GUEST without tenant sees none", actorIn(domain.RoleGuest, "", "D1"), ports.DepartmentFilter{}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, key, ok := buildDepartmentFilter(tt.caller)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantFilter, filter)
			assert.Equal(t, tt.wantKey, key)
		})
	}
}
