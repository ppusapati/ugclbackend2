package config

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

func Migrations(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "26062025_create_tables",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.User{}, &models.DairySite{}, &models.DprSite{}, &models.Contractor{},
					&models.Mnr{}, &models.Material{}, &models.Payment{}, &models.Diesel{}, &models.Eway{}, &models.Painting{},
					&models.Stock{}, &models.Water{}, &models.Wrapping{}, &models.Task{}, &models.Nmr_Vehicle{}, &models.VehicleLog{})
			},
		},
		{
			ID: "15102025_add_rbac_tables",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.Permission{}, &models.Role{}, &models.RolePermission{})
			},
		},
		{
			ID: "15102025_add_business_tables",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.BusinessVertical{}, &models.BusinessRole{},
					&models.UserBusinessRole{}, &models.BusinessRolePermission{})
			},
		},
		{
			ID: "20102025_add_business_vertical_id_to_operational_tables",
			Migrate: func(tx *gorm.DB) error {
				// Step 1: Add business_vertical_id column as NULLABLE first
				tables := []string{
					"waters", "dpr_sites", "materials", "payments",
					"wrappings", "eways", "dairy_sites", "diesels", "stocks",
				}

				for _, table := range tables {
					// Add column if it doesn't exist (nullable)
					if err := tx.Exec("ALTER TABLE " + table + " ADD COLUMN IF NOT EXISTS business_vertical_id uuid").Error; err != nil {
						return err
					}
				}

				// Step 2: Get or create a default business vertical
				var defaultBusiness models.BusinessVertical
				result := tx.Where("code = ?", "WATER").First(&defaultBusiness)

				if result.Error == gorm.ErrRecordNotFound {
					// Create default WATER business vertical if it doesn't exist
					defaultBusiness = models.BusinessVertical{
						ID:          uuid.New(),
						Name:        "Water Works",
						Code:        "WATER",
						Description: "Water supply and distribution business",
						IsActive:    true,
					}
					if err := tx.Create(&defaultBusiness).Error; err != nil {
						return err
					}
				} else if result.Error != nil {
					return result.Error
				}

				// Step 3: Update all existing records to use the default business vertical
				for _, table := range tables {
					if err := tx.Exec("UPDATE "+table+" SET business_vertical_id = ? WHERE business_vertical_id IS NULL", defaultBusiness.ID).Error; err != nil {
						return err
					}
				}

				// Step 4: Now make the column NOT NULL and add indexes
				for _, table := range tables {
					// Make column NOT NULL
					if err := tx.Exec("ALTER TABLE " + table + " ALTER COLUMN business_vertical_id SET NOT NULL").Error; err != nil {
						return err
					}

					// Create index
					indexName := "idx_" + table + "_business_vertical_id"
					if err := tx.Exec("CREATE INDEX IF NOT EXISTS " + indexName + " ON " + table + "(business_vertical_id)").Error; err != nil {
						return err
					}

					// Add foreign key constraint
					fkName := "fk_" + table + "_business_vertical"
					if err := tx.Exec("ALTER TABLE " + table + " ADD CONSTRAINT " + fkName + " FOREIGN KEY (business_vertical_id) REFERENCES business_verticals(id) ON DELETE RESTRICT").Error; err != nil {
						// Ignore error if constraint already exists
						continue
					}
				}

				return nil
			},
		},
		{
			ID: "25102025_add_module_tables",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.Module{})
			},
		},
	})
	// MigrateSites()
	// MigrateToNewRBAC()
	// MigrateExistingUsers()
	return m.Migrate()
}
