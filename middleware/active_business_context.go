package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

const defaultActiveBusinessClientKey = "default"

const activeBusinessContextCacheTTL = 30 * time.Second

type activeBusinessContextCacheEntry struct {
	ctx       *models.UserActiveBusinessContext
	found     bool
	expiresAt time.Time
}

type activeBusinessContextCacheStore struct {
	mu      sync.RWMutex
	entries map[string]activeBusinessContextCacheEntry
}

var activeBusinessContextCache = &activeBusinessContextCacheStore{
	entries: make(map[string]activeBusinessContextCacheEntry),
}

var activeBusinessContextLoadGroup singleflight.Group

func activeBusinessContextCacheKey(userID uuid.UUID, clientKey string) string {
	return userID.String() + ":" + clientKey
}

func (c *activeBusinessContextCacheStore) get(userID uuid.UUID, clientKey string) (*models.UserActiveBusinessContext, bool, bool) {
	key := activeBusinessContextCacheKey(userID, clientKey)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false, false
	}
	if !entry.found || entry.ctx == nil {
		return nil, true, false
	}

	ctxCopy := *entry.ctx
	return &ctxCopy, true, true
}

func (c *activeBusinessContextCacheStore) set(userID uuid.UUID, clientKey string, ctx *models.UserActiveBusinessContext, found bool) {
	key := activeBusinessContextCacheKey(userID, clientKey)

	var ctxCopy *models.UserActiveBusinessContext
	if found && ctx != nil {
		copied := *ctx
		ctxCopy = &copied
	}

	c.mu.Lock()
	c.entries[key] = activeBusinessContextCacheEntry{
		ctx:       ctxCopy,
		found:     found,
		expiresAt: time.Now().Add(activeBusinessContextCacheTTL),
	}
	c.mu.Unlock()
}

// GetActiveBusinessClientKey returns the current client key used to scope active business context.
func GetActiveBusinessClientKey(r *http.Request) string {
	if r == nil {
		return defaultActiveBusinessClientKey
	}

	clientKey := strings.TrimSpace(r.Header.Get("X-Client-ID"))
	if clientKey == "" {
		clientKey = strings.TrimSpace(r.URL.Query().Get("client_key"))
	}
	if clientKey == "" {
		return defaultActiveBusinessClientKey
	}

	return clientKey
}

// CanAccessBusiness checks whether the user has access to the given business vertical.
func CanAccessBusiness(userCtx *UserContext, businessID uuid.UUID) bool {
	if userCtx == nil {
		return false
	}

	if userCtx.IsSuperAdmin {
		return true
	}

	if RBACEnabled() {
		// Business-scoped assignments grant access to exactly that vertical.
		if len(unifiedBusinessAssignments(*userCtx.User, businessID)) > 0 {
			return true
		}
		// Global-role users have no business-scoped assignment but are system-wide
		// administrators. They may access any vertical they have explicit site access
		// entries for; the site-level check in the handler enforces that boundary.
		for _, a := range userCtx.User.RoleAssignments {
			if a.IsActive && a.Role.IsActive && a.Role.ScopeType == models.RoleScopeGlobal {
				return true
			}
		}
		return false
	}

	if userCtx.User.BusinessVerticalID != nil && *userCtx.User.BusinessVerticalID == businessID {
		return true
	}

	for _, ubr := range userCtx.User.UserBusinessRoles {
		if ubr.IsActive && ubr.BusinessRole.BusinessVerticalID == businessID {
			return true
		}
	}

	return false
}

// SaveActiveBusinessContext upserts the current active business for a user/client pair.
func SaveActiveBusinessContext(reqCtx context.Context, userID, businessID uuid.UUID, clientKey string) (*models.UserActiveBusinessContext, error) {
	if clientKey == "" {
		clientKey = defaultActiveBusinessClientKey
	}

	db, cleanup, err := config.DBFromContext(reqCtx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	ctxRecord := &models.UserActiveBusinessContext{}
	if err := db.Transaction(func(tx *gorm.DB) error {
		var existing models.UserActiveBusinessContext
		err := tx.Where("user_id = ? AND client_key = ?", userID, clientKey).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctxRecord.UserID = userID
				ctxRecord.BusinessID = businessID
				ctxRecord.ClientKey = clientKey
				return tx.Create(ctxRecord).Error
			}
			return err
		}

		existing.BusinessID = businessID
		if err := tx.Model(&existing).Updates(map[string]interface{}{
			"business_id": businessID,
			"updated_at":  time.Now(),
		}).Error; err != nil {
			return err
		}

		*ctxRecord = existing
		return nil
	}); err != nil {
		return nil, err
	}

	if err := db.Preload("Business").First(ctxRecord, "user_id = ? AND client_key = ?", userID, clientKey).Error; err != nil {
		return nil, err
	}

	activeBusinessContextCache.set(userID, clientKey, ctxRecord, true)

	return ctxRecord, nil
}

