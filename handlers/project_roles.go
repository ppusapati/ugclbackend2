package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// ProjectRoleHandler handles project role and permission management
type ProjectRoleHandler struct {
	db *gorm.DB
}

// NewProjectRoleHandler creates a new project role handler
func NewProjectRoleHandler() *ProjectRoleHandler {
	return &ProjectRoleHandler{
		db: config.DB,
	}
}

// CreateRoleRequest represents the request to create a role
type CreateRoleRequest struct {
	Code         string   `json:"code" binding:"required"`
	Name         string   `json:"name" binding:"required"`
	Description  string   `json:"description"`
	Permissions  []string `json:"permissions" binding:"required"`
	Level        int      `json:"level"`
	ParentRoleID *uuid.UUID `json:"parent_role_id"`
}

// UpdateRoleRequest represents the request to update a role
type UpdateRoleRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Permissions  []string `json:"permissions"`
	Level        int      `json:"level"`
	IsActive     *bool    `json:"is_active"`
}

// AssignRoleRequest represents the request to assign a role to a user
type AssignRoleRequest struct {
	UserID     string     `json:"user_id" binding:"required"`
	ProjectID  uuid.UUID  `json:"project_id" binding:"required"`
	RoleID     uuid.UUID  `json:"role_id" binding:"required"`
	ValidFrom  *time.Time `json:"valid_from"`
	ValidUntil *time.Time `json:"valid_until"`
	Notes      string     `json:"notes"`
}

// CreateRole creates a new project role
func (h *ProjectRoleHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get user ID from context
	claims := middleware.GetClaims(r)

	// Create role
	role := models.ProjectRole{
		Code:         req.Code,
		Name:         req.Name,
		Description:  req.Description,
		Permissions:  models.StringArray(req.Permissions),
		Level:        req.Level,
		ParentRoleID: req.ParentRoleID,
		IsActive:     true,
		IsSystemRole: false,
		CreatedBy:    claims.UserID,
	}

	if err := h.db.Create(&role).Error; err != nil {
		log.Printf("❌ Failed to create role: %v", err)
		http.Error(w, "Failed to create role", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Created project role: %s (ID: %s)", role.Name, role.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Role created successfully",
		"role":    role,
	})
}

// GetRole retrieves a role by ID
func (h *ProjectRoleHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roleID := vars["id"]

	var role models.ProjectRole
	if err := h.db.Preload("ParentRole").First(&role, "id = ?", roleID).Error; err != nil {
		http.Error(w, "Role not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(role)
}

// ListRoles lists all project roles
func (h *ProjectRoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	var roles []models.ProjectRole

	query := h.db.Preload("ParentRole")

	// Apply filters
	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		query = query.Where("is_active = ?", isActive == "true")
	}
	if isSystemRole := r.URL.Query().Get("is_system_role"); isSystemRole != "" {
		query = query.Where("is_system_role = ?", isSystemRole == "true")
	}

	if err := query.Order("level DESC, name ASC").Find(&roles).Error; err != nil {
		http.Error(w, "Failed to fetch roles", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"roles": roles,
		"count": len(roles),
	})
}

// UpdateRole updates a role
func (h *ProjectRoleHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roleID := vars["id"]

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var role models.ProjectRole
	if err := h.db.First(&role, "id = ?", roleID).Error; err != nil {
		http.Error(w, "Role not found", http.StatusNotFound)
		return
	}

	// Prevent modifying system roles
	if role.IsSystemRole {
		http.Error(w, "Cannot modify system roles", http.StatusForbidden)
		return
	}

	// Update fields
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}
	if len(req.Permissions) > 0 {
		role.Permissions = models.StringArray(req.Permissions)
	}
	if req.Level > 0 {
		role.Level = req.Level
	}
	if req.IsActive != nil {
		role.IsActive = *req.IsActive
	}

	if err := h.db.Save(&role).Error; err != nil {
		http.Error(w, "Failed to update role", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated project role: %s", roleID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Role updated successfully",
		"role":    role,
	})
}

