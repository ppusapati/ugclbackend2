package middleware

import (
	"context"
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

// businessIdentifierCacheKey scopes the cache (and the singleflight group) by
// tenant schema as well as the raw identifier: business codes like "WATER"
// are not globally unique across tenants, so a bare identifier key would let
// one tenant's cached UUID leak into another tenant's lookups.
func businessIdentifierCacheKey(schemaName, normalizedIdentifier string) string {
	return schemaName + "|" + normalizedIdentifier
}

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
	ctx := r.Context()

	// Try to get from URL path variables first
	vars := mux.Vars(r)
	if businessIdentifier, exists := vars["businessCode"]; exists {
		return resolveBusinessIdentifier(ctx, businessIdentifier)
	}
	if businessIdentifier, exists := vars["businessId"]; exists {
		return resolveBusinessIdentifier(ctx, businessIdentifier)
	}

	// Try to get from query parameter
	if businessIdentifier := r.URL.Query().Get("business_code"); businessIdentifier != "" {
		return resolveBusinessIdentifier(ctx, businessIdentifier)
	}
	if businessIdentifier := r.URL.Query().Get("business_id"); businessIdentifier != "" {
		return resolveBusinessIdentifier(ctx, businessIdentifier)
	}

	// Try to get from header
	if businessIdentifier := r.Header.Get("X-Business-Code"); businessIdentifier != "" {
		return resolveBusinessIdentifier(ctx, businessIdentifier)
	}
	if businessIdentifier := r.Header.Get("X-Business-ID"); businessIdentifier != "" {
		return resolveBusinessIdentifier(ctx, businessIdentifier)
	}

	// Try to extract from path (e.g., /api/v1/business/{code}/reports)
	pathParts := strings.Split(r.URL.Path, "/")
	for i, part := range pathParts {
		if part == "business" && i+1 < len(pathParts) {
			return resolveBusinessIdentifier(ctx, pathParts[i+1])
		}
	}

	return uuid.Nil
}

// resolveBusinessIdentifier converts business code, name, or UUID to UUID.
// Looks up codes/names against the tenant-scoped connection for the schema
// carried in ctx (falling back to the legacy global DB when no tenant schema
// is present, e.g. an unauthenticated route) — business_verticals rows are
// per-tenant, so resolving against the wrong schema can silently return
// another tenant's (or a pre-migration public-schema) UUID.
func resolveBusinessIdentifier(ctx context.Context, identifier string) uuid.UUID {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return uuid.Nil
	}

	// First try to parse as UUID
	if businessID, err := uuid.Parse(identifier); err == nil {
		return businessID
	}

	schemaName := config.TenantSchemaFromContext(ctx)
	normalizedIdentifier := strings.ToUpper(identifier)
	cacheKey := businessIdentifierCacheKey(schemaName, normalizedIdentifier)
	if cachedBusinessID, ok := businessIdentifierCache.get(cacheKey); ok {
		return cachedBusinessID
	}

	loaded, err, _ := businessIdentifierResolveGroup.Do(cacheKey, func() (interface{}, error) {
		if cachedBusinessID, ok := businessIdentifierCache.get(cacheKey); ok {
			return cachedBusinessID, nil
		}

		db, cleanup, dbErr := config.DBFromContext(ctx)
		if dbErr != nil {
			return uuid.Nil, dbErr
		}
		defer cleanup()

		var business models.BusinessVertical
		if dbErr := db.
			Where("is_active = ? AND (UPPER(code) = ? OR UPPER(name) = ?)", true, normalizedIdentifier, normalizedIdentifier).
			First(&business).Error; dbErr != nil {
			return uuid.Nil, dbErr
		}

		businessIdentifierCache.set(cacheKey, business.ID)
		return business.ID, nil
	})
	if err != nil {
		return uuid.Nil
	}

	return loaded.(uuid.UUID)
}

// ResolveBusinessIdentifier resolves a business code, name, or UUID to a business UUID,
// scoped to the tenant schema carried in ctx.
func ResolveBusinessIdentifier(ctx context.Context, identifier string) uuid.UUID {
	return resolveBusinessIdentifier(ctx, identifier)
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
		if dbErr := preloadAuthorizationGraph(config.DB). // config-db-ok: result cached process-wide in userCache, no request threaded into loadUserWithAuthGraph
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
