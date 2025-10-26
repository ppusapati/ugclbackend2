// models/user.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                 uuid.UUID         `gorm:"type:uuid;primaryKey"`
	Name               string            `gorm:"size:100;not null"`
	Email              string            `gorm:"size:100;uniqueIndex;not null"`
	Phone              string            `gorm:"size:15;uniqueIndex;not null"`
	PasswordHash       string            `gorm:"size:255;not null"`
	RoleID             *uuid.UUID        `gorm:"type:uuid"`                     // Global role system
	RoleModel          *Role             `gorm:"foreignKey:RoleID"`             // Relationship to global Role
	BusinessVerticalID *uuid.UUID        `gorm:"type:uuid"`                     // Primary business vertical
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID"` // Primary business relationship
	IsActive           bool              `gorm:"default:true"`
	CreatedAt          time.Time
	UpdatedAt          time.Time

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
	return false
}

// CanAssignRole checks if user can assign a role based on level hierarchy
func (u *User) CanAssignRole(targetRoleLevel int) bool {
	maxLevel := u.GetMaxAssignableLevel()
	// Can only assign roles with higher level number (lower privilege)
	return maxLevel < targetRoleLevel
}

// GetMaxAssignableLevel returns the highest level this user can assign
func (u *User) GetMaxAssignableLevel() int {
	userLevel := u.GetHighestRoleLevel()
	// User can assign one level below their own
	// Level 0 (Super Admin) can assign all (1-5)
	// Level 1 can assign 2-5
	// Level 2 can assign 3-5, etc.
	return userLevel + 1
}

// GetHighestRoleLevel returns user's highest privilege level (lowest number)
func (u *User) GetHighestRoleLevel() int {
	minLevel := 5 // Default to lowest privilege

	// Check global role level
	if u.RoleModel != nil {
		if u.RoleModel.Name == "super_admin" {
			return 0 // Super Admin is level 0
		}
		// System Admin and other global roles are typically level 1
		if u.RoleModel.IsGlobal {
			minLevel = 1
		}
	}

	// Check business role levels
	for _, ubr := range u.UserBusinessRoles {
		if ubr.IsActive && ubr.BusinessRole.ID != uuid.Nil {
			if ubr.BusinessRole.Level < minLevel {
				minLevel = ubr.BusinessRole.Level
			}
		}
	}

	return minLevel
}

// GetAllPermissions collects all permissions from global and business roles
func (u *User) GetAllPermissions() []string {
	permissions := make(map[string]bool) // Use map to avoid duplicates

	// Check for Super Admin wildcard
	if u.RoleModel != nil && u.RoleModel.Name == "super_admin" {
		return []string{"*:*:*"}
	}

	// Add global role permissions
	if u.RoleModel != nil {
		for _, perm := range u.RoleModel.Permissions {
			permissions[perm.Name] = true
		}
	}

	// Add business role permissions
	for _, ubr := range u.UserBusinessRoles {
		if ubr.IsActive && ubr.BusinessRole.ID != uuid.Nil {
			for _, perm := range ubr.BusinessRole.Permissions {
				permissions[perm.Name] = true
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(permissions))
	for perm := range permissions {
		result = append(result, perm)
	}

	return result
}

// HasPermissionInVertical checks if user has permission in specific vertical
func (u *User) HasPermissionInVertical(permission string, verticalID uuid.UUID) bool {
	// Super Admin has all permissions in all verticals
	if u.RoleModel != nil && u.RoleModel.Name == "super_admin" {
		return true
	}

	// Check if user has role in this vertical with the required permission
	for _, ubr := range u.UserBusinessRoles {
		if ubr.IsActive &&
			ubr.BusinessRole.ID != uuid.Nil &&
			ubr.BusinessRole.BusinessVerticalID == verticalID {
			// Check if this business role has the permission
			for _, perm := range ubr.BusinessRole.Permissions {
				if perm.Name == permission {
					return true
				}
			}
		}
	}

	return false
}
