package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

type createBusinessReq struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

type businessResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	UserCount   int64     `json:"user_count"`
	RoleCount   int64     `json:"role_count"`
}

type createBusinessRoleReq struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Level       int      `json:"level"`
	Permissions []string `json:"permissions"`
}

type businessRoleResponse struct {
	ID                 uuid.UUID                `json:"id"`
	Name               string                   `json:"name"`
	DisplayName        string                   `json:"display_name"`
	Description        string                   `json:"description"`
	Level              int                      `json:"level"`
	BusinessVerticalID uuid.UUID                `json:"business_vertical_id"`
	BusinessVertical   string                   `json:"business_vertical_name"`
	Permissions        []permissionResponse     `json:"permissions"`
	UserCount          int64                    `json:"user_count"`
}

type assignUserRoleReq struct {
	UserID         string `json:"user_id"`
	BusinessRoleID string `json:"business_role_id"`
}

// GetAllBusinessVerticals returns all business verticals
func GetAllBusinessVerticals(w http.ResponseWriter, r *http.Request) {
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

	var businesses []models.BusinessVertical
	if err := config.DB.Where("is_active = ?", true).
		Limit(limit).
		Offset(offset).
		Find(&businesses).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var total int64
	config.DB.Model(&models.BusinessVertical{}).Where("is_active = ?", true).Count(&total)

	// Convert to response format with counts
	businessResponses := make([]businessResponse, len(businesses))
	for i, business := range businesses {
		var userCount, roleCount int64
		config.DB.Model(&models.User{}).Where("business_vertical_id = ?", business.ID).Count(&userCount)
		config.DB.Model(&models.BusinessRole{}).Where("business_vertical_id = ? AND is_active = ?", business.ID, true).Count(&roleCount)

		businessResponses[i] = businessResponse{
			ID:          business.ID,
			Name:        business.Name,
			Code:        business.Code,
			Description: business.Description,
			IsActive:    business.IsActive,
			UserCount:   userCount,
			RoleCount:   roleCount,
		}
	}

	response := map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  businessResponses,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateBusinessVertical creates a new business vertical
func CreateBusinessVertical(w http.ResponseWriter, r *http.Request) {
	var req createBusinessReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	defaultSettings := "{}"
	business := models.BusinessVertical{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		IsActive:    true,
		Settings:    &defaultSettings,
	}

	if err := config.DB.Create(&business).Error; err != nil {
		http.Error(w, "failed to create business vertical: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create default roles for this business
	createDefaultBusinessRoles(business.ID)

	response := businessResponse{
		ID:          business.ID,
		Name:        business.Name,
		Code:        business.Code,
		Description: business.Description,
		IsActive:    business.IsActive,
		UserCount:   0,
		RoleCount:   0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetBusinessRoles returns all roles for a specific business vertical
func GetBusinessRoles(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "invalid business identifier", http.StatusBadRequest)
		return
	}

	var roles []models.BusinessRole
	if err := config.DB.Preload("Permissions").
		Preload("BusinessVertical").
		Where("business_vertical_id = ? AND is_active = ?", businessID, true).
		Order("level ASC").
		Find(&roles).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	roleResponses := make([]businessRoleResponse, len(roles))
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

		var userCount int64
		config.DB.Model(&models.UserBusinessRole{}).Where("business_role_id = ? AND is_active = ?", role.ID, true).Count(&userCount)

		roleResponses[i] = businessRoleResponse{
			ID:                 role.ID,
			Name:               role.Name,
			DisplayName:        role.DisplayName,
			Description:        role.Description,
			Level:              role.Level,
			BusinessVerticalID: role.BusinessVerticalID,
			BusinessVertical:   role.BusinessVertical.Name,
			Permissions:        permissions,
			UserCount:          userCount,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roleResponses)
}

// CreateBusinessRole creates a new role for a business vertical
func CreateBusinessRole(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "invalid business identifier", http.StatusBadRequest)
		return
	}

	var req createBusinessRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	role := models.BusinessRole{
		Name:               req.Name,
		DisplayName:        req.DisplayName,
		Description:        req.Description,
		Level:              req.Level,
		BusinessVerticalID: businessID,
		IsActive:           true,
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

	// Load for response
	config.DB.Preload("Permissions").Preload("BusinessVertical").First(&role, role.ID)

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

	response := businessRoleResponse{
		ID:                 role.ID,
		Name:               role.Name,
		DisplayName:        role.DisplayName,
		Description:        role.Description,
		Level:              role.Level,
		BusinessVerticalID: role.BusinessVerticalID,
		BusinessVertical:   role.BusinessVertical.Name,
		Permissions:        permissions,
		UserCount:          0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// AssignUserToBusinessRole assigns a user to a role in a business vertical
func AssignUserToBusinessRole(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "invalid business identifier", http.StatusBadRequest)
		return
	}

	var req assignUserRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	roleID, err := uuid.Parse(req.BusinessRoleID)
	if err != nil {
		http.Error(w, "invalid role ID", http.StatusBadRequest)
		return
	}

	// Verify user and role exist
	var user models.User
	if err := config.DB.First(&user, "id = ?", userID).Error; err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	var role models.BusinessRole
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", roleID, businessID).First(&role).Error; err != nil {
		http.Error(w, "role not found in this business", http.StatusNotFound)
		return
	}

	// Check if assignment already exists
	var existing models.UserBusinessRole
	if err := config.DB.Where("user_id = ? AND business_role_id = ?", userID, roleID).First(&existing).Error; err == nil {
		if existing.IsActive {
			http.Error(w, "user already has this role", http.StatusConflict)
			return
		} else {
			// Reactivate existing assignment
			existing.IsActive = true
			config.DB.Save(&existing)
		}
	} else {
		// Create new assignment
		currentUser := middleware.GetClaims(r)
		assignment := models.UserBusinessRole{
			UserID:         userID,
			BusinessRoleID: roleID,
			IsActive:       true,
		}
		if currentUser != nil {
			assignerID, _ := uuid.Parse(currentUser.UserID)
			assignment.AssignedBy = &assignerID
		}
		config.DB.Create(&assignment)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "user assigned to role successfully"})
}

