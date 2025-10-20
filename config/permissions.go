package config

import (
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// SeedPermissions creates default permissions and roles
func SeedPermissions() {
	// Define default permissions
	permissions := []models.Permission{
		// ===== Project Management =====
		{ID: uuid.New(), Name: "project:create", Resource: "project", Action: "create", Description: "Create project"},
		{ID: uuid.New(), Name: "project:read", Resource: "project", Action: "read", Description: "View project details"},
		{ID: uuid.New(), Name: "project:update", Resource: "project", Action: "update", Description: "Edit project"},
		{ID: uuid.New(), Name: "project:delete", Resource: "project", Action: "delete", Description: "Delete project"},
		{ID: uuid.New(), Name: "project:approve", Resource: "project", Action: "approve", Description: "Approve project"},
		{ID: uuid.New(), Name: "project:assign", Resource: "project", Action: "assign", Description: "Assign users to project"},

		// ===== Planning =====
		{ID: uuid.New(), Name: "planning:create", Resource: "planning", Action: "create", Description: "Create plans"},
		{ID: uuid.New(), Name: "planning:read", Resource: "planning", Action: "read", Description: "View plans"},
		{ID: uuid.New(), Name: "planning:update", Resource: "planning", Action: "update", Description: "Update plans"},
		{ID: uuid.New(), Name: "planning:approve", Resource: "planning", Action: "approve", Description: "Approve plans"},

		// ===== Purchases =====
		{ID: uuid.New(), Name: "purchase:create", Resource: "purchase", Action: "create", Description: "Create purchase order"},
		{ID: uuid.New(), Name: "purchase:read", Resource: "purchase", Action: "read", Description: "View purchase details"},
		{ID: uuid.New(), Name: "purchase:update", Resource: "purchase", Action: "update", Description: "Edit purchase order"},
		{ID: uuid.New(), Name: "purchase:approve", Resource: "purchase", Action: "approve", Description: "Approve purchase"},
		{ID: uuid.New(), Name: "purchase:delete", Resource: "purchase", Action: "delete", Description: "Delete purchase"},

		// ===== Inventory =====
		{ID: uuid.New(), Name: "inventory:create", Resource: "inventory", Action: "create", Description: "Add inventory item"},
		{ID: uuid.New(), Name: "inventory:read", Resource: "inventory", Action: "read", Description: "View inventory"},
		{ID: uuid.New(), Name: "inventory:update", Resource: "inventory", Action: "update", Description: "Edit inventory item"},
		{ID: uuid.New(), Name: "inventory:delete", Resource: "inventory", Action: "delete", Description: "Remove inventory item"},
		{ID: uuid.New(), Name: "inventory:approve", Resource: "inventory", Action: "approve", Description: "Approve inventory transfer"},

		// ===== HR & Payroll =====
		{ID: uuid.New(), Name: "hr:create", Resource: "hr", Action: "create", Description: "Add new employee"},
		{ID: uuid.New(), Name: "hr:read", Resource: "hr", Action: "read", Description: "View employee details"},
		{ID: uuid.New(), Name: "hr:update", Resource: "hr", Action: "update", Description: "Edit employee info"},
		{ID: uuid.New(), Name: "hr:delete", Resource: "hr", Action: "delete", Description: "Remove employee"},
		{ID: uuid.New(), Name: "payroll:generate", Resource: "payroll", Action: "generate", Description: "Generate payroll"},
		{ID: uuid.New(), Name: "payroll:approve", Resource: "payroll", Action: "approve", Description: "Approve payroll"},

		// ===== Finance =====
		{ID: uuid.New(), Name: "finance:create", Resource: "finance", Action: "create", Description: "Create financial entry"},
		{ID: uuid.New(), Name: "finance:read", Resource: "finance", Action: "read", Description: "View financial records"},
		{ID: uuid.New(), Name: "finance:update", Resource: "finance", Action: "update", Description: "Edit financial record"},
		{ID: uuid.New(), Name: "finance:approve", Resource: "finance", Action: "approve", Description: "Approve transactions"},

		// ===== Documents / DMS =====
		{ID: uuid.New(), Name: "document:upload", Resource: "document", Action: "create", Description: "Upload document"},
		{ID: uuid.New(), Name: "document:read", Resource: "document", Action: "read", Description: "View document"},
		{ID: uuid.New(), Name: "document:update", Resource: "document", Action: "update", Description: "Edit document metadata"},
		{ID: uuid.New(), Name: "document:delete", Resource: "document", Action: "delete", Description: "Delete document"},

		// ===== Reports & Analytics =====
		{ID: uuid.New(), Name: "report:read", Resource: "report", Action: "read", Description: "View reports"},
		{ID: uuid.New(), Name: "report:export", Resource: "report", Action: "export", Description: "Export reports"},
		{ID: uuid.New(), Name: "dashboard:view", Resource: "dashboard", Action: "read", Description: "View dashboards"},

		// ===== Admin / Users / Roles =====
		{ID: uuid.New(), Name: "user:create", Resource: "user", Action: "create", Description: "Create user"},
		{ID: uuid.New(), Name: "user:read", Resource: "user", Action: "read", Description: "View user"},
		{ID: uuid.New(), Name: "user:update", Resource: "user", Action: "update", Description: "Edit user"},
		{ID: uuid.New(), Name: "user:delete", Resource: "user", Action: "delete", Description: "Delete user"},
		{ID: uuid.New(), Name: "role:create", Resource: "role", Action: "create", Description: "Create role"},
		{ID: uuid.New(), Name: "role:read", Resource: "role", Action: "read", Description: "View roles"},
		{ID: uuid.New(), Name: "role:update", Resource: "role", Action: "update", Description: "Edit role permissions"},
		{ID: uuid.New(), Name: "role:delete", Resource: "role", Action: "delete", Description: "Delete roles"},
		{ID: uuid.New(), Name: "role:assign", Resource: "role", Action: "assign", Description: "Assign role to user"},
		{ID: uuid.New(), Name: "business:create", Resource: "business", Action: "create", Description: "Create business vertical"},
		{ID: uuid.New(), Name: "business:read", Resource: "business", Action: "read", Description: "View business vertical"},
		{ID: uuid.New(), Name: "business:update", Resource: "business", Action: "update", Description: "Edit business vertical"},
		{ID: uuid.New(), Name: "business:delete", Resource: "business", Action: "delete", Description: "Delete business vertical"},

		// ===== Solar Vertical Specific =====
		{ID: uuid.New(), Name: "solar:read_generation", Resource: "solar", Action: "read", Description: "View solar generation data"},
		{ID: uuid.New(), Name: "solar:manage_panels", Resource: "solar", Action: "manage", Description: "Manage solar panel configurations"},
		{ID: uuid.New(), Name: "solar:maintenance", Resource: "solar", Action: "maintenance", Description: "Perform solar equipment maintenance"},

		// ===== Water Vertical Specific =====
		{ID: uuid.New(), Name: "water:read_consumption", Resource: "water", Action: "read", Description: "View water consumption data"},
		{ID: uuid.New(), Name: "water:manage_supply", Resource: "water", Action: "manage", Description: "Manage water supply systems"},
		{ID: uuid.New(), Name: "water:quality_control", Resource: "water", Action: "quality_control", Description: "Manage water quality testing"},

		// ===== Contractor / Subcontractor Read-Only =====
		{ID: uuid.New(), Name: "contractor:project_read", Resource: "project", Action: "read", Description: "View projects (contractor)"},
		{ID: uuid.New(), Name: "contractor:inventory_read", Resource: "inventory", Action: "read", Description: "View inventory (contractor)"},
		{ID: uuid.New(), Name: "contractor:material_read", Resource: "materials", Action: "read", Description: "View materials (contractor)"},
	}

	// Create permissions if they don't exist
	for _, perm := range permissions {
		var existingPerm models.Permission
		if err := DB.Where("name = ?", perm.Name).First(&existingPerm).Error; err != nil {
			if err := DB.Create(&perm).Error; err != nil {
				log.Printf("❌ Error creating permission %s: %v", perm.Name, err)
			} else {
				log.Printf("✅ Created permission: %s (ID: %s)", perm.Name, perm.ID)
			}
		} else {
			log.Printf("ℹ️  Permission already exists: %s (ID: %s)", existingPerm.Name, existingPerm.ID)
		}
	}

	var allPerms []models.Permission
	if err := DB.Find(&allPerms).Error; err != nil {
		log.Fatalf("Failed to load permissions: %v", err)
	}
	permMap := make(map[string]models.Permission)
	for _, p := range allPerms {
		permMap[p.Name] = p
	}
	log.Printf("📋 Loaded %d permissions into permMap", len(permMap))

	// Define default roles
	globalRoles := []models.Role{
		{
			Name:        "super_admin",
			Description: "Full system access",
			Permissions: []models.Permission{
				{Name: "admin_all"},
				{Name: "manage_roles"},
			},
		},
		{
			Name:        "Admin",
			Description: "Head Office admin: manage users, roles, finance, HR, reports",
			IsGlobal:    true,
			IsActive:    true,
			Permissions: []models.Permission{
				// Filter only relevant perms: user:create/read/update/delete, role:assign, finance:*, hr:*, reports
				{Name: "user:create"},
				{Name: "user:read"},
				{Name: "user:update"},
				{Name: "user:delete"},
				{Name: "role:assign"},
				{Name: "finance:create"},
				{Name: "finance:read"},
				{Name: "finance:update"},
				{Name: "finance:approve"},
				{Name: "hr:create"},
				{Name: "hr:read"},
				{Name: "hr:update"},
				{Name: "hr:delete"},
				{Name: "payroll:generate"},
				{Name: "payroll:approve"},
				{Name: "report:read"},
				{Name: "report:export"},
				{Name: "document:upload"},
				{Name: "document:read"},
				{Name: "document:update"},
				{Name: "document:delete"},
			},
		},
		{
			Name:        "Manager",
			Description: "Department-level manager: approve projects, plans, purchases",
			IsGlobal:    true,
			IsActive:    true,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "project:approve"},
				{Name: "project:assign"},
				{Name: "planning:read"},
				{Name: "planning:update"},
				{Name: "planning:approve"},
				{Name: "purchase:read"},
				{Name: "purchase:update"},
				{Name: "purchase:approve"},
				{Name: "inventory:read"},
				{Name: "inventory:update"},
				{Name: "inventory:approve"},
				{Name: "report:read"},
				{Name: "report:export"},
			},
		},
		{
			Name:        "Consultant",
			Description: "Limited access to planning and project modules",
			IsGlobal:    true,
			IsActive:    true,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "planning:read"},
				{Name: "planning:update"},
			},
		},
		// {
		// 	Name:        "user",
		// 	Description: "Basic user access",
		// 	Permissions: []models.Permission{
		// 		{Name: "read_reports"},
		// 		{Name: "create_reports"},
		// 		{Name: "read_materials"},
		// 		{Name: "read_kpis"},
		// 	},
		// },
	}

	for _, roleData := range globalRoles {
		var role models.Role
		err := DB.Where("name = ?", roleData.Name).First(&role).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			role = models.Role{
				Name:        roleData.Name,
				Description: roleData.Description,
				IsGlobal:    roleData.IsGlobal,
				IsActive:    roleData.IsActive,
			}
			if err := DB.Create(&role).Error; err != nil {
				log.Printf("Error creating role %s: %v", roleData.Name, err)
				continue
			}
			log.Printf("Created role: %s", roleData.Name)
		} else if err != nil {
			log.Printf("DB error fetching role %s: %v", roleData.Name, err)
			continue
		}

		// Build slice of permission objects from names
		var permsToAssign []models.Permission
		log.Printf("🔍 Processing permissions for role '%s' (ID: %s)", role.Name, role.ID)
		for _, p := range roleData.Permissions {
			dbPerm, ok := permMap[p.Name]
			if !ok {
				log.Printf("  ❌ Permission '%s' not found in permMap for role '%s'", p.Name, role.Name)
				continue
			}
			log.Printf("  ✅ Found permission '%s' (ID: %s)", dbPerm.Name, dbPerm.ID)
			permsToAssign = append(permsToAssign, dbPerm)
		}

		log.Printf("📌 Assigning %d permissions to role '%s'", len(permsToAssign), role.Name)

		// First, delete existing permissions for this role (for idempotency)
		if err := DB.Exec("DELETE FROM role_permissions WHERE role_id = ?", role.ID).Error; err != nil {
			log.Printf("❌ Failed to clear existing permissions for role %s: %v", role.Name, err)
			continue
		}

		// Insert permissions directly using SQL for reliability
		successCount := 0
		for _, perm := range permsToAssign {
			rolePermission := models.RolePermission{
				RoleID:       role.ID,
				PermissionID: perm.ID,
				CreatedAt:    time.Now(),
			}

			// Use direct insert
			if err := DB.Create(&rolePermission).Error; err != nil {
				log.Printf("  ❌ Failed to insert permission '%s' for role '%s': %v", perm.Name, role.Name, err)
			} else {
				log.Printf("  ✅ Inserted permission '%s' into role_permissions", perm.Name)
				successCount++
			}
		}

		// Verify assignment
		var assignedCount int64
		DB.Table("role_permissions").Where("role_id = ?", role.ID).Count(&assignedCount)

		if assignedCount == int64(len(permsToAssign)) {
			log.Printf("✅ Successfully assigned %d/%d permissions to role '%s' (verified in DB)", successCount, len(permsToAssign), role.Name)
		} else {
			log.Printf("⚠️  Partial success: assigned %d/%d permissions to role '%s' (DB shows: %d)", successCount, len(permsToAssign), role.Name, assignedCount)
		}
	}
}

