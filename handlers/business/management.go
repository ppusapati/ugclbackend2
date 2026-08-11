package business

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// businessVerticalsCacheTTL is how long the paginated business verticals list response is cached.
const businessVerticalsCacheTTL = 10 * time.Minute

// authSvc is a package-level AuthService used by business handlers.
var authSvc = middleware.NewAuthService()

type businessVerticalsResponseCache struct {
	mu      sync.Mutex // get() deletes expired entries so always needs the write lock; Mutex is correct.
	entries map[string]businessVerticalsResponseEntry
}

type businessVerticalsResponseEntry struct {
	payload   []byte
	expiresAt time.Time
}

func (c *businessVerticalsResponseCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return e.payload, true
}

func (c *businessVerticalsResponseCache) set(key string, payload []byte) {
	c.mu.Lock()
	c.entries[key] = businessVerticalsResponseEntry{payload: payload, expiresAt: time.Now().Add(businessVerticalsCacheTTL)}
	c.mu.Unlock()
}

func (c *businessVerticalsResponseCache) invalidate() {
	c.mu.Lock()
	clear(c.entries)
	c.mu.Unlock()
}

// permissionResponse is a local DTO for permission information in responses
type permissionResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
}

var businessVerticalsCache = &businessVerticalsResponseCache{entries: make(map[string]businessVerticalsResponseEntry)}
var businessVerticalsLoadGroup singleflight.Group

type createBusinessReq struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

type updateBusinessReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
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
	Name          string          `json:"name"`
	DisplayName   string          `json:"display_name"`
	Description   string          `json:"description"`
	Level         int             `json:"level"`
	Permissions   json.RawMessage `json:"permissions"`
	PermissionIDs []string        `json:"permission_ids"`
	// Keep camelCase variant for compatibility with clients using JS conventions.
	PermissionIDsCamel []string `json:"permissionIds"`
}

type businessRoleResponse struct {
	ID                 uuid.UUID            `json:"id"`
	Name               string               `json:"name"`
	DisplayName        string               `json:"display_name"`
	Description        string               `json:"description"`
	Level              int                  `json:"level"`
	BusinessVerticalID uuid.UUID            `json:"business_vertical_id"`
	BusinessVertical   string               `json:"business_vertical_name"`
	Permissions        []permissionResponse `json:"permissions"`
	UserCount          int64                `json:"user_count"`
}

type assignUserRoleReq struct {
	UserID         string `json:"user_id"`
	BusinessRoleID string `json:"business_role_id"`
}

func resolveRolePermissionIDs(db *gorm.DB, req createBusinessRoleReq) ([]uuid.UUID, error) {
	idSet := make(map[uuid.UUID]struct{})
	permissionNames := make(map[string]struct{})

	for _, rawID := range req.PermissionIDs {
		parsedID, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			continue
		}
		idSet[parsedID] = struct{}{}
	}

	for _, rawID := range req.PermissionIDsCamel {
		parsedID, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			continue
		}
		idSet[parsedID] = struct{}{}
	}

	if len(req.Permissions) > 0 {
		var permObjects []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(req.Permissions, &permObjects); err == nil && len(permObjects) > 0 {
			for _, p := range permObjects {
				if parsedID, err := uuid.Parse(strings.TrimSpace(p.ID)); err == nil {
					idSet[parsedID] = struct{}{}
					continue
				}
				if name := strings.TrimSpace(p.Name); name != "" {
					permissionNames[name] = struct{}{}
				}
			}
		} else {
			var permNames []string
			if err := json.Unmarshal(req.Permissions, &permNames); err == nil {
				for _, name := range permNames {
					trimmed := strings.TrimSpace(name)
					if trimmed == "" {
						continue
					}
					permissionNames[trimmed] = struct{}{}
				}
			}
		}
	}

	if len(permissionNames) > 0 {
		names := make([]string, 0, len(permissionNames))
		for name := range permissionNames {
			names = append(names, name)
		}

		var permissionsByName []models.Permission
		if err := db.Select("id").Where("name IN ?", names).Find(&permissionsByName).Error; err != nil {
			return nil, err
		}

		for _, permission := range permissionsByName {
			idSet[permission.ID] = struct{}{}
		}
	}

	if len(idSet) == 0 {
		return nil, nil
	}

	requestedIDs := make([]uuid.UUID, 0, len(idSet))
	for id := range idSet {
		requestedIDs = append(requestedIDs, id)
	}

	var existingIDs []uuid.UUID
	if err := db.Model(&models.Permission{}).Where("id IN ?", requestedIDs).Pluck("id", &existingIDs).Error; err != nil {
		return nil, err
	}

	return existingIDs, nil
}

