package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTenantService(repo *mockTenantRepo) (ports.TenantService, *mockCache, *mockUpload, *mockMessaging) {
	cache := newMockCache()
	upload := newMockUpload()
	messaging := newMockMessaging()
	svc := NewTenantService(repo, cache, upload, messaging, newMockAuditService())
	return svc, cache, upload, messaging
}

func echoTenant(_ context.Context, t *domain.Tenant) (*domain.Tenant, error) { return t, nil }

func TestTenantService_Create(t *testing.T) {
	svc, cache, _, _ := newTestTenantService(&mockTenantRepo{CreateFn: echoTenant})

	tenant, err := svc.Create(context.Background(), &domain.Tenant{Name: "Plant A"}, nil, superCaller)
	require.NoError(t, err)
	assert.NotEmpty(t, tenant.ID)
	assert.Equal(t, domain.TenantKindHospital, tenant.Kind, "an unset kind defaults to HOSPITAL")
	assert.False(t, tenant.CreatedAt.IsZero())
	assert.Contains(t, cache.deleted, "tenant")

	tenant, err = svc.Create(context.Background(), &domain.Tenant{Name: "Plant B", Kind: domain.TenantKindFactory}, nil, superCaller)
	require.NoError(t, err)
	assert.Equal(t, domain.TenantKindFactory, tenant.Kind)

	_, err = svc.Create(context.Background(), &domain.Tenant{Name: "X", Kind: "BANK"}, nil, superCaller)
	assert.EqualError(t, err, "invalid tenant kind: BANK")
}

func TestTenantService_Create_OnlySuperMakesHidden(t *testing.T) {
	svc, _, _, _ := newTestTenantService(&mockTenantRepo{CreateFn: echoTenant})

	_, err := svc.Create(context.Background(), &domain.Tenant{Name: "dev", Hidden: true}, nil, actor(domain.RoleService, ""))
	assert.EqualError(t, err, "only SUPER can create a hidden tenant")

	_, err = svc.Create(context.Background(), &domain.Tenant{Name: "dev", Hidden: true}, nil, superCaller)
	assert.NoError(t, err)
}

func TestTenantService_Update_NotifiesOtherServices(t *testing.T) {
	repo := &mockTenantRepo{
		FindByIDFn: tenantsExist,
		UpdateFn: func(_ context.Context, id string, p domain.TenantPatch) (*domain.Tenant, error) {
			return &domain.Tenant{ID: id, Name: *p.Name}, nil
		},
	}
	svc, _, _, messaging := newTestTenantService(repo)
	var queues []string
	messaging.SendToDeviceFn = func(queue string, payload any) error {
		queues = append(queues, "device:"+queue)
		assert.Equal(t, map[string]string{"id": "T1", "name": "New"}, payload)
		return nil
	}
	messaging.SendToLegacyFn = func(queue string, _ any) error {
		queues = append(queues, "legacy:"+queue)
		return errors.New("queue down") // a failed notification must not fail the update
	}

	_, err := svc.Update(context.Background(), "T1", domain.TenantPatch{Name: ptr("New")}, nil, superCaller)
	require.NoError(t, err)
	assert.Equal(t, []string{"device:update-tenant", "legacy:update-tenant"}, queues)
}

func TestTenantService_Update_Guards(t *testing.T) {
	repo := &mockTenantRepo{
		FindByIDFn: tenantsExist,
		UpdateFn: func(context.Context, string, domain.TenantPatch) (*domain.Tenant, error) {
			t.Fatal("repo.Update must not be reached")
			return nil, nil
		},
	}
	svc, _, _, _ := newTestTenantService(repo)

	_, err := svc.Update(context.Background(), "T1", domain.TenantPatch{Kind: ptr(domain.TenantKind("BANK"))}, nil, superCaller)
	assert.EqualError(t, err, "invalid tenant kind: BANK")

	_, err = svc.Update(context.Background(), "T1", domain.TenantPatch{Hidden: ptr(true)}, nil, actor(domain.RoleService, ""))
	assert.EqualError(t, err, "only SUPER can create a hidden tenant")

	// A tenant the caller cannot see reads as not found.
	_, err = svc.Update(context.Background(), "HIDDEN", domain.TenantPatch{Name: ptr("x")}, nil, actor(domain.RoleService, ""))
	assert.EqualError(t, err, "tenant not found")

	_, err = svc.Update(context.Background(), "T1", domain.TenantPatch{Name: ptr("x")}, nil, nil)
	assert.EqualError(t, err, "unauthorized")
}

// --- labels ---