// SeedBusinessVerticals creates default business verticals
func SeedBusinessVerticals() {
	defaultBusinesses := []struct {
		Name        string
		Code        string
		Description string
	}{
		{
			Name:        "Water Works",
			Code:        "WATER",
			Description: "Water supply and distribution management",
		},
		{
			Name:        "Solar Works",
			Code:        "SOLAR",
			Description: "Solar energy generation and maintenance operations",
		},
		{
			Name:        "Head Office",
			Code:        "HO",
			Description: "Corporate administration and support services",
		},
		{
			Name:        "Contractors",
			Code:        "CONTRACTORS",
			Description: "Contractors / Subcontractors",
		},
	}

	log.Printf("🔍 Starting SeedBusinessVerticals...")

	for _, businessData := range defaultBusinesses {
		var business models.BusinessVertical
		err := DB.Where("code = ?", businessData.Code).First(&business).Error

		if err != nil {
			// Business vertical doesn't exist - create it
			log.Printf("📝 Creating new business vertical: %s", businessData.Name)

			defaultSettings := "{}"
			business = models.BusinessVertical{
				Name:        businessData.Name,
				Code:        businessData.Code,
				Description: businessData.Description,
				IsActive:    true,
				Settings:    &defaultSettings,
			}

			if err := DB.Create(&business).Error; err != nil {
				log.Printf("❌ Error creating business vertical %s: %v", businessData.Name, err)
				continue
			}

			log.Printf("✅ Created business vertical: %s (ID: %s)", businessData.Name, business.ID)
		} else {
			// Business vertical already exists
			log.Printf("ℹ️  Business vertical already exists: %s (ID: %s)", businessData.Name, business.ID)
		}

		// Create/update default roles for this business (regardless of whether BV is new or existing)
		log.Printf("🔍 Creating business roles for: %s", businessData.Name)
		createDefaultBusinessRoles(business.ID, businessData.Code)
	}

	log.Printf("✅ SeedBusinessVerticals completed")
}

