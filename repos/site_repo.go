package repos

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// SiteRepo handles site-related database operations with tenant scoping.
// Sites are always scoped to a business vertical.
type SiteRepo struct {
	*BaseRepo
}

// NewSiteRepo creates a new site repository with tenant context.
func NewSiteRepo(userCtx *middleware.UserContext) *SiteRepo {
	return &SiteRepo{
		BaseRepo: NewBaseRepo(userCtx),
	}
}

// GetByID retrieves a site by ID, ensuring it belongs to the tenant's business vertical.
func (sr *SiteRepo) GetByID(siteID uuid.UUID) (*models.Site, error) {
	var site models.Site

	// Apply tenant filtering - only get sites within the business vertical
	if err := sr.DB().Where("id = ?", siteID).First(&site).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("site not found or access denied: %w", err)
		}
		return nil, fmt.Errorf("failed to get site: %w", err)
	}

	// Double-check that the site belongs to the tenant
	if err := sr.EnforceBusinessVertical(&site); err != nil {
		return nil, fmt.Errorf("access denied: %w", err)
	}

	sr.LogQuery("site.get_by_id", map[string]interface{}{
		"site_id": siteID.String(),
	})

	return &site, nil
}

// ListByBusinessVertical lists all sites in the tenant's business vertical.
func (sr *SiteRepo) ListByBusinessVertical(limit, offset int) ([]models.Site, int64, error) {
	if sr.TenantID() == nil {
		// Super-admin context - use DBAdmin for cross-tenant queries
		var sites []models.Site
		var total int64

		if err := sr.DBAdmin().Model(&models.Site{}).Count(&total).Error; err != nil {
			return nil, 0, fmt.Errorf("failed to count sites: %w", err)
		}

		if err := sr.DBAdmin().
			Limit(limit).
			Offset(offset).
			Order("name ASC").
			Find(&sites).Error; err != nil {
			sr.log.Error("failed to list sites", "error", err)
			return nil, 0, fmt.Errorf("failed to list sites: %w", err)
		}

		sr.LogQuery("site.list_by_business_vertical (admin)", map[string]interface{}{
			"limit":  limit,
			"offset": offset,
		})

		return sites, total, nil
	}

	// Tenant-scoped query - DB() already applies business_vertical_id filter
	var sites []models.Site
	var total int64

	if err := sr.DB().Model(&models.Site{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count sites: %w", err)
	}

	if err := sr.DB().
		Limit(limit).
		Offset(offset).
		Order("name ASC").
		Find(&sites).Error; err != nil {
		sr.log.Error("failed to list sites", "error", err)
		return nil, 0, fmt.Errorf("failed to list sites: %w", err)
	}

	sr.LogQuery("site.list_by_business_vertical", map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})

	return sites, total, nil
}

// Create creates a new site within the tenant's business vertical.
func (sr *SiteRepo) Create(site *models.Site) error {
	// Ensure site is being created for the current tenant
	if sr.TenantID() != nil && site.BusinessVerticalID != *sr.TenantID() {
		return fmt.Errorf("cannot create site for different business vertical")
	}

	if err := sr.DB().Create(site).Error; err != nil {
		sr.log.Error("failed to create site", "error", err)
		return fmt.Errorf("failed to create site: %w", err)
	}

	sr.LogQuery("site.create", map[string]interface{}{
		"site_id": site.ID.String(),
	})

	return nil
}

// Update updates an existing site within the tenant context.
func (sr *SiteRepo) Update(siteID uuid.UUID, updates *models.Site) error {
	// Verify site exists and belongs to tenant
	site, err := sr.GetByID(siteID)
	if err != nil {
		return err
	}

	// Don't allow changing business vertical
	if updates.BusinessVerticalID.String() != site.BusinessVerticalID.String() {
		return fmt.Errorf("cannot change site's business vertical")
	}

	if err := sr.DB().Model(site).Updates(updates).Error; err != nil {
		sr.log.Error("failed to update site", "error", err)
		return fmt.Errorf("failed to update site: %w", err)
	}

	sr.LogQuery("site.update", map[string]interface{}{
		"site_id": siteID.String(),
	})

	return nil
}

// Delete deletes a site (soft-delete).
func (sr *SiteRepo) Delete(siteID uuid.UUID) error {
	// Verify site exists and belongs to tenant
	if _, err := sr.GetByID(siteID); err != nil {
		return err
	}

	if err := sr.DB().Delete(&models.Site{}, "id = ?", siteID).Error; err != nil {
		sr.log.Error("failed to delete site", "error", err)
		return fmt.Errorf("failed to delete site: %w", err)
	}

	sr.LogQuery("site.delete", map[string]interface{}{
		"site_id": siteID.String(),
	})

	return nil
}

// GetByCode retrieves a site by its code within the tenant's business vertical.
func (sr *SiteRepo) GetByCode(code string) (*models.Site, error) {
	var site models.Site

	if err := sr.DB().Where("code = ?", code).First(&site).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("site not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get site: %w", err)
	}

	sr.LogQuery("site.get_by_code", map[string]interface{}{
		"code": code,
	})

	return &site, nil
}

// CountByBusinessVertical returns the total number of sites in the tenant's business vertical.
func (sr *SiteRepo) CountByBusinessVertical() (int64, error) {
	var count int64

	if err := sr.DB().Model(&models.Site{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count sites: %w", err)
	}

	sr.LogQuery("site.count_by_business_vertical", map[string]interface{}{})

	return count, nil
}
