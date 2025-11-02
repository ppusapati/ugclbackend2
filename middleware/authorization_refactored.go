package middleware

import (
	"net/http"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

var authService = NewAuthService()

// Authorize is the main authorization middleware with flexible options
func Authorize(opts ...AuthOption) func(http.Handler) http.Handler {
	config := &AuthConfig{}
	for _, opt := range opts {
		opt(config)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Load user context
			userCtx, err := authService.LoadUserContext(r)
			if err != nil {
				handleAuthError(w, err)
				return
			}

			// Check global permissions
			if config.Permission != "" {
				if !authService.HasPermission(userCtx, config.Permission) {
					handleAuthError(w, ErrForbidden)
					return
				}
			}

			// Check any of permissions
			if len(config.AnyPermissions) > 0 {
				if !authService.HasAnyPermission(userCtx, config.AnyPermissions) {
					handleAuthError(w, ErrForbidden)
					return
				}
			}

			// Check business permissions
			if config.BusinessPermission != "" {
				if userCtx.BusinessContext == nil {
					handleAuthError(w, ErrBusinessNotSpecified)
					return
				}
				if !authService.HasBusinessPermission(userCtx, config.BusinessPermission) {
					handleAuthError(w, &AuthError{
						Code:    http.StatusForbidden,
						Message: "insufficient permissions for this business vertical",
					})
					return
				}
			}

			// Check business access (any access)
			if config.RequireBusinessAccess {
				if userCtx.BusinessContext == nil {
					handleAuthError(w, ErrBusinessNotSpecified)
					return
				}
				if !authService.HasBusinessAccess(userCtx) {
					handleAuthError(w, ErrNoBusinessAccess)
					return
				}
			}

			// Check super admin requirement
			if config.RequireSuperAdmin && !userCtx.IsSuperAdmin {
				handleAuthError(w, ErrForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AuthConfig holds authorization configuration
type AuthConfig struct {
	Permission            string
	AnyPermissions        []string
	BusinessPermission    string
	RequireBusinessAccess bool
	RequireSuperAdmin     bool
}

// AuthOption is a functional option for Authorize middleware
type AuthOption func(*AuthConfig)

// WithPermission requires a specific global permission
func WithPermission(permission string) AuthOption {
	return func(c *AuthConfig) {
		c.Permission = permission
	}
}

// WithAnyPermission requires any of the specified permissions
func WithAnyPermission(permissions ...string) AuthOption {
	return func(c *AuthConfig) {
		c.AnyPermissions = permissions
	}
}

// WithBusinessPermission requires a specific business permission
func WithBusinessPermission(permission string) AuthOption {
	return func(c *AuthConfig) {
		c.BusinessPermission = permission
	}
}

// WithBusinessAccess requires any access to the business
func WithBusinessAccess() AuthOption {
	return func(c *AuthConfig) {
		c.RequireBusinessAccess = true
	}
}

// WithSuperAdmin requires super admin privileges
func WithSuperAdmin() AuthOption {
	return func(c *AuthConfig) {
		c.RequireSuperAdmin = true
	}
}

// RequirePermission is a convenience wrapper for global permission check
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return Authorize(WithPermission(permission))
}

// RequireAnyPermission is a convenience wrapper for checking any of the permissions
func RequireAnyPermission(permissions []string) func(http.Handler) http.Handler {
	return Authorize(WithAnyPermission(permissions...))
}

// RequireBusinessPermission is a convenience wrapper for business permission check
func RequireBusinessPermission(permission string) func(http.Handler) http.Handler {
	return Authorize(WithBusinessPermission(permission))
}

// RequireBusinessAdmin ensures user is admin of the specified business
func RequireBusinessAdmin() func(http.Handler) http.Handler {
	return RequireBusinessPermission("business_admin")
}

// RequireBusinessAccess ensures user has any access to the specified business
func RequireBusinessAccess() func(http.Handler) http.Handler {
	return Authorize(WithBusinessAccess())
}

// RequireSuperAdmin ensures user is a super admin
func RequireSuperAdmin() func(http.Handler) http.Handler {
	return Authorize(WithSuperAdmin())
}

// RequireResourceOwnership checks if user owns the resource or has admin permissions
func RequireResourceOwnership(resourceType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userCtx, err := authService.LoadUserContext(r)
			if err != nil {
				handleAuthError(w, err)
				return
			}

			// Super admins can access everything
			if userCtx.IsSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}

			// Extract resource ID from URL
			resourceID := extractResourceID(r)
			if resourceID == "" {
				http.Error(w, "invalid resource path", http.StatusBadRequest)
				return
			}

			// Check ownership
			if !checkResourceOwnership(userCtx.User.ID.String(), resourceType, resourceID) {
				http.Error(w, "access denied - not resource owner", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserPermissions returns all permissions for the current user
func GetUserPermissions(r *http.Request) []string {
	userCtx, err := authService.LoadUserContext(r)
	if err != nil {
		return []string{}
	}
	return userCtx.GlobalPermissions
}

// GetBusinessPermissions returns all permissions for the current business context
func GetBusinessPermissions(r *http.Request) []string {
	userCtx, err := authService.LoadUserContext(r)
	if err != nil || userCtx.BusinessContext == nil {
		return []string{}
	}
	return userCtx.BusinessContext.Permissions
}

// GetUserBusinessContext returns user's business context (for backward compatibility)
func GetUserBusinessContext(r *http.Request) map[string]interface{} {
	userCtx, err := authService.LoadUserContext(r)
	if err != nil || userCtx.BusinessContext == nil {
		return nil
	}

	return map[string]interface{}{
		"business_id":    userCtx.BusinessContext.BusinessID,
		"business_roles": userCtx.BusinessContext.BusinessRoles,
		"permissions":    userCtx.BusinessContext.Permissions,
		"is_admin":       userCtx.BusinessContext.IsBusinessAdmin,
		"is_super_admin": userCtx.IsSuperAdmin,
	}
}

// HasBusinessPermissionInContext checks if user has permission in current business context
func HasBusinessPermissionInContext(r *http.Request, permission string) bool {
	userCtx, err := authService.LoadUserContext(r)
	if err != nil {
		return false
	}
	return authService.HasBusinessPermission(userCtx, permission)
}

// handleAuthError writes appropriate error response
func handleAuthError(w http.ResponseWriter, err error) {
	if authErr, ok := err.(*AuthError); ok {
		http.Error(w, authErr.Message, authErr.Code)
		return
	}
	http.Error(w, "authorization error", http.StatusInternalServerError)
}

// extractResourceID extracts resource ID from URL path
func extractResourceID(r *http.Request) string {
	// Try to get from gorilla mux vars first
	vars := GetMuxVars(r)
	if id, ok := vars["id"]; ok {
		return id
	}

	// Fallback to parsing URL path
	parts := splitPath(r.URL.Path)
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}

	return ""
}

// checkResourceOwnership verifies if user owns the specific resource
func checkResourceOwnership(userID, resourceType, resourceID string) bool {
	switch resourceType {
	case "reports":
		var count int64
		config.DB.Model(&models.User{}).
			Where("id = ? AND created_by = ?", resourceID, userID).
			Count(&count)
		return count > 0
	default:
		return false
	}
}
