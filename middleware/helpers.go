package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/sync/singleflight"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

const businessIdentifierCacheTTL = 15 * time.Minute

var businessIdentifierResolveGroup singleflight.Group

type businessIdentifierCacheStore struct {
	mu      sync.Mutex
	entries map[string]businessIdentifierCacheEntry
}

type businessIdentifierCacheEntry struct {
	businessID uuid.UUID
	expiresAt  time.Time
}

var businessIdentifierCache = &businessIdentifierCacheStore{entries: make(map[string]businessIdentifierCacheEntry)}

func (c *businessIdentifierCacheStore) get(identifier string) (uuid.UUID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[identifier]
	if !ok {
		return uuid.Nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, identifier)
		return uuid.Nil, false
	}
	return entry.businessID, true
}

func (c *businessIdentifierCacheStore) set(identifier string, businessID uuid.UUID) {
	c.mu.Lock()
	c.entries[identifier] = businessIdentifierCacheEntry{businessID: businessID, expiresAt: time.Now().Add(businessIdentifierCacheTTL)}
	c.mu.Unlock()
}

func (c *businessIdentifierCacheStore) invalidate() {
	c.mu.Lock()
	clear(c.entries)
	c.mu.Unlock()
}

// InvalidateBusinessIdentifierCache clears the identifier-to-business lookup cache.
func InvalidateBusinessIdentifierCache() {
	businessIdentifierCache.invalidate()
}

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
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return uuid.Nil
	}

	// First try to parse as UUID
	if businessID, err := uuid.Parse(identifier); err == nil {
		return businessID
	}

	normalizedIdentifier := strings.ToUpper(identifier)
	if cachedBusinessID, ok := businessIdentifierCache.get(normalizedIdentifier); ok {
		return cachedBusinessID
	}

	loaded, err, _ := businessIdentifierResolveGroup.Do(normalizedIdentifier, func() (interface{}, error) {
		if cachedBusinessID, ok := businessIdentifierCache.get(normalizedIdentifier); ok {
			return cachedBusinessID, nil
		}

		var business models.BusinessVertical
		if dbErr := config.DB.
			Where("is_active = ? AND (UPPER(code) = ? OR UPPER(name) = ?)", true, normalizedIdentifier, normalizedIdentifier).
			First(&business).Error; dbErr != nil {
			return uuid.Nil, dbErr
		}

		businessIdentifierCache.set(normalizedIdentifier, business.ID)
		return business.ID, nil
	})
	if err != nil {
		return uuid.Nil
	}

	return loaded.(uuid.UUID)
}

// ResolveBusinessIdentifier resolves a business code, name, or UUID to a business UUID.
func ResolveBusinessIdentifier(identifier string) uuid.UUID {
	return resolveBusinessIdentifier(identifier)
}

// GetCurrentBusinessID returns the business ID from the current request context
func GetCurrentBusinessID(r *http.Request) uuid.UUID {
	if businessID := getBusinessIDFromRequest(r); businessID != uuid.Nil {
		return businessID
	}

	userCtx, err := authService.LoadUserContext(r)
	if err != nil || userCtx == nil || userCtx.BusinessContext == nil {
		return uuid.Nil
	}

	return userCtx.BusinessContext.BusinessID
}

// GetUserRoleLevel returns highest role level for user (lowest number = highest privilege)
func GetUserRoleLevel(userID uuid.UUID) int {
	user, err := loadUserWithAuthGraph(userID)
	if err != nil {
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
	user, err := loadUserWithAuthGraph(userID)
	if err != nil {
		return false
	}

	return authService.IsSuperAdmin(user)
}

// HasPermissionInVertical checks if user has a specific permission in a business vertical
func HasPermissionInVertical(userID uuid.UUID, permission string, verticalID uuid.UUID) bool {
	user, err := loadUserWithAuthGraph(userID)
	if err != nil {
		return false
	}

	return user.HasPermissionInVertical(permission, verticalID)
}

// GetUserAccessibleVerticals returns list of vertical IDs user has access to
func GetUserAccessibleVerticals(userID uuid.UUID) []uuid.UUID {
	user, err := loadUserWithAuthGraph(userID)
	if err != nil {
		return []uuid.UUID{}
	}

	return authService.GetAccessibleBusinessVerticals(user)
}

func loadUserWithAuthGraph(userID uuid.UUID) (models.User, error) {
	cacheKey := userID.String()
	if cachedUser, ok := userCache.get(cacheKey); ok {
		return *cachedUser, nil
	}

	loaded, err, _ := userContextLoadGroup.Do(cacheKey, func() (interface{}, error) {
		if cachedUser, ok := userCache.get(cacheKey); ok {
			return cachedUser, nil
		}

		var freshUser models.User
		if dbErr := config.DB.
			Preload("RoleModel.Permissions").
			Preload("UserBusinessRoles", "is_active = ?", true).
			Preload("UserBusinessRoles.BusinessRole", "is_active = ?", true).
			Preload("UserBusinessRoles.BusinessRole.Permissions").
			Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
			First(&freshUser, "id = ?", userID).Error; dbErr != nil {
			return nil, dbErr
		}

		userCache.set(cacheKey, freshUser)
		cachedUser, ok := userCache.get(cacheKey)
		if !ok {
			return nil, ErrUserNotFound
		}
		return cachedUser, nil
	})
	if err != nil {
		return models.User{}, err
	}

	return *loaded.(*models.User), nil
}
