package config

import (
	"log"

	"p9e.in/ugcl/models"
)

// MigrateSites creates the sites and user_site_accesses tables
func MigrateSites() {
	log.Println("🔄 Running site migrations...")

	// Auto-migrate the Site and UserSiteAccess models
	if err := DB.AutoMigrate(
		&models.Site{},
		&models.UserSiteAccess{},
	); err != nil {
		log.Fatalf("❌ Failed to migrate site tables: %v", err)
	}

	log.Println("✅ Site tables migrated successfully")
}
