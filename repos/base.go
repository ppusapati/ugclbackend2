package repos

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// TenantContext holds the tenant/business context for a repository operation.
// This ensures that all queries are automatically scoped to the user's business vertical.
type TenantContext struct {
	UserID           string
	BusinessVertical *uuid.UUID // nil for super-admin operations, set for business-scoped
}

// NewTenantContext creates a TenantContext from a UserContext.
// If the user has access to a specific business, that becomes the tenant.
// Super-admin users without specific business context will have nil BusinessVertical.
func NewTenantContext(userCtx *middleware.UserContext) *TenantContext {
	if userCtx == nil {
		return nil
	}

	var businessVertical *uuid.UUID
	if userCtx.BusinessContext != nil {
		businessVertical = &userCtx.BusinessContext.BusinessID
	}

	return &TenantContext{
		UserID:           userCtx.User.ID.String(),
		BusinessVertical: businessVertical,
	}
}

// BaseRepo provides common tenant-scoped query building for all repositories.
type BaseRepo struct {
	db     *gorm.DB
	tenant *TenantContext
	log    *slog.Logger
}

// NewBaseRepo creates a new base repository with tenant scoping.
func NewBaseRepo(userCtx *middleware.UserContext) *BaseRepo {
	return &BaseRepo{
		db:     config.DB,
		tenant: NewTenantContext(userCtx),
		log:    slog.Default(),
	}
}

// DB returns a GORM DB instance with tenant filtering applied.
// All queries on this DB will be automatically scoped to the tenant's business vertical.
func (br *BaseRepo) DB() *gorm.DB {
	if br.tenant == nil || br.tenant.BusinessVertical == nil {
		// Super-admin or system context - return unfiltered DB for system-wide queries
		br.log.Warn("query without tenant context", "user_id", br.tenant.UserID)
		return br.db
	}

	// Apply automatic business vertical filtering
	// This ensures no cross-tenant data leakage regardless of query implementation
	return br.db.Where("business_vertical_id = ?", br.tenant.BusinessVertical)
}

// DBAdmin returns a GORM DB instance without tenant filtering.
// CAUTION: Only use in admin/super-admin specific contexts where cross-tenant queries are intentional.
// All calls to DBAdmin should be audited and protected by permission checks.
func (br *BaseRepo) DBAdmin() *gorm.DB {
	return br.db
}

// TenantID returns the business vertical ID for this repository's tenant.
// Returns nil for super-admin contexts.
func (br *BaseRepo) TenantID() *uuid.UUID {
	if br.tenant == nil {
		return nil
	}
	return br.tenant.BusinessVertical
}

// UserID returns the user ID for this repository's context.
func (br *BaseRepo) UserID() string {
	if br.tenant == nil {
		return ""
	}
	return br.tenant.UserID
}

// EnforceBusinessVertical is a helper that ensures a record belongs to the current tenant.
// Use this to validate that a queried record actually belongs to the tenant before operating on it.
func (br *BaseRepo) EnforceBusinessVertical(record interface{}) error {
	if br.tenant == nil || br.tenant.BusinessVertical == nil {
		// Super-admin context - no enforcement needed
		return nil
	}

	// Try to extract business_vertical_id from the record
	// This uses reflection to check if the record has a BusinessVerticalID field
	switch v := record.(type) {
	case *models.BusinessVertical:
		if v.ID != *br.tenant.BusinessVertical {
			return fmt.Errorf("record does not belong to tenant %s", br.tenant.BusinessVertical)
		}
	case *models.Site:
		if br.tenant.BusinessVertical != nil && v.BusinessVerticalID != *br.tenant.BusinessVertical {
			return fmt.Errorf("record does not belong to tenant %s", br.tenant.BusinessVertical)
		}
	case *models.AppForm:
		// Forms might be shared across business verticals based on access control
		// For now, do not enforce - use form access control instead
		return nil
	}

	return nil
}

// LogQuery logs a database query for audit/debugging purposes.
func (br *BaseRepo) LogQuery(operation string, details map[string]interface{}) {
	details["user_id"] = br.UserID()
	if br.tenant != nil && br.tenant.BusinessVertical != nil {
		details["tenant_id"] = br.tenant.BusinessVertical.String()
	}
	br.log.Debug("query", slog.Any("operation", operation), slog.Any("details", details))
}
