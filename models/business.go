package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BusinessVertical represents different business units (Solar Farm, Water Works, etc.)
type BusinessVertical struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"size:100;uniqueIndex;not null"` // e.g., "Solar Farm", "Water Works"
	Code        string    `gorm:"size:20;uniqueIndex;not null"`  // e.g., "SOLAR", "WATER"
	Description string    `gorm:"size:255"`
	IsActive    bool      `gorm:"default:true;index"`
	Settings    *string   `gorm:"type:jsonb"` // JSON field for business-specific settings
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Relationships
	Users         []User         `gorm:"foreignKey:BusinessVerticalID"`
	BusinessRoles []BusinessRole `gorm:"foreignKey:BusinessVerticalID"`
}

// BusinessRole represents roles within a specific business vertical
type BusinessRole struct {
	ID                 uuid.UUID        `gorm:"type:uuid;primaryKey"`
	Name               string           `gorm:"size:50;not null"`  // e.g., "admin", "manager", "operator"
	DisplayName        string           `gorm:"size:100;not null"` // e.g., "Solar Farm Admin", "Water Works Manager"
	Description        string           `gorm:"size:255"`
	BusinessVerticalID uuid.UUID        `gorm:"type:uuid;not null;index"`
	BusinessVertical   BusinessVertical `gorm:"foreignKey:BusinessVerticalID"`
	Permissions        []Permission     `gorm:"many2many:business_role_permissions;"`
	IsActive           bool             `gorm:"default:true;index:idx_business_roles_active_level"` // composite index with level
	Level              int              `gorm:"default:1;index:idx_business_roles_active_level"`    // composite index with is_active
	CreatedAt          time.Time
	UpdatedAt          time.Time

	// Users with this role in this business
	UserBusinessRoles []UserBusinessRole `gorm:"foreignKey:BusinessRoleID"`
}

// UserBusinessRole represents a user's role within a specific business vertical
type UserBusinessRole struct {
	ID             uuid.UUID    `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID    `gorm:"type:uuid;not null;index:idx_ubr_user_active"`
	User           User         `gorm:"foreignKey:UserID"`
	BusinessRoleID uuid.UUID    `gorm:"type:uuid;not null;index:idx_ubr_role_active"`
	BusinessRole   BusinessRole `gorm:"foreignKey:BusinessRoleID"`
	IsActive       bool         `gorm:"default:true;index:idx_ubr_user_active;index:idx_ubr_role_active"`
	AssignedAt     time.Time    `gorm:"default:CURRENT_TIMESTAMP"`
	AssignedBy     *uuid.UUID   `gorm:"type:uuid"` // Who assigned this role
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// BusinessRolePermission junction table
type BusinessRolePermission struct {
	BusinessRoleID uuid.UUID `gorm:"type:uuid;primaryKey"`
	PermissionID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt      time.Time
}

func (bv *BusinessVertical) BeforeCreate(tx *gorm.DB) (err error) {
	bv.ID = uuid.New()
	return
}

func (br *BusinessRole) BeforeCreate(tx *gorm.DB) (err error) {
	br.ID = uuid.New()
	return
}

func (ubr *UserBusinessRole) BeforeCreate(tx *gorm.DB) (err error) {
	ubr.ID = uuid.New()
	return
}

// HasPermissionInBusiness checks if a user has a specific permission in a business vertical
func (u *User) HasPermissionInBusiness(permissionName string, businessVerticalID uuid.UUID) bool {
	// Global super admin permission covers all business contexts.
	if u.HasPermission("admin_all") || u.HasPermission("*:*:*") {
		return true
	}

	for _, ubr := range u.UserBusinessRoles {
		if !ubr.IsActive {
			continue
		}
		if ubr.BusinessRole.ID == uuid.Nil {
			continue
		}
		if ubr.BusinessRole.BusinessVerticalID != businessVerticalID {
			continue
		}
		for _, perm := range ubr.BusinessRole.Permissions {
			if matchesPermission(perm.Name, permissionName) {
				return true
			}
		}
	}

	return false
}

// GetUserBusinessRoles returns all business roles for a user
func (u *User) GetUserBusinessRoles() []UserBusinessRole {
	roles := make([]UserBusinessRole, 0, len(u.UserBusinessRoles))
	for _, ubr := range u.UserBusinessRoles {
		if ubr.IsActive {
			roles = append(roles, ubr)
		}
	}
	return roles
}

// IsBusinessAdmin checks if user is admin of a specific business
func (u *User) IsBusinessAdmin(businessVerticalID uuid.UUID) bool {
	return u.HasPermissionInBusiness("business_admin", businessVerticalID)
}

// CanManageUser checks if current user can manage another user in a business context
func (u *User) CanManageUser(targetUserID uuid.UUID, businessVerticalID uuid.UUID) bool {
	// Super admin can manage anyone
	if u.HasPermission("admin_all") {
		return true
	}

	// Business admin can manage users in their business
	if u.IsBusinessAdmin(businessVerticalID) {
		return true
	}

	// This logic will be implemented in handlers with proper DB access
	return false
}
