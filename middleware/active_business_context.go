package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

const defaultActiveBusinessClientKey = "default"

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

	return ctx, nil
}

// GetStoredActiveBusinessContext returns the persisted active business for a user/client pair.
func GetStoredActiveBusinessContext(userID uuid.UUID, clientKey string) (*models.UserActiveBusinessContext, error) {
	if clientKey == "" {
		clientKey = defaultActiveBusinessClientKey
	}

	var ctx models.UserActiveBusinessContext
	err := config.DB.Preload("Business").First(&ctx, "user_id = ? AND client_key = ?", userID, clientKey).Error
	if err != nil {
		return nil, err
	}

	return &ctx, nil
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

	accessible := authService.GetAccessibleBusinessVerticals(userCtx.User)
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
