package config

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// SeedNewTenantRBACInput carries the details needed to create a brand-new
// tenant's first login. Unlike PrepareRBACMigration (which copies existing
// legacy data during the one-time production cutover), this seeds a
// genuinely empty schema from scratch — there is no prior data to copy.
type SeedNewTenantRBACInput struct {
	SuperAdminName     string
	SuperAdminEmail    string
	SuperAdminPhone    string
	SuperAdminPassword string
}

// SeedNewTenantRBAC provisions the minimal unified-RBAC starting state for a
// freshly created tenant schema: the wildcard permission, one global
// super_admin RBAC role holding it, and one user assigned to that role.
// Everything else — business verticals, additional roles, sites, forms — is
// built by the tenant's own super_admin through the normal admin UI after
// first login, the same as any tenant would configure their own structure.
// Idempotent: safe to retry if a prior provisioning attempt failed partway.
func SeedNewTenantRBAC(db *gorm.DB, input SeedNewTenantRBACInput) error {
	if db == nil {
		return errors.New("database is required")
	}
	if input.SuperAdminPhone == "" || input.SuperAdminPassword == "" {
		return errors.New("super admin phone and password are required")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		wildcard := models.Permission{
			Name:        "*:*:*",
			Description: "Full access to all resources and actions",
			Resource:    "*",
			Action:      "*",
		}
		if err := tx.Where("name = ?", wildcard.Name).
			Attrs(models.Permission{ID: uuid.New()}).
			FirstOrCreate(&wildcard).Error; err != nil {
			return fmt.Errorf("seed wildcard permission: %w", err)
		}

		superAdminRole := models.RBACRole{
			Name:        "super_admin",
			DisplayName: "Super Administrator",
			Description: "Full, unrestricted access to this tenant",
			ScopeType:   models.RoleScopeGlobal,
			Level:       0,
			IsActive:    true,
		}
		if err := tx.Where("name = ? AND scope_type = ?", superAdminRole.Name, models.RoleScopeGlobal).
			Attrs(models.RBACRole{ID: uuid.New()}).
			FirstOrCreate(&superAdminRole).Error; err != nil {
			return fmt.Errorf("seed super_admin role: %w", err)
		}

		if err := tx.Model(&superAdminRole).Association("Permissions").Append(&wildcard); err != nil {
			return fmt.Errorf("attach wildcard permission to super_admin role: %w", err)
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.SuperAdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash super admin password: %w", err)
		}

		superAdminUser := models.User{
			Name:         input.SuperAdminName,
			Email:        input.SuperAdminEmail,
			Phone:        input.SuperAdminPhone,
			PasswordHash: string(passwordHash),
			IsActive:     true,
		}
		if err := tx.Where("phone = ?", superAdminUser.Phone).
			Attrs(models.User{ID: uuid.New()}).
			FirstOrCreate(&superAdminUser).Error; err != nil {
			return fmt.Errorf("seed super admin user: %w", err)
		}

		scopeKey, err := superAdminRole.AssignmentScopeKey()
		if err != nil {
			return fmt.Errorf("derive assignment scope key: %w", err)
		}

		assignment := models.UserRoleAssignment{
			UserID:   superAdminUser.ID,
			RoleID:   superAdminRole.ID,
			ScopeKey: scopeKey,
			IsActive: true,
		}
		if err := tx.Where("user_id = ? AND role_id = ?", assignment.UserID, assignment.RoleID).
			Attrs(models.UserRoleAssignment{ID: uuid.New()}).
			FirstOrCreate(&assignment).Error; err != nil {
			return fmt.Errorf("assign super_admin role to user: %w", err)
		}

		return nil
	})
}
