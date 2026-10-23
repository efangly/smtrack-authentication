package domain

import "time"

// AuditAction represents the kind of event an audit log records.
type AuditAction string

const (
	AuditActionCreate  AuditAction = "CREATE"
	AuditActionUpdate  AuditAction = "UPDATE"
	AuditActionDelete  AuditAction = "DELETE"
	AuditActionRestore AuditAction = "RESTORE"
	AuditActionPurge   AuditAction = "PURGE"

	// Security events (ISO/IEC 27001 A.8.15). For a failed login the entity id is the
	// username that was tried.
	AuditActionLoginSuccess  AuditAction = "LOGIN_SUCCESS"
	AuditActionLoginFailure  AuditAction = "LOGIN_FAILURE"
	AuditActionLogout        AuditAction = "LOGOUT"
	AuditActionTokenRefresh  AuditAction = "TOKEN_REFRESH"
	AuditActionPasswordReset AuditAction = "PASSWORD_RESET"
	AuditActionAccessDenied  AuditAction = "ACCESS_DENIED"
	AuditActionAccessReport  AuditAction = "ACCESS_REPORT"
)

// AuditEntity represents the entity type an audit log records an event for.
type AuditEntity string

const (
	AuditEntityUser       AuditEntity = "USER"
	AuditEntityDepartment AuditEntity = "DEPARTMENT"
	AuditEntityTenant     AuditEntity = "TENANT"
)

// AuditLog is an immutable record of something that happened.
type AuditLog struct {
	ID        string
	Entity    AuditEntity
	EntityID  string
	Action    AuditAction
	ActorID   string
	ActorName string
	Changes   string
	// TenantID is the tenant the event concerns, when there is one.
	TenantID string
	// Request metadata, when the event came from an HTTP request.
	IP        string
	UserAgent string
	RequestID string
	CreatedAt time.Time
}
