package config

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// RunAllSeeding runs all seeding operations in the correct order
func RunAllSeeding() error {
	log.Println("=== Starting Database Seeding ===")

	// Step 1: Seed permissions first (required for roles)
	log.Println("\n[1/7] Seeding Permissions...")
	SeedPermissions()

	// Step 2: Seed business verticals and their roles
	log.Println("\n[2/7] Seeding Business Verticals...")
	SeedBusinessVerticals()

	// Step 3: Seed sites for each vertical
	log.Println("\n[3/7] Seeding Sites...")
	SeedSites()

	// Step 4: Seed ABAC attributes and sample policies
	log.Println("\n[4/7] Seeding ABAC Attributes and Policies...")
	if err := RunABACSeeding(DB); err != nil {
		log.Printf("Warning: ABAC seeding failed: %v", err)
	}

	// Step 5: Seed default workflows
	log.Println("\n[5/7] Seeding Workflows...")
	SeedWorkflows()

	// Step 6: Seed default users
	log.Println("\n[6/7] Seeding Default Users...")
	SeedUsers()

	// Step 7: Verify RBAC setup
	log.Println("\n[7/7] Verifying RBAC Migration...")
	VerifyRBACMigration()

	log.Println("\n=== Database Seeding Complete ===")
	return nil
}

// =====================================================
// Permissions & Roles Seeding
// =====================================================

