package middleware

import (
	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// ValidateRoleAssignment checks if user can assign role based on level hierarchy
// Returns true if currentUserLevel < targetRoleLevel (can only assign lower privilege roles)
func ValidateRoleAssignment(currentUserLevel, targetRoleLevel int) bool {
	// Can only assign roles with higher level number (lower privilege)
	// Level 0 (Super Admin) can assign 1-5
	// Level 1 can assign 2-5
	// Level 2 can assign 3-5, etc.
	return currentUserLevel < targetRoleLevel
}

// GetUserRoleLevel returns highest role level for user (lowest number = highest privilege)
func GetUserRoleLevel(userID uuid.UUID) int {
	var user models.User
	if err := config.DB.
		Preload("RoleModel").
		Preload("UserBusinessRoles.BusinessRole").
		First(&user, "id = ?", userID).Error; err != nil {
		return 5 // Default to lowest privilege if user not found
	}

	return user.GetHighestRoleLevel()
}

// CanUserAssignRole checks if a user can assign a specific role level
func CanUserAssignRole(userID uuid.UUID, targetRoleLevel int) bool {
	userLevel := GetUserRoleLevel(userID)
	return ValidateRoleAssignment(userLevel, targetRoleLevel)
}

// GetMaxAssignableLevel returns the highest level a user can assign
func GetMaxAssignableLevel(userID uuid.UUID) int {
	userLevel := GetUserRoleLevel(userID)
	// User can assign one level below their own
	return userLevel + 1
}

// IsSuperAdmin checks if user has super admin privileges
func IsSuperAdmin(userID uuid.UUID) bool {
	var user models.User
	if err := config.DB.
		Preload("RoleModel").
		First(&user, "id = ?", userID).Error; err != nil {
		return false
	}

	return user.RoleModel != nil && user.RoleModel.Name == "super_admin"
}

// HasPermissionInVertical checks if user has a specific permission in a business vertical
func HasPermissionInVertical(userID uuid.UUID, permission string, verticalID uuid.UUID) bool {
	var user models.User
	if err := config.DB.
		Preload("RoleModel.Permissions").
		Preload("UserBusinessRoles.BusinessRole.Permissions").
		First(&user, "id = ?", userID).Error; err != nil {
		return false
	}

	return user.HasPermissionInVertical(permission, verticalID)
}

// GetUserAccessibleVerticals returns list of vertical IDs user has access to
func GetUserAccessibleVerticals(userID uuid.UUID) []uuid.UUID {
	var user models.User
	if err := config.DB.
		Preload("RoleModel").
		Preload("UserBusinessRoles.BusinessRole").
		First(&user, "id = ?", userID).Error; err != nil {
		return []uuid.UUID{}
	}

	// Super Admin has access to all verticals
	if user.RoleModel != nil && user.RoleModel.Name == "super_admin" {
		var verticals []models.BusinessVertical
		config.DB.Where("is_active = ?", true).Find(&verticals)

		verticalIDs := make([]uuid.UUID, len(verticals))
		for i, v := range verticals {
			verticalIDs[i] = v.ID
		}
		return verticalIDs
	}

	// Get verticals from business roles
	verticalMap := make(map[uuid.UUID]bool)
	for _, ubr := range user.UserBusinessRoles {
		if ubr.IsActive && ubr.BusinessRole.ID != uuid.Nil {
			verticalMap[ubr.BusinessRole.BusinessVerticalID] = true
		}
	}

	verticalIDs := make([]uuid.UUID, 0, len(verticalMap))
	for id := range verticalMap {
		verticalIDs = append(verticalIDs, id)
	}

	return verticalIDs
}
