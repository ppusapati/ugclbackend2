package config

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// SeedABACAttributes seeds default system attributes
func SeedABACAttributes(db *gorm.DB) error {
	attributes := []models.Attribute{
		// User Attributes
		{
			Name:        "user.department",
			DisplayName: "User Department",
			Description: "Department the user belongs to",
			Type:        models.AttributeTypeUser,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
			Metadata: models.JSONMap{
				"allowed_values": []string{"engineering", "hr", "finance", "operations", "management"},
			},
		},
		{
			Name:        "user.clearance_level",
			DisplayName: "Security Clearance Level",
			Description: "Security clearance level of the user (1-5, higher is more access)",
			Type:        models.AttributeTypeUser,
			DataType:    models.DataTypeInteger,
			IsSystem:    true,
			IsActive:    true,
			Metadata: models.JSONMap{
				"min": 1,
				"max": 5,
			},
		},
		{
			Name:        "user.employment_type",
			DisplayName: "Employment Type",
			Description: "Type of employment (permanent, contract, consultant)",
			Type:        models.AttributeTypeUser,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
			Metadata: models.JSONMap{
				"allowed_values": []string{"permanent", "contract", "consultant", "intern"},
			},
		},
		{
			Name:        "user.location",
			DisplayName: "User Location",
			Description: "Geographic location of the user",
			Type:        models.AttributeTypeUser,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "user.manager_id",
			DisplayName: "Manager User ID",
			Description: "UUID of the user's manager",
			Type:        models.AttributeTypeUser,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "user.years_of_service",
			DisplayName: "Years of Service",
			Description: "Number of years the user has been employed",
			Type:        models.AttributeTypeUser,
			DataType:    models.DataTypeFloat,
			IsSystem:    true,
			IsActive:    true,
		},

		// Resource Attributes
		{
			Name:        "resource.sensitivity",
			DisplayName: "Data Sensitivity",
			Description: "Sensitivity level of the resource (public, internal, confidential, secret)",
			Type:        models.AttributeTypeResource,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
			Metadata: models.JSONMap{
				"allowed_values": []string{"public", "internal", "confidential", "secret"},
			},
		},
		{
			Name:        "resource.owner_id",
			DisplayName: "Resource Owner",
			Description: "UUID of the user who owns this resource",
			Type:        models.AttributeTypeResource,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "resource.project_id",
			DisplayName: "Associated Project",
			Description: "UUID of the project this resource belongs to",
			Type:        models.AttributeTypeResource,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "resource.cost_center",
			DisplayName: "Cost Center",
			Description: "Financial cost center for the resource",
			Type:        models.AttributeTypeResource,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "resource.amount",
			DisplayName: "Resource Amount/Value",
			Description: "Monetary value associated with the resource",
			Type:        models.AttributeTypeResource,
			DataType:    models.DataTypeFloat,
			IsSystem:    true,
			IsActive:    true,
		},

		// Environment Attributes
		{
			Name:        "environment.time_of_day",
			DisplayName: "Time of Day",
			Description: "Current time in HH:MM format",
			Type:        models.AttributeTypeEnvironment,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "environment.day_of_week",
			DisplayName: "Day of Week",
			Description: "Current day of week (Monday-Sunday)",
			Type:        models.AttributeTypeEnvironment,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "environment.ip_address",
			DisplayName: "IP Address",
			Description: "IP address of the request",
			Type:        models.AttributeTypeEnvironment,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "environment.location",
			DisplayName: "Request Location",
			Description: "Geographic location of the request",
			Type:        models.AttributeTypeEnvironment,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},
		{
			Name:        "environment.device_type",
			DisplayName: "Device Type",
			Description: "Type of device making the request (mobile, desktop, tablet)",
			Type:        models.AttributeTypeEnvironment,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
		},

		// Action Attributes
		{
			Name:        "action.operation_type",
			DisplayName: "Operation Type",
			Description: "Type of operation being performed (read, write, delete, approve)",
			Type:        models.AttributeTypeAction,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
			Metadata: models.JSONMap{
				"allowed_values": []string{"read", "create", "update", "delete", "approve", "reject"},
			},
		},
		{
			Name:        "action.risk_level",
			DisplayName: "Action Risk Level",
			Description: "Risk level of the action (low, medium, high, critical)",
			Type:        models.AttributeTypeAction,
			DataType:    models.DataTypeString,
			IsSystem:    true,
			IsActive:    true,
			Metadata: models.JSONMap{
				"allowed_values": []string{"low", "medium", "high", "critical"},
			},
		},
	}

	for _, attr := range attributes {
		var existing models.Attribute
		result := db.Where("name = ?", attr.Name).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			if err := db.Create(&attr).Error; err != nil {
				return fmt.Errorf("failed to create attribute %s: %v", attr.Name, err)
			}
			fmt.Printf("✓ Created attribute: %s\n", attr.Name)
		} else {
			fmt.Printf("→ Attribute already exists: %s\n", attr.Name)
		}
	}

	return nil
}

