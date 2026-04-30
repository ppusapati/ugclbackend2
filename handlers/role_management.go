package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/sync/singleflight"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

const unifiedRolesCacheTTL = 10 * time.Minute

type unifiedRolesCacheEntry struct {
	payload   []byte
	expiresAt time.Time
}

type unifiedRolesCacheStore struct {
	mu      sync.Mutex // get() deletes expired entries so always needs the write lock; Mutex is correct.
	entries map[string]unifiedRolesCacheEntry
}

var unifiedRolesCache = &unifiedRolesCacheStore{entries: make(map[string]unifiedRolesCacheEntry)}
var unifiedRolesLoadGroup singleflight.Group

var permissionsListLoadGroup singleflight.Group

const permissionsListCacheKey = "all"

var permissionsListCache = struct {
	mu      sync.Mutex
	entries map[string]unifiedRolesCacheEntry
}{
	entries: make(map[string]unifiedRolesCacheEntry),
}

func getCachedPermissionsList() ([]byte, bool) {
	permissionsListCache.mu.Lock()
	defer permissionsListCache.mu.Unlock()

	entry, ok := permissionsListCache.entries[permissionsListCacheKey]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(permissionsListCache.entries, permissionsListCacheKey)
		return nil, false
	}
	return entry.payload, true
}

func setCachedPermissionsList(payload []byte) {
	permissionsListCache.mu.Lock()
	permissionsListCache.entries[permissionsListCacheKey] = unifiedRolesCacheEntry{payload: payload, expiresAt: time.Now().Add(unifiedRolesCacheTTL)}
	permissionsListCache.mu.Unlock()
}

func invalidatePermissionsListCache() {
	permissionsListCache.mu.Lock()
	clear(permissionsListCache.entries)
	permissionsListCache.mu.Unlock()
}

func unifiedRolesCacheKey(includeBusiness bool, businessVerticalID string) string {
	return strconv.FormatBool(includeBusiness) + ":" + businessVerticalID
}

func (c *unifiedRolesCacheStore) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return entry.payload, true
}

func (c *unifiedRolesCacheStore) set(key string, payload []byte) {
	c.mu.Lock()
	c.entries[key] = unifiedRolesCacheEntry{payload: payload, expiresAt: time.Now().Add(unifiedRolesCacheTTL)}
	c.mu.Unlock()
}

func InvalidateUnifiedRolesCache() {
	unifiedRolesCache.mu.Lock()
	clear(unifiedRolesCache.entries)
	unifiedRolesCache.mu.Unlock()
}

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
	Permissions []PermissionResponse `json:"permissions"`
}

type PermissionResponse struct {
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
		permissions := make([]PermissionResponse, len(role.Permissions))
		for j, perm := range role.Permissions {
			permissions[j] = PermissionResponse{
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
	if payload, ok := getCachedPermissionsList(); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
		return
	}

	loaded, err, _ := permissionsListLoadGroup.Do(permissionsListCacheKey, func() (interface{}, error) {
		if payload, ok := getCachedPermissionsList(); ok {
			return payload, nil
		}

		var permissions []models.Permission
		if err := config.DB.Find(&permissions).Error; err != nil {
			return nil, err
		}

		// Convert to response format
		permResponses := make([]PermissionResponse, len(permissions))
		for i, perm := range permissions {
			permResponses[i] = PermissionResponse{
				ID:          perm.ID,
				Name:        perm.Name,
				Description: perm.Description,
				Resource:    perm.Resource,
				Action:      perm.Action,
			}
		}

		payload, marshalErr := json.Marshal(permResponses)
		if marshalErr != nil {
			return nil, marshalErr
		}
		setCachedPermissionsList(payload)
		return payload, nil
	})
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(loaded.([]byte))
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
	invalidatePermissionsListCache()
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
	InvalidateUnifiedRolesCache()

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
	permissions := make([]PermissionResponse, len(role.Permissions))
	for i, perm := range role.Permissions {
		permissions[i] = PermissionResponse{
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
	permissions := make([]PermissionResponse, len(role.Permissions))
	for i, perm := range role.Permissions {
		permissions[i] = PermissionResponse{
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

	// Invalidate cache for every user assigned this global role so updated permissions
	// apply immediately rather than after the 30s TTL expires.
	var affectedUserIDs []uuid.UUID
	config.DB.Model(&models.User{}).Where("role_id = ?", id).Pluck("id", &affectedUserIDs)
	for _, uid := range affectedUserIDs {
		middleware.InvalidateUserCache(uid.String())
	}
	InvalidateAdminUsersCache()
	InvalidateUnifiedRolesCache()

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
	InvalidateAdminUsersCache()
	InvalidateUnifiedRolesCache()

	w.WriteHeader(http.StatusNoContent)
}

// UnifiedRoleResponse represents a role that could be either global or business-specific
type UnifiedRoleResponse struct {
	ID                 uuid.UUID             `json:"id"`
	Name               string                `json:"name"`
	DisplayName        string                `json:"display_name,omitempty"`
	Description        string                `json:"description"`
	Level              int                   `json:"level"`
	IsActive           bool                  `json:"is_active"`
	IsGlobal           bool                  `json:"is_global"`
	BusinessVerticalID *uuid.UUID            `json:"business_vertical_id,omitempty"`
	BusinessVertical   *BusinessVerticalInfo `json:"business_vertical,omitempty"`
	Permissions        []PermissionResponse  `json:"permissions"`
	UserCount          int64                 `json:"user_count"`
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
	cacheKey := unifiedRolesCacheKey(includeBusiness, businessVerticalID)

	if payload, ok := unifiedRolesCache.get(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
		return
	}

	loaded, err, _ := unifiedRolesLoadGroup.Do(cacheKey, func() (interface{}, error) {
		if payload, ok := unifiedRolesCache.get(cacheKey); ok {
			return payload, nil
		}

		var unifiedRoles []UnifiedRoleResponse

		// 1. Fetch Global Roles
		var globalRoles []models.Role
		if err := config.DB.Preload("Permissions").
			Where("is_active = ?", true).
			Order("level ASC").
			Find(&globalRoles).Error; err != nil {
			return nil, err
		}

		// Get user counts for global roles
		globalRoleUserCounts := make(map[uuid.UUID]int64)
		if len(globalRoles) > 0 {
			globalRoleIDs := make([]uuid.UUID, len(globalRoles))
			for i, r := range globalRoles {
				globalRoleIDs[i] = r.ID
			}

			var globalCounts []struct {
				RoleID uuid.UUID
				Count  int64
			}
			config.DB.Model(&models.User{}).
				Select("role_id, COUNT(*) as count").
				Where("role_id IN ?", globalRoleIDs).
				Group("role_id").
				Scan(&globalCounts)

			for _, gc := range globalCounts {
				globalRoleUserCounts[gc.RoleID] = gc.Count
			}
		}

		// Convert global roles to unified format
		for _, role := range globalRoles {
			permissions := make([]PermissionResponse, len(role.Permissions))
			for j, perm := range role.Permissions {
				permissions[j] = PermissionResponse{
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
				return nil, err
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
				permissions := make([]PermissionResponse, len(role.Permissions))
				for j, perm := range role.Permissions {
					permissions[j] = PermissionResponse{
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

		payload, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			return nil, marshalErr
		}

		unifiedRolesCache.set(cacheKey, payload)
		return payload, nil
	})
	if err != nil {
		http.Error(w, "Failed to fetch unified roles: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(loaded.([]byte))
}