// SeedPermissions creates default permissions and roles
func SeedPermissions() {
	permissions := []models.Permission{
		// Super Admin Wildcard
		{ID: uuid.New(), Name: "*:*:*", Resource: "*", Action: "*", Description: "Super Admin wildcard - all permissions"},

		// Project Management
		{ID: uuid.New(), Name: "project:create", Resource: "project", Action: "create", Description: "Create project"},
		{ID: uuid.New(), Name: "project:read", Resource: "project", Action: "read", Description: "View project details"},
		{ID: uuid.New(), Name: "project:update", Resource: "project", Action: "update", Description: "Edit project"},
		{ID: uuid.New(), Name: "project:delete", Resource: "project", Action: "delete", Description: "Delete project"},
		{ID: uuid.New(), Name: "project:approve", Resource: "project", Action: "approve", Description: "Approve project"},
		{ID: uuid.New(), Name: "project:assign", Resource: "project", Action: "assign", Description: "Assign users to project"},

		// Planning
		{ID: uuid.New(), Name: "planning:create", Resource: "planning", Action: "create", Description: "Create plans"},
		{ID: uuid.New(), Name: "planning:read", Resource: "planning", Action: "read", Description: "View plans"},
		{ID: uuid.New(), Name: "planning:update", Resource: "planning", Action: "update", Description: "Update plans"},
		{ID: uuid.New(), Name: "planning:approve", Resource: "planning", Action: "approve", Description: "Approve plans"},

		// Purchases
		{ID: uuid.New(), Name: "purchase:create", Resource: "purchase", Action: "create", Description: "Create purchase order"},
		{ID: uuid.New(), Name: "purchase:read", Resource: "purchase", Action: "read", Description: "View purchase details"},
		{ID: uuid.New(), Name: "purchase:update", Resource: "purchase", Action: "update", Description: "Edit purchase order"},
		{ID: uuid.New(), Name: "purchase:approve", Resource: "purchase", Action: "approve", Description: "Approve purchase"},
		{ID: uuid.New(), Name: "purchase:delete", Resource: "purchase", Action: "delete", Description: "Delete purchase"},

		// Inventory
		{ID: uuid.New(), Name: "inventory:create", Resource: "inventory", Action: "create", Description: "Add inventory item"},
		{ID: uuid.New(), Name: "inventory:read", Resource: "inventory", Action: "read", Description: "View inventory"},
		{ID: uuid.New(), Name: "inventory:update", Resource: "inventory", Action: "update", Description: "Edit inventory item"},
		{ID: uuid.New(), Name: "inventory:delete", Resource: "inventory", Action: "delete", Description: "Remove inventory item"},
		{ID: uuid.New(), Name: "inventory:approve", Resource: "inventory", Action: "approve", Description: "Approve inventory transfer"},

		// HR & Payroll
		{ID: uuid.New(), Name: "hr:create", Resource: "hr", Action: "create", Description: "Add new employee"},
		{ID: uuid.New(), Name: "hr:read", Resource: "hr", Action: "read", Description: "View employee details"},
		{ID: uuid.New(), Name: "hr:update", Resource: "hr", Action: "update", Description: "Edit employee info"},
		{ID: uuid.New(), Name: "hr:delete", Resource: "hr", Action: "delete", Description: "Remove employee"},
		{ID: uuid.New(), Name: "payroll:generate", Resource: "payroll", Action: "generate", Description: "Generate payroll"},
		{ID: uuid.New(), Name: "payroll:approve", Resource: "payroll", Action: "approve", Description: "Approve payroll"},

		// Finance
		{ID: uuid.New(), Name: "finance:create", Resource: "finance", Action: "create", Description: "Create financial entry"},
		{ID: uuid.New(), Name: "finance:read", Resource: "finance", Action: "read", Description: "View financial records"},
		{ID: uuid.New(), Name: "finance:update", Resource: "finance", Action: "update", Description: "Edit financial record"},
		{ID: uuid.New(), Name: "finance:approve", Resource: "finance", Action: "approve", Description: "Approve transactions"},

		// Documents / DMS
		{ID: uuid.New(), Name: "document:upload", Resource: "document", Action: "create", Description: "Upload document"},
		{ID: uuid.New(), Name: "document:read", Resource: "document", Action: "read", Description: "View document"},
		{ID: uuid.New(), Name: "document:update", Resource: "document", Action: "update", Description: "Edit document metadata"},
		{ID: uuid.New(), Name: "document:delete", Resource: "document", Action: "delete", Description: "Delete document"},

		// Reports & Analytics
		{ID: uuid.New(), Name: "report:read", Resource: "report", Action: "read", Description: "View reports"},
		{ID: uuid.New(), Name: "report:export", Resource: "report", Action: "export", Description: "Export reports"},
		{ID: uuid.New(), Name: "dashboard:view", Resource: "dashboard", Action: "read", Description: "View dashboards"},

		// Admin / Users / Roles
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

		// Solar Vertical Specific
		{ID: uuid.New(), Name: "solar:read_generation", Resource: "solar", Action: "read", Description: "View solar generation data"},
		{ID: uuid.New(), Name: "solar:manage_panels", Resource: "solar", Action: "manage", Description: "Manage solar panel configurations"},
		{ID: uuid.New(), Name: "solar:maintenance", Resource: "solar", Action: "maintenance", Description: "Perform solar equipment maintenance"},

		// Water Vertical Specific
		{ID: uuid.New(), Name: "water:read_consumption", Resource: "water", Action: "read", Description: "View water consumption data"},
		{ID: uuid.New(), Name: "water:manage_supply", Resource: "water", Action: "manage", Description: "Manage water supply systems"},
		{ID: uuid.New(), Name: "water:quality_control", Resource: "water", Action: "quality_control", Description: "Manage water quality testing"},

		// Contractor / Subcontractor Read-Only
		{ID: uuid.New(), Name: "contractor:project_read", Resource: "project", Action: "read", Description: "View projects (contractor)"},
		{ID: uuid.New(), Name: "contractor:inventory_read", Resource: "inventory", Action: "read", Description: "View inventory (contractor)"},
		{ID: uuid.New(), Name: "contractor:material_read", Resource: "materials", Action: "read", Description: "View materials (contractor)"},

		// Site Management
		{ID: uuid.New(), Name: "site:manage_access", Resource: "site", Action: "manage", Description: "Manage user access to sites"},
		{ID: uuid.New(), Name: "site:view", Resource: "site", Action: "read", Description: "View sites"},

		// ABAC & Policy Management
		{ID: uuid.New(), Name: "manage_policies", Resource: "policy", Action: "manage", Description: "Manage access control policies"},
		{ID: uuid.New(), Name: "manage_attributes", Resource: "attribute", Action: "manage", Description: "Manage attribute definitions"},
		{ID: uuid.New(), Name: "manage_user_attributes", Resource: "user_attribute", Action: "manage", Description: "Assign attributes to users"},
		{ID: uuid.New(), Name: "manage_resource_attributes", Resource: "resource_attribute", Action: "manage", Description: "Assign attributes to resources"},
		{ID: uuid.New(), Name: "view_policy_evaluations", Resource: "policy_evaluation", Action: "read", Description: "View policy evaluation audit logs"},

		// Chat Permissions
		{ID: uuid.New(), Name: "chat:conversation:create", Resource: "chat_conversation", Action: "create", Description: "Create conversations"},
		{ID: uuid.New(), Name: "chat:conversation:read", Resource: "chat_conversation", Action: "read", Description: "View conversations"},
		{ID: uuid.New(), Name: "chat:conversation:update", Resource: "chat_conversation", Action: "update", Description: "Update conversations"},
		{ID: uuid.New(), Name: "chat:conversation:delete", Resource: "chat_conversation", Action: "delete", Description: "Delete conversations"},
		{ID: uuid.New(), Name: "chat:group:create", Resource: "chat_group", Action: "create", Description: "Create chat groups (admin only)"},
		{ID: uuid.New(), Name: "chat:message:create", Resource: "chat_message", Action: "create", Description: "Send messages"},
		{ID: uuid.New(), Name: "chat:message:read", Resource: "chat_message", Action: "read", Description: "View messages"},
		{ID: uuid.New(), Name: "chat:message:update", Resource: "chat_message", Action: "update", Description: "Edit own messages"},
		{ID: uuid.New(), Name: "chat:message:delete", Resource: "chat_message", Action: "delete", Description: "Delete messages"},
		{ID: uuid.New(), Name: "chat:participant:create", Resource: "chat_participant", Action: "create", Description: "Add participants to conversations"},
		{ID: uuid.New(), Name: "chat:participant:read", Resource: "chat_participant", Action: "read", Description: "View participants"},
		{ID: uuid.New(), Name: "chat:participant:update", Resource: "chat_participant", Action: "update", Description: "Update participant roles"},
		{ID: uuid.New(), Name: "chat:participant:delete", Resource: "chat_participant", Action: "delete", Description: "Remove participants"},
		{ID: uuid.New(), Name: "chat:reaction:create", Resource: "chat_reaction", Action: "create", Description: "Add reactions to messages"},
		{ID: uuid.New(), Name: "chat:reaction:read", Resource: "chat_reaction", Action: "read", Description: "View reactions"},
		{ID: uuid.New(), Name: "chat:reaction:delete", Resource: "chat_reaction", Action: "delete", Description: "Remove reactions"},
		{ID: uuid.New(), Name: "chat:attachment:create", Resource: "chat_attachment", Action: "create", Description: "Send attachments"},
		{ID: uuid.New(), Name: "chat:attachment:read", Resource: "chat_attachment", Action: "read", Description: "View attachments"},
	}

	// Create permissions if they don't exist
	for _, perm := range permissions {
		var existingPerm models.Permission
		if err := DB.Where("name = ?", perm.Name).First(&existingPerm).Error; err != nil {
			if err := DB.Create(&perm).Error; err != nil {
				log.Printf("Error creating permission %s: %v", perm.Name, err)
			} else {
				log.Printf("Created permission: %s", perm.Name)
			}
		}
	}

	// Load all permissions
	var allPerms []models.Permission
	if err := DB.Find(&allPerms).Error; err != nil {
		log.Fatalf("Failed to load permissions: %v", err)
	}
	permMap := make(map[string]models.Permission)
	for _, p := range allPerms {
		permMap[p.Name] = p
	}
	log.Printf("Loaded %d permissions", len(permMap))

	// Define global roles
	globalRoles := []models.Role{
		{
			Name:        "super_admin",
			Description: "Full system access",
			Level:       0,
			Permissions: []models.Permission{{Name: "*:*:*"}},
		},
		{
			Name:        "System_Admin",
			Description: "User and role management across system",
			IsGlobal:    true,
			IsActive:    true,
			Level:       1,
			Permissions: []models.Permission{
				{Name: "user:create"}, {Name: "user:read"}, {Name: "user:update"}, {Name: "user:delete"},
				{Name: "role:read"}, {Name: "role:assign"}, {Name: "business:read"},
			},
		},
		{
			Name:        "Admin",
			Description: "Head Office admin: manage users, roles, finance, HR, reports",
			IsGlobal:    true,
			IsActive:    true,
			Permissions: []models.Permission{
				{Name: "user:create"}, {Name: "user:read"}, {Name: "user:update"}, {Name: "user:delete"},
				{Name: "role:assign"},
				{Name: "finance:create"}, {Name: "finance:read"}, {Name: "finance:update"}, {Name: "finance:approve"},
				{Name: "hr:create"}, {Name: "hr:read"}, {Name: "hr:update"}, {Name: "hr:delete"},
				{Name: "payroll:generate"}, {Name: "payroll:approve"},
				{Name: "report:read"}, {Name: "report:export"},
				{Name: "document:upload"}, {Name: "document:read"}, {Name: "document:update"}, {Name: "document:delete"},
			},
		},
		{
			Name:        "Manager",
			Description: "Department-level manager: approve projects, plans, purchases",
			IsGlobal:    true,
			IsActive:    true,
			Permissions: []models.Permission{
				{Name: "project:read"}, {Name: "project:update"}, {Name: "project:approve"}, {Name: "project:assign"},
				{Name: "planning:read"}, {Name: "planning:update"}, {Name: "planning:approve"},
				{Name: "purchase:read"}, {Name: "purchase:update"}, {Name: "purchase:approve"},
				{Name: "inventory:read"}, {Name: "inventory:update"}, {Name: "inventory:approve"},
				{Name: "report:read"}, {Name: "report:export"},
			},
		},
		{
			Name:        "Consultant",
			Description: "Limited access to planning and project modules",
			IsGlobal:    true,
			IsActive:    true,
			Permissions: []models.Permission{
				{Name: "project:read"}, {Name: "project:update"},
				{Name: "planning:read"}, {Name: "planning:update"},
			},
		},
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

		// Build permission list
		var permsToAssign []models.Permission
		for _, p := range roleData.Permissions {
			if dbPerm, ok := permMap[p.Name]; ok {
				permsToAssign = append(permsToAssign, dbPerm)
			}
		}

		// Clear existing permissions
		DB.Exec("DELETE FROM role_permissions WHERE role_id = ?", role.ID)

		// Assign permissions
		for _, perm := range permsToAssign {
			rolePermission := models.RolePermission{
				RoleID:       role.ID,
				PermissionID: perm.ID,
				CreatedAt:    time.Now(),
			}
			DB.Create(&rolePermission)
		}

		var assignedCount int64
		DB.Table("role_permissions").Where("role_id = ?", role.ID).Count(&assignedCount)
		log.Printf("Assigned %d permissions to role '%s'", assignedCount, role.Name)
	}
}

