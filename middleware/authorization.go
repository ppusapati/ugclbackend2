package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// RequirePermission middleware checks if the authenticated user has the required permission
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			fmt.Println("Claims:", claims)
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			// Get user with role information
			var user models.User
			if err := config.DB.Preload("RoleModel.Permissions").First(&user, "id = ?", claims.UserID).Error; err != nil {
				http.Error(w, "user not found", http.StatusUnauthorized)
				fmt.Println("Error after user fetch:", err)
				return

			}
			// Super admins have all permissions
			if claims.Role == "super_admin" {
				next.ServeHTTP(w, r)
				return
			}

			if !user.HasPermission(permission) {
				http.Error(w, "insufficient permissions", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission checks if user has any of the provided permissions
func RequireAnyPermission(permissions []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			var user models.User
			if err := config.DB.Preload("RoleModel.Permissions").First(&user, "id = ?", claims.UserID).Error; err != nil {
				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}

			// Super admins have all permissions
			if user.HasPermission("admin_all") {
				next.ServeHTTP(w, r)
				return
			}

			hasPermission := false
			for _, permission := range permissions {
				if user.HasPermission(permission) {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				http.Error(w, "insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireResourceOwnership checks if user owns the resource or has admin permissions
func RequireResourceOwnership(resourceType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			var user models.User
			if err := config.DB.Preload("RoleModel.Permissions").First(&user, "id = ?", claims.UserID).Error; err != nil {
				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}

			// Super admins can access everything
			if user.HasPermission("admin_all") {
				next.ServeHTTP(w, r)
				return
			}

			// Extract resource ID from URL path
			pathParts := strings.Split(r.URL.Path, "/")
			if len(pathParts) < 2 {
				http.Error(w, "invalid resource path", http.StatusBadRequest)
				return
			}

			resourceID := pathParts[len(pathParts)-1]

			// Check ownership based on resource type
			if !checkResourceOwnership(user.ID.String(), resourceType, resourceID) {
				http.Error(w, "access denied - not resource owner", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// checkResourceOwnership verifies if user owns the specific resource
func checkResourceOwnership(userID, resourceType, resourceID string) bool {
	// This is a simplified example - implement based on your specific models
	switch resourceType {
	case "reports":
		// Check if user created the report
		var count int64
		config.DB.Model(&models.User{}).Where("id = ? AND created_by = ?", resourceID, userID).Count(&count)
		return count > 0
	default:
		return false
	}
}

// GetUserPermissions returns all permissions for the current user
func GetUserPermissions(r *http.Request) []string {
	claims := GetClaims(r)
	if claims == nil {
		return []string{}
	}

	var user models.User
	if err := config.DB.Preload("RoleModel.Permissions").First(&user, "id = ?", claims.UserID).Error; err != nil {
		return []string{}
	}

	var permissions []string
	if user.RoleModel != nil {
		for _, perm := range user.RoleModel.Permissions {
			permissions = append(permissions, perm.Name)
		}
	}

	return permissions
}
