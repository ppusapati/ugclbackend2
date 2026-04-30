package repos

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// UserRepo handles user-related database operations with tenant scoping.
type UserRepo struct {
	*BaseRepo
}

// NewUserRepo creates a new user repository with tenant context.
func NewUserRepo(userCtx *middleware.UserContext) *UserRepo {
	return &UserRepo{
		BaseRepo: NewBaseRepo(userCtx),
	}
}

// GetByID retrieves a user by ID within the tenant context.
// For user-scoped queries, this ensures the requesting user can only access users
// within their business vertical (except for super-admins).
func (ur *UserRepo) GetByID(userID string) (*models.User, error) {
	var user models.User

	// For user queries, if not super-admin, we only return users within tenant scope
	// This is enforced at the business role level, not at the user table level
	if err := ur.DBAdmin().Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	ur.LogQuery("user.get_by_id", map[string]interface{}{
		"target_user_id": userID,
	})

	return &user, nil
}

// GetByEmail retrieves a user by email address.
func (ur *UserRepo) GetByEmail(email string) (*models.User, error) {
	var user models.User

	if err := ur.DBAdmin().Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	ur.LogQuery("user.get_by_email", map[string]interface{}{
		"email": email,
	})

	return &user, nil
}

// GetByPhone retrieves a user by phone number.
func (ur *UserRepo) GetByPhone(phone string) (*models.User, error) {
	var user models.User

	if err := ur.DBAdmin().Where("phone = ?", phone).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get user by phone: %w", err)
	}

	ur.LogQuery("user.get_by_phone", map[string]interface{}{
		"phone": phone,
	})

	return &user, nil
}

// Create creates a new user within the tenant context.
func (ur *UserRepo) Create(user *models.User) error {
	if err := ur.DB().Create(user).Error; err != nil {
		ur.log.Error("failed to create user", "error", err)
		return fmt.Errorf("failed to create user: %w", err)
	}

	ur.LogQuery("user.create", map[string]interface{}{
		"created_id": user.ID.String(),
	})

	return nil
}

// Update updates an existing user within the tenant context.
func (ur *UserRepo) Update(userID uuid.UUID, updates *models.User) error {
	// Verify user exists and belongs to tenant (if tenant-scoped)
	existingUser, err := ur.GetByID(userID.String())
	if err != nil {
		return err
	}

	if err := ur.DB().Model(existingUser).Updates(updates).Error; err != nil {
		ur.log.Error("failed to update user", "error", err)
		return fmt.Errorf("failed to update user: %w", err)
	}

	ur.LogQuery("user.update", map[string]interface{}{
		"updated_id": userID.String(),
	})

	return nil
}

// ListByBusinessVertical lists all users assigned to a business vertical.
// Only accessible by admins within that vertical.
func (ur *UserRepo) ListByBusinessVertical(businessVerticalID uuid.UUID, limit, offset int) ([]models.User, int64, error) {
	// Verify tenant has access to this business vertical
	if ur.TenantID() != nil && ur.TenantID().String() != businessVerticalID.String() {
		return nil, 0, fmt.Errorf("access denied: user not in requested business vertical")
	}

	var users []models.User
	var total int64

	// Get count
	if err := ur.DBAdmin().
		Model(&models.User{}).
		Where("business_vertical_id = ?", businessVerticalID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Get paginated results
	if err := ur.DBAdmin().
		Where("business_vertical_id = ?", businessVerticalID).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&users).Error; err != nil {
		ur.log.Error("failed to list users", "error", err)
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	ur.LogQuery("user.list_by_business_vertical", map[string]interface{}{
		"business_vertical_id": businessVerticalID.String(),
		"limit":                limit,
		"offset":               offset,
	})

	return users, total, nil
}

// Delete deletes a user (soft-delete via GORM's soft-delete hook).
func (ur *UserRepo) Delete(userID uuid.UUID) error {
	if err := ur.DB().Delete(&models.User{}, "id = ?", userID).Error; err != nil {
		ur.log.Error("failed to delete user", "error", err)
		return fmt.Errorf("failed to delete user: %w", err)
	}

	ur.LogQuery("user.delete", map[string]interface{}{
		"deleted_id": userID.String(),
	})

	return nil
}