// =====================================================
// Business Verticals Seeding
// =====================================================

// SeedBusinessVerticals creates default business verticals and their roles
func SeedBusinessVerticals() {
	defaultBusinesses := []struct {
		Name        string
		Code        string
		Description string
	}{
		{Name: "Water Works", Code: "WATER", Description: "Water supply and distribution management"},
		{Name: "Solar Works", Code: "SOLAR", Description: "Solar energy generation and maintenance operations"},
		{Name: "Head Office", Code: "HO", Description: "Corporate administration and support services"},
		{Name: "Contractors", Code: "CONTRACTORS", Description: "Contractors / Subcontractors"},
	}

	for _, businessData := range defaultBusinesses {
		var business models.BusinessVertical
		err := DB.Where("code = ?", businessData.Code).First(&business).Error

		if err != nil {
			defaultSettings := "{}"
			business = models.BusinessVertical{
				Name:        businessData.Name,
				Code:        businessData.Code,
				Description: businessData.Description,
				IsActive:    true,
				Settings:    &defaultSettings,
			}

			if err := DB.Create(&business).Error; err != nil {
				log.Printf("Error creating business vertical %s: %v", businessData.Name, err)
				continue
			}
			log.Printf("Created business vertical: %s (ID: %s)", businessData.Name, business.ID)
		} else {
			log.Printf("Business vertical already exists: %s", businessData.Name)
		}

		createDefaultBusinessRoles(business.ID, businessData.Code)
	}
}