// GetAllBusinessVerticals returns all business verticals
func GetAllBusinessVerticals(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

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

	// Cache key includes the tenant schema so the shared process-wide cache
	// never serves one tenant's business-verticals page to another tenant.
	tenantSchema := config.TenantSchemaFromContext(r.Context())
	cacheKey := tenantSchema + ":" + strconv.Itoa(page) + ":" + strconv.Itoa(limit)
	if payload, ok := businessVerticalsCache.get(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
		return
	}

	loaded, err, _ := businessVerticalsLoadGroup.Do(cacheKey, func() (interface{}, error) {
		if payload, ok := businessVerticalsCache.get(cacheKey); ok {
			return payload, nil
		}

		offset := (page - 1) * limit

		var businesses []models.BusinessVertical
		if err := db.Where("is_active = ?", true).
			Limit(limit).
			Offset(offset).
			Find(&businesses).Error; err != nil {
			return nil, err
		}

		var total int64
		if err := db.Model(&models.BusinessVertical{}).Where("is_active = ?", true).Count(&total).Error; err != nil {
			return nil, err
		}

		// Get user counts for all businesses in one query
		userCounts := make(map[uuid.UUID]int64)
		var userCountResults []struct {
			BusinessVerticalID uuid.UUID
			Count              int64
		}
		db.Model(&models.User{}).
			Select("business_vertical_id, COUNT(*) as count").
			Where("business_vertical_id IN ?", func() []uuid.UUID {
				ids := make([]uuid.UUID, len(businesses))
				for i, b := range businesses {
					ids[i] = b.ID
				}
				return ids
			}()).
			Group("business_vertical_id").
			Scan(&userCountResults)

		for _, result := range userCountResults {
			userCounts[result.BusinessVerticalID] = result.Count
		}

		// Get role counts for all businesses in one query
		roleCounts := make(map[uuid.UUID]int64)
		var roleCountResults []struct {
			BusinessVerticalID uuid.UUID
			Count              int64
		}
		db.Model(&models.BusinessRole{}).
			Select("business_vertical_id, COUNT(*) as count").
			Where("business_vertical_id IN ? AND is_active = ?", func() []uuid.UUID {
				ids := make([]uuid.UUID, len(businesses))
				for i, b := range businesses {
					ids[i] = b.ID
				}
				return ids
			}(), true).
			Group("business_vertical_id").
			Scan(&roleCountResults)

		for _, result := range roleCountResults {
			roleCounts[result.BusinessVerticalID] = result.Count
		}

		// Convert to response format with counts
		businessResponses := make([]businessResponse, len(businesses))
		for i, business := range businesses {
			businessResponses[i] = businessResponse{
				ID:          business.ID,
				Name:        business.Name,
				Code:        business.Code,
				Description: business.Description,
				IsActive:    business.IsActive,
				UserCount:   userCounts[business.ID],
				RoleCount:   roleCounts[business.ID],
			}
		}

		response := map[string]interface{}{
			"total": total,
			"page":  page,
			"limit": limit,
			"data":  businessResponses,
		}
		payload, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			return nil, marshalErr
		}
		businessVerticalsCache.set(cacheKey, payload)
		return payload, nil
	})
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(loaded.([]byte))
}

