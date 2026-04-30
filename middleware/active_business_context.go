package middleware

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
func SaveActiveBusinessContext(userID, businessID uuid.UUID, clientKey string) (*models.UserActiveBusinessContext, error) {
	if clientKey == "" {
		clientKey = defaultActiveBusinessClientKey
	}

	ctx := &models.UserActiveBusinessContext{
		UserID:     userID,
		BusinessID: businessID,
		ClientKey:  clientKey,
	}

	if err := config.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "client_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"business_id", "updated_at"}),
	}).Create(ctx).Error; err != nil {
		return nil, err
	}

	if err := config.DB.Preload("Business").First(ctx, "user_id = ? AND client_key = ?", userID, clientKey).Error; err != nil {
		return nil, err
	}

	activeBusinessContextCache.set(userID, clientKey, ctx, true)

	return ctx, nil
}

// GetStoredActiveBusinessContext returns the persisted active business for a user/client pair.
func GetStoredActiveBusinessContext(userID uuid.UUID, clientKey string) (*models.UserActiveBusinessContext, error) {
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

		var ctx models.UserActiveBusinessContext
		result := config.DB.Preload("Business").
			Where("user_id = ? AND client_key = ?", userID, clientKey).
			Limit(1).
			Find(&ctx)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			activeBusinessContextCache.set(userID, clientKey, nil, false)
			return nil, gorm.ErrRecordNotFound
		}

		activeBusinessContextCache.set(userID, clientKey, &ctx, true)
		return &ctx, nil
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
	storedCtx, err := GetStoredActiveBusinessContext(userCtx.User.ID, clientKey)
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
