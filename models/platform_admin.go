package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlatformAdmin is a cross-tenant operator identity: someone who can create
// and provision new tenants. It lives in the control schema, never in any
// tenant's own schema — a platform admin operates above tenants, so it must
// be queryable without any tenant schema being known yet, the same reason
// Tenant lives there.
//
// This is deliberately a separate identity from models.User (which always
// lives inside one tenant's schema and represents someone using that
// tenant's application). A platform admin is not "a user of any tenant" —
// conflating the two would mean either giving some tenant's user cross-
// tenant power, or duplicating a platform admin's account into every
// tenant schema. Neither is right, so this is its own table with its own
// login flow (POST /api/v1/platform/login), separate from tenant login
// (POST /api/v1/login).
type PlatformAdmin struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name         string    `gorm:"size:150;not null"`
	Email        string    `gorm:"size:255;not null;uniqueIndex"`
	PasswordHash string    `gorm:"size:255;not null"`
	IsActive     bool      `gorm:"not null;default:true;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (PlatformAdmin) TableName() string {
	return "control.platform_admins"
}

func (a *PlatformAdmin) BeforeCreate(*gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
