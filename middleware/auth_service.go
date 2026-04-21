package middleware

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"
)

var userContextLoadGroup singleflight.Group
var superAdminVerticalsLoadGroup singleflight.Group

const superAdminVerticalsCacheTTL = 15 * time.Minute

type superAdminVerticalsCache struct {
	mu        sync.RWMutex
	ids       []uuid.UUID
	expiresAt time.Time
}

var superAdminAccessibleVerticalsCache superAdminVerticalsCache

func InvalidateAccessibleBusinessVerticalsCache() {
	superAdminAccessibleVerticalsCache.mu.Lock()
	superAdminAccessibleVerticalsCache.ids = nil
	superAdminAccessibleVerticalsCache.expiresAt = time.Time{}
	superAdminAccessibleVerticalsCache.mu.Unlock()
}

// PrewarmAuthorizationCaches proactively loads frequently-used auth data into memory.
// This avoids first-request spikes immediately after process startup.
func PrewarmAuthorizationCaches(userLimit int) {
	if userLimit <= 0 {
		return
	}

	// Preload active users with full auth graph used by LoadUserContext.
	var users []models.User
	if err := config.DB.
		Preload("RoleModel.Permissions").
		Preload("UserBusinessRoles", "is_active = ?", true).
		Preload("UserBusinessRoles.BusinessRole", "is_active = ?", true).
		Preload("UserBusinessRoles.BusinessRole.Permissions").
		Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
		Where("is_active = ?", true).
		Order("updated_at DESC").
		Limit(userLimit).
		Find(&users).Error; err != nil {
		log.Printf("[PREWARM] auth cache users load failed: %v", err)
	} else {
		for _, u := range users {
			userCache.set(u.ID.String(), u)
		}
		log.Printf("[PREWARM] auth cache loaded users: %d", len(users))
	}

	// Preload active business vertical IDs for super-admin fast path.
	var verticals []models.BusinessVertical
	if err := config.DB.Where("is_active = ?", true).Find(&verticals).Error; err != nil {
		log.Printf("[PREWARM] super-admin vertical cache load failed: %v", err)
		return
	}

	verticalIDs := make([]uuid.UUID, len(verticals))
	for i, v := range verticals {
		verticalIDs[i] = v.ID
	}

	superAdminAccessibleVerticalsCache.mu.Lock()
	superAdminAccessibleVerticalsCache.ids = make([]uuid.UUID, len(verticalIDs))
	copy(superAdminAccessibleVerticalsCache.ids, verticalIDs)
	superAdminAccessibleVerticalsCache.expiresAt = time.Now().Add(superAdminVerticalsCacheTTL)
	superAdminAccessibleVerticalsCache.mu.Unlock()

	log.Printf("[PREWARM] super-admin vertical cache loaded: %d", len(verticalIDs))
}

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

// LoadUserContext loads complete user context from request.
// The heavy Preload query is served from an in-process TTL cache (30 min) so that
// repeated concurrent requests from the same user do not each hit the database.
func (s *AuthService) LoadUserContext(r *http.Request) (*UserContext, error) {
	claims := GetClaims(r)
	if claims == nil {
		return nil, ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidUserID
	}

	// Try cache first — avoids 3 DB round-trips on every authenticated request.
	user, cached := userCache.get(claims.UserID)
	if !cached {
		// Deduplicate concurrent misses for the same user to avoid DB stampedes.
		loaded, loadErr, _ := userContextLoadGroup.Do(claims.UserID, func() (interface{}, error) {
			if cachedUser, ok := userCache.get(claims.UserID); ok {
				return cachedUser, nil
			}

			var freshUser models.User
			if err := config.DB.
				Preload("RoleModel.Permissions").
				Preload("UserBusinessRoles", "is_active = ?", true).
				Preload("UserBusinessRoles.BusinessRole", "is_active = ?", true).
				Preload("UserBusinessRoles.BusinessRole.Permissions").
				Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
				First(&freshUser, "id = ?", userID).Error; err != nil {
				return nil, err
			}

			userCache.set(claims.UserID, freshUser)
			return freshUser, nil
		})
		if loadErr != nil {
			return nil, ErrUserNotFound
		}
		user = loaded.(models.User)
	}

	ctx := &UserContext{
		User:         user,
		Claims:       claims,
		IsSuperAdmin: s.IsSuperAdmin(user),
	}

	// Load global permissions
	ctx.GlobalPermissions = s.GetGlobalPermissions(user)

	// Load business context from explicit request selector or stored active business context.
	if businessID, resolveErr := ResolveEffectiveBusinessID(r, ctx); resolveErr == nil {
		ctx.BusinessContext = s.LoadBusinessContext(user, businessID)
	} else if resolveErr != ErrBusinessNotSpecified {
		return nil, resolveErr
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
	return userLevel <= targetRoleLevel
}

// GetAccessibleBusinessVerticals returns list of business IDs user has access to
func (s *AuthService) GetAccessibleBusinessVerticals(user models.User) []uuid.UUID {
	if s.IsSuperAdmin(user) {
		// Fast path: cache hit.
		superAdminAccessibleVerticalsCache.mu.RLock()
		if len(superAdminAccessibleVerticalsCache.ids) > 0 && time.Now().Before(superAdminAccessibleVerticalsCache.expiresAt) {
			idsCopy := make([]uuid.UUID, len(superAdminAccessibleVerticalsCache.ids))
			copy(idsCopy, superAdminAccessibleVerticalsCache.ids)
			superAdminAccessibleVerticalsCache.mu.RUnlock()
			return idsCopy
		}
		superAdminAccessibleVerticalsCache.mu.RUnlock()

		// Slow path: deduplicate concurrent misses via singleflight.
		loaded, _, _ := superAdminVerticalsLoadGroup.Do("super_admin_verticals", func() (interface{}, error) {
			// Double-check inside the group in case another goroutine already populated it.
			superAdminAccessibleVerticalsCache.mu.RLock()
			if len(superAdminAccessibleVerticalsCache.ids) > 0 && time.Now().Before(superAdminAccessibleVerticalsCache.expiresAt) {
				idsCopy := make([]uuid.UUID, len(superAdminAccessibleVerticalsCache.ids))
				copy(idsCopy, superAdminAccessibleVerticalsCache.ids)
				superAdminAccessibleVerticalsCache.mu.RUnlock()
				return idsCopy, nil
			}
			superAdminAccessibleVerticalsCache.mu.RUnlock()

			var verticals []models.BusinessVertical
			config.DB.Where("is_active = ?", true).Find(&verticals)

			verticalIDs := make([]uuid.UUID, len(verticals))
			for i, v := range verticals {
				verticalIDs[i] = v.ID
			}

			superAdminAccessibleVerticalsCache.mu.Lock()
			superAdminAccessibleVerticalsCache.ids = make([]uuid.UUID, len(verticalIDs))
			copy(superAdminAccessibleVerticalsCache.ids, verticalIDs)
			superAdminAccessibleVerticalsCache.expiresAt = time.Now().Add(superAdminVerticalsCacheTTL)
			superAdminAccessibleVerticalsCache.mu.Unlock()

			return verticalIDs, nil
		})

		if ids, ok := loaded.([]uuid.UUID); ok {
			return ids
		}
		return nil
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
