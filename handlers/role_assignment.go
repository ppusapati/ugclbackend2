package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// AssignBusinessRoleRequest represents the request to assign a business role to a user
type AssignBusinessRoleRequest struct {
	BusinessRoleID string `json:"business_role_id"`
	AssignedBy     string `json:"assigned_by"`
}

// AssignBusinessRole - POST /api/users/:id/roles/assign
// Validates:
// - Current user can assign this role (level check)
// - Target user doesn't already have role in this vertical
// - Business vertical exists and is active
func AssignBusinessRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// Parse request
	var req AssignBusinessRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get current user from context
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Load current user with all roles
	var currentUser models.User
	if err := config.DB.
		Preload("RoleModel").
		Preload("UserBusinessRoles.BusinessRole").
		First(&currentUser, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "Current user not found", http.StatusNotFound)
		return
	}

	// Load target business role
	businessRoleID, err := uuid.Parse(req.BusinessRoleID)
	if err != nil {
		http.Error(w, "Invalid business role ID", http.StatusBadRequest)
		return
	}

	var businessRole models.BusinessRole
	if err := config.DB.
		Preload("BusinessVertical").
		First(&businessRole, "id = ?", businessRoleID).Error; err != nil {
		http.Error(w, "Business role not found", http.StatusNotFound)
		return
	}

	// Check if current user can assign this role (level validation)
	if !currentUser.CanAssignRole(businessRole.Level) {
		http.Error(w, "You don't have permission to assign this role", http.StatusForbidden)
		return
	}

	// Parse target user ID
	targetUserID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check if user already has this role in this vertical
	var existingRole models.UserBusinessRole
	err = config.DB.
		Joins("JOIN business_roles ON business_roles.id = user_business_roles.business_role_id").
		Where("user_business_roles.user_id = ? AND business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?",
			targetUserID, businessRole.BusinessVerticalID, true).
		First(&existingRole).Error

	if err == nil {
		http.Error(w, "User already has a role in this vertical", http.StatusConflict)
		return
	}

	// Create user business role assignment
	assignedByID, _ := uuid.Parse(claims.UserID)
	userBusinessRole := models.UserBusinessRole{
		UserID:         targetUserID,
		BusinessRoleID: businessRoleID,
		IsActive:       true,
		AssignedAt:     time.Now(),
		AssignedBy:     &assignedByID,
	}

	if err := config.DB.Create(&userBusinessRole).Error; err != nil {
		http.Error(w, "Failed to assign role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Load the created record with relationships
	config.DB.
		Preload("BusinessRole.BusinessVertical").
		Preload("User").
		First(&userBusinessRole, "id = ?", userBusinessRole.ID)

	response := map[string]interface{}{
		"success":            true,
		"user_business_role": userBusinessRole,
		"message":            "Role assigned successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RemoveBusinessRole - DELETE /api/users/:id/roles/:roleId
// Validates:
// - Current user can remove this role (level check)
// - Target user has this role
func RemoveBusinessRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	roleID := vars["roleId"]

	// Get current user from context
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Load current user with all roles
	var currentUser models.User
	if err := config.DB.
		Preload("RoleModel").
		Preload("UserBusinessRoles.BusinessRole").
		First(&currentUser, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "Current user not found", http.StatusNotFound)
		return
	}

	// Parse IDs
	targetUserID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	userBusinessRoleID, err := uuid.Parse(roleID)
	if err != nil {
		http.Error(w, "Invalid role ID", http.StatusBadRequest)
		return
	}

	// Load the user business role
	var userBusinessRole models.UserBusinessRole
	if err := config.DB.
		Preload("BusinessRole").
		Where("id = ? AND user_id = ?", userBusinessRoleID, targetUserID).
		First(&userBusinessRole).Error; err != nil {
		http.Error(w, "Role assignment not found", http.StatusNotFound)
		return
	}

	// Check if current user can remove this role
	if !currentUser.CanAssignRole(userBusinessRole.BusinessRole.Level) {
		http.Error(w, "You don't have permission to remove this role", http.StatusForbidden)
		return
	}

	// Delete the role assignment
	if err := config.DB.Delete(&userBusinessRole).Error; err != nil {
		http.Error(w, "Failed to remove role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Role removed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUserRoles - GET /api/users/:id/roles
// Returns all business roles for user with vertical info
func GetUserRoles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// Parse user ID
	targetUserID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Load user with business roles
	var user models.User
	if err := config.DB.
		Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
		Preload("UserBusinessRoles.BusinessRole.Permissions").
		First(&user, "id = ?", targetUserID).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Build business roles response
	businessRoles := []map[string]interface{}{}
	for _, ubr := range user.UserBusinessRoles {
		if ubr.IsActive && ubr.BusinessRole.ID != uuid.Nil {
			businessRoles = append(businessRoles, map[string]interface{}{
				"id":            ubr.ID,
				"role_id":       ubr.BusinessRole.ID,
				"role_name":     ubr.BusinessRole.DisplayName,
				"vertical_id":   ubr.BusinessRole.BusinessVerticalID,
				"vertical_name": ubr.BusinessRole.BusinessVertical.Name,
				"vertical_code": ubr.BusinessRole.BusinessVertical.Code,
				"level":         ubr.BusinessRole.Level,
				"assigned_at":   ubr.AssignedAt,
			})
		}
	}

	response := map[string]interface{}{
		"user_id":        user.ID,
		"business_roles": businessRoles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetAssignableRoles - GET /api/users/:id/assignable-roles?verticalId=xxx
// Returns roles current user can assign based on their level
func GetAssignableRoles(w http.ResponseWriter, r *http.Request) {
	verticalID := r.URL.Query().Get("verticalId")
	if verticalID == "" {
		http.Error(w, "verticalId parameter required", http.StatusBadRequest)
		return
	}

	// Get current user from context
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Load current user with all roles
	var currentUser models.User
	if err := config.DB.
		Preload("RoleModel").
		Preload("UserBusinessRoles.BusinessRole").
		First(&currentUser, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "Current user not found", http.StatusNotFound)
		return
	}

	// Parse vertical ID
	businessVerticalID, err := uuid.Parse(verticalID)
	if err != nil {
		http.Error(w, "Invalid vertical ID", http.StatusBadRequest)
		return
	}

	// Get user's highest role level
	userLevel := currentUser.GetHighestRoleLevel()

	// Load all business roles for this vertical
	var businessRoles []models.BusinessRole
	if err := config.DB.
		Where("business_vertical_id = ? AND is_active = ?", businessVerticalID, true).
		Order("level ASC").
		Find(&businessRoles).Error; err != nil {
		http.Error(w, "Failed to load roles: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter roles based on what current user can assign
	assignableRoles := []map[string]interface{}{}
	for _, role := range businessRoles {
		canAssign := currentUser.CanAssignRole(role.Level)
		assignableRoles = append(assignableRoles, map[string]interface{}{
			"id":           role.ID,
			"name":         role.Name,
			"display_name": role.DisplayName,
			"description":  role.Description,
			"level":        role.Level,
			"can_assign":   canAssign,
		})
	}

	response := map[string]interface{}{
		"vertical_id":      businessVerticalID,
		"user_level":       userLevel,
		"assignable_roles": assignableRoles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetVerticalRoles - GET /api/business-verticals/:id/roles
// Returns all roles for a business vertical
func GetVerticalRoles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	verticalID := vars["id"]

	// Parse vertical ID
	businessVerticalID, err := uuid.Parse(verticalID)
	if err != nil {
		http.Error(w, "Invalid vertical ID", http.StatusBadRequest)
		return
	}

	// Load all business roles for this vertical
	var businessRoles []models.BusinessRole
	if err := config.DB.
		Preload("Permissions").
		Where("business_vertical_id = ? AND is_active = ?", businessVerticalID, true).
		Order("level ASC").
		Find(&businessRoles).Error; err != nil {
		http.Error(w, "Failed to load roles: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response
	roles := []map[string]interface{}{}
	for _, role := range businessRoles {
		roles = append(roles, map[string]interface{}{
			"id":                role.ID,
			"name":              role.Name,
			"display_name":      role.DisplayName,
			"description":       role.Description,
			"level":             role.Level,
			"permissions_count": len(role.Permissions),
		})
	}

	response := map[string]interface{}{
		"vertical_id": businessVerticalID,
		"roles":       roles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
