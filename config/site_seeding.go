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
			Name:               "Ramanagara",
			Code:               "RAMANAGARA",
			Description:        "Ramanagara water distribution site",
			BusinessVerticalID: businessVerticalID,
			IsActive:           true,
		},
		{
			Name:               "Magadi",
			Code:               "MAGADI",
			Description:        "Magadi water distribution site",
			BusinessVerticalID: businessVerticalID,
			IsActive:           true,
		},
		{
			Name:               "VG Doddi",
			Code:               "VG_DODDI",
			Description:        "VG Doddi water distribution site",
			BusinessVerticalID: businessVerticalID,
			IsActive:           true,
		},
		{
			Name:               "Mallipatna",
			Code:               "MALLIPATNA",
			Description:        "Mallipatna water distribution site",
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
		{Name: "Handigund", Code: "HANDIGUND", Description: "Solar farm Handigund site", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Itnal", Code: "ITNAL", Description: "Solar farm Itnal site", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Malabad", Code: "MALABAD", Description: "Solar farm Malabad site", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Nagarmunavali", Code: "NAGARMUNAVALI", Description: "Solar farm Nagarmunavali site", BusinessVerticalID: businessVerticalID, IsActive: true},
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
