package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"
)

// AuthService provides centralized authorization logic
type AuthService struct{}

// NewAuthService creates a new instance of AuthService
func NewAuthService() *AuthService {
	return &AuthService{}
}

// UserContext contains all user authorization information
type UserContext struct {
	User              models.User
	Claims            *Claims
	IsSuperAdmin      bool
	GlobalPermissions []string
	BusinessContext   *BusinessContext
	SiteContext       *SiteAccessContext
}

// BusinessContext contains business-specific authorization info
type BusinessContext struct {
	BusinessID      uuid.UUID
	BusinessRoles   []models.UserBusinessRole
	Permissions     []string
	IsBusinessAdmin bool
}

// LoadUserContext loads complete user context from request
func (s *AuthService) LoadUserContext(r *http.Request) (*UserContext, error) {
	claims := GetClaims(r)
	if claims == nil {
		return nil, ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidUserID
	}

	// Load user with all relationships
	var user models.User
	if err := config.DB.
		Preload("RoleModel.Permissions").
		Preload("UserBusinessRoles.BusinessRole.Permissions").
		Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
		First(&user, "id = ?", userID).Error; err != nil {
		return nil, ErrUserNotFound
	}

	ctx := &UserContext{
		User:         user,
		Claims:       claims,
		IsSuperAdmin: s.IsSuperAdmin(user),
	}

	// Load global permissions
	ctx.GlobalPermissions = s.GetGlobalPermissions(user)

	// Load business context if business ID is present
	if businessID := getBusinessIDFromRequest(r); businessID != uuid.Nil {
		ctx.BusinessContext = s.LoadBusinessContext(user, businessID)
	}

	return ctx, nil
}

// IsSuperAdmin checks if user has super admin role
func (s *AuthService) IsSuperAdmin(user models.User) bool {
	if user.RoleModel != nil && user.RoleModel.Name == "super_admin" {
		return true
	}
	return user.HasPermission("admin_all")
}

// GetGlobalPermissions returns all global permissions for user
func (s *AuthService) GetGlobalPermissions(user models.User) []string {
	permissions := make([]string, 0)
	if user.RoleModel != nil {
		for _, perm := range user.RoleModel.Permissions {
			permissions = append(permissions, perm.Name)
		}
	}
	return permissions
}

// LoadBusinessContext loads business-specific context for user
func (s *AuthService) LoadBusinessContext(user models.User, businessID uuid.UUID) *BusinessContext {
	ctx := &BusinessContext{
		BusinessID:  businessID,
		Permissions: make([]string, 0),
	}

	// Super admins have all permissions (wildcard)
	// Instead of hardcoding, we give them the wildcard permission which grants everything
	if s.IsSuperAdmin(user) {
		ctx.Permissions = []string{"*:*:*"} // Wildcard grants all permissions dynamically
		ctx.IsBusinessAdmin = true
		return ctx
	}

	// Load business-specific roles
	for _, ubr := range user.UserBusinessRoles {
		if ubr.BusinessRole.BusinessVerticalID == businessID && ubr.IsActive {
			ctx.BusinessRoles = append(ctx.BusinessRoles, ubr)
			for _, perm := range ubr.BusinessRole.Permissions {
				ctx.Permissions = append(ctx.Permissions, perm.Name)
				if perm.Name == "business_admin" {
					ctx.IsBusinessAdmin = true
				}
			}
		}
	}

	return ctx
}

// HasPermission checks if user has a specific global permission
func (s *AuthService) HasPermission(ctx *UserContext, permission string) bool {
	if ctx.IsSuperAdmin {
		return true
	}

	for _, perm := range ctx.GlobalPermissions {
		if utils.MatchesPermission(perm, permission) {
			return true
		}
	}

	return false
}

// HasAnyPermission checks if user has any of the specified permissions
func (s *AuthService) HasAnyPermission(ctx *UserContext, permissions []string) bool {
	if ctx.IsSuperAdmin {
		return true
	}

	for _, permission := range permissions {
		for _, userPerm := range ctx.GlobalPermissions {
			if utils.MatchesPermission(userPerm, permission) {
				return true
			}
		}
	}

	return false
}

// HasBusinessPermission checks if user has permission in business context
func (s *AuthService) HasBusinessPermission(ctx *UserContext, permission string) bool {
	if ctx.IsSuperAdmin {
		return true
	}

	if ctx.BusinessContext == nil {
		return false
	}

	for _, perm := range ctx.BusinessContext.Permissions {
		if utils.MatchesPermission(perm, permission) {
			return true
		}
	}

	return false
}

// HasBusinessAccess checks if user has any access to the business
func (s *AuthService) HasBusinessAccess(ctx *UserContext) bool {
	if ctx.IsSuperAdmin {
		return true
	}

	if ctx.BusinessContext == nil {
		return false
	}

	return len(ctx.BusinessContext.BusinessRoles) > 0
}

// GetUserRoleLevel returns highest role level for user
func (s *AuthService) GetUserRoleLevel(user models.User) int {
	return user.GetHighestRoleLevel()
}

// CanAssignRole checks if user can assign a specific role level
func (s *AuthService) CanAssignRole(userLevel, targetRoleLevel int) bool {
	return userLevel < targetRoleLevel
}

// GetAccessibleBusinessVerticals returns list of business IDs user has access to
func (s *AuthService) GetAccessibleBusinessVerticals(user models.User) []uuid.UUID {
	if s.IsSuperAdmin(user) {
		var verticals []models.BusinessVertical
		config.DB.Where("is_active = ?", true).Find(&verticals)

		verticalIDs := make([]uuid.UUID, len(verticals))
		for i, v := range verticals {
			verticalIDs[i] = v.ID
		}
		return verticalIDs
	}

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

// Common errors
var (
	ErrUnauthorized         = &AuthError{Code: http.StatusUnauthorized, Message: "unauthorized"}
	ErrForbidden            = &AuthError{Code: http.StatusForbidden, Message: "insufficient permissions"}
	ErrUserNotFound         = &AuthError{Code: http.StatusUnauthorized, Message: "user not found"}
	ErrInvalidUserID        = &AuthError{Code: http.StatusUnauthorized, Message: "invalid user ID"}
	ErrBusinessNotSpecified = &AuthError{Code: http.StatusBadRequest, Message: "business vertical not specified"}
	ErrNoBusinessAccess     = &AuthError{Code: http.StatusForbidden, Message: "no access to this business vertical"}
)

// AuthError represents an authorization error
type AuthError struct {
	Code    int
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
