package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"gorm.io/gorm"
)

// Persistence models: the shape of the tables in migrations/. Column names are
// known only here; the core sees domain types (ADR 0001).
//
// Write-protected columns (`<-:false`) are filled by the database: seq is an
// identity column that the import tool sets itself.

type tenantModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	Seq         int64  `gorm:"column:seq;<-:false"`
	Name        string `gorm:"column:name"`
	Kind        string `gorm:"column:kind"`
	Legacy      bool   `gorm:"column:legacy"`
	Hidden      bool   `gorm:"column:hidden"`
	Address     *string
	Tel         *string
	ContactName *string
	ContactTel  *string
	Latitude    *string
	Longitude   *string
	Pic         *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
	DeletedBy   *string

	Profile     *tenantProfileModel `gorm:"foreignKey:TenantID;references:ID"`
	Departments []departmentModel   `gorm:"foreignKey:TenantID;references:ID"`
}

func (tenantModel) TableName() string { return "tenants" }

type tenantProfileModel struct {
	TenantID  string `gorm:"column:tenant_id;primaryKey"`
	Labels    labelsJSON
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (tenantProfileModel) TableName() string { return "tenant_profiles" }

type departmentModel struct {
	ID        string `gorm:"column:id;primaryKey"`
	Seq       int64  `gorm:"column:seq;<-:false"`
	TenantID  string
	Name      string
	Legacy    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
	DeletedBy *string

	Tenant *tenantModel `gorm:"foreignKey:TenantID;references:ID"`
}

func (departmentModel) TableName() string { return "departments" }

type userModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	TenantID    string
	Username    string
	Password    string
	Status      bool
	Role        string
	Display     *string
	Pic         *string
	Comment     *string
	CreatedBy   *string
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
	DeletedBy   *string

	Tenant      *tenantModel      `gorm:"foreignKey:TenantID;references:ID"`
	Departments []departmentModel `gorm:"many2many:user_departments;joinForeignKey:UserID;joinReferences:DepartmentID"`
}

func (userModel) TableName() string { return "users" }

// userDepartmentModel is one row of the user/department link table.
type userDepartmentModel struct {
	UserID       string `gorm:"column:user_id;primaryKey"`
	DepartmentID string `gorm:"column:department_id;primaryKey"`
}

func (userDepartmentModel) TableName() string { return "user_departments" }

type auditLogModel struct {
	ID        string `gorm:"column:id;primaryKey"`
	Entity    string
	EntityID  string
	Action    string
	ActorID   string
	ActorName string
	Changes   string
	TenantID  string
	IP        string
	UserAgent string
	RequestID string
	CreatedAt time.Time
}

func (auditLogModel) TableName() string { return "audit_logs" }

// labelsJSON stores domain.Labels in a jsonb column.
type labelsJSON domain.Labels

func (l labelsJSON) Value() (driver.Value, error) {
	if l == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]map[string]string(l))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (l *labelsJSON) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*l = nil
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported labels type %T", src)
	}
	var m map[string]map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	*l = labelsJSON(m)
	return nil
}

func deletedAt(d gorm.DeletedAt) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func (m tenantModel) toDomain() domain.Tenant {
	t := domain.Tenant{
		ID: m.ID, Seq: int(m.Seq), Name: m.Name, Kind: domain.TenantKind(m.Kind),
		Legacy: m.Legacy, Hidden: m.Hidden,
		Address: m.Address, Tel: m.Tel, ContactName: m.ContactName, ContactTel: m.ContactTel,
		Latitude: m.Latitude, Longitude: m.Longitude, Pic: m.Pic,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		DeletedAt: deletedAt(m.DeletedAt), DeletedBy: m.DeletedBy,
	}
	if m.Profile != nil {
		t.Labels = domain.Labels(m.Profile.Labels)
	}
	for _, d := range m.Departments {
		t.Departments = append(t.Departments, d.toDomain())
	}
	return t
}

func tenantModelFrom(t *domain.Tenant) tenantModel {
	return tenantModel{
		ID: t.ID, Name: t.Name, Kind: string(t.Kind), Legacy: t.Legacy, Hidden: t.Hidden,
		Address: t.Address, Tel: t.Tel, ContactName: t.ContactName, ContactTel: t.ContactTel,
		Latitude: t.Latitude, Longitude: t.Longitude, Pic: t.Pic,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func (m departmentModel) toDomain() domain.Department {
	d := domain.Department{
		ID: m.ID, Seq: int(m.Seq), TenantID: m.TenantID, Name: m.Name, Legacy: m.Legacy,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		DeletedAt: deletedAt(m.DeletedAt), DeletedBy: m.DeletedBy,
	}
	if m.Tenant != nil {
		t := m.Tenant.toDomain()
		d.Tenant = &t
	}
	return d
}

func departmentModelFrom(d *domain.Department) departmentModel {
	return departmentModel{
		ID: d.ID, TenantID: d.TenantID, Name: d.Name, Legacy: d.Legacy,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func (m userModel) toDomain() domain.User {
	u := domain.User{
		ID: m.ID, TenantID: m.TenantID, Username: m.Username, Password: m.Password,
		Status: m.Status, Role: domain.Role(m.Role),
		Display: m.Display, Pic: m.Pic, Comment: m.Comment, CreateBy: m.CreatedBy,
		LastLoginAt: m.LastLoginAt, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		DeletedAt: deletedAt(m.DeletedAt), DeletedBy: m.DeletedBy,
	}
	if m.Tenant != nil {
		t := m.Tenant.toDomain()
		u.Tenant = &t
	}
	for _, d := range m.Departments {
		u.Departments = append(u.Departments, d.toDomain())
	}
	return u
}

func userModelFrom(u *domain.User) userModel {
	return userModel{
		ID: u.ID, TenantID: u.TenantID, Username: u.Username, Password: u.Password,
		Status: u.Status, Role: string(u.Role),
		Display: u.Display, Pic: u.Pic, Comment: u.Comment, CreatedBy: u.CreateBy,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func (m auditLogModel) toDomain() domain.AuditLog {
	return domain.AuditLog{
		ID: m.ID, Entity: domain.AuditEntity(m.Entity), EntityID: m.EntityID,
		Action: domain.AuditAction(m.Action), ActorID: m.ActorID, ActorName: m.ActorName,
		Changes: m.Changes, TenantID: m.TenantID, IP: m.IP, UserAgent: m.UserAgent, RequestID: m.RequestID,
		CreatedAt: m.CreatedAt,
	}
}

func auditLogModelFrom(l *domain.AuditLog) auditLogModel {
	return auditLogModel{
		ID: l.ID, Entity: string(l.Entity), EntityID: l.EntityID, Action: string(l.Action),
		ActorID: l.ActorID, ActorName: l.ActorName, Changes: l.Changes, TenantID: l.TenantID, IP: l.IP, UserAgent: l.UserAgent, RequestID: l.RequestID,
		CreatedAt: l.CreatedAt,
	}
}
