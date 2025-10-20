// models/user.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name                 string     `gorm:"size:100;not null"`
	Email                string     `gorm:"size:100;uniqueIndex;not null"`
	Phone                string     `gorm:"size:15;uniqueIndex;not null"`
	PasswordHash         string     `gorm:"size:255;not null"`
	Role                 string     `gorm:"size:50;not null;default:'user'"` // Keep for backward compatibility
	RoleID               *uuid.UUID `gorm:"type:uuid"`                       // Global role system
	RoleModel            *Role      `gorm:"foreignKey:RoleID"`               // Relationship to global Role
	BusinessVerticalID   *uuid.UUID `gorm:"type:uuid"`                       // Primary business vertical
	BusinessVertical     *BusinessVertical `gorm:"foreignKey:BusinessVerticalID"` // Primary business relationship
	IsActive             bool       `gorm:"default:true"`
	CreatedAt            time.Time
	UpdatedAt            time.Time

	// Business role relationships
	UserBusinessRoles []UserBusinessRole `gorm:"foreignKey:UserID"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID = uuid.New()
	return
}
// HasPermission checks if user has a specific permission
func (u *User) HasPermission(permissionName string) bool {
	if u.RoleModel != nil {
		return u.RoleModel.HasPermission(permissionName)
	}
	// Fallback to legacy role system
	return u.hasLegacyPermission(permissionName)
}

// hasLegacyPermission provides backward compatibility for string-based roles
func (u *User) hasLegacyPermission(permissionName string) bool {
	switch u.Role {
	case "super_admin", "Super Admin":
		return true // Super admin has all permissions
	case "admin":
		return isAdminPermission(permissionName)
	case "project_coordinator":
		return isProjectCoordinatorPermission(permissionName)
	case "user":
		return isUserPermission(permissionName)
	default:
		return false
	}
}

func isAdminPermission(permission string) bool {
	adminPermissions := []string{
		"read_reports", "create_reports", "update_reports", "delete_reports",
		"read_users", "create_users", "update_users",
		"read_materials", "create_materials", "update_materials", "delete_materials",
		"read_payments", "create_payments", "update_payments", "delete_payments",
		"read_kpis",
	}
	for _, p := range adminPermissions {
		if p == permission {
			return true
		}
	}
	return false
}

func isProjectCoordinatorPermission(permission string) bool {
	coordinatorPermissions := []string{
		"read_reports", "create_reports", "update_reports",
		"read_materials", "create_materials", "update_materials",
		"read_payments", "create_payments", "update_payments",
		"read_kpis",
	}
	for _, p := range coordinatorPermissions {
		if p == permission {
			return true
		}
	}
	return false
}

func isUserPermission(permission string) bool {
	userPermissions := []string{
		"read_reports", "create_reports",
		"read_materials",
		"read_kpis",
	}
	for _, p := range userPermissions {
		if p == permission {
			return true
		}
	}
	return false
}