// createDefaultBusinessRoles creates default roles for a business vertical
func createDefaultBusinessRoles(businessID uuid.UUID, businessCode string) {
	var defaultRoles []models.BusinessRole

	switch businessCode {
	case "HO":
		defaultRoles = getHORoles(businessID)
	case "WATER":
		defaultRoles = getWaterRoles(businessID)
	case "SOLAR":
		defaultRoles = getSolarRoles(businessID)
	case "CONTRACTORS":
		defaultRoles = getContractorRoles(businessID)
	default:
		log.Printf("Unknown business code: %s", businessCode)
		return
	}

	// Load permissions
	var allPerms []models.Permission
	if err := DB.Find(&allPerms).Error; err != nil {
		log.Printf("Failed to load permissions: %v", err)
		return
	}
	permMap := make(map[string]models.Permission)
	for _, p := range allPerms {
		permMap[p.Name] = p
	}

	for _, roleData := range defaultRoles {
		var role models.BusinessRole
		err := DB.Where("name = ? AND business_vertical_id = ?", roleData.Name, businessID).First(&role).Error

		if err != nil {
			role = models.BusinessRole{
				Name:               roleData.Name,
				DisplayName:        roleData.DisplayName,
				Description:        roleData.Description,
				Level:              roleData.Level,
				BusinessVerticalID: businessID,
				IsActive:           true,
			}

			if err := DB.Create(&role).Error; err != nil {
				log.Printf("Error creating business role %s: %v", roleData.Name, err)
				continue
			}
			log.Printf("Created business role: %s", roleData.DisplayName)
		}

		// Assign permissions
		if len(roleData.Permissions) > 0 {
			DB.Exec("DELETE FROM business_role_permissions WHERE business_role_id = ?", role.ID)

			for _, permName := range roleData.Permissions {
				if dbPerm, ok := permMap[permName.Name]; ok {
					brp := models.BusinessRolePermission{
						BusinessRoleID: role.ID,
						PermissionID:   dbPerm.ID,
						CreatedAt:      time.Now(),
					}
					DB.Create(&brp)
				}
			}
		}
	}
}

func getHORoles(businessID uuid.UUID) []models.BusinessRole {
	return []models.BusinessRole{
		{
			Name: "HO_Admin", DisplayName: "Head Office Admin", Description: "Full access to HO modules",
			BusinessVerticalID: businessID, Level: 1, IsActive: true,
		},
		{
			Name: "HO_Manager", DisplayName: "Head Office Manager", Description: "Manage projects, purchase, planning, reports",
			BusinessVerticalID: businessID, Level: 2, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:read"}, {Name: "project:update"}, {Name: "project:approve"}, {Name: "project:assign"},
				{Name: "planning:read"}, {Name: "planning:update"}, {Name: "planning:approve"},
				{Name: "purchase:read"}, {Name: "purchase:update"}, {Name: "purchase:approve"},
				{Name: "inventory:read"}, {Name: "inventory:update"}, {Name: "inventory:approve"},
				{Name: "report:read"}, {Name: "report:export"},
			},
		},
		{
			Name: "HO_HR", DisplayName: "Head Office HR", Description: "Access HR & Payroll modules",
			BusinessVerticalID: businessID, Level: 3, IsActive: true,
			Permissions: []models.Permission{
				{Name: "hr:create"}, {Name: "hr:read"}, {Name: "hr:update"}, {Name: "hr:delete"},
				{Name: "payroll:generate"}, {Name: "payroll:approve"},
			},
		},
		{
			Name: "HO_Consultant", DisplayName: "Head Office Consultant", Description: "Read/write access to Projects & Planning",
			BusinessVerticalID: businessID, Level: 4, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:read"}, {Name: "project:update"},
				{Name: "planning:read"}, {Name: "planning:update"},
			},
		},
	}
}

func getWaterRoles(businessID uuid.UUID) []models.BusinessRole {
	return []models.BusinessRole{
		{
			Name: "Water_Admin", DisplayName: "Water Works Admin", Description: "Full control within Water vertical",
			BusinessVerticalID: businessID, Level: 1, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:create"}, {Name: "project:read"}, {Name: "project:update"}, {Name: "project:delete"},
				{Name: "project:approve"}, {Name: "project:assign"},
				{Name: "planning:create"}, {Name: "planning:read"}, {Name: "planning:update"}, {Name: "planning:approve"},
				{Name: "purchase:create"}, {Name: "purchase:read"}, {Name: "purchase:update"}, {Name: "purchase:approve"}, {Name: "purchase:delete"},
				{Name: "inventory:create"}, {Name: "inventory:read"}, {Name: "inventory:update"}, {Name: "inventory:delete"}, {Name: "inventory:approve"},
				{Name: "water:read_consumption"}, {Name: "water:manage_supply"}, {Name: "water:quality_control"},
				{Name: "report:read"}, {Name: "report:export"},
				{Name: "document:upload"}, {Name: "document:read"}, {Name: "document:update"}, {Name: "document:delete"},
				{Name: "site:manage_access"}, {Name: "site:view"},
			},
		},
		{
			Name: "Project_Coordinator", DisplayName: "Water Project Coordinator", Description: "Manage projects, assign tasks",
			BusinessVerticalID: businessID, Level: 2, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:read"}, {Name: "project:update"}, {Name: "project:assign"},
				{Name: "planning:read"},
			},
		},
		{
			Name: "Engineer", DisplayName: "Water Engineer", Description: "Execute tasks, manage water system & inventory",
			BusinessVerticalID: businessID, Level: 4, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:read"}, {Name: "project:update"},
				{Name: "inventory:create"}, {Name: "inventory:read"}, {Name: "inventory:update"},
				{Name: "water:read_consumption"}, {Name: "water:manage_supply"}, {Name: "water:quality_control"},
			},
		},
		{
			Name: "Supervisor", DisplayName: "Water Supervisor", Description: "Supervise field execution",
			BusinessVerticalID: businessID, Level: 4, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "inventory:read"}, {Name: "inventory:update"},
				{Name: "water:read_consumption"},
			},
		},
		{
			Name: "Operator", DisplayName: "Water Operator", Description: "Operate water systems",
			BusinessVerticalID: businessID, Level: 5, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "inventory:create"},
				{Name: "water:read_consumption"}, {Name: "water:manage_supply"},
			},
		},
	}
}

