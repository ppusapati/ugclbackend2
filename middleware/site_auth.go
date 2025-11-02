package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

type contextKey string

const (
	siteAccessKey contextKey = "site_access"
)

// SiteAccessContext contains site-level access information
type SiteAccessContext struct {
	AccessibleSiteIDs []uuid.UUID            `json:"accessibleSiteIds"`
	SitePermissions   map[uuid.UUID]SitePerm `json:"sitePermissions"`
}

// SitePerm represents permissions for a specific site
type SitePerm struct {
	CanRead   bool `json:"canRead"`
	CanCreate bool `json:"canCreate"`
	CanUpdate bool `json:"canUpdate"`
	CanDelete bool `json:"canDelete"`
}

// RequireSiteAccess middleware checks if user has access to at least one site in the business vertical
// This should be used after RequireBusinessAccess middleware
func RequireSiteAccess() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
			// GetUser returns a value (models.User); check the ID for a zero value instead of comparing to nil
			if user.ID == uuid.Nil {
				http.Error(w, "user not found in context", http.StatusUnauthorized)
				return
			}

			businessContext := GetUserBusinessContext(r)
			if businessContext == nil {
				http.Error(w, "business context not found", http.StatusBadRequest)
				return
			}

			businessID, ok := businessContext["business_id"].(uuid.UUID)
			if !ok {
				http.Error(w, "invalid business context", http.StatusInternalServerError)
				return
			}

			// Get all sites the user has access to in this business vertical
			var siteAccess []models.UserSiteAccess
			err := config.DB.
				Joins("JOIN sites ON sites.id = user_site_accesses.site_id").
				Where("user_site_accesses.user_id = ? AND sites.business_vertical_id = ?", user.ID, businessID).
				Find(&siteAccess).Error

			if err != nil {
				http.Error(w, "failed to retrieve site access", http.StatusInternalServerError)
				return
			}

			// Check if user has access to at least one site
			if len(siteAccess) == 0 {
				http.Error(w, "no site access granted", http.StatusForbidden)
				return
			}

			// Build site access context
			siteIDs := make([]uuid.UUID, 0, len(siteAccess))
			sitePerms := make(map[uuid.UUID]SitePerm)

			for _, access := range siteAccess {
				siteIDs = append(siteIDs, access.SiteID)
				sitePerms[access.SiteID] = SitePerm{
					CanRead:   access.CanRead,
					CanCreate: access.CanCreate,
					CanUpdate: access.CanUpdate,
					CanDelete: access.CanDelete,
				}
			}

			siteAccessCtx := SiteAccessContext{
				AccessibleSiteIDs: siteIDs,
				SitePermissions:   sitePerms,
			}

			// Add to request context
			ctx := context.WithValue(r.Context(), siteAccessKey, siteAccessCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSiteAccessContext retrieves site access information from request context
func GetSiteAccessContext(r *http.Request) *SiteAccessContext {
	if ctx := r.Context().Value(siteAccessKey); ctx != nil {
		if siteCtx, ok := ctx.(SiteAccessContext); ok {
			return &siteCtx
		}
	}
	return nil
}

// CanAccessSite checks if user can access a specific site
func CanAccessSite(r *http.Request, siteID uuid.UUID) bool {
	siteCtx := GetSiteAccessContext(r)
	if siteCtx == nil {
		return false
	}

	for _, id := range siteCtx.AccessibleSiteIDs {
		if id == siteID {
			return true
		}
	}
	return false
}

// CanPerformSiteAction checks if user can perform a specific action in a site
func CanPerformSiteAction(r *http.Request, siteID uuid.UUID, action string) bool {
	siteCtx := GetSiteAccessContext(r)
	if siteCtx == nil {
		return false
	}

	perm, ok := siteCtx.SitePermissions[siteID]
	if !ok {
		return false
	}

	switch action {
	case "read":
		return perm.CanRead
	case "create":
		return perm.CanCreate
	case "update":
		return perm.CanUpdate
	case "delete":
		return perm.CanDelete
	default:
		return false
	}
}

// CanCreateInSite checks if user can create in a specific site
func CanCreateInSite(r *http.Request, siteID uuid.UUID) bool {
	siteCtx := GetSiteAccessContext(r)
	if siteCtx == nil {
		return false
	}

	if perm, ok := siteCtx.SitePermissions[siteID]; ok {
		return perm.CanCreate
	}
	return false
}

// CanUpdateInSite checks if user can update in a specific site
func CanUpdateInSite(r *http.Request, siteID uuid.UUID) bool {
	siteCtx := GetSiteAccessContext(r)
	if siteCtx == nil {
		return false
	}

	if perm, ok := siteCtx.SitePermissions[siteID]; ok {
		return perm.CanUpdate
	}
	return false
}

// CanDeleteInSite checks if user can delete in a specific site
func CanDeleteInSite(r *http.Request, siteID uuid.UUID) bool {
	siteCtx := GetSiteAccessContext(r)
	if siteCtx == nil {
		return false
	}

	if perm, ok := siteCtx.SitePermissions[siteID]; ok {
		return perm.CanDelete
	}
	return false
}
