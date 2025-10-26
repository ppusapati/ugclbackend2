package config

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"p9e.in/ugcl/models"
)

// OldForm represents the old Form model structure for migration purposes
type OldForm struct {
	ID                 uuid.UUID `gorm:"type:uuid;primary_key"`
	Code               string    `gorm:"size:50"`
	Name               string    `gorm:"size:100"`
	Description        string    `gorm:"type:text"`
	Module             string    `gorm:"size:50"`
	Route              string    `gorm:"size:200"`
	Icon               string    `gorm:"size:50"`
	RequiredPermission string    `gorm:"size:100"`
	IsActive           bool
	DisplayOrder       int
	CreatedAt          time.Time
	UpdatedAt          time.Time
	BusinessVerticals  []models.BusinessVertical `gorm:"many2many:business_vertical_forms"`
}

// TableName specifies the table name for OldForm
func (OldForm) TableName() string {
	return "forms"
}

// MigrateFormToAppForm migrates data from the old Form model to the new AppForm model
func MigrateFormToAppForm() error {
	log.Println("🔄 Starting migration from Form to AppForm...")

	// Check if old forms table exists
	if !DB.Migrator().HasTable("forms") {
		log.Println("⚠️  Old 'forms' table does not exist, skipping migration")
		return nil
	}

	// Get all existing forms
	var oldForms []OldForm
	if err := DB.Preload("BusinessVerticals").Find(&oldForms).Error; err != nil {
		log.Printf("❌ Error fetching old forms: %v", err)
		return err
	}

	log.Printf("📋 Found %d forms to migrate", len(oldForms))

	// Get or create modules
	moduleMap := make(map[string]uuid.UUID)
	moduleNames := map[string]string{
		"project":  "Projects",
		"hr":       "Human Resources",
		"finance":  "Finance",
		"solar":    "Solar Energy",
		"water":    "Water Works",
		"admin":    "Administration",
	}

	displayOrder := 0
	for code, name := range moduleNames {
		var module models.Module
		err := DB.Where("code = ?", code).First(&module).Error
		if err != nil {
			// Create module if it doesn't exist
			module = models.Module{
				Code:         code,
				Name:         name,
				Description:  name + " module",
				Icon:         code,
				DisplayOrder: displayOrder,
				IsActive:     true,
			}
			if err := DB.Create(&module).Error; err != nil {
				log.Printf("❌ Error creating module %s: %v", code, err)
				continue
			}
			log.Printf("✅ Created module: %s", code)
		}
		moduleMap[code] = module.ID
		displayOrder++
	}

	// Migrate each form
	migratedCount := 0
	for _, oldForm := range oldForms {
		// Check if AppForm already exists with this code
		var existingAppForm models.AppForm
		err := DB.Where("code = ?", oldForm.Code).First(&existingAppForm).Error
		if err == nil {
			log.Printf("⚠️  AppForm with code '%s' already exists, skipping", oldForm.Code)
			continue
		}

		// Get module ID
		moduleID, ok := moduleMap[oldForm.Module]
		if !ok {
			log.Printf("⚠️  Unknown module '%s' for form '%s', skipping", oldForm.Module, oldForm.Code)
			continue
		}

		// Collect vertical codes from business verticals
		var verticalCodes []string
		for _, bv := range oldForm.BusinessVerticals {
			verticalCodes = append(verticalCodes, bv.Code)
		}

		// Create new AppForm
		appForm := models.AppForm{
			Code:                oldForm.Code,
			Title:               oldForm.Name,
			Description:         oldForm.Description,
			Version:             "1.0.0",
			ModuleID:            moduleID,
			Route:               oldForm.Route,
			Icon:                oldForm.Icon,
			DisplayOrder:        oldForm.DisplayOrder,
			RequiredPermission:  oldForm.RequiredPermission,
			AccessibleVerticals: verticalCodes,
			FormSchema:          json.RawMessage("{}"),
			Steps:               json.RawMessage("[]"),
			CoreFields:          json.RawMessage("[]"),
			Validations:         json.RawMessage("{}"),
			Dependencies:        json.RawMessage("[]"),
			InitialState:        "draft",
			IsActive:            oldForm.IsActive,
			CreatedBy:           "system_migration",
		}

		if err := DB.Create(&appForm).Error; err != nil {
			log.Printf("❌ Error migrating form '%s': %v", oldForm.Code, err)
			continue
		}

		log.Printf("✅ Migrated form: %s -> %v verticals", oldForm.Code, verticalCodes)
		migratedCount++
	}

	log.Printf("🎉 Migration completed: %d/%d forms migrated successfully", migratedCount, len(oldForms))
	return nil
}

// DropOldFormTables drops the old Form and BusinessVerticalForm tables
// WARNING: This will permanently delete the old tables and their data
func DropOldFormTables() error {
	log.Println("⚠️  Dropping old Form tables...")

	// Drop the many-to-many join table first
	if err := DB.Migrator().DropTable("business_vertical_forms"); err != nil {
		log.Printf("❌ Error dropping business_vertical_forms table: %v", err)
		return err
	}
	log.Println("✅ Dropped table: business_vertical_forms")

	// Drop the forms table
	if err := DB.Migrator().DropTable("forms"); err != nil {
		log.Printf("❌ Error dropping forms table: %v", err)
		return err
	}
	log.Println("✅ Dropped table: forms")

	log.Println("🎉 Old Form tables dropped successfully")
	return nil
}
