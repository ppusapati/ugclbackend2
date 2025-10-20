package config

import (
	"log"

	"github.com/google/uuid"
	"p9e.in/ugcl/models"
)

// SeedSites creates default sites for each business vertical
func SeedSites() {
	log.Println("🔄 Seeding default sites...")

	// Get business verticals
	var waterBusiness, solarBusiness models.BusinessVertical

	// Find Water Works business vertical
	if err := DB.Where("code = ?", "WATER").First(&waterBusiness).Error; err != nil {
		log.Printf("⚠️  Water Works business vertical not found, skipping water sites: %v", err)
	} else {
		seedWaterSites(waterBusiness.ID)
	}

	// Find Solar Works business vertical
	if err := DB.Where("code = ?", "SOLAR").First(&solarBusiness).Error; err != nil {
		log.Printf("⚠️  Solar Works business vertical not found, skipping solar sites: %v", err)
	} else {
		seedSolarSites(solarBusiness.ID)
	}

	log.Println("✅ Site seeding completed")
}

// seedWaterSites creates 4 default sites for Water Works
func seedWaterSites(businessVerticalID uuid.UUID) {
	waterSites := []models.Site{
		{
			Name:               "Water Site A",
			Code:               "WATER_SITE_A",
			Description:        "Primary water distribution site",
			BusinessVerticalID: businessVerticalID,
			IsActive:           true,
		},
		{
			Name:               "Water Site B",
			Code:               "WATER_SITE_B",
			Description:        "Secondary water distribution site",
			BusinessVerticalID: businessVerticalID,
			IsActive:           true,
		},
		{
			Name:               "Water Site C",
			Code:               "WATER_SITE_C",
			Description:        "Water treatment facility",
			BusinessVerticalID: businessVerticalID,
			IsActive:           true,
		},
		{
			Name:               "Water Site D",
			Code:               "WATER_SITE_D",
			Description:        "Reserve water storage site",
			BusinessVerticalID: businessVerticalID,
			IsActive:           true,
		},
	}

	for _, site := range waterSites {
		var existing models.Site
		err := DB.Where("code = ?", site.Code).First(&existing).Error
		if err != nil {
			// Site doesn't exist, create it
			if err := DB.Create(&site).Error; err != nil {
				log.Printf("❌ Error creating site %s: %v", site.Name, err)
			} else {
				log.Printf("✅ Created site: %s (ID: %s)", site.Name, site.ID)
			}
		} else {
			log.Printf("ℹ️  Site already exists: %s (ID: %s)", existing.Name, existing.ID)
		}
	}
}

// seedSolarSites creates 12 default sites for Solar Works
func seedSolarSites(businessVerticalID uuid.UUID) {
	solarSites := []models.Site{
		{Name: "Solar Site 01", Code: "SOLAR_SITE_01", Description: "Solar panel array 01", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 02", Code: "SOLAR_SITE_02", Description: "Solar panel array 02", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 03", Code: "SOLAR_SITE_03", Description: "Solar panel array 03", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 04", Code: "SOLAR_SITE_04", Description: "Solar panel array 04", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 05", Code: "SOLAR_SITE_05", Description: "Solar panel array 05", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 06", Code: "SOLAR_SITE_06", Description: "Solar panel array 06", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 07", Code: "SOLAR_SITE_07", Description: "Solar panel array 07", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 08", Code: "SOLAR_SITE_08", Description: "Solar panel array 08", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 09", Code: "SOLAR_SITE_09", Description: "Solar panel array 09", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 10", Code: "SOLAR_SITE_10", Description: "Solar panel array 10", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 11", Code: "SOLAR_SITE_11", Description: "Solar panel array 11", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Solar Site 12", Code: "SOLAR_SITE_12", Description: "Solar panel array 12", BusinessVerticalID: businessVerticalID, IsActive: true},
	}

	for _, site := range solarSites {
		var existing models.Site
		err := DB.Where("code = ?", site.Code).First(&existing).Error
		if err != nil {
			// Site doesn't exist, create it
			if err := DB.Create(&site).Error; err != nil {
				log.Printf("❌ Error creating site %s: %v", site.Name, err)
			} else {
				log.Printf("✅ Created site: %s (ID: %s)", site.Name, site.ID)
			}
		} else {
			log.Printf("ℹ️  Site already exists: %s (ID: %s)", existing.Name, existing.ID)
		}
	}
}
