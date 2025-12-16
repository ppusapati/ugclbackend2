package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// GetFormsForVertical returns all forms accessible in a specific business vertical
// GET /api/v1/business/{vertical}/forms
func GetFormsForVertical(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	verticalCode := vars["businessCode"]

	if verticalCode == "" {
		http.Error(w, "vertical code is required", http.StatusBadRequest)
		return
	}

	log.Printf("📋 Fetching forms for vertical: %s, user: %s", verticalCode, claims.UserID)

	// Get the user to check permissions
	var user models.User
	if err := config.DB.Preload("RoleModel.Permissions").
		First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Get forms for this vertical using JSONB contains operator
	var forms []models.AppForm
	query := config.DB.
		Preload("Module").
		Where("is_active = ?", true).
		Where("accessible_verticals @> ?", `["`+verticalCode+`"]`).
		Order("display_order ASC, title ASC")

	if err := query.Find(&forms).Error; err != nil {
		log.Printf("❌ Error fetching forms: %v", err)
		http.Error(w, "failed to fetch forms", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Found %d forms for vertical %s", len(forms), verticalCode)

	// Convert to DTOs and filter by user permissions
	var formDTOs []models.AppFormDTO
	moduleMap := make(map[string][]models.AppFormDTO)

	for _, form := range forms {
		// Check if user has required permission
		if form.RequiredPermission != "" && !user.HasPermission(form.RequiredPermission) {
			log.Printf("   ⊘ Skipping form %s - user lacks permission %s", form.Code, form.RequiredPermission)
			continue
		}

		dto := form.ToDTO()
		formDTOs = append(formDTOs, dto)

		// Group by module
		moduleCode := dto.Module
		if moduleCode != "" {
			moduleMap[moduleCode] = append(moduleMap[moduleCode], dto)
		}
	}

	log.Printf("✅ Returning %d forms after permission filtering", len(formDTOs))

	// Get modules for response
	var modules []models.Module
	config.DB.Where("is_active = ?", true).Order("display_order ASC").Find(&modules)

	response := map[string]interface{}{
		"forms":   formDTOs,
		"modules": moduleMap,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetFormByCode returns a specific form by its code with full schema
// GET /api/v1/business/{vertical}/forms/{code}
func GetFormByCode(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	verticalCode := vars["businessCode"]
	formCode := vars["code"]

	// if verticalCode == "" || formCode == "" {
	// 	http.Error(w, "vertical code and form code are required", http.StatusBadRequest)
	// 	return
	// }

	log.Printf("📋 Fetching form: %s for vertical: %s", formCode, verticalCode)

	// Get the user to check permissions
	var user models.User
	if err := config.DB.Preload("RoleModel.Permissions").
		First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Get the form
	var form models.AppForm
	if err := config.DB.
		Preload("Module").
		Where("code = ? AND is_active = ?", formCode, true).
		// Where("accessible_verticals @> ?", `["`+verticalCode+`"]`).
		First(&form).Error; err != nil {
		log.Printf("❌ Form not found: %s", formCode)
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	// Check permission
	if form.RequiredPermission != "" && !user.HasPermission(form.RequiredPermission) {
		log.Printf("❌ User lacks permission %s for form %s", form.RequiredPermission, formCode)
		http.Error(w, "forbidden - insufficient permissions", http.StatusForbidden)
		return
	}

	// Return full form with schema
	response := form.ToDTOWithSchema()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetAllForms returns all forms in the system (admin only)
// GET /api/v1/admin/forms
func GetAllAppForms(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	fmt.Println("Hey buddy i am here")
	// Check if user is admin
	var user models.User
	if err := config.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	var forms []models.AppForm
	if err := config.DB.
		Preload("Module").
		Order("module_id ASC, display_order ASC").
		Find(&forms).Error; err != nil {
		http.Error(w, "failed to fetch forms", http.StatusInternalServerError)
		return
	}

	var formDTOs []models.AppFormDTO
	for _, form := range forms {
		formDTOs = append(formDTOs, form.ToDTO())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"forms": formDTOs,
		"count": len(formDTOs),
	})
}

// UpdateFormVerticalAccess updates which verticals have access to a form (admin only)
// POST /api/v1/admin/forms/{formCode}/verticals
func UpdateFormVerticalAccess(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is admin
	var user models.User
	if err := config.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	if !user.HasPermission("admin_all") || !user.HasPermission("super_admin") {
		http.Error(w, "forbidden - admin access required", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	var requestBody struct {
		VerticalCodes []string `json:"vertical_codes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get the form
	var form models.AppForm
	if err := config.DB.Where("code = ?", formCode).First(&form).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	// Update accessible verticals
	form.AccessibleVerticals = requestBody.VerticalCodes
	if err := config.DB.Save(&form).Error; err != nil {
		log.Printf("❌ Error updating form: %v", err)
		http.Error(w, "failed to update form", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated form %s vertical access to: %v", formCode, requestBody.VerticalCodes)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":          "form vertical access updated successfully",
		"form":             formCode,
		"vertical_codes":   requestBody.VerticalCodes,
		"accessible_count": len(requestBody.VerticalCodes),
	})
}

// CreateForm creates a new form (admin only)
// POST /api/v1/admin/forms
func CreateForm(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	fmt.Println(claims)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is admin
	var user models.User
	if err := config.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	var form models.AppForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		fmt.Println(err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	form.CreatedBy = claims.UserID

	// Get the module to retrieve its schema name
	var module models.Module
	if err := config.DB.First(&module, "id = ?", form.ModuleID).Error; err != nil {
		log.Printf("❌ Module not found for form %s: %v", form.Code, err)
		http.Error(w, "module not found", http.StatusBadRequest)
		return
	}

	// Generate table name if not provided
	if form.DBTableName == "" {
		// Generate table name from form code (sanitized)
		form.DBTableName = generateTableName(form.Code)
	}

	// Create form record in database first
	if err := config.DB.Create(&form).Error; err != nil {
		log.Printf("❌ Error creating form: %v", err)
		http.Error(w, "failed to create form", http.StatusInternalServerError)
		return
	}

	// Create dedicated table for the form in the module's schema
	var schemaName string
	var tableCreated bool
	if module.SchemaName != "" {
		formTableManager := NewFormTableManager()
		if err := formTableManager.CreateFormTableInSchema(&form, module.SchemaName); err != nil {
			log.Printf("⚠️  Warning: Failed to create dedicated table for form %s in schema %s: %v", form.Code, module.SchemaName, err)
			// Don't fail the request - the form is created, table creation is optional
		} else {
			schemaName = module.SchemaName
			tableCreated = true
			log.Printf("✅ Created dedicated table %s.%s for form %s", module.SchemaName, form.DBTableName, form.Code)
		}
	}

	log.Printf("✅ Created new form: %s", form.Code)

	response := map[string]interface{}{
		"message": "form created successfully",
		"form":    form.ToDTO(),
	}

	if tableCreated {
		response["schema_name"] = schemaName
		response["table_name"] = form.DBTableName
		response["full_table_name"] = fmt.Sprintf("%s.%s", schemaName, form.DBTableName)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// generateTableName generates a valid PostgreSQL table name from form code
func generateTableName(formCode string) string {
	// Convert to lowercase
	name := strings.ToLower(formCode)

	// Replace spaces and hyphens with underscores
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")

	// Remove any characters that are not letters, digits, or underscores
	var result strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			result.WriteRune(c)
		}
	}
	name = result.String()

	// Ensure it starts with a letter or underscore (prefix with underscore if starts with digit)
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}

	// Limit length (PostgreSQL identifier limit is 63 bytes)
	if len(name) > 63 {
		name = name[:63]
	}

	return name
}

// UpdateForm updates an existing form (admin only)
// PUT /api/v1/admin/app-forms/{formCode}
func UpdateForm(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is admin
	var user models.User
	if err := config.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	// Get existing form
	var existingForm models.AppForm
	if err := config.DB.Where("code = ?", formCode).First(&existingForm).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	// Parse update request
	var updateData models.AppForm
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		log.Printf("❌ Error decoding update request for form %s: %v", formCode, err)
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("📝 Updating form: %s, title=%s, description=%s", formCode, updateData.Title, updateData.Description)

	// Update allowed fields
	if updateData.Title != "" {
		existingForm.Title = updateData.Title
	}
	if updateData.Description != "" {
		existingForm.Description = updateData.Description
	}
	if updateData.ModuleID != uuid.Nil {
		existingForm.ModuleID = updateData.ModuleID
	}
	if len(updateData.FormSchema) > 0 {
		existingForm.FormSchema = updateData.FormSchema
	}
	if len(updateData.Steps) > 0 {
		existingForm.Steps = updateData.Steps
	}
	if len(updateData.CoreFields) > 0 {
		existingForm.CoreFields = updateData.CoreFields
	}
	if len(updateData.Validations) > 0 {
		existingForm.Validations = updateData.Validations
	}
	if len(updateData.Dependencies) > 0 {
		existingForm.Dependencies = updateData.Dependencies
	}
	if updateData.WorkflowID != nil {
		existingForm.WorkflowID = updateData.WorkflowID
	}
	if updateData.InitialState != "" {
		existingForm.InitialState = updateData.InitialState
	}
	if updateData.RequiredPermission != "" {
		existingForm.RequiredPermission = updateData.RequiredPermission
	}
	if updateData.DisplayOrder > 0 {
		existingForm.DisplayOrder = updateData.DisplayOrder
	}
	if len(updateData.AccessibleVerticals) > 0 {
		existingForm.AccessibleVerticals = updateData.AccessibleVerticals
	}
	if updateData.DBTableName != "" {
		existingForm.DBTableName = updateData.DBTableName
	}

	// Save updates
	if err := config.DB.Save(&existingForm).Error; err != nil {
		log.Printf("❌ Error updating form: %v", err)
		http.Error(w, "failed to update form", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated form: %s", formCode)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "form updated successfully",
		"form":    existingForm.ToDTO(),
	})
}
