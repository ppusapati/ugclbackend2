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
					&models.WebPushSubscription{},
					&models.MobilePushToken{},
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
			ID: "20260425_notifications_web_push_subscriptions",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&models.WebPushSubscription{}); err != nil {
					return err
				}

				queries := []string{
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_web_push_subscriptions_endpoint ON web_push_subscriptions(endpoint)",
					"CREATE INDEX IF NOT EXISTS idx_web_push_subscriptions_user_id ON web_push_subscriptions(user_id)",
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
			ID: "20260425_notifications_mobile_push_tokens",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&models.MobilePushToken{}); err != nil {
					return err
				}

				queries := []string{
					"ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS enable_mobile_push BOOLEAN DEFAULT TRUE",
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_mobile_push_tokens_token ON mobile_push_tokens(token)",
					"CREATE INDEX IF NOT EXISTS idx_mobile_push_tokens_user_active ON mobile_push_tokens(user_id, is_active)",
					"CREATE INDEX IF NOT EXISTS idx_mobile_push_tokens_device_id ON mobile_push_tokens(device_id)",
					"UPDATE notification_preferences SET enable_mobile_push = TRUE WHERE enable_mobile_push IS NULL",
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
		{
			ID: "20260425_add_user_login_events",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&models.UserLoginEvent{}); err != nil {
					return err
				}

				queries := []string{
					"CREATE INDEX IF NOT EXISTS idx_user_login_events_login_at_desc ON user_login_events(login_at DESC)",
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
			ID: "20260426_create_third_party_integrations",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&models.ThirdPartyIntegration{}); err != nil {
					return err
				}

				queries := []string{
					"CREATE INDEX IF NOT EXISTS idx_third_party_integrations_status ON third_party_integrations(status)",
					"CREATE INDEX IF NOT EXISTS idx_third_party_integrations_created_at ON third_party_integrations(created_at DESC)",
					// Seed the manage_integrations permission so it can be assigned to admin roles.
					`INSERT INTO permissions (id, name, description, resource, action, created_at, updated_at)
					 VALUES (gen_random_uuid(), 'manage_integrations', 'Create, update and delete third-party integrations', 'integrations', 'manage', NOW(), NOW())
					 ON CONFLICT (name) DO NOTHING`,
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
			ID: "20260426_extend_third_party_integrations_for_document_ai",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&models.ThirdPartyIntegration{}); err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID: "20260426_project_phase1_execution_controls",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(
					&models.WBSNode{},
					&models.TaskDependency{},
					&models.BOQItem{},
					&models.MBEntry{},
					&models.RABill{},
					&models.RABillLine{},
				); err != nil {
					return err
				}

				queries := []string{
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_wbs_nodes_project_code ON wbs_nodes(project_id, code)",
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_task_dependencies_unique_pair ON task_dependencies(project_id, predecessor_task_id, successor_task_id)",
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_boq_items_project_code ON boq_items(project_id, code)",
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_mb_entries_project_entry_number ON mb_entries(project_id, entry_number)",
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_ra_bills_project_bill_number ON ra_bills(project_id, bill_number)",
					"CREATE INDEX IF NOT EXISTS idx_ra_bill_lines_bill_id ON ra_bill_lines(ra_bill_id)",
					"ALTER TABLE wbs_nodes ADD CONSTRAINT chk_wbs_node_type_phase1 CHECK (node_type IN ('package', 'activity', 'milestone'))",
					"ALTER TABLE task_dependencies ADD CONSTRAINT chk_task_dep_type_phase1 CHECK (dependency_type IN ('FS','SS','FF','SF'))",
					"ALTER TABLE boq_items ADD CONSTRAINT chk_boq_status_phase1 CHECK (status IN ('planned', 'in-progress', 'completed', 'cancelled'))",
					"ALTER TABLE ra_bills ADD CONSTRAINT chk_ra_bill_status_phase1 CHECK (status IN ('draft', 'submitted', 'approved', 'rejected', 'paid'))",
				}

				for _, q := range queries {
					_ = tx.Exec(q).Error
				}

				type permissionSeed struct {
					Name        string
					Description string
					Resource    string
					Action      string
				}

				permissionSeeds := []permissionSeed{
					{Name: "project:wbs_read", Description: "View project WBS nodes", Resource: "project", Action: "wbs_read"},
					{Name: "project:wbs_manage", Description: "Create and update project WBS nodes", Resource: "project", Action: "wbs_manage"},
					{Name: "task:dependency_read", Description: "View task dependencies", Resource: "task", Action: "dependency_read"},
					{Name: "task:dependency_manage", Description: "Manage task dependencies", Resource: "task", Action: "dependency_manage"},
					{Name: "project:boq_read", Description: "View bill of quantities", Resource: "project", Action: "boq_read"},
					{Name: "project:boq_manage", Description: "Manage bill of quantities", Resource: "project", Action: "boq_manage"},
					{Name: "project:mb_read", Description: "View measurement book entries", Resource: "project", Action: "mb_read"},
					{Name: "project:mb_manage", Description: "Manage measurement book entries", Resource: "project", Action: "mb_manage"},
					{Name: "project:billing_read", Description: "View RA bills", Resource: "project", Action: "billing_read"},
					{Name: "project:billing_manage", Description: "Create and edit RA bills", Resource: "project", Action: "billing_manage"},
					{Name: "project:billing_submit", Description: "Submit RA bills for approval", Resource: "project", Action: "billing_submit"},
					{Name: "project:billing_approve", Description: "Approve or reject RA bills", Resource: "project", Action: "billing_approve"},
					{Name: "project:billing_pay", Description: "Mark approved RA bills as paid", Resource: "project", Action: "billing_pay"},
				}

				for _, seed := range permissionSeeds {
					if err := tx.Exec(
						"INSERT INTO permissions (id, name, description, resource, action, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NOW(), NOW()) ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description, resource = EXCLUDED.resource, action = EXCLUDED.action, updated_at = NOW()",
						uuid.New(), seed.Name, seed.Description, seed.Resource, seed.Action,
					).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		{
			ID: "20260427_document_context_columns",
			Migrate: func(tx *gorm.DB) error {
				// Ensure first-class project/task context columns exist for legacy DBs
				// that were initialized before these fields were introduced.
				return tx.AutoMigrate(&models.Document{})
			},
		},
		{
			ID: "20260428_form_submission_location_metadata",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					"ALTER TABLE form_submissions ADD COLUMN IF NOT EXISTS latitude NUMERIC(10,8)",
					"ALTER TABLE form_submissions ADD COLUMN IF NOT EXISTS longitude NUMERIC(11,8)",
					"CREATE INDEX IF NOT EXISTS idx_form_submissions_latitude ON form_submissions(latitude)",
					"CREATE INDEX IF NOT EXISTS idx_form_submissions_longitude ON form_submissions(longitude)",
				}

				for _, q := range queries {
					if err := tx.Exec(q).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
		// ─────────────────────────────────────────────────────────────────────
		// Phase 4 – Hot-path composite & partial performance indexes
		//
		// Rationale per table:
		//
		// notifications:
		//   - Primary inbox query: WHERE user_id = ? ORDER BY created_at DESC
		//   - Status-filtered inbox: WHERE user_id = ? AND status = ? ORDER BY created_at DESC
		//   Both resolve with a single index scan instead of a heap-sort pass.
		//
		// form_submissions:
		//   - Listing page: WHERE form_code = ? AND deleted_at IS NULL ORDER BY submitted_at DESC
		//   - State-filtered: WHERE form_code = ? AND current_state = ? … ORDER BY submitted_at DESC
		//   - User's own: WHERE submitted_by = ? … ORDER BY submitted_at DESC
		//   - Business scope: WHERE business_vertical_id = ? … ORDER BY submitted_at DESC
		//   Partial (WHERE deleted_at IS NULL) keeps index smaller and enables index-only scans.
		//
		// documents:
		//   - Listing: WHERE business_vertical_id = ? AND deleted_at IS NULL ORDER BY created_at DESC
		//   - Category filter: WHERE category_id = ? AND deleted_at IS NULL ORDER BY created_at DESC
		//   - Dedup check on upload: WHERE file_hash = ? AND deleted_at IS NULL (First())
		//   category_id has no single-column index; business_vertical_id has no index on this table.
		//
		// workflow_transitions:
		//   - History lookup: WHERE submission_id = ? ORDER BY transitioned_at ASC
		//   Existing single-column indexes on submission_id and transitioned_at force a sort step.
		//
		// report_definitions:
		//   - Listing: WHERE business_vertical_id = ? AND deleted_at IS NULL ORDER BY created_at DESC
		//   Existing single-column index cannot satisfy the ORDER BY without a sort.
		//
		// report_executions:
		//   - History: WHERE report_id = ? ORDER BY started_at DESC
		//   Existing single-column index on report_id requires an extra sort pass.
		//
		// dashboards:
		//   - Listing: WHERE business_vertical_id = ? AND is_active = ? AND deleted_at IS NULL
		//   Partial index drops soft-deleted rows from the index entirely.
		//
		// document_audit_logs:
		//   - Audit listing: WHERE document_id = ? ORDER BY created_at DESC
		//   Existing single-column index on document_id has no ordering guarantee.
		//
		// attendance_sessions:
		//   - GetAttendanceLogs: WHERE business_vertical_id = ? AND status = ? ORDER BY check_in_at DESC
		//   - GetActiveAttendanceSessions: same filter, ORDER BY last_seen_at DESC
		//   - Employee timeline: WHERE user_id = ? ORDER BY check_in_at DESC
		//   Only site-scoped index existed; business-vertical scoped queries were doing full scans.
		// ─────────────────────────────────────────────────────────────────────
		{
			ID: "20260429_perf_indexes_hotpath",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					// ── notifications ──────────────────────────────────────────────────
					// Base inbox scan (no status filter)
					"CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC)",
					// Status-filtered inbox (read / unread / dismissed)
					"CREATE INDEX IF NOT EXISTS idx_notifications_user_status_created ON notifications(user_id, status, created_at DESC)",

					// ── form_submissions ───────────────────────────────────────────────
					// Listing page – form + ordering; partial drops soft-deleted rows
					"CREATE INDEX IF NOT EXISTS idx_form_submissions_code_submitted ON form_submissions(form_code, submitted_at DESC) WHERE deleted_at IS NULL",
					// Workflow state filter on top of form listing
					"CREATE INDEX IF NOT EXISTS idx_form_submissions_code_state_submitted ON form_submissions(form_code, current_state, submitted_at DESC) WHERE deleted_at IS NULL",
					// User's own submissions
					"CREATE INDEX IF NOT EXISTS idx_form_submissions_submitter_submitted ON form_submissions(submitted_by, submitted_at DESC) WHERE deleted_at IS NULL",
					// Business-vertical scoped listing (generic admin views)
					"CREATE INDEX IF NOT EXISTS idx_form_submissions_bv_submitted ON form_submissions(business_vertical_id, submitted_at DESC) WHERE deleted_at IS NULL",

					// ── documents ──────────────────────────────────────────────────────
					// Primary listing – business_vertical_id has NO existing index on documents
					"CREATE INDEX IF NOT EXISTS idx_documents_bv_created ON documents(business_vertical_id, created_at DESC) WHERE deleted_at IS NULL",
					// Category filter; category_id has NO existing index on documents
					"CREATE INDEX IF NOT EXISTS idx_documents_category_created ON documents(category_id, created_at DESC) WHERE deleted_at IS NULL",
					// Dedup check during upload: WHERE file_hash = ? AND deleted_at IS NULL
					"CREATE INDEX IF NOT EXISTS idx_documents_file_hash ON documents(file_hash) WHERE deleted_at IS NULL",

					// ── workflow_transitions ────────────────────────────────────────────
					// Covers GetSubmissionWorkflowHistory ORDER BY transitioned_at ASC
					"CREATE INDEX IF NOT EXISTS idx_workflow_transitions_submission_time ON workflow_transitions(submission_id, transitioned_at ASC)",

					// ── report_definitions ─────────────────────────────────────────────
					// Listing ordered by recency within a business vertical
					"CREATE INDEX IF NOT EXISTS idx_report_defs_bv_created ON report_definitions(business_vertical_id, created_at DESC) WHERE deleted_at IS NULL",

					// ── report_executions ──────────────────────────────────────────────
					// Execution history ordered newest-first per report
					"CREATE INDEX IF NOT EXISTS idx_report_executions_report_started ON report_executions(report_id, started_at DESC)",

					// ── dashboards ─────────────────────────────────────────────────────
					// Listing active dashboards per business vertical
					"CREATE INDEX IF NOT EXISTS idx_dashboards_bv_active ON dashboards(business_vertical_id, is_active) WHERE deleted_at IS NULL",

					// ── document_audit_logs ────────────────────────────────────────────
					// Audit history per document ordered newest-first
					"CREATE INDEX IF NOT EXISTS idx_document_audit_logs_doc_created ON document_audit_logs(document_id, created_at DESC)",

					// ── attendance_sessions ────────────────────────────────────────────
					// GetAttendanceLogs: business vertical + status + time range ordering
					"CREATE INDEX IF NOT EXISTS idx_attendance_sessions_bv_status_checkin ON attendance_sessions(business_vertical_id, status, check_in_at DESC) WHERE deleted_at IS NULL",
					// GetActiveAttendanceSessions: business vertical + active status + recency
					"CREATE INDEX IF NOT EXISTS idx_attendance_sessions_bv_status_lastseen ON attendance_sessions(business_vertical_id, status, last_seen_at DESC) WHERE deleted_at IS NULL",
					// Employee timeline: user's own sessions sorted by check-in time
					"CREATE INDEX IF NOT EXISTS idx_attendance_sessions_user_checkin ON attendance_sessions(user_id, check_in_at DESC) WHERE deleted_at IS NULL",
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
			ID: "20260626_perf_indexes_latency_throughput",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					// Business users listing join path:
					// user_business_roles -> business_roles filtered by business_vertical_id + is_active,
					// then grouped/plucked by user_id.
					"CREATE INDEX IF NOT EXISTS idx_business_roles_vertical_active_id ON business_roles(business_vertical_id, is_active, id)",
					"CREATE INDEX IF NOT EXISTS idx_ubr_role_active_user ON user_business_roles(business_role_id, is_active, user_id)",

					// Attendance APIs with business scope + optional site/user filters + recency order.
					"CREATE INDEX IF NOT EXISTS idx_attendance_sessions_bv_site_status_lastseen ON attendance_sessions(business_vertical_id, site_id, status, last_seen_at DESC) WHERE deleted_at IS NULL",
					"CREATE INDEX IF NOT EXISTS idx_attendance_sessions_bv_user_status_lastseen ON attendance_sessions(business_vertical_id, user_id, status, last_seen_at DESC) WHERE deleted_at IS NULL",

					// Recent login history endpoint: WHERE user_id = ? ORDER BY login_at DESC LIMIT n.
					"CREATE INDEX IF NOT EXISTS idx_user_login_events_user_login_at_desc ON user_login_events(user_id, login_at DESC)",
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
			ID: "20260626_perf_indexes_jsonb_form_rendering",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					// Accelerate app form vertical/role JSONB membership checks.
					"CREATE INDEX IF NOT EXISTS idx_app_forms_accessible_verticals_gin ON app_forms USING GIN (accessible_verticals)",
					"CREATE INDEX IF NOT EXISTS idx_app_forms_allowed_roles_gin ON app_forms USING GIN (allowed_roles)",

					// Accelerate active form list ordering path.
					"CREATE INDEX IF NOT EXISTS idx_app_forms_active_display_title ON app_forms(is_active, display_order ASC, title ASC)",

					// Keep module-level JSONB access checks fast if used by menu rendering.
					"CREATE INDEX IF NOT EXISTS idx_modules_accessible_verticals_gin ON modules USING GIN (accessible_verticals)",
				}

				for _, q := range queries {
					if err := tx.Exec(q).Error; err != nil {
						return err
					}
				}

				return nil
			},
		},
	})

	return m.Migrate()
}
