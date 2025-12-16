package config

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// Migrations runs all database migrations in a single consolidated migration
func Migrations(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "20251212_consolidated_schema",
			Migrate: func(tx *gorm.DB) error {
				// =====================================================
				// Step 1: Enable Extensions
				// =====================================================
				if err := tx.Exec("CREATE EXTENSION IF NOT EXISTS postgis").Error; err != nil {
					return err
				}

				// =====================================================
				// Step 2: Core Tables (Users, Permissions, Roles)
				// =====================================================
				if err := tx.AutoMigrate(
					&models.User{},
					&models.Permission{},
					&models.Role{},
					&models.RolePermission{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 3: Business Vertical Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.BusinessVertical{},
					&models.BusinessRole{},
					&models.UserBusinessRole{},
					&models.BusinessRolePermission{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 4: Site Management Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.Site{},
					&models.UserSiteAccess{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 5: Module & App Form Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.Module{},
					&models.AppForm{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 6: ABAC Tables (Attributes & Policies)
				// =====================================================
				if err := tx.AutoMigrate(
					&models.Attribute{},
					&models.UserAttribute{},
					&models.ResourceAttribute{},
					&models.Policy{},
					&models.PolicyRule{},
					&models.PolicyEvaluation{},
					&models.PolicyVersion{},
					&models.PolicyApprovalRequest{},
					&models.PolicyApproval{},
					&models.PolicyChangeLog{},
					&models.PolicyApprovalWorkflow{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 7: Workflow & Form Submission Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.WorkflowDefinition{},
					&models.FormSubmission{},
					&models.WorkflowTransition{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 8: Notification System Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.NotificationRule{},
					&models.NotificationRecipient{},
					&models.Notification{},
					&models.NotificationPreference{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 9: Document Management Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.DocumentCategory{},
					&models.DocumentTag{},
					&models.Document{},
					&models.DocumentVersion{},
					&models.DocumentPermission{},
					&models.DocumentShare{},
					&models.DocumentAuditLog{},
					&models.DocumentRetentionPolicy{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 10: Report Builder & Analytics Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.ReportDefinition{},
					&models.ReportExecution{},
					&models.Dashboard{},
				); err != nil {
					return err
				}

				if err := tx.AutoMigrate(
					&models.ReportWidget{},
					&models.ReportTemplate{},
					&models.ReportShare{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 11: Project Management Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.Project{},
					&models.Zone{},
					&models.Node{},
					&models.Tasks{},
					&models.BudgetAllocation{},
				); err != nil {
					return err
				}

				if err := tx.AutoMigrate(
					&models.TaskAssignment{},
					&models.TaskAuditLog{},
					&models.TaskComment{},
					&models.TaskAttachment{},
				); err != nil {
					return err
				}

				if err := tx.AutoMigrate(
					&models.ProjectRole{},
					&models.UserProjectRole{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 12: Chat System Tables
				// =====================================================
				// Create Conversation first (no circular FK now)
				if err := tx.AutoMigrate(&models.Conversation{}); err != nil {
					return err
				}

				// Then create dependent tables
				if err := tx.AutoMigrate(
					&models.ChatMessage{},
					&models.ChatParticipant{},
					&models.ChatTypingIndicator{},
				); err != nil {
					return err
				}

				if err := tx.AutoMigrate(
					&models.ChatAttachment{},
					&models.ChatReaction{},
					&models.ChatReadReceipt{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 13: Operational/Legacy Tables
				// =====================================================
				if err := tx.AutoMigrate(
					&models.DairySite{},
					&models.DprSite{},
					&models.Contractor{},
					&models.Mnr{},
					&models.Material{},
					&models.Payment{},
					&models.Diesel{},
					&models.Eway{},
					&models.Painting{},
					&models.Stock{},
					&models.Water{},
					&models.Wrapping{},
					&models.Task{},
					&models.Nmr_Vehicle{},
					&models.VehicleLog{},
				); err != nil {
					return err
				}

				// =====================================================
				// Step 14: Add business_vertical_id to operational tables
				// =====================================================
				tables := []string{
					"waters", "dpr_sites", "materials", "payments",
					"wrappings", "eways", "dairy_sites", "diesels", "stocks",
				}

				for _, table := range tables {
					if err := tx.Exec("ALTER TABLE " + table + " ADD COLUMN IF NOT EXISTS business_vertical_id uuid").Error; err != nil {
						return err
					}
				}

				// Get or create default WATER business vertical
				var defaultBusiness models.BusinessVertical
				result := tx.Where("code = ?", "WATER").First(&defaultBusiness)

				if result.Error == gorm.ErrRecordNotFound {
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

				// Update existing records with default business vertical
				for _, table := range tables {
					if err := tx.Exec("UPDATE "+table+" SET business_vertical_id = ? WHERE business_vertical_id IS NULL", defaultBusiness.ID).Error; err != nil {
						return err
					}
				}

				// Add constraints and indexes
				for _, table := range tables {
					// Make column NOT NULL (may fail if column already NOT NULL)
					_ = tx.Exec("ALTER TABLE " + table + " ALTER COLUMN business_vertical_id SET NOT NULL").Error

					// Create index
					indexName := "idx_" + table + "_business_vertical_id"
					if err := tx.Exec("CREATE INDEX IF NOT EXISTS " + indexName + " ON " + table + "(business_vertical_id)").Error; err != nil {
						return err
					}

					// Add foreign key constraint (ignore if exists)
					fkName := "fk_" + table + "_business_vertical"
					_ = tx.Exec("ALTER TABLE " + table + " ADD CONSTRAINT " + fkName + " FOREIGN KEY (business_vertical_id) REFERENCES business_verticals(id) ON DELETE RESTRICT").Error
				}

				// =====================================================
				// Step 15: Create Performance Indexes
				// =====================================================
				indexes := []string{
					"CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation_created ON chat_messages(conversation_id, created_at DESC)",
					"CREATE INDEX IF NOT EXISTS idx_chat_participants_user_joined ON chat_participants(user_id, joined_at DESC)",
					"CREATE INDEX IF NOT EXISTS idx_chat_reactions_emoji ON chat_reactions(reaction)",
				}

				for _, idx := range indexes {
					if err := tx.Exec(idx).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		// Migration to add schema_name column to modules table
		{
			ID: "20251214_add_module_schema_name",
			Migrate: func(tx *gorm.DB) error {
				// Add schema_name column to modules table
				if err := tx.Exec("ALTER TABLE modules ADD COLUMN IF NOT EXISTS schema_name VARCHAR(63)").Error; err != nil {
					return err
				}
				return nil
			},
		},
		// Migration to add accessible_verticals and required_permission columns to modules table
		{
			ID: "20251214_add_module_access_control",
			Migrate: func(tx *gorm.DB) error {
				// Add accessible_verticals column to modules table
				if err := tx.Exec("ALTER TABLE modules ADD COLUMN IF NOT EXISTS accessible_verticals JSONB DEFAULT '[]'").Error; err != nil {
					return err
				}
				// Add required_permission column to modules table
				if err := tx.Exec("ALTER TABLE modules ADD COLUMN IF NOT EXISTS required_permission VARCHAR(100)").Error; err != nil {
					return err
				}
				return nil
			},
		},
	})

	return m.Migrate()
}
