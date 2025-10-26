package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Permission represents a specific action that can be performed
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"size:100;uniqueIndex;not null"` // e.g., "read_reports", "create_users"
	Description string    `gorm:"size:255"`
	Resource    string    `gorm:"size:50;not null"` // e.g., "reports", "users", "admin"
	Action      string    `gorm:"size:50;not null"` // e.g., "read", "write", "delete"
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Role represents a collection of permissions (Global roles)
type Role struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey"`
	Name        string       `gorm:"size:50;uniqueIndex;not null"`
	Description string       `gorm:"size:255"`
	IsActive    bool         `gorm:"default:true"`
	IsGlobal    bool         `gorm:"default:true"` // Global roles vs business-specific roles
	Level       int          `gorm:"default:5"`     // Hierarchy level (0=super_admin, 1=system_admin, etc.)
	Permissions []Permission `gorm:"many2many:role_permissions;"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RolePermission junction table for many-to-many relationship
type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt    time.Time
}

func (p *Permission) BeforeCreate(tx *gorm.DB) (err error) {
	p.ID = uuid.New()
	return
}

func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	r.ID = uuid.New()
	return
}

// HasPermission checks if a role has a specific permission
func (r *Role) HasPermission(permissionName string) bool {
	for _, perm := range r.Permissions {
		if perm.Name == permissionName {
			return true
		}
	}
	return false
}