func getSolarRoles(businessID uuid.UUID) []models.BusinessRole {
	return []models.BusinessRole{
		{
			Name: "Solar_Admin", DisplayName: "Solar Admin", Description: "Full Solar vertical access",
			BusinessVerticalID: businessID, Level: 1, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:create"}, {Name: "project:read"}, {Name: "project:update"}, {Name: "project:delete"},
				{Name: "project:approve"}, {Name: "project:assign"},
				{Name: "planning:create"}, {Name: "planning:read"}, {Name: "planning:update"}, {Name: "planning:approve"},
				{Name: "purchase:create"}, {Name: "purchase:read"}, {Name: "purchase:update"}, {Name: "purchase:approve"}, {Name: "purchase:delete"},
				{Name: "inventory:create"}, {Name: "inventory:read"}, {Name: "inventory:update"}, {Name: "inventory:delete"}, {Name: "inventory:approve"},
				{Name: "solar:read_generation"}, {Name: "solar:manage_panels"}, {Name: "solar:maintenance"},
				{Name: "report:read"}, {Name: "report:export"},
				{Name: "document:upload"}, {Name: "document:read"}, {Name: "document:update"}, {Name: "document:delete"},
				{Name: "site:manage_access"}, {Name: "site:view"},
			},
		},
		{
			Name: "Area_Project_Manager", DisplayName: "Solar Area Project Manager", Description: "Manage projects, plans, approvals",
			BusinessVerticalID: businessID, Level: 2, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:read"}, {Name: "project:update"}, {Name: "project:approve"}, {Name: "project:assign"},
				{Name: "planning:read"}, {Name: "planning:update"}, {Name: "planning:approve"},
			},
		},
		{
			Name: "Sr_Engineer", DisplayName: "Solar Sr Engineer", Description: "Manage panels, solar generation, maintenance",
			BusinessVerticalID: businessID, Level: 3, IsActive: true,
			Permissions: []models.Permission{
				{Name: "solar:read_generation"}, {Name: "solar:manage_panels"}, {Name: "solar:maintenance"},
			},
		},
	}
}

func getContractorRoles(businessID uuid.UUID) []models.BusinessRole {
	return []models.BusinessRole{
		{
			Name: "Sub_Contractor", DisplayName: "Sub Contractor", Description: "Read-only access to Projects, Materials, Inventory",
			BusinessVerticalID: businessID, Level: 5, IsActive: true,
			Permissions: []models.Permission{
				{Name: "project:read"},
				{Name: "inventory:read"},
			},
		},
	}
}

// =====================================================
// Workflow Seeding
// =====================================================

