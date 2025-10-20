package middleware

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// RequireBusinessPermission checks if user has permission in a specific business vertical
func RequireBusinessPermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Get business ID from URL path or query parameter
			businessID := getBusinessIDFromRequest(r)
			if businessID == uuid.Nil {
				http.Error(w, "business vertical not specified", http.StatusBadRequest)
				return
			}

			// Get user with both global and business roles
			var user models.User
			if err := config.DB.Preload("RoleModel.Permissions").
				Preload("UserBusinessRoles.BusinessRole.Permissions").
				First(&user, "id = ?", claims.UserID).Error; err != nil {
				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}

			// Super admin has all permissions in all businesses
			if user.HasPermission("admin_all") || isSuperAdmin(user) {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user has permission in this specific business
			if !hasPermissionInBusiness(user, permission, businessID) {
				http.Error(w, "insufficient permissions for this business vertical", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireBusinessAdmin ensures user is admin of the specified business
func RequireBusinessAdmin() func(http.Handler) http.Handler {
	return RequireBusinessPermission("business_admin")
}

// RequireBusinessAccess ensures user has any access to the specified business
func RequireBusinessAccess() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			businessID := getBusinessIDFromRequest(r)
			if businessID == uuid.Nil {
				http.Error(w, "business vertical not specified", http.StatusBadRequest)
				return
			}

			var user models.User
			if err := config.DB.Preload("RoleModel.Permissions").First(&user, "id = ?", claims.UserID).Error; err != nil {
				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}

			// Super admin has access to all business verticals
			if user.HasPermission("admin_all") || isSuperAdmin(user) {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user has any role in this specific business
			var count int64
			config.DB.Model(&models.UserBusinessRole{}).
				Joins("JOIN business_roles ON user_business_roles.business_role_id = business_roles.id").
				Where("user_business_roles.user_id = ? AND business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?",
					user.ID, businessID, true).
				Count(&count)

			if count == 0 {
				http.Error(w, "no access to this business vertical", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getBusinessIDFromRequest extracts business ID from URL path or query parameters
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
	if err := config.DB.Where("UPPER(code) = UPPER(?) AND is_active = ?", identifier, true).First(&business).Error; err == nil {
		return business.ID
	}

	// Try to find by business name (case-insensitive)
	if err := config.DB.Where("UPPER(name) = UPPER(?) AND is_active = ?", identifier, true).First(&business).Error; err == nil {
		return business.ID
	}

	return uuid.Nil
}

// GetCurrentBusinessID returns the business ID from the current request context
func GetCurrentBusinessID(r *http.Request) uuid.UUID {
	return getBusinessIDFromRequest(r)
}

// GetUserBusinessContext returns user's business roles and permissions for the current business
func GetUserBusinessContext(r *http.Request) map[string]interface{} {
	claims := GetClaims(r)
	if claims == nil {
		return nil
	}

	businessID := getBusinessIDFromRequest(r)
	if businessID == uuid.Nil {
		return nil
	}

	var user models.User
	if err := config.DB.Preload("RoleModel.Permissions").
		Preload("UserBusinessRoles.BusinessRole.Permissions").
		Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
		First(&user, "id = ?", claims.UserID).Error; err != nil {
		return nil
	}

	// Check if user is super admin
	isSuperAdminUser := user.HasPermission("admin_all") || isSuperAdmin(user)

	// Find roles for this specific business
	var businessRoles []models.UserBusinessRole
	var permissions []string

	if isSuperAdminUser {
		// Super admin has all permissions
		permissions = []string{
			"admin_all", "business_admin", "business_manage_users", "business_manage_roles",
			"read_reports", "create_reports", "update_reports", "delete_reports",
			"read_users", "create_users", "update_users", "delete_users",
			"read_materials", "create_materials", "update_materials", "delete_materials",
			"read_payments", "create_payments", "update_payments", "delete_payments",
			"read_kpis", "business_view_analytics",
		}
	} else {
		// Regular user - check business-specific roles
		for _, ubr := range user.UserBusinessRoles {
			if ubr.BusinessRole.BusinessVerticalID == businessID && ubr.IsActive {
				businessRoles = append(businessRoles, ubr)
				for _, perm := range ubr.BusinessRole.Permissions {
					permissions = append(permissions, perm.Name)
				}
			}
		}
	}

	return map[string]interface{}{
		"business_id":    businessID,
		"business_roles": businessRoles,
		"permissions":    permissions,
		"is_admin":       isSuperAdminUser || isBusinessAdmin(user, businessID),
		"is_super_admin": isSuperAdminUser,
	}
}

// isSuperAdmin checks if user has super admin role (legacy or new system)
func isSuperAdmin(user models.User) bool {
	// Check legacy role system
	if user.Role == "super_admin" || user.Role == "Super Admin" {
		return true
	}

	// Check new role system
	if user.RoleModel != nil && (user.RoleModel.Name == "super_admin" || user.RoleModel.Name == "Super Admin") {
		return true
	}

	return false
}

// hasPermissionInBusiness checks if user has a specific permission in a business vertical
func hasPermissionInBusiness(user models.User, permissionName string, businessVerticalID uuid.UUID) bool {
	// Check if user has any active roles in this business
	for _, ubr := range user.UserBusinessRoles {
		if ubr.BusinessRole.BusinessVerticalID == businessVerticalID && ubr.IsActive {
			// Check if this role has the required permission
			for _, perm := range ubr.BusinessRole.Permissions {
				if perm.Name == permissionName {
					return true
				}
			}
		}
	}

	return false
}

// isBusinessAdmin checks if user is admin of a specific business
func isBusinessAdmin(user models.User, businessVerticalID uuid.UUID) bool {
	return hasPermissionInBusiness(user, "business_admin", businessVerticalID)
}

// HasBusinessPermissionInContext checks if user has permission in current business context
func HasBusinessPermissionInContext(r *http.Request, permission string) bool {
	businessContext := GetUserBusinessContext(r)
	if businessContext == nil {
		return false
	}

	// Super admins have all permissions
	if isSuperAdmin, ok := businessContext["super_admin"].(bool); ok && isSuperAdmin {
		return true
	}

	// Check in user's permissions
	if permissions, ok := businessContext["permissions"].([]string); ok {
		for _, perm := range permissions {
			if perm == permission {
				return true
			}
		}
	}

	return false
}

// GetBusinessPermissions returns all permissions for the current business context
func GetBusinessPermissions(r *http.Request) []string {
	businessContext := GetUserBusinessContext(r)
	if businessContext == nil {
		return []string{}
	}

	if permissions, ok := businessContext["permissions"].([]string); ok {
		return permissions
	}

	return []string{}
}