func TestTenantService_UpdateLabels(t *testing.T) {
	good := domain.Labels{"tenant": {"th": "โรงงาน", "en": "Factory"}, "department": {"en": "Line"}}
	tests := []struct {
		name    string
		caller  *domain.Actor
		tenant  string
		labels  domain.Labels
		wantErr string
	}{
		{"SUPER", superCaller, "T1", good, ""},
		{"SERVICE", actor(domain.RoleService, ""), "T2", good, ""},
		{"ADMIN of the tenant", actor(domain.RoleAdmin, "T1"), "T1", good, ""},
		{"LEGACY_ADMIN of the tenant", actor(domain.RoleLegacyAdmin, "T1"), "T1", good, ""},
		{"ADMIN of another tenant", actor(domain.RoleAdmin, "T1"), "T2", good, "tenant not found"},
		{"USER of the tenant", actor(domain.RoleUser, "T1"), "T1", good, "tenant not found"},
		{"empty profile resets to defaults", superCaller, "T1", domain.Labels{}, ""},
		{"unknown label", superCaller, "T1", domain.Labels{"site": {"en": "x"}}, `unknown label "site"`},
		{"needs th or en", superCaller, "T1", domain.Labels{"tenant": {"fr": "usine"}}, `label "tenant" needs a th or en text`},
		{"empty text", superCaller, "T1", domain.Labels{"tenant": {"en": ""}}, `label "tenant" needs a th or en text`},
		{"too long", superCaller, "T1", domain.Labels{"tenant": {"en": string(make([]rune, 101))}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var saved domain.Labels
			repo := &mockTenantRepo{
				FindByIDFn:   tenantsExist,
				SaveLabelsFn: func(_ context.Context, _ string, l domain.Labels) error { saved = l; return nil },
			}
			svc, cache, _, _ := newTestTenantService(repo)

			_, err := svc.UpdateLabels(context.Background(), tt.tenant, tt.labels, tt.caller)
			switch {
			case tt.name == "too long":
				assert.Error(t, err)
			case tt.wantErr == "":
				assert.NoError(t, err)
				assert.Equal(t, tt.labels, saved)
				assert.Subset(t, cache.deleted, []string{"tenant", "department", "user"})
			default:
				assert.EqualError(t, err, tt.wantErr)
				assert.Nil(t, saved, "nothing may be saved")
			}
		})
	}
}

func TestLabels_WithDefaults(t *testing.T) {
	own := domain.Labels{"department": {"th": "สายการผลิต", "en": "Line"}}
	got := own.WithDefaults(domain.TenantKindFactory)
	assert.Equal(t, own["department"], got["department"], "the tenant's own label wins")
	assert.Equal(t, "Factory", got["tenant"]["en"], "an unset label takes the default for the kind")

	assert.Equal(t, "Ward", domain.Labels(nil).WithDefaults(domain.TenantKindHospital)["department"]["en"])
	assert.Equal(t, "Organisation", domain.Labels(nil).WithDefaults(domain.TenantKindOther)["tenant"]["en"])
}

// --- delete and restore ---

func TestTenantService_Remove_OnlyWhenEmpty(t *testing.T) {
	deleted := false
	var by string
	repo := &mockTenantRepo{
		FindByIDFn: tenantsExist,
		DeleteFn: func(_ context.Context, id, deletedBy string) (*domain.Tenant, error) {
			deleted = true
			by = deletedBy
			return &domain.Tenant{ID: id}, nil
		},
	}
	svc, cache, _, _ := newTestTenantService(repo)

	repo.HasMembersFn = func(context.Context, string) (bool, error) { return true, nil }
	_, err := svc.Remove(context.Background(), "T1", superCaller)
	assert.EqualError(t, err, "tenant is not empty")
	assert.False(t, deleted)

	repo.HasMembersFn = func(context.Context, string) (bool, error) { return false, nil }
	_, err = svc.Remove(context.Background(), "T1", superCaller)
	require.NoError(t, err)
	assert.True(t, deleted)
	assert.Equal(t, "SUPER-1", by)
	assert.Subset(t, cache.deleted, []string{"tenant", "department", "user"})
}

func TestTenantService_Restore(t *testing.T) {
	now := time.Now()
	repo := &mockTenantRepo{
		FindAnyByIDFn: func(_ context.Context, id string) (*domain.Tenant, error) {
			switch id {
			case "GONE":
				return &domain.Tenant{ID: id, DeletedAt: &now}, nil
			case "HIDDEN-GONE":
				return &domain.Tenant{ID: id, Hidden: true, DeletedAt: &now}, nil
			case "LIVE":
				return &domain.Tenant{ID: id}, nil
			}
			return nil, nil
		},
		RestoreFn: func(_ context.Context, id string) (*domain.Tenant, error) { return &domain.Tenant{ID: id}, nil },
	}
	svc, _, _, _ := newTestTenantService(repo)

	_, err := svc.Restore(context.Background(), "GONE", superCaller)
	assert.NoError(t, err)
	for _, id := range []string{"LIVE", "MISSING"} {
		_, err = svc.Restore(context.Background(), id, superCaller)
		assert.EqualError(t, err, "tenant not found", id)
	}
	_, err = svc.Restore(context.Background(), "HIDDEN-GONE", actor(domain.RoleService, ""))
	assert.EqualError(t, err, "tenant not found")
	_, err = svc.Restore(context.Background(), "GONE", nil)
	assert.EqualError(t, err, "unauthorized")
}

func TestBuildTenantFilter(t *testing.T) {
	tests := []struct {
		role       domain.Role
		tenantID   string
		wantFilter ports.TenantFilter
		wantKey    string
		wantOK     bool
	}{
		{domain.RoleSuper, "", ports.TenantFilter{}, "tenant", true},
		{domain.RoleService, "", ports.TenantFilter{ExcludeHidden: true}, "tenant:service", true},
		{domain.RoleAdmin, "T1", ports.TenantFilter{ID: "T1", ExcludeHidden: true}, "tenant:T1", true},
		{domain.RoleUser, "T1", ports.TenantFilter{ID: "T1"}, "tenant:T1:member", true},
		{domain.RoleAdmin, "", ports.TenantFilter{}, "", false},
		{domain.RoleGuest, "", ports.TenantFilter{}, "", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.role)+"/"+tt.tenantID, func(t *testing.T) {
			filter, key, ok := buildTenantFilter(&domain.Actor{Role: tt.role, TenantID: tt.tenantID})
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantFilter, filter)
			assert.Equal(t, tt.wantKey, key)
		})
	}
}