// DeleteRole soft deletes a role
func (h *ProjectRoleHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roleID := vars["id"]

	var role models.ProjectRole
	if err := h.db.First(&role, "id = ?", roleID).Error; err != nil {
		http.Error(w, "Role not found", http.StatusNotFound)
		return
	}

	// Prevent deleting system roles
	if role.IsSystemRole {
		http.Error(w, "Cannot delete system roles", http.StatusForbidden)
		return
	}

	// Check if role is assigned to any users
	var count int64
	h.db.Model(&models.UserProjectRole{}).Where("role_id = ? AND is_active = ?", roleID, true).Count(&count)
	if count > 0 {
		http.Error(w, "Cannot delete role that is assigned to users", http.StatusBadRequest)
		return
	}

	// Soft delete
	role.IsActive = false
	if err := h.db.Save(&role).Error; err != nil {
		http.Error(w, "Failed to delete role", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Deleted project role: %s", roleID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Role deleted successfully",
	})
}

// AssignRoleToUser assigns a role to a user for a project
func (h *ProjectRoleHandler) AssignRoleToUser(w http.ResponseWriter, r *http.Request) {
	var req AssignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Verify project exists
	var project models.Project
	if err := h.db.First(&project, "id = ?", req.ProjectID).Error; err != nil {
		http.Error(w, "Project not found", http.StatusBadRequest)
		return
	}

	// Verify role exists and is active
	var role models.ProjectRole
	if err := h.db.First(&role, "id = ? AND is_active = ?", req.RoleID, true).Error; err != nil {
		http.Error(w, "Role not found or inactive", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	claims := middleware.GetClaims(r)

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Deactivate any existing active role for this user-project combination
	tx.Model(&models.UserProjectRole{}).
		Where("user_id = ? AND project_id = ? AND is_active = ?", req.UserID, req.ProjectID, true).
		Update("is_active", false)

	// Create new role assignment
	now := time.Now()
	assignment := models.UserProjectRole{
		UserID:     req.UserID,
		ProjectID:  req.ProjectID,
		RoleID:     req.RoleID,
		AssignedBy: claims.UserID,
		AssignedAt: now,
		ValidFrom:  req.ValidFrom,
		ValidUntil: req.ValidUntil,
		IsActive:   true,
		Notes:      req.Notes,
	}

	if err := tx.Create(&assignment).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to assign role: %v", err)
		http.Error(w, "Failed to assign role", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Assigned role %s to user %s for project %s", role.Name, req.UserID, req.ProjectID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Role assigned successfully",
		"assignment": assignment,
	})
}

// RevokeRoleFromUser revokes a user's role for a project
func (h *ProjectRoleHandler) RevokeRoleFromUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assignmentID := vars["id"]

	var assignment models.UserProjectRole
	if err := h.db.First(&assignment, "id = ?", assignmentID).Error; err != nil {
		http.Error(w, "Role assignment not found", http.StatusNotFound)
		return
	}

	// Deactivate the assignment
	assignment.IsActive = false
	if err := h.db.Save(&assignment).Error; err != nil {
		http.Error(w, "Failed to revoke role", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Revoked role assignment: %s", assignmentID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Role revoked successfully",
	})
}

// GetUserProjectRoles retrieves all roles for a user in a project
func (h *ProjectRoleHandler) GetUserProjectRoles(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	projectID := r.URL.Query().Get("project_id")

	if userID == "" || projectID == "" {
		http.Error(w, "user_id and project_id are required", http.StatusBadRequest)
		return
	}

	var assignments []models.UserProjectRole
	query := h.db.Preload("Project").Preload("Role")

	if err := query.Where("user_id = ? AND project_id = ? AND is_active = ?", userID, projectID, true).
		Find(&assignments).Error; err != nil {
		http.Error(w, "Failed to fetch role assignments", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"assignments": assignments,
		"count":       len(assignments),
	})
}

