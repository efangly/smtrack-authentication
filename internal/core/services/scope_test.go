package services

import (
	"context"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func actor(role domain.Role, hosID, wardID string) *domain.Actor {
	return &domain.Actor{ID: "C1", Role: role, HosID: hosID, WardID: wardID}
}

// --- users ---

func TestUserService_FindOne_TenantScope(t *testing.T) {
	tests := []struct {
		name    string
		caller  *domain.Actor
		id      string
		userHos string
		wantErr string
	}{
		{"SUPER reads any hospital", actor(domain.RoleSuper, "", ""), "U2", "H2", ""},
		{"SERVICE reads any hospital", actor(domain.RoleService, "", ""), "U2", "H2", ""},
		{"ADMIN reads own hospital", actor(domain.RoleAdmin, "H1", ""), "U2", "H1", ""},
		{"ADMIN cannot read other hospital", actor(domain.RoleAdmin, "H1", ""), "U2", "H2", "user not found"},
		{"LEGACY_ADMIN cannot read other hospital", actor(domain.RoleLegacyAdmin, "H1", ""), "U2", "H2", "user not found"},
		{"ADMIN without hospital cannot read ward-less user", actor(domain.RoleAdmin, "", ""), "U2", "", "user not found"},
		{"USER cannot read another user", actor(domain.RoleUser, "H1", "W1"), "U2", "H1", "user not found"},
		{"USER reads self", actor(domain.RoleUser, "H1", "W1"), "C1", "H9", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockUserRepo{FindByIDFn: findUserIn(domain.RoleUser, tt.userHos)}
			svc, _, _ := newTestUserService(repo)

			_, err := svc.FindOne(context.Background(), tt.id, tt.caller)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestUserService_FindOne_RequiresCaller(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{FindByIDFn: findUser(domain.RoleUser)})
	_, err := svc.FindOne(context.Background(), "U1", nil)
	assert.EqualError(t, err, "unauthorized")
}

func TestUserService_UpdateAndRemove_TenantScope(t *testing.T) {
	tests := []struct {
		name    string
		caller  *domain.Actor
		userHos string
		wantErr string
	}{
		{"ADMIN own hospital", actor(domain.RoleAdmin, "H1", ""), "H1", ""},
		{"ADMIN other hospital", actor(domain.RoleAdmin, "H1", ""), "H2", "user not found"},
		{"LEGACY_ADMIN other hospital", actor(domain.RoleLegacyAdmin, "H1", ""), "H2", "user not found"},
		{"SERVICE other hospital", actor(domain.RoleService, "H1", ""), "H2", ""},
		{"SUPER other hospital", actor(domain.RoleSuper, "", ""), "H2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrote := false
			repo := &mockUserRepo{
				FindByIDFn: findUserIn(domain.RoleUser, tt.userHos),
				UpdateFn: func(_ context.Context, id string, _ map[string]any) (*domain.User, error) {
					wrote = true
					return &domain.User{ID: id}, nil
				},
				DeleteFn: func(_ context.Context, id string) (*domain.User, error) {
					wrote = true
					return &domain.User{ID: id}, nil
				},
			}
			svc, _, _ := newTestUserService(repo)

			_, updErr := svc.Update(context.Background(), "U2", map[string]any{"display": "x"}, nil, tt.caller)
			_, rmErr := svc.Remove(context.Background(), "U2", tt.caller)
			if tt.wantErr == "" {
				assert.NoError(t, updErr)
				assert.NoError(t, rmErr)
			} else {
				assert.EqualError(t, updErr, tt.wantErr)
				assert.EqualError(t, rmErr, tt.wantErr)
				assert.False(t, wrote, "repository must not be written")
			}
		})
	}
}

// wardsIn returns a ward lookup where W1 is in H1 and W2 is in H2.
func wardsIn(_ context.Context, id string) (*domain.Ward, error) {
	switch id {
	case "W1":
		return &domain.Ward{ID: "W1", HosID: "H1"}, nil
	case "W2":
		return &domain.Ward{ID: "W2", HosID: "H2"}, nil
	}
	return nil, nil
}

func TestUserService_Create_WardMustBeInTenant(t *testing.T) {
	tests := []struct {
		name    string
		caller  *domain.Actor
		wardID  string
		wantErr string
	}{
		{"ADMIN into own hospital", actor(domain.RoleAdmin, "H1", ""), "W1", ""},
		{"ADMIN into other hospital", actor(domain.RoleAdmin, "H1", ""), "W2", "ward not found"},
		{"ADMIN into missing ward", actor(domain.RoleAdmin, "H1", ""), "W9", "ward not found"},
		{"SERVICE into any hospital", actor(domain.RoleService, "H1", ""), "W2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockUserRepo{
				CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) { return user, nil },
			}
			svc, _, _, wards := newTestUserServiceWithWards(repo)
			wards.FindByIDFn = wardsIn

			_, err := svc.Create(context.Background(), &domain.User{Username: "u1", Role: domain.RoleUser, WardID: tt.wardID}, tt.caller)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestUserService_Update_WardMoveMustStayInTenant(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUserIn(domain.RoleUser, "H1"), // currently in W1
		UpdateFn: func(_ context.Context, id string, _ map[string]any) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}
	svc, _, _, wards := newTestUserServiceWithWards(repo)
	lookups := 0
	wards.FindByIDFn = func(ctx context.Context, id string) (*domain.Ward, error) {
		lookups++
		return wardsIn(ctx, id)
	}
	admin := actor(domain.RoleAdmin, "H1", "")

	_, err := svc.Update(context.Background(), "U2", map[string]any{"wardId": "W2"}, nil, admin)
	assert.EqualError(t, err, "ward not found")

	// Resubmitting the current ward needs no lookup and succeeds.
	lookups = 0
	_, err = svc.Update(context.Background(), "U2", map[string]any{"wardId": "W1"}, nil, admin)
	require.NoError(t, err)
	assert.Zero(t, lookups)
}

// --- hospitals ---

func TestHospitalService_FindOne_Scope(t *testing.T) {
	tests := []struct {
		name   string
		caller *domain.Actor
		hosID  string
		want   bool
	}{
		{"SUPER sees development", actor(domain.RoleSuper, "", ""), developmentHosID, true},
		{"SERVICE sees other", actor(domain.RoleService, "", ""), "H2", true},
		{"SERVICE does not see development", actor(domain.RoleService, "", ""), developmentHosID, false},
		{"ADMIN sees own", actor(domain.RoleAdmin, "H1", ""), "H1", true},
		{"ADMIN does not see other", actor(domain.RoleAdmin, "H1", ""), "H2", false},
		{"ADMIN without hospital sees nothing", actor(domain.RoleAdmin, "", ""), "", false},
		{"USER sees hospital of own ward", actor(domain.RoleUser, "H1", "W1"), "H1", true},
		{"USER does not see other", actor(domain.RoleUser, "H1", "W1"), "H2", false},
		{"GUEST without ward sees nothing", actor(domain.RoleGuest, "", ""), "H1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockHospitalRepo{
				FindByIDFn: func(_ context.Context, id string) (*domain.Hospital, error) {
					wardID := "W1"
					if id != "H1" {
						wardID = "W-" + id
					}
					return &domain.Hospital{ID: id, Wards: []domain.Ward{{ID: wardID, HosID: id}}}, nil
				},
			}
			svc, _, _, _ := newTestHospitalService(repo)

			_, err := svc.FindOne(context.Background(), tt.hosID, tt.caller)
			if tt.want {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, "hospital not found")
			}
		})
	}
}

