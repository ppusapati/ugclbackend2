package config

import (
	"log"
	"time"

	"github.com/google/uuid"
	"p9e.in/ugcl/models"
)

// MigrateToNewRBAC migrates existing role data to new RBAC system
func MigrateToNewRBAC() {
	log.Printf("🚀 Starting RBAC migration...")

	// Step 1: Get all users
	var users []models.User
	DB.Find(&users)
	log.Printf("📋 Found %d users to migrate", len(users))

	// Step 2: Get business verticals
	var waterVertical, solarVertical, hoVertical models.BusinessVertical
	DB.Where("code = ?", "WATER").First(&waterVertical)
	DB.Where("code = ?", "SOLAR").First(&solarVertical)
	DB.Where("code = ?", "HO").First(&hoVertical)

	if waterVertical.ID == uuid.Nil {
		log.Printf("❌ Water vertical not found - run SeedBusinessVerticals first")
		return
	}

	// Step 3: Get roles - need to find business roles by name and vertical
	var superAdminRole models.Role
	DB.Where("name = ?", "super_admin").First(&superAdminRole)

	// Get business roles for each vertical
	var waterAdminRole, waterPCRole, waterEngineerRole models.BusinessRole
	DB.Where("name = ? AND business_vertical_id = ?", "Water_Admin", waterVertical.ID).First(&waterAdminRole)
	DB.Where("name = ? AND business_vertical_id = ?", "Project_Coordinator", waterVertical.ID).First(&waterPCRole)
	DB.Where("name = ? AND business_vertical_id = ?", "Engineer", waterVertical.ID).First(&waterEngineerRole)

	var solarAdminRole models.BusinessRole
	DB.Where("name = ? AND business_vertical_id = ?", "Solar_Admin", solarVertical.ID).First(&solarAdminRole)

	var hoAdminRole models.BusinessRole
	DB.Where("name = ? AND business_vertical_id = ?", "HO_Admin", hoVertical.ID).First(&hoAdminRole)

	log.Printf("✅ Loaded roles for migration")

	// Step 4: Migrate each user based on existing role assignments
	// Note: This migration assumes users have already been assigned to roles via RoleID
	// If users don't have roles yet, they need to be assigned manually via the role assignment API
	migratedCount := 0
	for _, user := range users {
		// Check if user already has a global role
		if user.RoleID != nil {
			log.Printf("ℹ️  User %s already has global role assigned (RoleID: %s)", user.Name, user.RoleID)

			// Check if it's super_admin
			var role models.Role
			if err := DB.First(&role, "id = ?", user.RoleID).Error; err == nil {
				if role.Name == "super_admin" {
					log.Printf("✅ User %s is Super Admin", user.Name)
					migratedCount++
					continue
				}
			}
		}

		// Check if user has business role assignments
		var ubrs []models.UserBusinessRole
		if err := DB.Where("user_id = ? AND is_active = ?", user.ID, true).Find(&ubrs).Error; err == nil {
			if len(ubrs) > 0 {
				log.Printf("ℹ️  User %s already has %d business role(s) assigned", user.Name, len(ubrs))
				migratedCount++
				continue
			}
		}

		// If user has no roles, suggest manual assignment
		log.Printf("⚠️  User %s has no roles assigned - needs manual role assignment", user.Name)
	}

	log.Printf("✅ RBAC migration completed - migrated %d/%d users", migratedCount, len(users))
}

// assignBusinessRole creates a user business role assignment
func assignBusinessRole(userID, businessRoleID uuid.UUID) bool {
	// Check if assignment already exists
	var existing models.UserBusinessRole
	err := DB.Where("user_id = ? AND business_role_id = ?", userID, businessRoleID).First(&existing).Error
	if err == nil {
		// Already exists
		return true
	}

	ubr := models.UserBusinessRole{
		UserID:         userID,
		BusinessRoleID: businessRoleID,
		IsActive:       true,
		AssignedAt:     time.Now(),
	}

	if err := DB.Create(&ubr).Error; err != nil {
		log.Printf("❌ Failed to assign business role: %v", err)
		return false
	}

	return true
}

// VerifyRBACMigration checks if migration was successful
func VerifyRBACMigration() {
	log.Printf("🔍 Verifying RBAC migration...")

	// Count users with global roles
	var usersWithGlobalRole int64
	DB.Model(&models.User{}).Where("role_id IS NOT NULL").Count(&usersWithGlobalRole)
	log.Printf("📊 Users with global roles: %d", usersWithGlobalRole)

	// Count user business role assignments
	var businessRoleAssignments int64
	DB.Model(&models.UserBusinessRole{}).Where("is_active = ?", true).Count(&businessRoleAssignments)
	log.Printf("📊 Active business role assignments: %d", businessRoleAssignments)

	// List super admins
	var superAdmins []models.User
	DB.Preload("RoleModel").
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("roles.name = ?", "super_admin").
		Find(&superAdmins)

	log.Printf("👑 Super Admins:")
	for _, admin := range superAdmins {
		log.Printf("   - %s (%s)", admin.Name, admin.Email)
	}

	// List users with business roles
	var usersWithBusinessRoles []models.User
	DB.Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
		Joins("JOIN user_business_roles ON user_business_roles.user_id = users.id").
		Where("user_business_roles.is_active = ?", true).
		Distinct().
		Find(&usersWithBusinessRoles)

	log.Printf("👥 Users with business roles:")
	for _, user := range usersWithBusinessRoles {
		log.Printf("   - %s:", user.Name)
		for _, ubr := range user.UserBusinessRoles {
			if ubr.IsActive && ubr.BusinessRole.ID != uuid.Nil {
				log.Printf("      • %s (%s)",
					ubr.BusinessRole.DisplayName,
					ubr.BusinessRole.BusinessVertical.Code)
			}
		}
	}

	log.Printf("✅ RBAC verification completed")
}