// createDefaultBusinessRoles creates default roles for a business vertical based on its code
func createDefaultBusinessRoles(businessID uuid.UUID, businessCode string) {
	var defaultRoles []models.BusinessRole

	// Create roles based on business vertical code
	switch businessCode {
	case "HO": // Head Office roles
		defaultRoles = []models.BusinessRole{
		{
			ID:                 uuid.New(),
			Name:               "HO_Admin",
			DisplayName:        "Head Office Admin",
			Description:        "Full access to HO modules",
			BusinessVerticalID: businessID,
			Permissions:        []models.Permission{}, // all HO perms: projects, planning, purchases, inventory, hr, payroll, finance, reports
			Level:              1,
			IsActive:           true,
		},
		{
			ID:                 uuid.New(),
			Name:               "HO_Manager",
			DisplayName:        "Head Office Manager",
			Description:        "Manage projects, purchase, planning, reports",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "project:approve"},
				{Name: "project:assign"},
				{Name: "planning:read"},
				{Name: "planning:update"},
				{Name: "planning:approve"},
				{Name: "purchase:read"},
				{Name: "purchase:update"},
				{Name: "purchase:approve"},
				{Name: "inventory:read"},
				{Name: "inventory:update"},
				{Name: "inventory:approve"},
				{Name: "report:read"},
				{Name: "report:export"},
			},
			Level:    2,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "HO_HR",
			DisplayName:        "Head Office HR",
			Description:        "Access HR & Payroll modules",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "hr:create"},
				{Name: "hr:read"},
				{Name: "hr:update"},
				{Name: "hr:delete"},
				{Name: "payroll:generate"},
				{Name: "payroll:approve"},
			},
			Level:    3,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "HO_Consultant",
			DisplayName:        "Head Office Consultant",
			Description:        "Read/write access to Projects & Planning",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "planning:read"},
				{Name: "planning:update"},
			},
			Level:    4,
			IsActive: true,
		},
	}

	case "WATER": // Water Works roles
		defaultRoles = []models.BusinessRole{
		{
			ID:                 uuid.New(),
			Name:               "Water_Admin",
			DisplayName:        "Water Works Admin",
			Description:        "Full control within Water vertical",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				// Project module
				{Name: "project:create"},
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "project:delete"},
				{Name: "project:approve"},
				{Name: "project:assign"},

				// Planning module
				{Name: "planning:create"},
				{Name: "planning:read"},
				{Name: "planning:update"},
				{Name: "planning:approve"},

				// Purchase module
				{Name: "purchase:create"},
				{Name: "purchase:read"},
				{Name: "purchase:update"},
				{Name: "purchase:approve"},
				{Name: "purchase:delete"},

				// Inventory module
				{Name: "inventory:create"},
				{Name: "inventory:read"},
				{Name: "inventory:update"},
				{Name: "inventory:delete"},
				{Name: "inventory:approve"},

				// Water-specific ERP permissions
				{Name: "water:read_consumption"},
				{Name: "water:manage_supply"},
				{Name: "water:quality_control"},

				// Reports
				{Name: "report:read"},
				{Name: "report:export"},

				// Documents
				{Name: "document:upload"},
				{Name: "document:read"},
				{Name: "document:update"},
				{Name: "document:delete"},
			},
			// all water-related permissions
			Level:    1,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Project_Coordinator",
			DisplayName:        "Water Project Coordinator",
			Description:        "Manage projects, assign tasks",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "project:assign"},
				{Name: "planning:read"},
			},
			Level:    2,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Sr_Deputy_PM",
			DisplayName:        "Sr Deputy Project Manager",
			Description:        "Approve projects & plans",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "project:approve"},
				{Name: "planning:read"},
				{Name: "planning:update"},
				{Name: "planning:approve"},
			},
			Level:    2,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Engineer",
			DisplayName:        "Water Engineer",
			Description:        "Execute tasks, manage water system & inventory",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "inventory:create"},
				{Name: "inventory:read"},
				{Name: "inventory:update"},
				{Name: "water:read_consumption"},
				{Name: "water:manage_supply"},
				{Name: "water:quality_control"},
			},
			Level:    4,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Supervisor",
			DisplayName:        "Water Supervisor",
			Description:        "Supervise field execution",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "inventory:read"},
				{Name: "inventory:update"},
				{Name: "water:read_consumption"},
			},
			Level:    4,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Operator",
			DisplayName:        "Water Operator",
			Description:        "Operate water systems",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "inventory:create"},
				{Name: "water:read_consumption"},
				{Name: "water:manage_supply"},
			},
			Level:    5,
			IsActive: true,
		},
	}

	case "SOLAR": // Solar Works roles
		defaultRoles = []models.BusinessRole{
		{
			ID:                 uuid.New(),
			Name:               "Solar_Admin",
			DisplayName:        "Solar Admin",
			Description:        "Full Solar vertical access",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:create"},
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "project:delete"},
				{Name: "project:approve"},
				{Name: "project:assign"},

				// Planning module
				{Name: "planning:create"},
				{Name: "planning:read"},
				{Name: "planning:update"},
				{Name: "planning:approve"},

				// Purchase module
				{Name: "purchase:create"},
				{Name: "purchase:read"},
				{Name: "purchase:update"},
				{Name: "purchase:approve"},
				{Name: "purchase:delete"},

				// Inventory module
				{Name: "inventory:create"},
				{Name: "inventory:read"},
				{Name: "inventory:update"},
				{Name: "inventory:delete"},
				{Name: "inventory:approve"},

				// Solar-specific ERP permissions
				{Name: "solar:read_generation"}, // View solar generation data
				{Name: "solar:manage_panels"},   // Configure solar panels
				{Name: "solar:maintenance"},     // Perform maintenance tasks

				// Reports
				{Name: "report:read"},
				{Name: "report:export"},

				// Documents
				{Name: "document:upload"},
				{Name: "document:read"},
				{Name: "document:update"},
				{Name: "document:delete"},
			}, // all solar perms
			Level:    1,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Area_Project_Manager",
			DisplayName:        "Solar Area Project Manager",
			Description:        "Manage projects, plans, approvals",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "project:update"},
				{Name: "project:approve"},
				{Name: "project:assign"},
				{Name: "planning:read"},
				{Name: "planning:update"},
				{Name: "planning:approve"},
			},
			Level:    2,
			IsActive: true,
		},
		{
			ID:                 uuid.New(),
			Name:               "Sr_Engineer",
			DisplayName:        "Solar Sr Engineer",
			Description:        "Manage panels, solar generation, maintenance",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "solar:read_generation"},
				{Name: "solar:manage_panels"},
				{Name: "solar:maintenance"},
			},
			Level:    3,
			IsActive: true,
		},
	}

	case "CONTRACTORS": // Contractor roles
		defaultRoles = []models.BusinessRole{
		{
			ID:                 uuid.New(),
			Name:               "Sub_Contractor",
			DisplayName:        "Sub Contractor",
			Description:        "Read-only access to Projects, Materials, Inventory",
			BusinessVerticalID: businessID,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "inventory:read"},
				{Name: "materials:read"},
			},
			Level:    5,
			IsActive: true,
		},
	}

	default:
		log.Printf("⚠️  Unknown business code: %s, skipping role creation", businessCode)
		return
	}

	// Load all permissions into a map for business roles
	var allPermsForBusiness []models.Permission
	if err := DB.Find(&allPermsForBusiness).Error; err != nil {
		log.Printf("❌ Failed to load permissions for business roles: %v", err)
		return
	}
	businessPermMap := make(map[string]models.Permission)
	for _, p := range allPermsForBusiness {
		businessPermMap[p.Name] = p
	}

	for _, roleData := range defaultRoles {
		var role models.BusinessRole
		err := DB.Where("name = ? AND business_vertical_id = ?", roleData.Name, businessID).First(&role).Error

		if err != nil {
			// Business role doesn't exist - create it
			role = models.BusinessRole{
				Name:               roleData.Name,
				DisplayName:        roleData.DisplayName,
				Description:        roleData.Description,
				Level:              roleData.Level,
				BusinessVerticalID: businessID,
				IsActive:           true,
			}

			if err := DB.Create(&role).Error; err != nil {
				log.Printf("❌ Error creating business role %s: %v", roleData.Name, err)
				continue
			}

			log.Printf("✅ Created business role: %s (ID: %s)", roleData.DisplayName, role.ID)
		} else {
			// Business role already exists
			log.Printf("ℹ️  Business role '%s' already exists (ID: %s)", roleData.DisplayName, role.ID)
		}

		// Assign permissions (whether role is new or existing)
		if len(roleData.Permissions) == 0 {
			log.Printf("⚠️  No permissions defined for business role '%s'", roleData.DisplayName)
			continue
		}

		// Build permission list from permMap
		var permsToAssign []models.Permission
		log.Printf("🔍 Processing permissions for business role '%s'", roleData.DisplayName)
		for _, permName := range roleData.Permissions {
			dbPerm, ok := businessPermMap[permName.Name]
			if !ok {
				log.Printf("  ❌ Permission '%s' not found for business role '%s'", permName.Name, roleData.DisplayName)
				continue
			}
			log.Printf("  ✅ Found permission '%s' (ID: %s)", dbPerm.Name, dbPerm.ID)
			permsToAssign = append(permsToAssign, dbPerm)
		}

		// Assign permissions using direct SQL for reliability
		if len(permsToAssign) > 0 {
			// First, delete existing permissions for this role (for idempotency)
			if err := DB.Exec("DELETE FROM business_role_permissions WHERE business_role_id = ?", role.ID).Error; err != nil {
				log.Printf("❌ Failed to clear existing permissions for business role %s: %v", roleData.DisplayName, err)
				continue
			}

			// Insert permissions directly
			successCount := 0
			for _, perm := range permsToAssign {
				businessRolePermission := models.BusinessRolePermission{
					BusinessRoleID: role.ID,
					PermissionID:   perm.ID,
					CreatedAt:      time.Now(),
				}

				if err := DB.Create(&businessRolePermission).Error; err != nil {
					log.Printf("  ❌ Failed to insert permission '%s' for business role '%s': %v", perm.Name, roleData.DisplayName, err)
				} else {
					log.Printf("  ✅ Inserted permission '%s' into business_role_permissions", perm.Name)
					successCount++
				}
			}

			// Verify assignment
			var assignedCount int64
			DB.Table("business_role_permissions").Where("business_role_id = ?", role.ID).Count(&assignedCount)

			if assignedCount == int64(len(permsToAssign)) {
				log.Printf("✅ Successfully assigned %d/%d permissions to business role '%s' (verified in DB)", successCount, len(permsToAssign), roleData.DisplayName)
			} else {
				log.Printf("⚠️  Partial success: assigned %d/%d permissions to business role '%s' (DB shows: %d)", successCount, len(permsToAssign), roleData.DisplayName, assignedCount)
			}
		} else {
			log.Printf("⚠️  No permissions to assign to business role '%s'", roleData.DisplayName)
		}
	}
}
