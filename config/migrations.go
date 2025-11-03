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
		{
			ID: "29102025_add_abac_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create ABAC tables: attributes, user_attributes, resource_attributes
				return tx.AutoMigrate(
					&models.Attribute{},
					&models.UserAttribute{},
					&models.ResourceAttribute{},
				)
			},
		},
		{
			ID: "29102025_add_policy_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create Policy-based authorization tables
				return tx.AutoMigrate(
					&models.Policy{},
					&models.PolicyRule{},
					&models.PolicyEvaluation{},
				)
			},
		},
		{
			ID: "29102025_add_policy_approval_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create Policy approval workflow tables
				return tx.AutoMigrate(
					&models.PolicyVersion{},
					&models.PolicyApprovalRequest{},
					&models.PolicyApproval{},
					&models.PolicyChangeLog{},
					&models.PolicyApprovalWorkflow{},
				)
			},
		},
		{
			ID: "02112025_add_workflow_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create workflow tables
				// Note: WorkflowAction, WorkflowState, and WorkflowTransitionDef are NOT database tables
				// They are helper structs stored as JSONB within workflow_definitions
				return tx.AutoMigrate(
					&models.WorkflowDefinition{}, // Table: workflow_definitions
					&models.FormSubmission{},     // Table: form_submissions
					&models.WorkflowTransition{}, // Table: workflow_transitions
				)
			},
		},
		{
			ID: "02112025_add_notification_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create notification system tables
				return tx.AutoMigrate(
					&models.NotificationRule{},       // Table: notification_rules
					&models.NotificationRecipient{},  // Table: notification_recipients
					&models.Notification{},           // Table: notifications
					&models.NotificationPreference{}, // Table: notification_preferences
				)
			},
		},
		{
			ID: "02112025_add_document_management_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create document management system tables
				return tx.AutoMigrate(
					&models.DocumentCategory{},        // Table: document_categories
					&models.DocumentTag{},             // Table: document_tags
					&models.Document{},                // Table: documents
					&models.DocumentVersion{},         // Table: document_versions
					&models.DocumentPermission{},      // Table: document_permissions
					&models.DocumentShare{},           // Table: document_shares
					&models.DocumentAuditLog{},        // Table: document_audit_logs
					&models.DocumentRetentionPolicy{}, // Table: document_retention_policies
				)
			},
		},
		{
			ID: "03112025_add_app_form_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create app form and module tables
				return tx.AutoMigrate(
					&models.AppForm{}, // Table: app_forms
				)
			},
		},
		{
			ID: "03112025_add_report_builder_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create report builder and analytics tables
				// IMPORTANT: Order matters for FKs. Create parent tables before children.
				// Dashboards must exist before report_widgets (FK: report_widgets.dashboard_id -> dashboards.id)
				// ReportDefinitions must exist before report_widgets (FK: report_widgets.report_id -> report_definitions.id)
				if err := tx.AutoMigrate(
					&models.ReportDefinition{}, // Table: report_definitions
					&models.ReportExecution{},  // Table: report_executions
					&models.Dashboard{},        // Table: dashboards (parent)
				); err != nil {
					return err
				}

				return tx.AutoMigrate(
					&models.ReportWidget{},   // Table: report_widgets (child depends on dashboards, report_definitions)
					&models.ReportTemplate{}, // Table: report_templates
					&models.ReportShare{},    // Table: report_shares
				)
			},
		},
		{
			ID: "03112025_enable_postgis_extension",
			Migrate: func(tx *gorm.DB) error {
				// Enable PostGIS extension for geometry types
				return tx.Exec("CREATE EXTENSION IF NOT EXISTS postgis").Error
			},
		},
		{
			ID: "03112025_add_project_management_tables",
			Migrate: func(tx *gorm.DB) error {
				// Create project management tables
				// Order: Projects -> Zones -> Nodes -> Tasks -> related tables
				if err := tx.AutoMigrate(
					&models.Project{},          // Table: projects (parent)
					&models.Zone{},             // Table: zones (depends on projects)
					&models.Node{},             // Table: nodes (depends on projects, zones)
					&models.Tasks{},            // Table: tasks (depends on projects, zones, nodes)
					&models.BudgetAllocation{}, // Table: budget_allocations (depends on projects, tasks)
				); err != nil {
					return err
				}

				// Create child tables that depend on tasks
				if err := tx.AutoMigrate(
					&models.TaskAssignment{}, // Table: task_assignments
					&models.TaskAuditLog{},   // Table: task_audit_logs
					&models.TaskComment{},    // Table: task_comments
					&models.TaskAttachment{}, // Table: task_attachments
				); err != nil {
					return err
				}

				// Create project role tables
				return tx.AutoMigrate(
					&models.ProjectRole{},     // Table: project_roles
					&models.UserProjectRole{}, // Table: user_project_roles
				)
			},
		},
	})
	// Seed permissions and roles
	// SeedPermissions()

	// Seed business verticals
	// SeedBusinessVerticals()

	// Seed sites
	// SeedSites()
	// MigrateSites()
	// MigrateToNewRBAC()
	// MigrateExistingUsers()
	return m.Migrate()
}
