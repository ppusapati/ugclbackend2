package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Tenant is the registry row for a schema-isolated tenant. It lives in the
// control schema, never in a tenant's own schema, since it must be
// queryable before any tenant schema is known (login resolves it first).
//
// Deliberately carries display metadata only (name, slug, logo, color) and
// nothing that varies feature behavior between tenants — every tenant runs
// identical functionality. If per-tenant behavior variance is ever needed,
// that is a separate, deliberate addition, not something this model
// anticipates.
type Tenant struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name          string    `gorm:"size:150;not null"`
	Slug          string    `gorm:"size:80;not null;uniqueIndex"`
	SchemaName    string    `gorm:"size:63;not null;uniqueIndex"`
	LogoURL       string    `gorm:"size:500"`
	PrimaryColor  string    `gorm:"size:20"`
	Status        string    `gorm:"size:20;not null;default:'provisioning';index"`
	IsActive      bool      `gorm:"not null;default:true;index"`
	ProvisionedAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Tenant) TableName() string {
	return "control.tenants"
}

const (
	TenantStatusProvisioning = "provisioning"
	TenantStatusActive       = "active"
	TenantStatusFailed       = "failed"
	TenantStatusInactive     = "inactive"
)

func (t *Tenant) BeforeCreate(*gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}
