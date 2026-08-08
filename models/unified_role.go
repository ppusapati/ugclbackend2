package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RoleScopeType string

const (
	RoleScopeGlobal           RoleScopeType = "global"
	RoleScopeBusinessVertical RoleScopeType = "business_vertical"
)

type RBACRole struct {
	ID                 uuid.UUID         `gorm:"type:uuid;primaryKey"`
	Name               string            `gorm:"size:50;not null;index:idx_rbac_roles_name"`
	DisplayName        string            `gorm:"size:100;not null"`
	Description        string            `gorm:"size:255"`
	ScopeType          RoleScopeType     `gorm:"size:32;not null;index"`
	BusinessVerticalID *uuid.UUID        `gorm:"type:uuid;index"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID"`
	Level              int               `gorm:"not null;default:5;index"`
	IsActive           bool              `gorm:"not null;default:true;index"`
	Permissions        []Permission      `gorm:"many2many:rbac_role_permissions;joinForeignKey:RoleID;joinReferences:PermissionID"`
	LegacySourceType   *string           `gorm:"size:32;uniqueIndex:idx_rbac_roles_legacy_source"`
	LegacySourceID     *uuid.UUID        `gorm:"type:uuid;uniqueIndex:idx_rbac_roles_legacy_source"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// UnifiedRole is retained as a source compatibility alias during the legacy cutover.
// New code must use RBACRole.
type UnifiedRole = RBACRole

func (RBACRole) TableName() string {
	return "rbac_roles"
}

func (r *RBACRole) BeforeCreate(*gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return r.ValidateScope()
}

func (r *RBACRole) BeforeSave(*gorm.DB) error {
	return r.ValidateScope()
}

func (r RBACRole) ValidateScope() error {
	switch r.ScopeType {
	case RoleScopeGlobal:
		if r.BusinessVerticalID != nil {
			return errors.New("global role cannot reference a business vertical")
		}
	case RoleScopeBusinessVertical:
		if r.BusinessVerticalID == nil || *r.BusinessVerticalID == uuid.Nil {
			return errors.New("business role requires a business vertical")
		}
	default:
		return errors.New("invalid role scope type")
	}

	return nil
}

func (r RBACRole) AssignmentScopeKey() (string, error) {
	if err := r.ValidateScope(); err != nil {
		return "", err
	}

	if r.ScopeType == RoleScopeGlobal {
		return string(RoleScopeGlobal), nil
	}

	return string(RoleScopeBusinessVertical) + ":" + r.BusinessVerticalID.String(), nil
}

func (r RBACRole) HasPermission(permissionName string) bool {
	for _, permission := range r.Permissions {
		if matchesPermission(permission.Name, permissionName) {
			return true
		}
	}
	return false
}

type RBACRolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt    time.Time
}

func (RBACRolePermission) TableName() string {
	return "rbac_role_permissions"
}

type UserRoleAssignment struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID           uuid.UUID  `gorm:"type:uuid;not null;index:idx_user_role_assignments_user_active;uniqueIndex:idx_user_role_assignments_active_scope,where:is_active = true"`
	User             User       `gorm:"foreignKey:UserID"`
	RoleID           uuid.UUID  `gorm:"type:uuid;not null;index:idx_user_role_assignments_role_active"`
	Role             RBACRole   `gorm:"foreignKey:RoleID"`
	ScopeKey         string     `gorm:"size:80;not null;index:idx_user_role_assignments_scope;uniqueIndex:idx_user_role_assignments_active_scope,where:is_active = true"`
	IsActive         bool       `gorm:"not null;default:true;index:idx_user_role_assignments_user_active;index:idx_user_role_assignments_role_active"`
	AssignedAt       time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP"`
	AssignedBy       *uuid.UUID `gorm:"type:uuid"`
	LegacySourceType *string    `gorm:"size:32;uniqueIndex:idx_user_role_assignments_legacy_source"`
	LegacySourceID   *uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_user_role_assignments_legacy_source"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (UserRoleAssignment) TableName() string {
	return "user_role_assignments"
}

func (a *UserRoleAssignment) BeforeCreate(*gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