// SeedSamplePolicies creates sample policies to demonstrate ABAC capabilities
func SeedSamplePolicies(db *gorm.DB) error {
	// Get super admin user for created_by
	var superAdmin models.User
	if err := db.Where("email = ?", "admin@ugcl.com").First(&superAdmin).Error; err != nil {
		fmt.Println("Warning: Super admin not found, skipping policy seeding")
		return nil
	}

	policies := []models.Policy{
		{
			Name:        "restrict_high_value_purchases_after_hours",
			DisplayName: "Restrict High-Value Purchases After Hours",
			Description: "Deny purchases over ₹100,000 outside business hours (9 AM - 5 PM) except for super admins",
			Effect:      models.PolicyEffectDeny,
			Priority:    100,
			Status:      models.PolicyStatusActive,
			Actions:     models.JSONArray{"purchase:create", "purchase:approve"},
			Resources:   models.JSONArray{"purchase"},
			Conditions: models.JSONMap{
				"AND": []interface{}{
					map[string]interface{}{
						"attribute": "resource.amount",
						"operator":  ">",
						"value":     100000,
					},
					map[string]interface{}{
						"OR": []interface{}{
							map[string]interface{}{
								"attribute": "environment.hour",
								"operator":  "<",
								"value":     9,
							},
							map[string]interface{}{
								"attribute": "environment.hour",
								"operator":  ">=",
								"value":     17,
							},
						},
					},
					map[string]interface{}{
						"attribute": "user.role",
						"operator":  "!=",
						"value":     "super_admin",
					},
				},
			},
			ValidFrom: time.Now(),
			CreatedBy: superAdmin.ID,
		},
		{
			Name:        "allow_resource_owner_full_access",
			DisplayName: "Allow Resource Owner Full Access",
			Description: "Resource owners always have full access to their own resources",
			Effect:      models.PolicyEffectAllow,
			Priority:    90,
			Status:      models.PolicyStatusActive,
			Actions:     models.JSONArray{"*"},
			Resources:   models.JSONArray{"*"},
			Conditions: models.JSONMap{
				"AND": []interface{}{
					map[string]interface{}{
						"attribute": "user.id",
						"operator":  "=",
						"value":     "{{resource.owner_id}}",
					},
				},
			},
			ValidFrom: time.Now(),
			CreatedBy: superAdmin.ID,
		},
		{
			Name:        "restrict_confidential_data_by_clearance",
			DisplayName: "Restrict Confidential Data by Clearance Level",
			Description: "Deny access to confidential resources if user clearance level is below 3",
			Effect:      models.PolicyEffectDeny,
			Priority:    80,
			Status:      models.PolicyStatusActive,
			Actions:     models.JSONArray{"read", "update", "delete"},
			Resources:   models.JSONArray{"*"},
			Conditions: models.JSONMap{
				"AND": []interface{}{
					map[string]interface{}{
						"attribute": "resource.sensitivity",
						"operator":  "IN",
						"value":     []string{"confidential", "secret"},
					},
					map[string]interface{}{
						"attribute": "user.clearance_level",
						"operator":  "<",
						"value":     3,
					},
				},
			},
			ValidFrom: time.Now(),
			CreatedBy: superAdmin.ID,
		},
		{
			Name:        "allow_manager_approve_subordinate_requests",
			DisplayName: "Allow Manager to Approve Subordinate Requests",
			Description: "Managers can approve requests created by their subordinates",
			Effect:      models.PolicyEffectAllow,
			Priority:    70,
			Status:      models.PolicyStatusActive,
			Actions:     models.JSONArray{"approve", "reject"},
			Resources:   models.JSONArray{"purchase", "leave", "expense"},
			Conditions: models.JSONMap{
				"AND": []interface{}{
					map[string]interface{}{
						"attribute": "user.id",
						"operator":  "=",
						"value":     "{{resource.approver_id}}",
					},
					map[string]interface{}{
						"attribute": "user.id",
						"operator":  "=",
						"value":     "{{resource.creator_manager_id}}",
					},
				},
			},
			ValidFrom: time.Now(),
			CreatedBy: superAdmin.ID,
		},
		{
			Name:        "restrict_weekend_operations",
			DisplayName: "Restrict Weekend Operations",
			Description: "Deny critical operations on weekends except for super admins",
			Effect:      models.PolicyEffectDeny,
			Priority:    60,
			Status:      models.PolicyStatusDraft, // Draft for demonstration
			Actions:     models.JSONArray{"create", "update", "delete", "approve"},
			Resources:   models.JSONArray{"payment", "purchase", "finance"},
			Conditions: models.JSONMap{
				"AND": []interface{}{
					map[string]interface{}{
						"attribute": "environment.day_of_week",
						"operator":  "IN",
						"value":     []string{"Saturday", "Sunday"},
					},
					map[string]interface{}{
						"attribute": "user.role",
						"operator":  "!=",
						"value":     "super_admin",
					},
				},
			},
			ValidFrom: time.Now(),
			CreatedBy: superAdmin.ID,
		},
	}

	for _, policy := range policies {
		var existing models.Policy
		result := db.Where("name = ?", policy.Name).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			if err := db.Create(&policy).Error; err != nil {
				return fmt.Errorf("failed to create policy %s: %v", policy.Name, err)
			}
			fmt.Printf("✓ Created policy: %s\n", policy.DisplayName)
		} else {
			fmt.Printf("→ Policy already exists: %s\n", policy.DisplayName)
		}
	}

	return nil
}

// RunABACSeeding runs all ABAC seeding functions
func RunABACSeeding(db *gorm.DB) error {
	fmt.Println("\n🌱 Seeding ABAC Attributes...")
	if err := SeedABACAttributes(db); err != nil {
		return fmt.Errorf("failed to seed attributes: %v", err)
	}

	fmt.Println("\n📋 Seeding Sample Policies...")
	if err := SeedSamplePolicies(db); err != nil {
		return fmt.Errorf("failed to seed policies: %v", err)
	}

	fmt.Println("\n✅ ABAC seeding completed successfully!")
	return nil
}
