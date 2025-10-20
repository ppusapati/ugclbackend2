package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

func main() {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Connect to database
	dsn := os.Getenv("DB_DSN")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	config.DB = db

	fmt.Println("🚀 Starting authorization system migration...")

	// Run migrations
	if err := config.Migrations(db); err != nil {
		log.Fatal("Migration failed:", err)
	}
	fmt.Println("✅ Database migrations completed")

	// Seed permissions and roles
	config.SeedPermissions()
	fmt.Println("✅ Permissions and roles seeded")

	// Migrate existing users to new role system
	migrateExistingUsers()
	fmt.Println("✅ Existing users migrated")

	fmt.Println("🎉 Authorization system migration completed successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Update your main.go to use routes.RegisterRoutesV2()")
	fmt.Println("2. Test the new endpoints with /api/v1/test/auth")
	fmt.Println("3. Create new roles via /api/v1/admin/roles")
}

func migrateExistingUsers() {
	var users []models.User
	if err := config.DB.Find(&users).Error; err != nil {
		log.Printf("Error fetching users: %v", err)
		return
	}

	for _, user := range users {
		if user.RoleID != nil {
			continue // Already migrated
		}

		// Map legacy role to new role system
		var roleName string
		switch user.Role {
		case "Super Admin", "super_admin":
			roleName = "super_admin"
		case "admin":
			roleName = "admin"
		case "project_coordinator":
			roleName = "project_coordinator"
		case "user":
			roleName = "user"
		default:
			roleName = "user" // Default fallback
		}

		// Find the role
		var role models.Role
		if err := config.DB.Where("name = ?", roleName).First(&role).Error; err != nil {
			log.Printf("Role %s not found for user %s", roleName, user.Name)
			continue
		}

		// Update user with new role
		user.RoleID = &role.ID
		if err := config.DB.Save(&user).Error; err != nil {
			log.Printf("Error updating user %s: %v", user.Name, err)
		} else {
			fmt.Printf("✅ Migrated user %s to role %s\n", user.Name, roleName)
		}
	}
}