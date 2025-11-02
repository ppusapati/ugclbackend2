package middleware

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// GetMuxVars extracts mux variables from request
func GetMuxVars(r *http.Request) map[string]string {
	return mux.Vars(r)
}

// splitPath splits URL path into parts
func splitPath(path string) []string {
	parts := strings.Split(path, "/")
	// Remove empty strings
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// getBusinessIDFromRequest extracts business ID from URL path, query parameters, or headers
// Supports both UUID and business codes/names
func getBusinessIDFromRequest(r *http.Request) uuid.UUID {
	// Try to get from URL path variables first
	vars := mux.Vars(r)
	if businessIdentifier, exists := vars["businessCode"]; exists {
		return resolveBusinessIdentifier(businessIdentifier)
	}
	if businessIdentifier, exists := vars["businessId"]; exists {
		return resolveBusinessIdentifier(businessIdentifier)
	}

	// Try to get from query parameter
	if businessIdentifier := r.URL.Query().Get("business_code"); businessIdentifier != "" {
		return resolveBusinessIdentifier(businessIdentifier)
	}
	if businessIdentifier := r.URL.Query().Get("business_id"); businessIdentifier != "" {
		return resolveBusinessIdentifier(businessIdentifier)
	}

	// Try to get from header
	if businessIdentifier := r.Header.Get("X-Business-Code"); businessIdentifier != "" {
		return resolveBusinessIdentifier(businessIdentifier)
	}
	if businessIdentifier := r.Header.Get("X-Business-ID"); businessIdentifier != "" {
		return resolveBusinessIdentifier(businessIdentifier)
	}

	// Try to extract from path (e.g., /api/v1/business/{code}/reports)
	pathParts := strings.Split(r.URL.Path, "/")
	for i, part := range pathParts {
		if part == "business" && i+1 < len(pathParts) {
			return resolveBusinessIdentifier(pathParts[i+1])
		}
	}

	return uuid.Nil
}

// resolveBusinessIdentifier converts business code, name, or UUID to UUID
func resolveBusinessIdentifier(identifier string) uuid.UUID {
	// First try to parse as UUID
	if businessID, err := uuid.Parse(identifier); err == nil {
		return businessID
	}

	// Try to find by business code (case-insensitive)
	var business models.BusinessVertical
	if err := config.DB.Where("UPPER(code) = UPPER(?) AND is_active = ?", identifier, true).
		First(&business).Error; err == nil {
		return business.ID
	}

	// Try to find by business name (case-insensitive)
	if err := config.DB.Where("UPPER(name) = UPPER(?) AND is_active = ?", identifier, true).
		First(&business).Error; err == nil {
		return business.ID
	}

	return uuid.Nil
}

// GetCurrentBusinessID returns the business ID from the current request context
func GetCurrentBusinessID(r *http.Request) uuid.UUID {
	return getBusinessIDFromRequest(r)
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

// ValidateRoleAssignment checks if user can assign role based on level hierarchy
// Returns true if currentUserLevel < targetRoleLevel (can only assign lower privilege roles)
func ValidateRoleAssignment(currentUserLevel, targetRoleLevel int) bool {
	return currentUserLevel < targetRoleLevel
}

// GetMaxAssignableLevel returns the highest level a user can assign
func GetMaxAssignableLevel(userID uuid.UUID) int {
	userLevel := GetUserRoleLevel(userID)
	return userLevel + 1
}

// IsSuperAdminByID checks if user has super admin privileges by user ID
func IsSuperAdminByID(userID uuid.UUID) bool {
	var user models.User
	if err := config.DB.
		Preload("RoleModel").
		First(&user, "id = ?", userID).Error; err != nil {
		return false
	}

	return authService.IsSuperAdmin(user)
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

	return authService.GetAccessibleBusinessVerticals(user)
}
