package masters

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// GetModules returns all modules filtered by user's business vertical and permissions
// GET /api/v1/modules
// Query params:
//   - vertical: (optional) filter by specific vertical code
//   - all: (optional) if "true", returns all modules for super admin (grouped by vertical)
func GetModules(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user with roles and permissions
	var user models.User
	if err := config.DB.Preload("RoleModel.Permissions").
		Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
		Preload("UserBusinessRoles.BusinessRole.Permissions").
		First(&user, "id = ?", claims.UserID).Error; err != nil {
		log.Printf("Error fetching user: %v", err)
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Check if user is super admin
	isSuperAdmin := user.HasPermission("admin_all") || user.HasPermission("super_admin") || user.HasPermission("*:*:*")

	// Get query parameters
	verticalCode := r.URL.Query().Get("vertical")
	showAll := r.URL.Query().Get("all") == "true"

	var modules []models.Module
	if err := config.DB.
		Where("is_active = ?", true).
		Order("display_order ASC, name ASC").
		Find(&modules).Error; err != nil {
		log.Printf("Error fetching modules: %v", err)
		http.Error(w, "failed to fetch modules", http.StatusInternalServerError)
		return
	}

	// If super admin and requesting all modules grouped by vertical
	if isSuperAdmin && showAll {
		// Get all business verticals
		var verticals []models.BusinessVertical
		config.DB.Where("is_active = ?", true).Order("name ASC").Find(&verticals)

		// Group modules by vertical
		verticalModules := make(map[string][]models.Module)
		for _, vertical := range verticals {
			var filteredModules []models.Module
			for _, module := range modules {
				if module.IsAccessibleInVertical(vertical.Code) {
					filteredModules = append(filteredModules, module)
				}
			}
			if len(filteredModules) > 0 {
				verticalModules[vertical.Code] = filteredModules
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_super_admin":      true,
			"verticals":           verticals,
			"modules_by_vertical": verticalModules,
			"all_modules":         modules,
			"count":               len(modules),
		})
		return
	}

	// Get user's accessible verticals
	userVerticals := getUserVerticalCodes(&user)

	// If specific vertical requested, use that; otherwise use user's verticals
	var targetVerticals []string
	if verticalCode != "" {
		// Check if user has access to requested vertical
		if !isSuperAdmin && !contains(userVerticals, verticalCode) {
			http.Error(w, "forbidden - no access to this vertical", http.StatusForbidden)
			return
		}
		targetVerticals = []string{verticalCode}
	} else {
		targetVerticals = userVerticals
	}

	// Filter modules by accessible verticals and permissions
	var filteredModules []models.Module
	for _, module := range modules {
		// Check if module is accessible in any of user's verticals
		accessible := false
		for _, vCode := range targetVerticals {
			if module.IsAccessibleInVertical(vCode) {
				accessible = true
				break
			}
		}

		if !accessible && !isSuperAdmin {
			continue
		}

		// Check if user has required permission (if specified)
		if module.RequiredPermission != "" && !isSuperAdmin {
			if !user.HasPermission(module.RequiredPermission) {
				continue
			}
		}

		filteredModules = append(filteredModules, module)
	}

	log.Printf("Returning %d modules for user %s (verticals: %v)", len(filteredModules), claims.UserID, targetVerticals)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"modules":        filteredModules,
		"count":          len(filteredModules),
		"user_verticals": targetVerticals,
		"is_super_admin": isSuperAdmin,
	})
}

// getUserVerticalCodes returns the list of vertical codes the user has access to
func getUserVerticalCodes(user *models.User) []string {
	verticalMap := make(map[string]bool)
	for _, ubr := range user.UserBusinessRoles {
		if ubr.IsActive && ubr.BusinessRole.BusinessVerticalID != uuid.Nil && ubr.BusinessRole.BusinessVertical.IsActive {
			verticalMap[ubr.BusinessRole.BusinessVertical.Code] = true
		}
	}
	var codes []string
	for code := range verticalMap {
		codes = append(codes, code)
	}
	return codes
}

// contains checks if a slice contains a specific string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func CreateModule(w http.ResponseWriter, r *http.Request) {
	log.Println("CreateModule called")
	claims := middleware.GetClaims(r)
	log.Println("Claims:", claims)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var module models.Module
	if err := json.NewDecoder(r.Body).Decode(&module); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	log.Println("Module to create:", module)

	// Generate schema name from module code
	schemaManager := handlers.NewSchemaManager()
	schemaName := schemaManager.GenerateSchemaName(module.Code)
	module.SchemaName = schemaName

	// Create database schema for this module
	if err := schemaManager.CreateSchema(schemaName); err != nil {
		log.Printf("Failed to create schema for module %s: %v", module.Code, err)
		http.Error(w, fmt.Sprintf("failed to create module schema: %v", err), http.StatusInternalServerError)
		return
	}

	// Create module record in database
	if err := config.DB.Create(&module).Error; err != nil {
		// Attempt to clean up the schema if module creation fails
		_ = schemaManager.DropSchema(schemaName, true)
		log.Printf("Failed to create module %s: %v", module.Code, err)
		http.Error(w, "failed to create module", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully created module %s with schema %s", module.Code, schemaName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"module":      module,
		"schema_name": schemaName,
		"message":     fmt.Sprintf("Module created with dedicated database schema: %s", schemaName),
	})
}