// GetProjectUsers retrieves all users assigned to a project
func (h *ProjectRoleHandler) GetProjectUsers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var assignments []models.UserProjectRole
	if err := h.db.Preload("Role").
		Where("project_id = ? AND is_active = ?", projectID, true).
		Find(&assignments).Error; err != nil {
		http.Error(w, "Failed to fetch project users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": assignments,
		"count": len(assignments),
	})
}

// CheckUserPermission checks if a user has a specific permission for a project
func (h *ProjectRoleHandler) CheckUserPermission(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	projectID := r.URL.Query().Get("project_id")
	permission := r.URL.Query().Get("permission")

	if userID == "" || projectID == "" || permission == "" {
		http.Error(w, "user_id, project_id, and permission are required", http.StatusBadRequest)
		return
	}

	// Get user's active role for the project
	var assignment models.UserProjectRole
	if err := h.db.Preload("Role").
		Where("user_id = ? AND project_id = ? AND is_active = ?", userID, projectID, true).
		First(&assignment).Error; err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"has_permission": false,
			"message":        "User has no active role in this project",
		})
		return
	}

	// Check if role has the permission
	hasPermission := false
	for _, perm := range assignment.Role.Permissions {
		if perm == permission || perm == "admin_all" || perm == "project:*" {
			hasPermission = true
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"has_permission": hasPermission,
		"role":           assignment.Role.Name,
		"role_level":     assignment.Role.Level,
	})
}

// GetAvailablePermissions returns the list of available project permissions
func (h *ProjectRoleHandler) GetAvailablePermissions(w http.ResponseWriter, r *http.Request) {
	permissions := []map[string]interface{}{
		{"code": "project:create", "name": "Create Projects", "description": "Create new projects"},
		{"code": "project:read", "name": "View Projects", "description": "View project details"},
		{"code": "project:update", "name": "Update Projects", "description": "Update project information"},
		{"code": "project:delete", "name": "Delete Projects", "description": "Delete projects"},

		{"code": "task:create", "name": "Create Tasks", "description": "Create new tasks"},
		{"code": "task:read", "name": "View Tasks", "description": "View task details"},
		{"code": "task:update", "name": "Update Tasks", "description": "Update task information"},
		{"code": "task:update:own", "name": "Update Own Tasks", "description": "Update only assigned tasks"},
		{"code": "task:delete", "name": "Delete Tasks", "description": "Delete tasks"},
		{"code": "task:assign", "name": "Assign Tasks", "description": "Assign tasks to users"},
		{"code": "task:assign:limited", "name": "Assign Tasks (Limited)", "description": "Assign tasks within scope"},
		{"code": "task:submit", "name": "Submit Tasks", "description": "Submit tasks for approval"},
		{"code": "task:approve", "name": "Approve Tasks", "description": "Approve or reject tasks"},
		{"code": "task:verify", "name": "Verify Tasks", "description": "Verify task completion"},
		{"code": "task:execute", "name": "Execute Tasks", "description": "Execute assigned tasks"},
		{"code": "task:comment", "name": "Comment on Tasks", "description": "Add comments to tasks"},

		{"code": "budget:view", "name": "View Budget", "description": "View budget information"},
		{"code": "budget:allocate", "name": "Allocate Budget", "description": "Allocate budget to tasks"},
		{"code": "budget:manage", "name": "Manage Budget", "description": "Full budget management access"},

		{"code": "user:assign", "name": "Assign Users", "description": "Assign users to projects and roles"},
		{"code": "user:remove", "name": "Remove Users", "description": "Remove users from projects"},

		{"code": "report:view", "name": "View Reports", "description": "View project reports"},
		{"code": "report:export", "name": "Export Reports", "description": "Export project data"},

		{"code": "admin_all", "name": "Full Admin Access", "description": "All permissions"},
		{"code": "project:*", "name": "All Project Permissions", "description": "All project-related permissions"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"permissions": permissions,
		"count":       len(permissions),
	})
}