// GetBusinessUsers returns all users in a business vertical with their roles
func GetBusinessUsers(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "invalid business identifier", http.StatusBadRequest)
		return
	}

	var userBusinessRoles []models.UserBusinessRole
	if err := config.DB.Preload("User").
		Preload("BusinessRole").
		Joins("JOIN business_roles ON user_business_roles.business_role_id = business_roles.id").
		Where("business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?", businessID, true).
		Find(&userBusinessRoles).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Group by user
	userMap := make(map[uuid.UUID]map[string]interface{})
	for _, ubr := range userBusinessRoles {
		if _, exists := userMap[ubr.UserID]; !exists {
			userMap[ubr.UserID] = map[string]interface{}{
				"id":    ubr.User.ID,
				"name":  ubr.User.Name,
				"email": ubr.User.Email,
				"phone": ubr.User.Phone,
				"roles": []map[string]interface{}{},
			}
		}
		
		roles := userMap[ubr.UserID]["roles"].([]map[string]interface{})
		roles = append(roles, map[string]interface{}{
			"id":           ubr.BusinessRole.ID,
			"name":         ubr.BusinessRole.Name,
			"display_name": ubr.BusinessRole.DisplayName,
			"level":        ubr.BusinessRole.Level,
			"assigned_at":  ubr.AssignedAt,
		})
		userMap[ubr.UserID]["roles"] = roles
	}

	// Convert to array
	var users []map[string]interface{}
	for _, user := range userMap {
		users = append(users, user)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// createDefaultBusinessRoles creates default roles for a new business vertical
func createDefaultBusinessRoles(businessID uuid.UUID) {
	defaultRoles := []struct {
		Name        string
		DisplayName string
		Description string
		Level       int
		Permissions []string
	}{
		{
			Name:        "admin",
			DisplayName: "Business Administrator",
			Description: "Full administrative access to this business vertical",
			Level:       1,
			Permissions: []string{"business_admin", "read_reports", "create_reports", "update_reports", "delete_reports", 
				"read_users", "create_users", "update_users", "read_materials", "create_materials", "update_materials", "delete_materials"},
		},
		{
			Name:        "manager",
			DisplayName: "Business Manager",
			Description: "Management access to business operations",
			Level:       2,
			Permissions: []string{"read_reports", "create_reports", "update_reports", "read_materials", "create_materials", "update_materials"},
		},
		{
			Name:        "supervisor",
			DisplayName: "Supervisor",
			Description: "Supervisory access to daily operations",
			Level:       3,
			Permissions: []string{"read_reports", "create_reports", "read_materials"},
		},
		{
			Name:        "operator",
			DisplayName: "Operator",
			Description: "Basic operational access",
			Level:       4,
			Permissions: []string{"read_reports", "create_reports"},
		},
	}

	for _, roleData := range defaultRoles {
		role := models.BusinessRole{
			Name:               roleData.Name,
			DisplayName:        roleData.DisplayName,
			Description:        roleData.Description,
			Level:              roleData.Level,
			BusinessVerticalID: businessID,
			IsActive:           true,
		}

		if err := config.DB.Create(&role).Error; err != nil {
			continue
		}

		// Assign permissions
		for _, permName := range roleData.Permissions {
			var permission models.Permission
			if err := config.DB.Where("name = ?", permName).First(&permission).Error; err != nil {
				continue
			}
			config.DB.Model(&role).Association("Permissions").Append(&permission)
		}
	}
}

// GetUserBusinessAccess returns all business verticals the current user can access
func GetUserBusinessAccess(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var user models.User
	if err := config.DB.Preload("RoleModel.Permissions").
		Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
		First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	var accessibleBusinesses []map[string]interface{}

	// Check if user is super admin
	if user.HasPermission("admin_all") || user.Role == "super_admin" || user.Role == "Super Admin" {
		// Super admin can access all business verticals
		var allBusinesses []models.BusinessVertical
		if err := config.DB.Where("is_active = ?", true).Find(&allBusinesses).Error; err != nil {
			http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		for _, business := range allBusinesses {
			accessibleBusinesses = append(accessibleBusinesses, map[string]interface{}{
				"id":          business.ID,
				"name":        business.Name,
				"code":        business.Code,
				"description": business.Description,
				"access_type": "super_admin",
				"roles":       []string{"Super Administrator"},
				"permissions": []string{"all"},
			})
		}
	} else {
		// Regular user - only businesses they have roles in
		businessMap := make(map[uuid.UUID]map[string]interface{})
		
		for _, ubr := range user.UserBusinessRoles {
			if ubr.IsActive {
				businessID := ubr.BusinessRole.BusinessVerticalID
				if _, exists := businessMap[businessID]; !exists {
					businessMap[businessID] = map[string]interface{}{
						"id":          ubr.BusinessRole.BusinessVertical.ID,
						"name":        ubr.BusinessRole.BusinessVertical.Name,
						"code":        ubr.BusinessRole.BusinessVertical.Code,
						"description": ubr.BusinessRole.BusinessVertical.Description,
						"access_type": "business_role",
						"roles":       []string{},
						"permissions": []string{},
					}
				}
				
				// Add role
				roles := businessMap[businessID]["roles"].([]string)
				roles = append(roles, ubr.BusinessRole.DisplayName)
				businessMap[businessID]["roles"] = roles
			}
		}
		
		// Convert map to slice
		for _, business := range businessMap {
			accessibleBusinesses = append(accessibleBusinesses, business)
		}
	}

	response := map[string]interface{}{
		"user_id":              claims.UserID,
		"user_name":           claims.Name,
		"user_role":           user.Role,
		"is_super_admin":      user.HasPermission("admin_all") || user.Role == "super_admin" || user.Role == "Super Admin",
		"accessible_businesses": accessibleBusinesses,
		"total_businesses":     len(accessibleBusinesses),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetSuperAdminDashboard returns comprehensive dashboard data for super admins
func GetSuperAdminDashboard(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var user models.User
	if err := config.DB.Preload("RoleModel.Permissions").First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Verify super admin access
	if !user.HasPermission("admin_all") && user.Role != "super_admin" && user.Role != "Super Admin" {
		http.Error(w, "super admin access required", http.StatusForbidden)
		return
	}

	// Get all business verticals with statistics
	var businesses []models.BusinessVertical
	if err := config.DB.Where("is_active = ?", true).Find(&businesses).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var businessStats []map[string]interface{}
	var totalUsers, totalRoles int64

	for _, business := range businesses {
		var userCount, roleCount int64
		
		// Count users in this business
		config.DB.Model(&models.UserBusinessRole{}).
			Joins("JOIN business_roles ON user_business_roles.business_role_id = business_roles.id").
			Where("business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?", business.ID, true).
			Count(&userCount)
		
		// Count roles in this business
		config.DB.Model(&models.BusinessRole{}).
			Where("business_vertical_id = ? AND is_active = ?", business.ID, true).
			Count(&roleCount)

		businessStats = append(businessStats, map[string]interface{}{
			"id":          business.ID,
			"name":        business.Name,
			"code":        business.Code,
			"description": business.Description,
			"user_count":  userCount,
			"role_count":  roleCount,
			"created_at":  business.CreatedAt,
		})

		totalUsers += userCount
		totalRoles += roleCount
	}

	// Get global statistics
	var globalUserCount, globalRoleCount int64
	config.DB.Model(&models.User{}).Where("is_active = ?", true).Count(&globalUserCount)
	config.DB.Model(&models.Role{}).Where("is_active = ?", true).Count(&globalRoleCount)

	response := map[string]interface{}{
		"super_admin": map[string]interface{}{
			"user_id":   claims.UserID,
			"name":      claims.Name,
			"role":      user.Role,
		},
		"global_stats": map[string]interface{}{
			"total_users":              globalUserCount,
			"total_global_roles":       globalRoleCount,
			"total_business_verticals": len(businesses),
			"total_business_users":     totalUsers,
			"total_business_roles":     totalRoles,
		},
		"business_verticals": businessStats,
		"permissions": []string{
			"Can access all business verticals",
			"Can create/modify business verticals", 
			"Can assign users to any business role",
			"Can view all reports and analytics",
			"Can manage global system settings",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetBusinessInfo returns business information by code, name, or ID
func GetBusinessInfo(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business not found", http.StatusNotFound)
		return
	}

	var business models.BusinessVertical
	if err := config.DB.First(&business, "id = ?", businessID).Error; err != nil {
		http.Error(w, "business not found", http.StatusNotFound)
		return
	}

	// Get business statistics
	var userCount, roleCount int64
	config.DB.Model(&models.UserBusinessRole{}).
		Joins("JOIN business_roles ON user_business_roles.business_role_id = business_roles.id").
		Where("business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?", businessID, true).
		Count(&userCount)
	
	config.DB.Model(&models.BusinessRole{}).
		Where("business_vertical_id = ? AND is_active = ?", businessID, true).
		Count(&roleCount)

	response := map[string]interface{}{
		"id":          business.ID,
		"name":        business.Name,
		"code":        business.Code,
		"description": business.Description,
		"is_active":   business.IsActive,
		"user_count":  userCount,
		"role_count":  roleCount,
		"created_at":  business.CreatedAt,
		"url_examples": map[string]string{
			"by_code": "/api/v1/business/" + business.Code + "/users",
			"by_name": "/api/v1/business/" + strings.ReplaceAll(business.Name, " ", "%20") + "/users",
			"by_id":   "/api/v1/business/" + business.ID.String() + "/users",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}