// GetStoredActiveBusinessContext returns the persisted active business for a user/client pair.
func GetStoredActiveBusinessContext(userID uuid.UUID, clientKey string) (*models.UserActiveBusinessContext, error) {
	return GetStoredActiveBusinessContextWithContext(context.Background(), userID, clientKey)
}

// GetStoredActiveBusinessContextWithContext returns the persisted active business for a user/client pair
// using the caller's context so request timeouts/cancellation propagate to the DB layer.
func GetStoredActiveBusinessContextWithContext(reqCtx context.Context, userID uuid.UUID, clientKey string) (*models.UserActiveBusinessContext, error) {
	if clientKey == "" {
		clientKey = defaultActiveBusinessClientKey
	}

	if cachedCtx, cached, found := activeBusinessContextCache.get(userID, clientKey); cached {
		if found {
			return cachedCtx, nil
		}
		return nil, gorm.ErrRecordNotFound
	}

	loaded, loadErr, _ := activeBusinessContextLoadGroup.Do(activeBusinessContextCacheKey(userID, clientKey), func() (interface{}, error) {
		if cachedCtx, cached, found := activeBusinessContextCache.get(userID, clientKey); cached {
			if found {
				return cachedCtx, nil
			}
			return nil, gorm.ErrRecordNotFound
		}

		db, cleanup, err := config.DBFromContext(reqCtx)
		if err != nil {
			return nil, err
		}
		defer cleanup()

		var record models.UserActiveBusinessContext
		result := db.WithContext(reqCtx).Preload("Business").
			Where("user_id = ? AND client_key = ?", userID, clientKey).
			Limit(1).
			Find(&record)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			activeBusinessContextCache.set(userID, clientKey, nil, false)
			return nil, gorm.ErrRecordNotFound
		}

		activeBusinessContextCache.set(userID, clientKey, &record, true)
		return &record, nil
	})
	if loadErr != nil {
		return nil, loadErr
	}

	return loaded.(*models.UserActiveBusinessContext), nil
}

// ResolveEffectiveBusinessID determines the active business for the request.
func ResolveEffectiveBusinessID(r *http.Request, userCtx *UserContext) (uuid.UUID, error) {
	if userCtx == nil {
		return uuid.Nil, ErrUnauthorized
	}

	if requestedBusinessID := getBusinessIDFromRequest(r); requestedBusinessID != uuid.Nil {
		if !CanAccessBusiness(userCtx, requestedBusinessID) {
			return uuid.Nil, ErrNoBusinessAccess
		}
		return requestedBusinessID, nil
	}

	clientKey := GetActiveBusinessClientKey(r)
	storedCtx, err := GetStoredActiveBusinessContextWithContext(r.Context(), userCtx.User.ID, clientKey)
	if err == nil {
		if CanAccessBusiness(userCtx, storedCtx.BusinessID) {
			return storedCtx.BusinessID, nil
		}
	}

	if userCtx.User.BusinessVerticalID != nil && CanAccessBusiness(userCtx, *userCtx.User.BusinessVerticalID) {
		return *userCtx.User.BusinessVerticalID, nil
	}

	accessible := authService.GetAccessibleBusinessVerticals(*userCtx.User)
	if len(accessible) == 1 {
		return accessible[0], nil
	}

	return uuid.Nil, ErrBusinessNotSpecified
}

// GinActiveBusinessContextMiddleware loads validated active business context into Gin.
func GinActiveBusinessContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userCtx, err := authService.LoadUserContext(c.Request)
		if err != nil {
			handleGinAuthError(c, err)
			return
		}

		businessID, err := ResolveEffectiveBusinessID(c.Request, userCtx)
		if err != nil {
			handleGinAuthError(c, err)
			return
		}

		c.Set("business_id", businessID)
		c.Next()
	}
}

func handleGinAuthError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "authorization error"

	var authErr *AuthError
	if errors.As(err, &authErr) {
		status = authErr.Code
		message = authErr.Message
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		status = http.StatusNotFound
		message = "active business context not found"
	}

	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
