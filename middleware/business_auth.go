package middleware

// import (
// 	"net/http"

// 	"github.com/google/uuid"
// 	"p9e.in/ugcl/config"
// 	"p9e.in/ugcl/models"
// )

// // resolveBusinessIdentifier converts business code, name, or UUID to UUID
// func resolveBusinessIdentifier(identifier string) uuid.UUID {
// 	// First try to parse as UUID
// 	if businessID, err := uuid.Parse(identifier); err == nil {
// 		return businessID
// 	}

// 	// Try to find by business code (case-insensitive)
// 	var business models.BusinessVertical
// 	if err := config.DB.Where("UPPER(code) = UPPER(?) AND is_active = ?", identifier, true).First(&business).Error; err == nil {
// 		return business.ID
// 	}

// 	// Try to find by business name (case-insensitive)
// 	if err := config.DB.Where("UPPER(name) = UPPER(?) AND is_active = ?", identifier, true).First(&business).Error; err == nil {
// 		return business.ID
// 	}

// 	return uuid.Nil
// }

// // GetCurrentBusinessID returns the business ID from the current request context
// func GetCurrentBusinessID(r *http.Request) uuid.UUID {
// 	return getBusinessIDFromRequest(r)
// }

// // isSuperAdmin checks if user has super admin role
// func isSuperAdmin(user models.User) bool {
// 	// Check role system
// 	if user.RoleModel != nil && user.RoleModel.Name == "super_admin" {
// 		return true
// 	}

// 	return false
// }

// // hasPermissionInBusiness checks if user has a specific permission in a business vertical
// func hasPermissionInBusiness(user models.User, permissionName string, businessVerticalID uuid.UUID) bool {
// 	// Check if user has any active roles in this business
// 	for _, ubr := range user.UserBusinessRoles {
// 		if ubr.BusinessRole.BusinessVerticalID == businessVerticalID && ubr.IsActive {
// 			// Check if this role has the required permission
// 			for _, perm := range ubr.BusinessRole.Permissions {
// 				if perm.Name == permissionName {
// 					return true
// 				}
// 			}
// 		}
// 	}

// 	return false
// }

// // isBusinessAdmin checks if user is admin of a specific business
// func isBusinessAdmin(user models.User, businessVerticalID uuid.UUID) bool {
// 	return hasPermissionInBusiness(user, "business_admin", businessVerticalID)
// }

// // GetBusinessPermissions returns all permissions for the current business context
// func GetBusinessPermissions(r *http.Request) []string {
// 	businessContext := GetUserBusinessContext(r)
// 	if businessContext == nil {
// 		return []string{}
// 	}

// 	if permissions, ok := businessContext["permissions"].([]string); ok {
// 		return permissions
// 	}

// 	return []string{}
// }
