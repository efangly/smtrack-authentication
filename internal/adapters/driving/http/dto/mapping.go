package dto

import (
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// The response types are the JSON the API returns. The core's domain types
// carry no tags (ADR 0001), so every response goes through one of these.

// TenantSummary is a tenant as embedded in a user or department.
type TenantSummary struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Kind   domain.TenantKind `json:"kind"`
	Legacy bool              `json:"legacy"`
	Pic    *string           `json:"pic"`
	Labels domain.Labels     `json:"labels"`
}

// TenantResponse is a tenant with its label profile and visible departments.
type TenantResponse struct {
	ID          string               `json:"id"`
	Seq         int                  `json:"seq"`
	Name        string               `json:"name"`
	Kind        domain.TenantKind    `json:"kind"`
	Legacy      bool                 `json:"legacy"`
	Hidden      bool                 `json:"hidden"`
	Address     *string              `json:"address"`
	Tel         *string              `json:"tel"`
	ContactName *string              `json:"contactName"`
	ContactTel  *string              `json:"contactTel"`
	Latitude    *string              `json:"latitude"`
	Longitude   *string              `json:"longitude"`
	Pic         *string              `json:"pic"`
	Labels      domain.Labels        `json:"labels"`
	Departments []DepartmentResponse `json:"departments"`
	CreatedAt   time.Time            `json:"createdAt"`
	UpdatedAt   time.Time            `json:"updatedAt"`
	DeletedAt   *time.Time           `json:"deletedAt,omitempty"`
}

// DepartmentResponse is a department, with its tenant when one was loaded.
type DepartmentResponse struct {
	ID        string         `json:"id"`
	Seq       int            `json:"seq"`
	TenantID  string         `json:"tenantId"`
	Name      string         `json:"name"`
	Legacy    bool           `json:"legacy"`
	Tenant    *TenantSummary `json:"tenant,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt *time.Time     `json:"deletedAt,omitempty"`
}

// UserResponse is a user. It never carries the password hash.
type UserResponse struct {
	ID            string               `json:"id"`
	TenantID      string               `json:"tenantId"`
	DepartmentIDs []string             `json:"departmentIds"`
	Username      string               `json:"username"`
	Status        bool                 `json:"status"`
	Role          domain.Role          `json:"role"`
	Display       *string              `json:"display"`
	Pic           *string              `json:"pic"`
	Comment       *string              `json:"comment"`
	CreateBy      *string              `json:"createBy"`
	LastLoginAt   *time.Time           `json:"lastLoginAt"`
	Tenant        *TenantSummary       `json:"tenant,omitempty"`
	Departments   []DepartmentResponse `json:"departments"`
	CreatedAt     time.Time            `json:"createdAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
	DeletedAt     *time.Time           `json:"deletedAt,omitempty"`
}

// AuditLogResponse is one audit log entry.
type AuditLogResponse struct {
	ID        string             `json:"id"`
	Entity    domain.AuditEntity `json:"entity"`
	EntityID  string             `json:"entityId"`
	Action    domain.AuditAction `json:"action"`
	ActorID   string             `json:"actorId"`
	ActorName string             `json:"actorName"`
	Changes   string             `json:"changes"`
	TenantID  string             `json:"tenantId"`
	IP        string             `json:"ip"`
	UserAgent string             `json:"userAgent"`
	RequestID string             `json:"requestId"`
	CreatedAt time.Time          `json:"createdAt"`
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func tenantSummary(t *domain.Tenant) *TenantSummary {
	if t == nil {
		return nil
	}
	return &TenantSummary{
		ID: t.ID, Name: t.Name, Kind: t.Kind, Legacy: t.Legacy, Pic: t.Pic,
		Labels: t.EffectiveLabels(),
	}
}

// FromTenant converts a domain tenant to its JSON shape.
func FromTenant(t *domain.Tenant) TenantResponse {
	r := TenantResponse{
		ID: t.ID, Seq: t.Seq, Name: t.Name, Kind: t.Kind, Legacy: t.Legacy, Hidden: t.Hidden,
		Address: t.Address, Tel: t.Tel, ContactName: t.ContactName, ContactTel: t.ContactTel,
		Latitude: t.Latitude, Longitude: t.Longitude, Pic: t.Pic,
		Labels:      t.EffectiveLabels(),
		Departments: FromDepartments(t.Departments),
		CreatedAt:   t.CreatedAt, UpdatedAt: t.UpdatedAt, DeletedAt: t.DeletedAt,
	}
	return r
}

// FromTenants converts domain tenants to their JSON shape.
func FromTenants(tenants []domain.Tenant) []TenantResponse {
	out := make([]TenantResponse, len(tenants))
	for i := range tenants {
		out[i] = FromTenant(&tenants[i])
	}
	return out
}

// FromDepartment converts a domain department to its JSON shape.
func FromDepartment(d *domain.Department) DepartmentResponse {
	return DepartmentResponse{
		ID: d.ID, Seq: d.Seq, TenantID: d.TenantID, Name: d.Name, Legacy: d.Legacy,
		Tenant:    tenantSummary(d.Tenant),
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, DeletedAt: d.DeletedAt,
	}
}

// FromDepartments converts domain departments to their JSON shape.
func FromDepartments(departments []domain.Department) []DepartmentResponse {
	out := make([]DepartmentResponse, len(departments))
	for i := range departments {
		out[i] = FromDepartment(&departments[i])
	}
	return out
}

// FromUser converts a domain user to its JSON shape, leaving the password out.
func FromUser(u *domain.User) UserResponse {
	return UserResponse{
		ID: u.ID, TenantID: u.TenantID, DepartmentIDs: nonNil(u.LinkedDepartmentIDs()),
		Username: u.Username, Status: u.Status, Role: u.Role,
		Display: u.Display, Pic: u.Pic, Comment: u.Comment, CreateBy: u.CreateBy,
		LastLoginAt: u.LastLoginAt,
		Tenant:      tenantSummary(u.Tenant),
		Departments: FromDepartments(u.Departments),
		CreatedAt:   u.CreatedAt, UpdatedAt: u.UpdatedAt, DeletedAt: u.DeletedAt,
	}
}

// FromUsers converts domain users to their JSON shape.
func FromUsers(users []domain.User) []UserResponse {
	out := make([]UserResponse, len(users))
	for i := range users {
		out[i] = FromUser(&users[i])
	}
	return out
}

// FromAuditLogs converts domain audit logs to their JSON shape.
func FromAuditLogs(logs []domain.AuditLog) []AuditLogResponse {
	out := make([]AuditLogResponse, len(logs))
	for i, l := range logs {
		out[i] = AuditLogResponse{
			ID: l.ID, Entity: l.Entity, EntityID: l.EntityID, Action: l.Action,
			ActorID: l.ActorID, ActorName: l.ActorName, Changes: l.Changes,
			TenantID: l.TenantID, IP: l.IP, UserAgent: l.UserAgent, RequestID: l.RequestID,
			CreatedAt: l.CreatedAt,
		}
	}
	return out
}
