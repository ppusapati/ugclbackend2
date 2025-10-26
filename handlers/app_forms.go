package handlers

import (
	"encoding/json"
	"log"
	"net/http"

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

	if verticalCode == "" || formCode == "" {
		http.Error(w, "vertical code and form code are required", http.StatusBadRequest)
		return
	}

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
		Where("accessible_verticals @> ?", `["`+verticalCode+`"]`).
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

	// Check if user is admin
	var user models.User
	if err := config.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	if !user.HasPermission("admin_all") {
		http.Error(w, "forbidden - admin access required", http.StatusForbidden)
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

	if !user.HasPermission("admin_all") {
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

	if !user.HasPermission("admin_all") {
		http.Error(w, "forbidden - admin access required", http.StatusForbidden)
		return
	}

	var form models.AppForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	form.CreatedBy = claims.UserID

	if err := config.DB.Create(&form).Error; err != nil {
		log.Printf("❌ Error creating form: %v", err)
		http.Error(w, "failed to create form", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Created new form: %s", form.Code)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "form created successfully",
		"form":    form.ToDTO(),
	})
}
