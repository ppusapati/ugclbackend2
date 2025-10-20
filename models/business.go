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
	IsActive    bool      `gorm:"default:true"`
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
	BusinessVerticalID uuid.UUID        `gorm:"type:uuid;not null"`
	BusinessVertical   BusinessVertical `gorm:"foreignKey:BusinessVerticalID"`
	Permissions        []Permission     `gorm:"many2many:business_role_permissions;"`
	IsActive           bool             `gorm:"default:true"`
	Level              int              `gorm:"default:1"` // Hierarchy level (1=highest, 5=lowest)
	CreatedAt          time.Time
	UpdatedAt          time.Time

	// Users with this role in this business
	UserBusinessRoles []UserBusinessRole `gorm:"foreignKey:BusinessRoleID"`
}

// UserBusinessRole represents a user's role within a specific business vertical
type UserBusinessRole struct {
	ID             uuid.UUID    `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID    `gorm:"type:uuid;not null"`
	User           User         `gorm:"foreignKey:UserID"`
	BusinessRoleID uuid.UUID    `gorm:"type:uuid;not null"`
	BusinessRole   BusinessRole `gorm:"foreignKey:BusinessRoleID"`
	IsActive       bool         `gorm:"default:true"`
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
	// This method will be called from handlers where config.DB is available
	// For now, return false - implement in handlers with proper DB access
	return false

	// Fallback to global permissions if user is super admin
	return u.HasPermission("admin_all")
}

// GetUserBusinessRoles returns all business roles for a user
func (u *User) GetUserBusinessRoles() []UserBusinessRole {
	// This method will be implemented in handlers with proper DB access
	return []UserBusinessRole{}
}

// GetBusinessVerticals returns all active business verticals
func GetBusinessVerticals() []BusinessVertical {
	// This method will be implemented in handlers with proper DB access
	return []BusinessVertical{}
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
