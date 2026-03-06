package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

func newTestHospitalService(repo *mockHospitalRepo) (ports.HospitalService, *mockCache, *mockUpload, *mockMessaging) {
	cache := newMockCache()
	upload := newMockUpload()
	msg := newMockMessaging()
	svc := NewHospitalService(repo, cache, upload, msg)
	return svc, cache, upload, msg
}

// --- Create ---

func TestHospitalService_Create_Success(t *testing.T) {
	repo := &mockHospitalRepo{
		CreateFn: func(_ context.Context, h *domain.Hospital) (*domain.Hospital, error) {
			return h, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.Create(context.Background(), &domain.Hospital{HosName: "Hospital A"}, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, h.ID)
	assert.Equal(t, "Hospital A", h.HosName)
	assert.False(t, h.CreatedAt.IsZero())
}

func TestHospitalService_Create_WithFile(t *testing.T) {
	repo := &mockHospitalRepo{
		CreateFn: func(_ context.Context, h *domain.Hospital) (*domain.Hospital, error) {
			return h, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.Create(context.Background(), &domain.Hospital{HosName: "H1"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "H1", h.HosName)
}

func TestHospitalService_Create_PreservesID(t *testing.T) {
	repo := &mockHospitalRepo{
		CreateFn: func(_ context.Context, h *domain.Hospital) (*domain.Hospital, error) {
			return h, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.Create(context.Background(), &domain.Hospital{ID: "CUSTOM", HosName: "H1"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM", h.ID)
}

func TestHospitalService_Create_RepoError(t *testing.T) {
	repo := &mockHospitalRepo{
		CreateFn: func(_ context.Context, _ *domain.Hospital) (*domain.Hospital, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	_, err := svc.Create(context.Background(), &domain.Hospital{HosName: "H1"}, nil)
	assert.EqualError(t, err, "db error")
}

// --- FindAll ---

func TestHospitalService_FindAll_Success(t *testing.T) {
	repo := &mockHospitalRepo{
		FindAllFn: func(_ context.Context, _ map[string]any, _ []map[string]any) ([]domain.Hospital, error) {
			return []domain.Hospital{{ID: "H1"}, {ID: "H2"}}, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	hospitals, err := svc.FindAll(context.Background(), &ports.JwtPayload{Role: "SUPER"})
	require.NoError(t, err)
	assert.Len(t, hospitals, 2)
}

func TestHospitalService_FindAll_RepoError(t *testing.T) {
	repo := &mockHospitalRepo{
		FindAllFn: func(_ context.Context, _ map[string]any, _ []map[string]any) ([]domain.Hospital, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	_, err := svc.FindAll(context.Background(), &ports.JwtPayload{Role: "SUPER"})
	assert.EqualError(t, err, "db error")
}

// --- FindOne ---

func TestHospitalService_FindOne_Success(t *testing.T) {
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id, HosName: "Test Hospital"}, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.FindOne(context.Background(), "H1")
	require.NoError(t, err)
	assert.Equal(t, "H1", h.ID)
}

func TestHospitalService_FindOne_NotFound(t *testing.T) {
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, _ string) (*domain.Hospital, error) {
			return nil, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	_, err := svc.FindOne(context.Background(), "nonexistent")
	assert.EqualError(t, err, "hospital not found")
}

// --- Update ---

func TestHospitalService_Update_Success(t *testing.T) {
	deviceCalled := false
	legacyCalled := false
	repo := &mockHospitalRepo{
		UpdateFn: func(_ context.Context, _ string, _ map[string]any) (*domain.Hospital, error) {
			return &domain.Hospital{ID: "H1", HosName: "Updated"}, nil
		},
	}
	svc, _, _, msg := newTestHospitalService(repo)
	msg.SendToDeviceFn = func(string, any) error { deviceCalled = true; return nil }
	msg.SendToLegacyFn = func(string, any) error { legacyCalled = true; return nil }

	h, err := svc.Update(context.Background(), "H1", map[string]any{"hosName": "Updated"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Updated", h.HosName)
	assert.True(t, deviceCalled)
	assert.True(t, legacyCalled)
}

// --- Remove ---

func TestHospitalService_Remove_Success(t *testing.T) {
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id}, nil
		},
		DeleteFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id}, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.Remove(context.Background(), "H1")
	require.NoError(t, err)
	assert.Equal(t, "H1", h.ID)
}

func TestHospitalService_Remove_NotFound(t *testing.T) {
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, _ string) (*domain.Hospital, error) {
			return nil, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	_, err := svc.Remove(context.Background(), "nonexistent")
	assert.EqualError(t, err, "hospital not found")
}

func TestHospitalService_Remove_WithPic(t *testing.T) {
	pic := "http://example.com/hospital/pic.jpg"
	deleteCalled := false
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id, HosPic: &pic}, nil
		},
		DeleteFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id, HosPic: &pic}, nil
		},
	}
	svc, _, upload, _ := newTestHospitalService(repo)
	upload.DeleteFn = func(string, string) error { deleteCalled = true; return nil }

	_, err := svc.Remove(context.Background(), "H1")
	require.NoError(t, err)
	assert.True(t, deleteCalled)
}

// --- buildHospitalConditions ---

func TestBuildHospitalConditions(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		hosID          string
		expectConds    bool
		expectNotConds int
		expectKey      string
	}{
		{"SUPER", "SUPER", "", false, 0, "hospital"},
		{"ADMIN", "ADMIN", "H1", true, 1, "hospital:H1"},
		{"LEGACY_ADMIN", "LEGACY_ADMIN", "H2", true, 1, "hospital:H2"},
		{"SERVICE", "SERVICE", "", false, 1, "hospital:HID-DEVELOPMENT"},
		{"USER default", "USER", "", false, 0, "hospital"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &ports.JwtPayload{Role: tt.role, HosID: tt.hosID}
			conds, notConds, key := buildHospitalConditions(caller)
			if tt.expectConds {
				assert.NotNil(t, conds)
			} else {
				assert.Nil(t, conds)
			}
			assert.Len(t, notConds, tt.expectNotConds)
			assert.Equal(t, tt.expectKey, key)
		})
	}
}
