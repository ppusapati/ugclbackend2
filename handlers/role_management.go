package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

type createRoleReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type roleResponse struct {
	ID          uuid.UUID            `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	IsActive    bool                 `json:"is_active"`
	Permissions []permissionResponse `json:"permissions"`
}

type permissionResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
}

// GetAllRoles returns all roles with their permissions
func GetAllRoles(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	limit := 10

	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	offset := (page - 1) * limit

	var roles []models.Role
	if err := config.DB.
		Preload("Permissions").
		Where("is_active = ?", true).
		Limit(limit).
		Offset(offset).
		Find(&roles).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var total int64
	if err := config.DB.
		Model(&models.Role{}).
		Where("is_active = ?", true).
		Count(&total).Error; err != nil {
		http.Error(w, "DB count error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	roleResponses := make([]roleResponse, len(roles))
	for i, role := range roles {
		permissions := make([]permissionResponse, len(role.Permissions))
		for j, perm := range role.Permissions {
			permissions[j] = permissionResponse{
				ID:          perm.ID,
				Name:        perm.Name,
				Description: perm.Description,
				Resource:    perm.Resource,
				Action:      perm.Action,
			}
		}

		roleResponses[i] = roleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			IsActive:    role.IsActive,
			Permissions: permissions,
		}
	}

	response := map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  roleResponses,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetAllPermissions returns all available permissions
func GetAllPermissions(w http.ResponseWriter, r *http.Request) {
	var permissions []models.Permission
	if err := config.DB.Find(&permissions).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	permResponses := make([]permissionResponse, len(permissions))
	for i, perm := range permissions {
		permResponses[i] = permissionResponse{
			ID:          perm.ID,
			Name:        perm.Name,
			Description: perm.Description,
			Resource:    perm.Resource,
			Action:      perm.Action,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permResponses)
}

func CreatePermission(w http.ResponseWriter, r *http.Request) {
	var perm models.Permission
	if err := json.NewDecoder(r.Body).Decode(&perm); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := config.DB.Create(&perm).Error; err != nil {
		http.Error(w, "failed to create permission: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perm)
}

// CreateRole creates a new role with specified permissions
func CreateRole(w http.ResponseWriter, r *http.Request) {
	var req createRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Create role
	role := models.Role{
		Name:        req.Name,
		Description: req.Description,
		IsActive:    true,
	}

	if err := config.DB.Create(&role).Error; err != nil {
		http.Error(w, "failed to create role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Assign permissions
	for _, permName := range req.Permissions {
		var permission models.Permission
		if err := config.DB.Where("name = ?", permName).First(&permission).Error; err != nil {
			continue // Skip invalid permissions
		}
		config.DB.Model(&role).Association("Permissions").Append(&permission)
	}

	// Load permissions for response
	config.DB.Preload("Permissions").First(&role, role.ID)

	// Convert to response format
	permissions := make([]permissionResponse, len(role.Permissions))
	for i, perm := range role.Permissions {
		permissions[i] = permissionResponse{
			ID:          perm.ID,
			Name:        perm.Name,
			Description: perm.Description,
			Resource:    perm.Resource,
			Action:      perm.Action,
		}
	}

	response := roleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		IsActive:    role.IsActive,
		Permissions: permissions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateRole updates an existing role
func UpdateRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roleID := vars["id"]

	id, err := uuid.Parse(roleID)
	if err != nil {
		http.Error(w, "invalid role ID", http.StatusBadRequest)
		return
	}

	var req createRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Get existing role
	var role models.Role
	if err := config.DB.Preload("Permissions").First(&role, "id = ?", id).Error; err != nil {
		http.Error(w, "role not found", http.StatusNotFound)
		return
	}

	// Update basic fields
	role.Name = req.Name
	role.Description = req.Description

	if err := config.DB.Save(&role).Error; err != nil {
		http.Error(w, "failed to update role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Clear existing permissions
	config.DB.Model(&role).Association("Permissions").Clear()

	// Assign new permissions
	for _, permName := range req.Permissions {
		var permission models.Permission
		if err := config.DB.Where("name = ?", permName).First(&permission).Error; err != nil {
			continue // Skip invalid permissions
		}
		config.DB.Model(&role).Association("Permissions").Append(&permission)
	}

	// Reload with permissions
	config.DB.Preload("Permissions").First(&role, role.ID)

	// Convert to response format
	permissions := make([]permissionResponse, len(role.Permissions))
	for i, perm := range role.Permissions {
		permissions[i] = permissionResponse{
			ID:          perm.ID,
			Name:        perm.Name,
			Description: perm.Description,
			Resource:    perm.Resource,
			Action:      perm.Action,
		}
	}

	response := roleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		IsActive:    role.IsActive,
		Permissions: permissions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteRole soft deletes a role
func DeleteRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roleID := vars["id"]

	id, err := uuid.Parse(roleID)
	if err != nil {
		http.Error(w, "invalid role ID", http.StatusBadRequest)
		return
	}

	var role models.Role
	if err := config.DB.First(&role, "id = ?", id).Error; err != nil {
		http.Error(w, "role not found", http.StatusNotFound)
		return
	}

	// Check if any users are using this role
	var userCount int64
	config.DB.Model(&models.User{}).Where("role_id = ?", id).Count(&userCount)
	if userCount > 0 {
		http.Error(w, "cannot delete role: users are assigned to this role", http.StatusBadRequest)
		return
	}

	// Soft delete
	role.IsActive = false
	if err := config.DB.Save(&role).Error; err != nil {
		http.Error(w, "failed to delete role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