func TestHospitalService_FindOne_RequiresCaller(t *testing.T) {
	svc, _, _, _ := newTestHospitalService(&mockHospitalRepo{})
	_, err := svc.FindOne(context.Background(), "H1", nil)
	assert.EqualError(t, err, "unauthorized")
}

// --- wards ---

func TestWardService_FindOne_Scope(t *testing.T) {
	tests := []struct {
		name   string
		caller *domain.Actor
		hosID  string
		want   bool
	}{
		{"SUPER sees development", actor(domain.RoleSuper, "", ""), developmentHosID, true},
		{"SERVICE does not see development", actor(domain.RoleService, "", ""), developmentHosID, false},
		{"SERVICE sees other", actor(domain.RoleService, "", ""), "H2", true},
		{"ADMIN sees own", actor(domain.RoleAdmin, "H1", ""), "H1", true},
		{"ADMIN does not see other", actor(domain.RoleAdmin, "H1", ""), "H2", false},
		{"LEGACY_ADMIN does not see other", actor(domain.RoleLegacyAdmin, "H1", ""), "H2", false},
		{"USER listing is unscoped, so lookup is too", actor(domain.RoleUser, "H1", "W1"), "H2", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockWardRepo{
				FindByIDFn: func(_ context.Context, id string) (*domain.Ward, error) {
					return &domain.Ward{ID: id, HosID: tt.hosID}, nil
				},
			}
			svc, _, _ := newTestWardService(repo)

			_, err := svc.FindOne(context.Background(), "W9", tt.caller)
			if tt.want {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, "ward not found")
			}
		})
	}
}

func TestWardService_FindOne_RequiresCaller(t *testing.T) {
	svc, _, _ := newTestWardService(&mockWardRepo{})
	_, err := svc.FindOne(context.Background(), "W1", nil)
	assert.EqualError(t, err, "unauthorized")
}