// SeedWorkflows creates default workflow definitions
func SeedWorkflows() {
	log.Println("Seeding default workflows...")

	// Standard Approval Workflow - Draft -> Submitted -> Approved/Rejected
	approvalWorkflow := models.WorkflowDefinition{
		Code:         "standard_approval",
		Name:         "Standard Approval Workflow",
		Description:  "Basic approval workflow with draft, submit, approve, and reject states",
		Version:      "1.0.0",
		InitialState: "draft",
		States: []byte(`[
			{"code": "draft", "name": "Draft", "description": "Initial draft state", "color": "gray", "is_final": false},
			{"code": "submitted", "name": "Submitted", "description": "Submitted for review", "color": "blue", "is_final": false},
			{"code": "approved", "name": "Approved", "description": "Approved by reviewer", "color": "green", "is_final": true},
			{"code": "rejected", "name": "Rejected", "description": "Rejected by reviewer", "color": "red", "is_final": true}
		]`),
		Transitions: []byte(`[
			{"from": "draft", "to": "submitted", "action": "submit", "label": "Submit for Review", "required_permission": ""},
			{"from": "submitted", "to": "approved", "action": "approve", "label": "Approve", "required_permission": "workflow:approve"},
			{"from": "submitted", "to": "rejected", "action": "reject", "label": "Reject", "required_permission": "workflow:approve"},
			{"from": "rejected", "to": "draft", "action": "revise", "label": "Revise", "required_permission": ""}
		]`),
		IsActive: true,
	}

	// Multi-Level Approval Workflow
	multiLevelWorkflow := models.WorkflowDefinition{
		Code:         "multi_level_approval",
		Name:         "Multi-Level Approval Workflow",
		Description:  "Approval workflow with multiple review levels",
		Version:      "1.0.0",
		InitialState: "draft",
		States: []byte(`[
			{"code": "draft", "name": "Draft", "description": "Initial draft state", "color": "gray", "is_final": false},
			{"code": "submitted", "name": "Submitted", "description": "Submitted for L1 review", "color": "blue", "is_final": false},
			{"code": "l1_approved", "name": "L1 Approved", "description": "Approved by L1 reviewer", "color": "yellow", "is_final": false},
			{"code": "l2_approved", "name": "L2 Approved", "description": "Approved by L2 reviewer", "color": "green", "is_final": true},
			{"code": "rejected", "name": "Rejected", "description": "Rejected", "color": "red", "is_final": true}
		]`),
		Transitions: []byte(`[
			{"from": "draft", "to": "submitted", "action": "submit", "label": "Submit", "required_permission": ""},
			{"from": "submitted", "to": "l1_approved", "action": "l1_approve", "label": "L1 Approve", "required_permission": "workflow:l1_approve"},
			{"from": "submitted", "to": "rejected", "action": "reject", "label": "Reject", "required_permission": "workflow:l1_approve"},
			{"from": "l1_approved", "to": "l2_approved", "action": "l2_approve", "label": "L2 Approve", "required_permission": "workflow:l2_approve"},
			{"from": "l1_approved", "to": "rejected", "action": "reject", "label": "Reject", "required_permission": "workflow:l2_approve"},
			{"from": "rejected", "to": "draft", "action": "revise", "label": "Revise", "required_permission": ""}
		]`),
		IsActive: true,
	}

	// Simple Task Workflow
	taskWorkflow := models.WorkflowDefinition{
		Code:         "simple_task",
		Name:         "Simple Task Workflow",
		Description:  "Basic task workflow: Open -> In Progress -> Completed",
		Version:      "1.0.0",
		InitialState: "open",
		States: []byte(`[
			{"code": "open", "name": "Open", "description": "Task is open", "color": "gray", "is_final": false},
			{"code": "in_progress", "name": "In Progress", "description": "Task is being worked on", "color": "blue", "is_final": false},
			{"code": "completed", "name": "Completed", "description": "Task is completed", "color": "green", "is_final": true},
			{"code": "cancelled", "name": "Cancelled", "description": "Task was cancelled", "color": "red", "is_final": true}
		]`),
		Transitions: []byte(`[
			{"from": "open", "to": "in_progress", "action": "start", "label": "Start Work", "required_permission": ""},
			{"from": "in_progress", "to": "completed", "action": "complete", "label": "Mark Complete", "required_permission": ""},
			{"from": "open", "to": "cancelled", "action": "cancel", "label": "Cancel", "required_permission": ""},
			{"from": "in_progress", "to": "cancelled", "action": "cancel", "label": "Cancel", "required_permission": ""}
		]`),
		IsActive: true,
	}

	workflows := []models.WorkflowDefinition{approvalWorkflow, multiLevelWorkflow, taskWorkflow}

	log.Printf("Attempting to seed %d workflows...", len(workflows))

	for _, wf := range workflows {
		log.Printf("Processing workflow: %s (code: %s)", wf.Name, wf.Code)

		var existing models.WorkflowDefinition
		err := DB.Where("code = ?", wf.Code).First(&existing).Error
		if err != nil {
			log.Printf("Workflow %s not found, creating new one...", wf.Code)
			result := DB.Create(&wf)
			if result.Error != nil {
				log.Printf("❌ Error creating workflow %s: %v", wf.Name, result.Error)
			} else {
				log.Printf("✅ Created workflow: %s (%s) - ID: %s", wf.Name, wf.Code, wf.ID)
			}
		} else {
			log.Printf("⏭️ Workflow already exists: %s (ID: %s)", wf.Name, existing.ID)
		}
	}

	// Verify count after seeding
	var count int64
	DB.Model(&models.WorkflowDefinition{}).Count(&count)
	log.Printf("Total workflows in database after seeding: %d", count)

	log.Println("Workflow seeding completed")
}

// =====================================================
// Sites Seeding
// =====================================================

// SeedSites creates default sites for each business vertical
func SeedSites() {
	log.Println("Seeding default sites...")

	var waterBusiness, solarBusiness models.BusinessVertical

	if err := DB.Where("code = ?", "WATER").First(&waterBusiness).Error; err != nil {
		log.Printf("Water Works business vertical not found: %v", err)
	} else {
		seedWaterSites(waterBusiness.ID)
	}

	if err := DB.Where("code = ?", "SOLAR").First(&solarBusiness).Error; err != nil {
		log.Printf("Solar Works business vertical not found: %v", err)
	} else {
		seedSolarSites(solarBusiness.ID)
	}

	log.Println("Site seeding completed")
}

func seedWaterSites(businessVerticalID uuid.UUID) {
	waterSites := []models.Site{
		{Name: "Ramanagara", Code: "RAMANAGARA", Description: "Ramanagara water distribution site", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Magadi", Code: "MAGADI", Description: "Magadi water distribution site", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "VG Doddi", Code: "VG_DODDI", Description: "VG Doddi water distribution site", BusinessVerticalID: businessVerticalID, IsActive: true},
		{Name: "Mallipatna", Code: "MALLIPATNA", Description: "Mallipatna water distribution site", BusinessVerticalID: businessVerticalID, IsActive: true},
	}

	for _, site := range waterSites {
		var existing models.Site
		err := DB.Where("code = ?", site.Code).First(&existing).Error
		if err != nil {
			if err := DB.Create(&site).Error; err != nil {
				log.Printf("Error creating site %s: %v", site.Name, err)
			} else {
				log.Printf("Created site: %s", site.Name)
			}
		}
	}
}

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
			if err := DB.Create(&site).Error; err != nil {
				log.Printf("Error creating site %s: %v", site.Name, err)
			} else {
				log.Printf("Created site: %s", site.Name)
			}
		}
	}
}

// =====================================================
// ABAC Seeding
// =====================================================

