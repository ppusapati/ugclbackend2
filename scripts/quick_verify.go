package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type RolePermissionJoin struct {
	RoleID         string `gorm:"column:role_id"`
	RoleName       string `gorm:"column:role_name"`
	PermissionID   string `gorm:"column:permission_id"`
	PermissionName string `gorm:"column:permission_name"`
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	dsn := os.Getenv("DB_DSN")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	fmt.Println("========================================")
	fmt.Println("VERIFICATION: Consultant Role Permissions")
	fmt.Println("========================================\n")

	// Query the Consultant role and its permissions
	var results []RolePermissionJoin
	query := `
		SELECT
			r.id as role_id,
			r.name as role_name,
			p.id as permission_id,
			p.name as permission_name
		FROM roles r
		LEFT JOIN role_permissions rp ON r.id = rp.role_id
		LEFT JOIN permissions p ON rp.permission_id = p.id
		WHERE r.name = 'Consultant'
		ORDER BY p.name
	`

	if err := db.Raw(query).Scan(&results).Error; err != nil {
		log.Fatal("Query failed:", err)
	}

	if len(results) == 0 {
		fmt.Println("❌ No Consultant role found or no permissions assigned!")
		return
	}

	fmt.Printf("Role: %s (ID: %s)\n\n", results[0].RoleName, results[0].RoleID)
	fmt.Println("Assigned Permissions:")
	fmt.Println("---------------------")

	foundPlanningUpdate := false
	for _, r := range results {
		if r.PermissionName != "" {
			status := "✅"
			if r.PermissionName == "planning:update" {
				status = "🎯"
				foundPlanningUpdate = true
			}
			fmt.Printf("%s %s (ID: %s)\n", status, r.PermissionName, r.PermissionID)
		}
	}

	fmt.Printf("\nTotal permissions assigned: %d\n", len(results))

	if foundPlanningUpdate {
		fmt.Println("\n🎉 SUCCESS: planning:update permission is correctly assigned to Consultant role!")
	} else {
		fmt.Println("\n❌ PROBLEM: planning:update permission is NOT assigned to Consultant role!")
	}

	fmt.Println("\n========================================")
	fmt.Println("Expected Permissions for Consultant:")
	fmt.Println("- project:read")
	fmt.Println("- project:update")
	fmt.Println("- planning:read")
	fmt.Println("- planning:update")
	fmt.Println("========================================")
}