// CreateBusinessVertical creates a new business vertical
func CreateBusinessVertical(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

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

	if err := db.Create(&business).Error; err != nil {
		http.Error(w, "failed to create business vertical: "+err.Error(), http.StatusInternalServerError)
		return
	}
	middleware.InvalidateAccessibleBusinessVerticalsCache()
	middleware.InvalidateBusinessIdentifierCache()
	handlers.InvalidateAdminUsersCache()
	businessVerticalsCache.invalidate()

	// Create default roles for this business
	createDefaultBusinessRoles(db, business.ID)

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

// UpdateBusinessVertical updates an existing business vertical by ID
func UpdateBusinessVertical(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	vars := mux.Vars(r)
	businessID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid business id", http.StatusBadRequest)
		return
	}

	var req updateBusinessReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var business models.BusinessVertical
	if err := db.Where("id = ?", businessID).First(&business).Error; err != nil {
		http.Error(w, "business vertical not found", http.StatusNotFound)
		return
	}

	if req.Name != nil {
		business.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		business.Description = strings.TrimSpace(*req.Description)
	}
	if req.IsActive != nil {
		business.IsActive = *req.IsActive
	}

	if business.Name == "" {
		http.Error(w, "business name is required", http.StatusBadRequest)
		return
	}

	if err := db.Save(&business).Error; err != nil {
		http.Error(w, "failed to update business vertical: "+err.Error(), http.StatusInternalServerError)
		return
	}

	middleware.InvalidateAccessibleBusinessVerticalsCache()
	middleware.InvalidateBusinessIdentifierCache()
	handlers.InvalidateAdminUsersCache()
	businessVerticalsCache.invalidate()

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
	json.NewEncoder(w).Encode(response)
}

// DeleteBusinessVertical deactivates an existing business vertical by ID.
// This is a safe delete to avoid foreign key issues with historical references.
func DeleteBusinessVertical(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	vars := mux.Vars(r)
	businessID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid business id", http.StatusBadRequest)
		return
	}

	var business models.BusinessVertical
	if err := db.Where("id = ?", businessID).First(&business).Error; err != nil {
		http.Error(w, "business vertical not found", http.StatusNotFound)
		return
	}

	if !business.IsActive {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Business vertical already deleted"})
		return
	}

	if err := db.Model(&business).Update("is_active", false).Error; err != nil {
		http.Error(w, "failed to delete business vertical: "+err.Error(), http.StatusInternalServerError)
		return
	}

	middleware.InvalidateAccessibleBusinessVerticalsCache()
	middleware.InvalidateBusinessIdentifierCache()
	handlers.InvalidateAdminUsersCache()
	businessVerticalsCache.invalidate()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Business vertical deleted successfully"})
}

// LEGACY — pre-unification business role system (models.BusinessRole). The
// authoritative system is unified RBAC (models.RBACRole with
// scope_type=business_vertical); see GET /admin/rbac/roles?scope_type=business_vertical
// in handlers/rbac_management.go. This and the following legacy business-role
// CRUD functions (CreateBusinessRole, UpdateBusinessRole, DeleteBusinessRole,
// AssignUserToBusinessRole, createDefaultBusinessRoles) are not called by any
// web or mobile client. Candidates for removal per
// docs/RBAC_CUTOVER_RUNBOOK.md's Removal Gate.

// GetBusinessRoles returns all roles for a specific business vertical
func GetBusinessRoles(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "invalid business identifier", http.StatusBadRequest)
		return
	}

	var roles []models.BusinessRole
	if err := db.Preload("Permissions").
		Preload("BusinessVertical").
		Where("business_vertical_id = ? AND is_active = ?", businessID, true).
		Order("level ASC").
		Find(&roles).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get user counts for all roles in a single query
	userCountsByRole := make(map[uuid.UUID]int64)
	var roleUserCounts []struct {
		BusinessRoleID uuid.UUID
		Count          int64
	}
	db.Model(&models.UserBusinessRole{}).
		Select("business_role_id, COUNT(*) as count").
		Where("business_role_id IN ? AND is_active = ?", func() []uuid.UUID {
			ids := make([]uuid.UUID, len(roles))
			for i, r := range roles {
				ids[i] = r.ID
			}
			return ids
		}(), true).
		Group("business_role_id").
		Scan(&roleUserCounts)

	for _, result := range roleUserCounts {
		userCountsByRole[result.BusinessRoleID] = result.Count
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

		roleResponses[i] = businessRoleResponse{
			ID:                 role.ID,
			Name:               role.Name,
			DisplayName:        role.DisplayName,
			Description:        role.Description,
			Level:              role.Level,
			BusinessVerticalID: role.BusinessVerticalID,
			BusinessVertical:   role.BusinessVertical.Name,
			Permissions:        permissions,
			UserCount:          userCountsByRole[role.ID],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roleResponses)
}