// SeedABACAttributes seeds default system attributes
func SeedABACAttributes(db *gorm.DB) error {
	attributes := []models.Attribute{
		// User Attributes
		{Name: "user.department", DisplayName: "User Department", Description: "Department the user belongs to", Type: models.AttributeTypeUser, DataType: models.DataTypeString, IsSystem: true, IsActive: true},
		{Name: "user.clearance_level", DisplayName: "Security Clearance Level", Description: "Security clearance level (1-5)", Type: models.AttributeTypeUser, DataType: models.DataTypeInteger, IsSystem: true, IsActive: true},
		{Name: "user.employment_type", DisplayName: "Employment Type", Description: "Type of employment", Type: models.AttributeTypeUser, DataType: models.DataTypeString, IsSystem: true, IsActive: true},
		{Name: "user.location", DisplayName: "User Location", Description: "Geographic location of the user", Type: models.AttributeTypeUser, DataType: models.DataTypeString, IsSystem: true, IsActive: true},

		// Resource Attributes
		{Name: "resource.sensitivity", DisplayName: "Data Sensitivity", Description: "Sensitivity level of the resource", Type: models.AttributeTypeResource, DataType: models.DataTypeString, IsSystem: true, IsActive: true},
		{Name: "resource.owner_id", DisplayName: "Resource Owner", Description: "UUID of the user who owns this resource", Type: models.AttributeTypeResource, DataType: models.DataTypeString, IsSystem: true, IsActive: true},
		{Name: "resource.project_id", DisplayName: "Associated Project", Description: "UUID of the project this resource belongs to", Type: models.AttributeTypeResource, DataType: models.DataTypeString, IsSystem: true, IsActive: true},

		// Environment Attributes
		{Name: "environment.time_of_day", DisplayName: "Time of Day", Description: "Current time in HH:MM format", Type: models.AttributeTypeEnvironment, DataType: models.DataTypeString, IsSystem: true, IsActive: true},
		{Name: "environment.day_of_week", DisplayName: "Day of Week", Description: "Current day of week", Type: models.AttributeTypeEnvironment, DataType: models.DataTypeString, IsSystem: true, IsActive: true},
		{Name: "environment.ip_address", DisplayName: "IP Address", Description: "IP address of the request", Type: models.AttributeTypeEnvironment, DataType: models.DataTypeString, IsSystem: true, IsActive: true},

		// Action Attributes
		{Name: "action.operation_type", DisplayName: "Operation Type", Description: "Type of operation being performed", Type: models.AttributeTypeAction, DataType: models.DataTypeString, IsSystem: true, IsActive: true},
		{Name: "action.risk_level", DisplayName: "Action Risk Level", Description: "Risk level of the action", Type: models.AttributeTypeAction, DataType: models.DataTypeString, IsSystem: true, IsActive: true},
	}

	for _, attr := range attributes {
		var existing models.Attribute
		result := db.Where("name = ?", attr.Name).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			if err := db.Create(&attr).Error; err != nil {
				return fmt.Errorf("failed to create attribute %s: %v", attr.Name, err)
			}
			log.Printf("Created attribute: %s", attr.Name)
		}
	}

	return nil
}

// RunABACSeeding runs all ABAC seeding functions
func RunABACSeeding(db *gorm.DB) error {
	log.Println("Seeding ABAC Attributes...")
	if err := SeedABACAttributes(db); err != nil {
		return fmt.Errorf("failed to seed attributes: %v", err)
	}
	log.Println("ABAC seeding completed")
	return nil
}

// =====================================================
// RBAC Migration & Verification
// =====================================================

// MigrateToNewRBAC migrates existing role data to new RBAC system
func MigrateToNewRBAC() {
	log.Printf("Starting RBAC migration...")

	var users []models.User
	DB.Find(&users)
	log.Printf("Found %d users to migrate", len(users))

	var waterVertical, solarVertical, hoVertical models.BusinessVertical
	DB.Where("code = ?", "WATER").First(&waterVertical)
	DB.Where("code = ?", "SOLAR").First(&solarVertical)
	DB.Where("code = ?", "HO").First(&hoVertical)

	if waterVertical.ID == uuid.Nil {
		log.Printf("Water vertical not found - run SeedBusinessVerticals first")
		return
	}

	migratedCount := 0
	for _, user := range users {
		if user.RoleID != nil {
			var role models.Role
			if err := DB.First(&role, "id = ?", user.RoleID).Error; err == nil {
				if role.Name == "super_admin" {
					migratedCount++
					continue
				}
			}
		}

		var ubrs []models.UserBusinessRole
		if err := DB.Where("user_id = ? AND is_active = ?", user.ID, true).Find(&ubrs).Error; err == nil {
			if len(ubrs) > 0 {
				migratedCount++
				continue
			}
		}

		log.Printf("User %s has no roles assigned", user.Name)
	}

	log.Printf("RBAC migration completed - migrated %d/%d users", migratedCount, len(users))
}

// VerifyRBACMigration checks if migration was successful
func VerifyRBACMigration() {
	log.Printf("Verifying RBAC migration...")

	var usersWithGlobalRole int64
	DB.Model(&models.User{}).Where("role_id IS NOT NULL").Count(&usersWithGlobalRole)
	log.Printf("Users with global roles: %d", usersWithGlobalRole)

	var businessRoleAssignments int64
	DB.Model(&models.UserBusinessRole{}).Where("is_active = ?", true).Count(&businessRoleAssignments)
	log.Printf("Active business role assignments: %d", businessRoleAssignments)

	log.Printf("RBAC verification completed")
}

// =====================================================
// User Seeding
// =====================================================

