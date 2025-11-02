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

// UnifiedRoleResponse represents a role that could be either global or business-specific
type UnifiedRoleResponse struct {
	ID                 uuid.UUID            `json:"id"`
	Name               string               `json:"name"`
	DisplayName        string               `json:"display_name,omitempty"`
	Description        string               `json:"description"`
	Level              int                  `json:"level"`
	IsActive           bool                 `json:"is_active"`
	IsGlobal           bool                 `json:"is_global"`
	BusinessVerticalID *uuid.UUID           `json:"business_vertical_id,omitempty"`
	BusinessVertical   *BusinessVerticalInfo `json:"business_vertical,omitempty"`
	Permissions        []permissionResponse `json:"permissions"`
	UserCount          int64                `json:"user_count"`
}

type BusinessVerticalInfo struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

// GetAllRolesUnified returns both global roles and business roles in a single response
// Query params:
//   - include_business=true|false (default: true) - Include business roles
//   - business_vertical_id=uuid - Filter by specific vertical (optional)
func GetAllRolesUnified(w http.ResponseWriter, r *http.Request) {
	includeBusiness := r.URL.Query().Get("include_business") != "false" // Default true
	businessVerticalID := r.URL.Query().Get("business_vertical_id")

	var unifiedRoles []UnifiedRoleResponse

	// 1. Fetch Global Roles
	var globalRoles []models.Role
	if err := config.DB.Preload("Permissions").
		Where("is_active = ?", true).
		Order("level ASC").
		Find(&globalRoles).Error; err != nil {
		http.Error(w, "Failed to fetch global roles: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get user counts for global roles
	globalRoleUserCounts := make(map[uuid.UUID]int64)
	for _, role := range globalRoles {
		var count int64
		config.DB.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&count)
		globalRoleUserCounts[role.ID] = count
	}

	// Convert global roles to unified format
	for _, role := range globalRoles {
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

		unifiedRoles = append(unifiedRoles, UnifiedRoleResponse{
			ID:                 role.ID,
			Name:               role.Name,
			DisplayName:        role.Name, // Global roles don't have separate display name
			Description:        role.Description,
			Level:              role.Level,
			IsActive:           role.IsActive,
			IsGlobal:           true,
			BusinessVerticalID: nil,
			BusinessVertical:   nil,
			Permissions:        permissions,
			UserCount:          globalRoleUserCounts[role.ID],
		})
	}

	// 2. Fetch Business Roles (if requested)
	if includeBusiness {
		query := config.DB.Preload("Permissions").
			Preload("BusinessVertical").
			Where("is_active = ?", true)

		// Filter by specific vertical if provided
		if businessVerticalID != "" {
			if verticalUUID, err := uuid.Parse(businessVerticalID); err == nil {
				query = query.Where("business_vertical_id = ?", verticalUUID)
			}
		}

		var businessRoles []models.BusinessRole
		if err := query.Order("level ASC").Find(&businessRoles).Error; err != nil {
			http.Error(w, "Failed to fetch business roles: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get user counts for business roles
		businessRoleUserCounts := make(map[uuid.UUID]int64)
		if len(businessRoles) > 0 {
			roleIDs := make([]uuid.UUID, len(businessRoles))
			for i, r := range businessRoles {
				roleIDs[i] = r.ID
			}

			var roleUserCounts []struct {
				BusinessRoleID uuid.UUID
				Count          int64
			}
			config.DB.Model(&models.UserBusinessRole{}).
				Select("business_role_id, COUNT(*) as count").
				Where("business_role_id IN ? AND is_active = ?", roleIDs, true).
				Group("business_role_id").
				Scan(&roleUserCounts)

			for _, result := range roleUserCounts {
				businessRoleUserCounts[result.BusinessRoleID] = result.Count
			}
		}

		// Convert business roles to unified format
		for _, role := range businessRoles {
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

			verticalInfo := &BusinessVerticalInfo{
				ID:   role.BusinessVertical.ID,
				Code: role.BusinessVertical.Code,
				Name: role.BusinessVertical.Name,
			}

			unifiedRoles = append(unifiedRoles, UnifiedRoleResponse{
				ID:                 role.ID,
				Name:               role.Name,
				DisplayName:        role.DisplayName,
				Description:        role.Description,
				Level:              role.Level,
				IsActive:           role.IsActive,
				IsGlobal:           false,
				BusinessVerticalID: &role.BusinessVerticalID,
				BusinessVertical:   verticalInfo,
				Permissions:        permissions,
				UserCount:          businessRoleUserCounts[role.ID],
			})
		}
	}

	response := map[string]interface{}{
		"roles": unifiedRoles,
		"total": len(unifiedRoles),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
