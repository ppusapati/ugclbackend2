package repos

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// BusinessVerticalRepo handles business vertical operations at the admin level.
// Business verticals are tenant-level entities and this repo primarily serves super-admins.
type BusinessVerticalRepo struct {
	*BaseRepo
}

// NewBusinessVerticalRepo creates a new business vertical repository.
func NewBusinessVerticalRepo(userCtx *middleware.UserContext) *BusinessVerticalRepo {
	return &BusinessVerticalRepo{
		BaseRepo: NewBaseRepo(userCtx),
	}
}

// GetByID retrieves a business vertical by ID.
// If user is scoped to a business, they can only view their own business.
// Super-admins can view any business.
func (bvr *BusinessVerticalRepo) GetByID(businessVerticalID uuid.UUID) (*models.BusinessVertical, error) {
	// Verify access: user can only access their own business unless super-admin
	if bvr.TenantID() != nil && bvr.TenantID().String() != businessVerticalID.String() {
		return nil, fmt.Errorf("access denied: user not in requested business vertical")
	}

	var bv models.BusinessVertical

	if err := bvr.DBAdmin().Where("id = ?", businessVerticalID).First(&bv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("business vertical not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get business vertical: %w", err)
	}

	bvr.LogQuery("business_vertical.get_by_id", map[string]interface{}{
		"business_vertical_id": businessVerticalID.String(),
	})

	return &bv, nil
}

// GetByCode retrieves a business vertical by its code.
func (bvr *BusinessVerticalRepo) GetByCode(code string) (*models.BusinessVertical, error) {
	var bv models.BusinessVertical

	if err := bvr.DBAdmin().Where("code = ?", code).First(&bv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("business vertical not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get business vertical: %w", err)
	}

	// Verify access
	if bvr.TenantID() != nil && bvr.TenantID().String() != bv.ID.String() {
		return nil, fmt.Errorf("access denied: user not in requested business vertical")
	}

	bvr.LogQuery("business_vertical.get_by_code", map[string]interface{}{
		"code": code,
	})

	return &bv, nil
}

// ListAll lists all business verticals.
// Super-admins get all; scoped users get only their own.
func (bvr *BusinessVerticalRepo) ListAll(limit, offset int) ([]models.BusinessVertical, int64, error) {
	var bvs []models.BusinessVertical
	var total int64
	var query *gorm.DB

	// If user is scoped to a business, only return that business
	if bvr.TenantID() != nil {
		query = bvr.DBAdmin().Where("id = ?", bvr.TenantID())
	} else {
		// Super-admin - return all businesses
		query = bvr.DBAdmin()
	}

	if err := query.Model(&models.BusinessVertical{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count business verticals: %w", err)
	}

	if err := query.
		Limit(limit).
		Offset(offset).
		Order("name ASC").
		Find(&bvs).Error; err != nil {
		bvr.log.Error("failed to list business verticals", "error", err)
		return nil, 0, fmt.Errorf("failed to list business verticals: %w", err)
	}

	bvr.LogQuery("business_vertical.list_all", map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})

	return bvs, total, nil
}

// Create creates a new business vertical.
// Only super-admins should call this (permission check is at handler level).
func (bvr *BusinessVerticalRepo) Create(bv *models.BusinessVertical) error {
	if err := bvr.DBAdmin().Create(bv).Error; err != nil {
		bvr.log.Error("failed to create business vertical", "error", err)
		return fmt.Errorf("failed to create business vertical: %w", err)
	}

	bvr.LogQuery("business_vertical.create", map[string]interface{}{
		"created_id": bv.ID.String(),
	})

	return nil
}

// Update updates an existing business vertical.
func (bvr *BusinessVerticalRepo) Update(businessVerticalID uuid.UUID, updates *models.BusinessVertical) error {
	// Verify access
	if bvr.TenantID() != nil && bvr.TenantID().String() != businessVerticalID.String() {
		return fmt.Errorf("access denied: user not in requested business vertical")
	}

	// Verify record exists
	bv, err := bvr.GetByID(businessVerticalID)
	if err != nil {
		return err
	}

	if err := bvr.DBAdmin().Model(bv).Updates(updates).Error; err != nil {
		bvr.log.Error("failed to update business vertical", "error", err)
		return fmt.Errorf("failed to update business vertical: %w", err)
	}

	bvr.LogQuery("business_vertical.update", map[string]interface{}{
		"updated_id": businessVerticalID.String(),
	})

	return nil
}

// Delete deletes a business vertical (soft-delete).
func (bvr *BusinessVerticalRepo) Delete(businessVerticalID uuid.UUID) error {
	// Verify access
	if bvr.TenantID() != nil && bvr.TenantID().String() != businessVerticalID.String() {
		return fmt.Errorf("access denied: user not in requested business vertical")
	}

	if err := bvr.DBAdmin().Delete(&models.BusinessVertical{}, "id = ?", businessVerticalID).Error; err != nil {
		bvr.log.Error("failed to delete business vertical", "error", err)
		return fmt.Errorf("failed to delete business vertical: %w", err)
	}

	bvr.LogQuery("business_vertical.delete", map[string]interface{}{
		"deleted_id": businessVerticalID.String(),
	})

	return nil
}

// GetAllWithModules retrieves all business verticals with their associated modules.
func (bvr *BusinessVerticalRepo) GetAllWithModules() ([]models.BusinessVertical, error) {
	var bvs []models.BusinessVertical

	query := bvr.DBAdmin().
		Preload("Modules").
		Order("name ASC")

	// If user is scoped, only return their business vertical
	if bvr.TenantID() != nil {
		query = query.Where("id = ?", bvr.TenantID())
	}

	if err := query.Find(&bvs).Error; err != nil {
		bvr.log.Error("failed to get business verticals with modules", "error", err)
		return nil, fmt.Errorf("failed to get business verticals with modules: %w", err)
	}

	bvr.LogQuery("business_vertical.get_all_with_modules", map[string]interface{}{})

	return bvs, nil
}