// SeedUsers creates default users including super admin and vertical-specific users
func SeedUsers() {
	log.Println("Seeding default users...")

	// Get the super_admin role
	var superAdminRole models.Role
	if err := DB.Where("name = ?", "super_admin").First(&superAdminRole).Error; err != nil {
		log.Printf("Error: super_admin role not found. Run SeedPermissions first: %v", err)
		return
	}

	// Get business verticals
	var waterVertical, solarVertical, hoVertical models.BusinessVertical
	DB.Where("code = ?", "WATER").First(&waterVertical)
	DB.Where("code = ?", "SOLAR").First(&solarVertical)
	DB.Where("code = ?", "HO").First(&hoVertical)

	// Get business roles for each vertical
	var waterAdminRole, solarAdminRole, hoAdminRole models.BusinessRole
	DB.Where("name = ? AND business_vertical_id = ?", "Water_Admin", waterVertical.ID).First(&waterAdminRole)
	DB.Where("name = ? AND business_vertical_id = ?", "Solar_Admin", solarVertical.ID).First(&solarAdminRole)
	DB.Where("name = ? AND business_vertical_id = ?", "HO_Admin", hoVertical.ID).First(&hoAdminRole)

	// Default password for all seeded users (should be changed on first login)
	defaultPassword := "Welcome@123"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return
	}

	// Define users to seed
	usersToSeed := []struct {
		Name               string
		Email              string
		Phone              string
		RoleID             *uuid.UUID        // Global role (for super admin)
		BusinessVerticalID *uuid.UUID        // Primary business vertical
		BusinessRoleID     *uuid.UUID        // Business-specific role
		Description        string
	}{
		// Super Admin - has access to everything
		{
			Name:        "Super Admin",
			Email:       "admin@ugcl.com",
			Phone:       "9999999999",
			RoleID:      &superAdminRole.ID,
			Description: "Super Administrator with full system access",
		},
		// Water Works Admin
		{
			Name:               "Water Admin",
			Email:              "water.admin@ugcl.com",
			Phone:              "9999999901",
			BusinessVerticalID: &waterVertical.ID,
			BusinessRoleID:     &waterAdminRole.ID,
			Description:        "Water Works Administrator",
		},
		// Water Works Engineer
		{
			Name:               "Water Engineer",
			Email:              "water.engineer@ugcl.com",
			Phone:              "9999999902",
			BusinessVerticalID: &waterVertical.ID,
			Description:        "Water Works Engineer",
		},
		// Solar Works Admin
		{
			Name:               "Solar Admin",
			Email:              "solar.admin@ugcl.com",
			Phone:              "9999999903",
			BusinessVerticalID: &solarVertical.ID,
			BusinessRoleID:     &solarAdminRole.ID,
			Description:        "Solar Works Administrator",
		},
		// Solar Works Engineer
		{
			Name:               "Solar Engineer",
			Email:              "solar.engineer@ugcl.com",
			Phone:              "9999999904",
			BusinessVerticalID: &solarVertical.ID,
			Description:        "Solar Works Engineer",
		},
		// Head Office Admin
		{
			Name:               "HO Admin",
			Email:              "ho.admin@ugcl.com",
			Phone:              "9999999905",
			BusinessVerticalID: &hoVertical.ID,
			BusinessRoleID:     &hoAdminRole.ID,
			Description:        "Head Office Administrator",
		},
	}

	for _, userData := range usersToSeed {
		var existingUser models.User
		err := DB.Where("email = ?", userData.Email).First(&existingUser).Error

		if err == nil {
			log.Printf("User already exists: %s (%s)", userData.Name, userData.Email)
			continue
		}

		// Create the user
		user := models.User{
			Name:               userData.Name,
			Email:              userData.Email,
			Phone:              userData.Phone,
			PasswordHash:       string(passwordHash),
			RoleID:             userData.RoleID,
			BusinessVerticalID: userData.BusinessVerticalID,
			IsActive:           true,
		}

		if err := DB.Create(&user).Error; err != nil {
			log.Printf("Error creating user %s: %v", userData.Name, err)
			continue
		}

		log.Printf("Created user: %s (%s) - %s", userData.Name, userData.Email, userData.Description)

		// Assign business role if specified
		if userData.BusinessRoleID != nil && *userData.BusinessRoleID != uuid.Nil {
			ubr := models.UserBusinessRole{
				UserID:         user.ID,
				BusinessRoleID: *userData.BusinessRoleID,
				IsActive:       true,
				AssignedAt:     time.Now(),
			}

			if err := DB.Create(&ubr).Error; err != nil {
				log.Printf("Error assigning business role to %s: %v", userData.Name, err)
			} else {
				log.Printf("  -> Assigned business role to %s", userData.Name)
			}
		}

		// For engineers, assign them the Engineer role in their vertical
		if userData.BusinessRoleID == nil && userData.BusinessVerticalID != nil {
			var engineerRole models.BusinessRole
			roleName := ""

			if *userData.BusinessVerticalID == waterVertical.ID {
				roleName = "Engineer"
			} else if *userData.BusinessVerticalID == solarVertical.ID {
				roleName = "Sr_Engineer"
			}

			if roleName != "" {
				if err := DB.Where("name = ? AND business_vertical_id = ?", roleName, *userData.BusinessVerticalID).First(&engineerRole).Error; err == nil {
					ubr := models.UserBusinessRole{
						UserID:         user.ID,
						BusinessRoleID: engineerRole.ID,
						IsActive:       true,
						AssignedAt:     time.Now(),
					}

					if err := DB.Create(&ubr).Error; err != nil {
						log.Printf("Error assigning engineer role to %s: %v", userData.Name, err)
					} else {
						log.Printf("  -> Assigned %s role to %s", roleName, userData.Name)
					}
				}
			}
		}
	}

	log.Println("User seeding completed")
	log.Println("========================================")
	log.Println("DEFAULT CREDENTIALS (change immediately!):")
	log.Println("----------------------------------------")
	log.Println("Super Admin:    admin@ugcl.com / Welcome@123")
	log.Println("Water Admin:    water.admin@ugcl.com / Welcome@123")
	log.Println("Water Engineer: water.engineer@ugcl.com / Welcome@123")
	log.Println("Solar Admin:    solar.admin@ugcl.com / Welcome@123")
	log.Println("Solar Engineer: solar.engineer@ugcl.com / Welcome@123")
	log.Println("HO Admin:       ho.admin@ugcl.com / Welcome@123")
	log.Println("========================================")
}