// LEGACY — see the banner above GetBusinessRoles.

// CreateBusinessRole creates a new role for a business vertical
func CreateBusinessRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

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

	if err := db.Create(&role).Error; err != nil {
		http.Error(w, "failed to create role: "+err.Error(), http.StatusInternalServerError)
		return
	}
	handlers.InvalidateUnifiedRolesCache()

	permissionIDs, err := resolveRolePermissionIDs(db, req)
	if err != nil {
		http.Error(w, "failed to resolve permissions", http.StatusInternalServerError)
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, permissionID := range permissionIDs {
			if err := tx.Exec(
				"INSERT INTO business_role_permissions (business_role_id, permission_id, created_at) VALUES (?, ?, NOW()) ON CONFLICT (business_role_id, permission_id) DO NOTHING",
				role.ID,
				permissionID,
			).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		http.Error(w, "failed to assign role permissions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Load for response
	db.Preload("Permissions").Preload("BusinessVertical").First(&role, role.ID)

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

// LEGACY — see the banner above GetBusinessRoles.

// UpdateBusinessRole updates an existing business role with permissions
func UpdateBusinessRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "invalid business identifier", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	roleID, err := uuid.Parse(vars["roleId"])
	if err != nil {
		http.Error(w, "invalid role ID", http.StatusBadRequest)
		return
	}

	var req createBusinessRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Get existing role and verify it belongs to this business
	var role models.BusinessRole
	if err := db.Where("id = ? AND business_vertical_id = ?", roleID, businessID).First(&role).Error; err != nil {
		http.Error(w, "role not found in this business", http.StatusNotFound)
		return
	}

	// Update basic fields
	role.Name = req.Name
	role.DisplayName = req.DisplayName
	role.Description = req.Description
	role.Level = req.Level

	if err := db.Save(&role).Error; err != nil {
		http.Error(w, "failed to update role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	permissionIDs, err := resolveRolePermissionIDs(db, req)
	if err != nil {
		http.Error(w, "failed to resolve permissions", http.StatusInternalServerError)
		return
	}

	// Clear and replace permission mapping atomically, and fail fast on DB errors.
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM business_role_permissions WHERE business_role_id = ?", role.ID).Error; err != nil {
			return err
		}

		for _, permissionID := range permissionIDs {
			if err := tx.Exec(
				"INSERT INTO business_role_permissions (business_role_id, permission_id, created_at) VALUES (?, ?, NOW())",
				role.ID,
				permissionID,
			).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		http.Error(w, "failed to update role permissions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Load fresh role with permissions for response
	var updatedRole models.BusinessRole
	if err := db.
		Preload("BusinessVertical").
		Preload("Permissions").
		First(&updatedRole, "id = ?", role.ID).Error; err != nil {
		http.Error(w, "role not found after update", http.StatusInternalServerError)
		return
	}

	permissions := make([]permissionResponse, len(updatedRole.Permissions))
	for i, perm := range updatedRole.Permissions {
		permissions[i] = permissionResponse{
			ID:          perm.ID,
			Name:        perm.Name,
			Description: perm.Description,
			Resource:    perm.Resource,
			Action:      perm.Action,
		}
	}

	response := businessRoleResponse{
		ID:                 updatedRole.ID,
		Name:               updatedRole.Name,
		DisplayName:        updatedRole.DisplayName,
		Description:        updatedRole.Description,
		Level:              updatedRole.Level,
		BusinessVerticalID: updatedRole.BusinessVerticalID,
		BusinessVertical:   updatedRole.BusinessVertical.Name,
		Permissions:        permissions,
		UserCount:          0,
	}

	// Invalidate cache for every user currently assigned this business role so
	// updated permissions apply immediately rather than after the 30s TTL expires.
	var affectedUserIDs []uuid.UUID
	db.Model(&models.UserBusinessRole{}).
		Where("business_role_id = ? AND is_active = ?", role.ID, true).
		Pluck("user_id", &affectedUserIDs)
	for _, uid := range affectedUserIDs {
		middleware.InvalidateUserCache(uid.String())
	}
	handlers.InvalidateUnifiedRolesCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// LEGACY — see the banner above GetBusinessRoles.

// DeleteBusinessRole deactivates a business role within the current business context.
func DeleteBusinessRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "invalid business identifier", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	roleID, err := uuid.Parse(vars["roleId"])
	if err != nil {
		http.Error(w, "invalid role ID", http.StatusBadRequest)
		return
	}

	var role models.BusinessRole
	if err := db.Where("id = ? AND business_vertical_id = ?", roleID, businessID).First(&role).Error; err != nil {
		http.Error(w, "role not found in this business", http.StatusNotFound)
		return
	}

	if !role.IsActive {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "role already deleted"})
		return
	}

	// Refuse deletion when active users are assigned to this role.
	var activeAssignments int64
	db.Model(&models.UserBusinessRole{}).
		Where("business_role_id = ? AND is_active = ?", role.ID, true).
		Count(&activeAssignments)

	if activeAssignments > 0 {
		http.Error(w, "cannot delete role: users are assigned to this role", http.StatusBadRequest)
		return
	}

	role.IsActive = false
	if err := db.Save(&role).Error; err != nil {
		http.Error(w, "failed to delete role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	handlers.InvalidateUnifiedRolesCache()
	handlers.InvalidateAdminUsersCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "role deleted successfully"})
}

// LEGACY — see the banner above GetBusinessRoles.

// AssignUserToBusinessRole assigns a user to a role in a business vertical
func AssignUserToBusinessRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

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
	if err := db.First(&user, "id = ?", userID).Error; err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	var role models.BusinessRole
	if err := db.Where("id = ? AND business_vertical_id = ?", roleID, businessID).First(&role).Error; err != nil {
		http.Error(w, "role not found in this business", http.StatusNotFound)
		return
	}

	// Check if assignment already exists
	var existing models.UserBusinessRole
	if err := db.Where("user_id = ? AND business_role_id = ?", userID, roleID).First(&existing).Error; err == nil {
		if existing.IsActive {
			http.Error(w, "user already has this role", http.StatusConflict)
			return
		} else {
			// Reactivate existing assignment
			existing.IsActive = true
			db.Save(&existing)
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
		db.Create(&assignment)
	}

	// Evict auth cache so assigned permissions are reflected immediately.
	middleware.InvalidateUserCache(userID.String())
	handlers.InvalidateAdminUsersCache()
	handlers.InvalidateUnifiedRolesCache()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "user assigned to role successfully"})
}

// GetBusinessUsers returns all users in a business vertical with their roles
func GetBusinessUsers(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "invalid business identifier", http.StatusBadRequest)
		return
	}

	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	limit := 50 // Default limit for users list

	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	offset := (page - 1) * limit

	// Union of user IDs from the legacy business-role table and unified RBAC
	// role assignments scoped to this business vertical.
	userIDSet := make(map[uuid.UUID]struct{})

	var legacyUserIDs []uuid.UUID
	db.Table("user_business_roles").
		Select("DISTINCT user_business_roles.user_id").
		Joins("JOIN business_roles ON user_business_roles.business_role_id = business_roles.id").
		Where("business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?", businessID, true).
		Pluck("user_id", &legacyUserIDs)
	for _, id := range legacyUserIDs {
		userIDSet[id] = struct{}{}
	}

	var rbacUserIDs []uuid.UUID
	db.Table("user_role_assignments").
		Select("DISTINCT user_role_assignments.user_id").
		Joins("JOIN rbac_roles ON user_role_assignments.role_id = rbac_roles.id").
		Where("rbac_roles.business_vertical_id = ? AND rbac_roles.scope_type = ? AND rbac_roles.is_active = ? AND user_role_assignments.is_active = ?",
			businessID, models.RoleScopeBusinessVertical, true, true).
		Pluck("user_id", &rbacUserIDs)
	for _, id := range rbacUserIDs {
		userIDSet[id] = struct{}{}
	}

	allUserIDs := make([]uuid.UUID, 0, len(userIDSet))
	for id := range userIDSet {
		allUserIDs = append(allUserIDs, id)
	}
	sort.Slice(allUserIDs, func(i, j int) bool { return allUserIDs[i].String() < allUserIDs[j].String() })

	totalUsers := int64(len(allUserIDs))

	pageStart := offset
	if pageStart > len(allUserIDs) {
		pageStart = len(allUserIDs)
	}
	pageEnd := pageStart + limit
	if pageEnd > len(allUserIDs) {
		pageEnd = len(allUserIDs)
	}
	pageUserIDs := allUserIDs[pageStart:pageEnd]

	userMap := make(map[uuid.UUID]map[string]interface{})

	if len(pageUserIDs) > 0 {
		var userRecords []models.User
		if err := db.Where("id IN ?", pageUserIDs).Find(&userRecords).Error; err != nil {
			http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for _, u := range userRecords {
			userMap[u.ID] = map[string]interface{}{
				"id":    u.ID,
				"name":  u.Name,
				"email": u.Email,
				"phone": u.Phone,
				"roles": []map[string]interface{}{},
			}
		}

		// Legacy business-role assignments for this page of users.
		var userBusinessRoles []models.UserBusinessRole
		if err := db.Preload("BusinessRole").
			Joins("JOIN business_roles ON user_business_roles.business_role_id = business_roles.id").
			Where("user_business_roles.user_id IN ? AND business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?", pageUserIDs, businessID, true).
			Find(&userBusinessRoles).Error; err != nil {
			http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for _, ubr := range userBusinessRoles {
			entry, ok := userMap[ubr.UserID]
			if !ok {
				continue
			}
			roles := entry["roles"].([]map[string]interface{})
			roles = append(roles, map[string]interface{}{
				"id":           ubr.BusinessRole.ID,
				"name":         ubr.BusinessRole.Name,
				"display_name": ubr.BusinessRole.DisplayName,
				"level":        ubr.BusinessRole.Level,
				"assigned_at":  ubr.AssignedAt,
			})
			entry["roles"] = roles
		}

		// Unified RBAC role assignments for this page of users.
		var rbacAssignments []models.UserRoleAssignment
		if err := db.Preload("Role").
			Joins("JOIN rbac_roles ON user_role_assignments.role_id = rbac_roles.id").
			Where("user_role_assignments.user_id IN ? AND rbac_roles.business_vertical_id = ? AND rbac_roles.scope_type = ? AND rbac_roles.is_active = ? AND user_role_assignments.is_active = ?",
				pageUserIDs, businessID, models.RoleScopeBusinessVertical, true, true).
			Find(&rbacAssignments).Error; err != nil {
			http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for _, a := range rbacAssignments {
			entry, ok := userMap[a.UserID]
			if !ok {
				continue
			}
			roles := entry["roles"].([]map[string]interface{})
			roles = append(roles, map[string]interface{}{
				"id":           a.Role.ID,
				"name":         a.Role.Name,
				"display_name": a.Role.DisplayName,
				"level":        a.Role.Level,
				"assigned_at":  a.AssignedAt,
			})
			entry["roles"] = roles
		}
	}

	// Preserve the sorted user-ID order in the response.
	users := make([]map[string]interface{}, 0, len(pageUserIDs))
	for _, id := range pageUserIDs {
		if entry, ok := userMap[id]; ok {
			users = append(users, entry)
		}
	}

	// Return paginated response
	response := map[string]interface{}{
		"total": totalUsers,
		"page":  page,
		"limit": limit,
		"data":  users,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// LEGACY, still actively called from CreateBusinessVertical below. Seeds
// default roles into the legacy models.BusinessRole table only — it does not
// create any equivalent unified RBAC role (models.RBACRole with
// scope_type=business_vertical), so every new business vertical currently
// gets legacy-only default roles with no RBAC counterpart. This is a known
// gap: new verticals need RBAC default roles seeded some other way (e.g.
// manually via POST /admin/rbac/roles) until this function is either removed
// or ported to seed RBACRole instead.

// createDefaultBusinessRoles creates default roles for a new business vertical
func createDefaultBusinessRoles(db *gorm.DB, businessID uuid.UUID) {
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

	permissionNameSet := make(map[string]struct{})
	for _, roleData := range defaultRoles {
		for _, permName := range roleData.Permissions {
			permissionNameSet[strings.TrimSpace(permName)] = struct{}{}
		}
	}

	permissionNames := make([]string, 0, len(permissionNameSet))
	for permName := range permissionNameSet {
		if permName == "" {
			continue
		}
		permissionNames = append(permissionNames, permName)
	}

	permissionByName := make(map[string]models.Permission, len(permissionNames))
	if len(permissionNames) > 0 {
		var permissions []models.Permission
		if err := db.Where("name IN ?", permissionNames).Find(&permissions).Error; err == nil {
			for _, permission := range permissions {
				permissionByName[permission.Name] = permission
			}
		}
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

		if err := db.Create(&role).Error; err != nil {
			continue
		}

		// Assign permissions
		for _, permName := range roleData.Permissions {
			permission, ok := permissionByName[permName]
			if !ok {
				continue
			}
			db.Model(&role).Association("Permissions").Append(&permission)
		}
	}
}

// GetUserBusinessAccess returns all business verticals the current user can access
func GetUserBusinessAccess(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	userCtx, err := authSvc.LoadUserContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	claims := userCtx.Claims
	user := userCtx.User

	var accessibleBusinesses []map[string]interface{}

	// Check if user is super admin
	isSuperAdmin := userCtx.IsSuperAdmin
	if user.HasPermission("admin_all") || isSuperAdmin {
		// Super admin can access all business verticals
		var allBusinesses []models.BusinessVertical
		if err := db.Where("is_active = ?", true).Find(&allBusinesses).Error; err != nil {
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
		// Regular user - businesses from legacy business roles and RBAC business-scoped assignments.
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

		// RBAC business-scoped assignments (new system).
		for _, a := range user.RoleAssignments {
			if !a.IsActive || !a.Role.IsActive || a.Role.ScopeType != models.RoleScopeBusinessVertical {
				continue
			}
			if a.Role.BusinessVerticalID == nil {
				continue
			}
			bid := *a.Role.BusinessVerticalID
			if _, exists := businessMap[bid]; !exists {
				bv := a.Role.BusinessVertical
				if bv == nil {
					continue
				}
				businessMap[bid] = map[string]interface{}{
					"id":          bv.ID,
					"name":        bv.Name,
					"code":        bv.Code,
					"description": bv.Description,
					"access_type": "business_role",
					"roles":       []string{},
					"permissions": []string{},
				}
			}
			roles := businessMap[bid]["roles"].([]string)
			roles = append(roles, a.Role.DisplayName)
			businessMap[bid]["roles"] = roles
		}

		// Convert map to slice
		for _, business := range businessMap {
			accessibleBusinesses = append(accessibleBusinesses, business)
		}

		// Global-role users with no business-scoped assignment: add every vertical
		// they have a site access entry in so the mobile can resolve a business context.
		if len(accessibleBusinesses) == 0 && middleware.HasActiveGlobalRBACRole(*userCtx.User) {
			var siteBVs []struct {
				ID          uuid.UUID
				Name        string
				Code        string
				Description string
			}
			db.Raw(`
				SELECT DISTINCT bv.id, bv.name, bv.code, bv.description
				FROM user_site_accesses usa
				JOIN sites s ON s.id = usa.site_id
				JOIN business_verticals bv ON bv.id = s.business_vertical_id
				WHERE usa.user_id = ? AND s.is_active = true AND bv.is_active = true
			`, userCtx.User.ID).Scan(&siteBVs)
			for _, bv := range siteBVs {
				accessibleBusinesses = append(accessibleBusinesses, map[string]interface{}{
					"id":          bv.ID,
					"name":        bv.Name,
					"code":        bv.Code,
					"description": bv.Description,
					"access_type": "business_role",
					"roles":       []string{},
					"permissions": []string{},
				})
			}
		}
	}

	globalRoleName := ""
	if user.RoleModel != nil {
		globalRoleName = user.RoleModel.Name
	}

	response := map[string]interface{}{
		"user_id":               claims.UserID,
		"user_name":             claims.Name,
		"global_role":           globalRoleName,
		"is_super_admin":        isSuperAdmin,
		"accessible_businesses": accessibleBusinesses,
		"total_businesses":      len(accessibleBusinesses),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetSuperAdminDashboard returns comprehensive dashboard data for super admins
func GetSuperAdminDashboard(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var user models.User
	if err := db.Preload("RoleModel.Permissions").Preload("RoleAssignments.Role").First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Verify super admin access via both legacy role and RBAC assignments.
	isSuperAdmin2 := (user.RoleModel != nil && user.RoleModel.Name == "super_admin")
	if !isSuperAdmin2 {
		for _, a := range user.RoleAssignments {
			if a.IsActive && a.Role.IsActive && a.Role.Name == "super_admin" {
				isSuperAdmin2 = true
				break
			}
		}
	}
	if !user.HasPermission("admin_all") && !isSuperAdmin2 {
		http.Error(w, "super admin access required", http.StatusForbidden)
		return
	}

	// Get all business verticals with statistics
	var businesses []models.BusinessVertical
	if err := db.Where("is_active = ?", true).Find(&businesses).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get user counts for all businesses in a single query
	dashboardUserCounts := make(map[uuid.UUID]int64)
	var dashboardUserCountResults []struct {
		BusinessVerticalID uuid.UUID
		Count              int64
	}
	db.Table("user_business_roles").
		Select("business_roles.business_vertical_id, COUNT(DISTINCT user_business_roles.user_id) as count").
		Joins("JOIN business_roles ON user_business_roles.business_role_id = business_roles.id").
		Where("business_roles.business_vertical_id IN ? AND user_business_roles.is_active = ?", func() []uuid.UUID {
			ids := make([]uuid.UUID, len(businesses))
			for i, b := range businesses {
				ids[i] = b.ID
			}
			return ids
		}(), true).
		Group("business_roles.business_vertical_id").
		Scan(&dashboardUserCountResults)

	for _, result := range dashboardUserCountResults {
		dashboardUserCounts[result.BusinessVerticalID] = result.Count
	}

	// Get role counts for all businesses in a single query
	dashboardRoleCounts := make(map[uuid.UUID]int64)
	var dashboardRoleCountResults []struct {
		BusinessVerticalID uuid.UUID
		Count              int64
	}
	db.Model(&models.BusinessRole{}).
		Select("business_vertical_id, COUNT(*) as count").
		Where("business_vertical_id IN ? AND is_active = ?", func() []uuid.UUID {
			ids := make([]uuid.UUID, len(businesses))
			for i, b := range businesses {
				ids[i] = b.ID
			}
			return ids
		}(), true).
		Group("business_vertical_id").
		Scan(&dashboardRoleCountResults)

	for _, result := range dashboardRoleCountResults {
		dashboardRoleCounts[result.BusinessVerticalID] = result.Count
	}

	var businessStats []map[string]interface{}
	var totalUsers, totalRoles int64

	for _, business := range businesses {
		userCount := dashboardUserCounts[business.ID]
		roleCount := dashboardRoleCounts[business.ID]

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
	db.Model(&models.User{}).Where("is_active = ?", true).Count(&globalUserCount)
	db.Model(&models.Role{}).Where("is_active = ?", true).Count(&globalRoleCount)

	globalRole := ""
	if user.RoleModel != nil {
		globalRole = user.RoleModel.Name
	}

	response := map[string]interface{}{
		"super_admin": map[string]interface{}{
			"user_id":     claims.UserID,
			"name":        claims.Name,
			"global_role": globalRole,
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
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business not found", http.StatusNotFound)
		return
	}

	var business models.BusinessVertical
	if err := db.First(&business, "id = ?", businessID).Error; err != nil {
		http.Error(w, "business not found", http.StatusNotFound)
		return
	}

	// Get business statistics
	var userCount, roleCount int64
	db.Model(&models.UserBusinessRole{}).
		Joins("JOIN business_roles ON user_business_roles.business_role_id = business_roles.id").
		Where("business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?", businessID, true).
		Count(&userCount)

	db.Model(&models.BusinessRole{}).
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
