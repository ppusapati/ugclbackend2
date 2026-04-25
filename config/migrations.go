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
		{
			ID: "20260415_add_attendance_tracking",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(
					&models.AttendanceSession{},
					&models.AttendanceEvent{},
					&models.TrackingPing{},
				); err != nil {
					return err
				}

				indexes := []string{
					"CREATE UNIQUE INDEX IF NOT EXISTS uq_attendance_sessions_active_user ON attendance_sessions(user_id) WHERE deleted_at IS NULL AND status = 'active'",
					"CREATE INDEX IF NOT EXISTS idx_attendance_sessions_site_last_seen ON attendance_sessions(site_id, last_seen_at DESC) WHERE deleted_at IS NULL",
					"CREATE INDEX IF NOT EXISTS idx_attendance_events_site_type_time ON attendance_events(site_id, event_type, event_time DESC) WHERE deleted_at IS NULL",
					"CREATE INDEX IF NOT EXISTS idx_tracking_pings_site_inside_time ON tracking_pings(site_id, inside_geofence, ping_time DESC) WHERE deleted_at IS NULL",
				}

				for _, idx := range indexes {
					if err := tx.Exec(idx).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "20260414_webhook_and_active_business_context",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(
					&models.Webhook{},
					&models.WebhookDelivery{},
					&models.WebhookLog{},
					&models.UserActiveBusinessContext{},
				); err != nil {
					return err
				}

				indexes := []string{
					"CREATE INDEX IF NOT EXISTS idx_webhooks_business_id_is_active ON webhooks(business_id, is_active)",
					"CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id_status ON webhook_deliveries(webhook_id, status)",
					"CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status_next_retry ON webhook_deliveries(status, next_retry_at) WHERE status = 'RETRY_SCHEDULED'",
					"CREATE INDEX IF NOT EXISTS idx_webhook_logs_webhook_id ON webhook_logs(webhook_id)",
					"CREATE INDEX IF NOT EXISTS idx_webhook_logs_delivery_id ON webhook_logs(delivery_id)",
					"CREATE INDEX IF NOT EXISTS idx_user_active_business_user_client ON user_active_business_contexts(user_id, client_key)",
				}

				for _, idx := range indexes {
					if err := tx.Exec(idx).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "20260421_perf_indexes_auth_policy",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					"CREATE INDEX IF NOT EXISTS idx_users_is_active ON users (is_active)",
					"CREATE INDEX IF NOT EXISTS idx_users_role_id ON users (role_id)",
					"CREATE INDEX IF NOT EXISTS idx_users_business_vertical_id ON users (business_vertical_id)",
					"CREATE INDEX IF NOT EXISTS idx_business_verticals_is_active ON business_verticals (is_active)",
					"CREATE INDEX IF NOT EXISTS idx_ubr_user_active ON user_business_roles (user_id, is_active)",
					"CREATE INDEX IF NOT EXISTS idx_ubr_role_active ON user_business_roles (business_role_id, is_active)",
					"CREATE INDEX IF NOT EXISTS idx_policies_status ON policies (status)",
					"CREATE INDEX IF NOT EXISTS idx_policies_effect ON policies (effect)",
					"CREATE INDEX IF NOT EXISTS idx_policies_status_priority_created ON policies (status, priority DESC, created_at DESC)",
				}

				for _, q := range queries {
					if err := tx.Exec(q).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "20260421_perf_indexes_auth_policy_v2",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					"CREATE INDEX IF NOT EXISTS idx_users_is_active_updated_at ON users (is_active, updated_at DESC)",
				}

				for _, q := range queries {
					if err := tx.Exec(q).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "20260422_perf_indexes_admin_boot",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					"CREATE INDEX IF NOT EXISTS idx_roles_is_active_level ON roles (is_active, level ASC)",
					"CREATE INDEX IF NOT EXISTS idx_business_roles_is_active_level ON business_roles (is_active, level ASC)",
					"CREATE INDEX IF NOT EXISTS idx_app_forms_module_display_order ON app_forms (module_id, display_order ASC)",
				}

				for _, q := range queries {
					if err := tx.Exec(q).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},

		{
			ID: "20260425_notifications_add_conversation_message_id",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					"ALTER TABLE notifications ADD COLUMN IF NOT EXISTS conversation_id UUID",
					"ALTER TABLE notifications ADD COLUMN IF NOT EXISTS message_id UUID",
					"CREATE INDEX IF NOT EXISTS idx_notifications_conversation_id ON notifications(conversation_id)",
					"CREATE INDEX IF NOT EXISTS idx_notifications_message_id ON notifications(message_id)",
				}
				for _, q := range queries {
					if err := tx.Exec(q).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			ID: "20260425_backfill_attendance_permissions",
			Migrate: func(tx *gorm.DB) error {
				type permissionSeed struct {
					Name        string
					Description string
					Resource    string
					Action      string
				}

				permissionSeeds := []permissionSeed{
					{Name: "attendance:checkin", Description: "Check in to a site attendance session", Resource: "attendance", Action: "checkin"},
					{Name: "attendance:heartbeat", Description: "Send attendance heartbeat updates", Resource: "attendance", Action: "heartbeat"},
					{Name: "attendance:checkout", Description: "Check out from a site attendance session", Resource: "attendance", Action: "checkout"},
					{Name: "attendance:read", Description: "View attendance sessions, logs, and timelines", Resource: "attendance", Action: "read"},
					{Name: "attendance:headcount", Description: "View live attendance headcount by site", Resource: "attendance", Action: "headcount"},
				}

				permissionIDs := make(map[string]uuid.UUID, len(permissionSeeds))
				for _, seed := range permissionSeeds {
					// Repair any legacy rows that were inserted with empty name for the same resource/action.
					if err := tx.Exec(
						"UPDATE permissions SET name = ?, description = ?, updated_at = NOW() WHERE (name = '' OR name IS NULL) AND resource = ? AND action = ?",
						seed.Name, seed.Description, seed.Resource, seed.Action,
					).Error; err != nil {
						return err
					}

					// Upsert by permission name to keep migration idempotent across environments.
					if err := tx.Exec(
						"INSERT INTO permissions (id, name, description, resource, action, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NOW(), NOW()) ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description, resource = EXCLUDED.resource, action = EXCLUDED.action, updated_at = NOW()",
						uuid.New(), seed.Name, seed.Description, seed.Resource, seed.Action,
					).Error; err != nil {
						return err
					}

					perm := models.Permission{}
					if err := tx.Select("id").Where("name = ?", seed.Name).First(&perm).Error; err != nil {
						return err
					}
					permissionIDs[seed.Name] = perm.ID
				}

				rolePermissions := map[string][]string{
					"Water_Admin":          {"attendance:checkin", "attendance:heartbeat", "attendance:checkout", "attendance:read", "attendance:headcount"},
					"Project_Coordinator":  {"attendance:read", "attendance:headcount"},
					"Engineer":             {"attendance:checkin", "attendance:heartbeat", "attendance:checkout"},
					"Supervisor":           {"attendance:checkin", "attendance:heartbeat", "attendance:checkout", "attendance:read", "attendance:headcount"},
					"Operator":             {"attendance:checkin", "attendance:heartbeat", "attendance:checkout"},
					"Solar_Admin":          {"attendance:checkin", "attendance:heartbeat", "attendance:checkout", "attendance:read", "attendance:headcount"},
					"Area_Project_Manager": {"attendance:read", "attendance:headcount"},
					"Sr_Engineer":          {"attendance:checkin", "attendance:heartbeat", "attendance:checkout", "attendance:read", "attendance:headcount"},
					"HO_Manager":           {"attendance:read", "attendance:headcount"},
					"HO_HR":                {"attendance:read", "attendance:headcount"},
				}

				for roleName, permNames := range rolePermissions {
					var roles []models.BusinessRole
					if err := tx.Where("name = ? AND is_active = ?", roleName, true).Find(&roles).Error; err != nil {
						return err
					}

					for _, role := range roles {
						for _, permName := range permNames {
							permID, ok := permissionIDs[permName]
							if !ok {
								continue
							}

							if err := tx.Exec(
								"INSERT INTO business_role_permissions (business_role_id, permission_id, created_at) SELECT ?, ?, NOW() WHERE NOT EXISTS (SELECT 1 FROM business_role_permissions WHERE business_role_id = ? AND permission_id = ?)",
								role.ID, permID, role.ID, permID,
							).Error; err != nil {
								return err
							}
						}
					}
				}

				return nil
			},
		},
	})

	return m.Migrate()
}
