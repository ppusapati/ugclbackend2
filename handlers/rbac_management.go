package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

type RBACRoleRequest struct {
	Name               string               `json:"name"`
	DisplayName        string               `json:"display_name"`
	Description        string               `json:"description"`
	ScopeType          models.RoleScopeType `json:"scope_type"`
	BusinessVerticalID *uuid.UUID           `json:"business_vertical_id"`
	Level              int                  `json:"level"`
	PermissionIDs      []uuid.UUID          `json:"permission_ids"`
	IsActive           *bool                `json:"is_active,omitempty"`
}

type RBACRoleResponse struct {
	ID                 uuid.UUID             `json:"id"`
	Name               string                `json:"name"`
	DisplayName        string                `json:"display_name"`
	Description        string                `json:"description"`
	ScopeType          models.RoleScopeType  `json:"scope_type"`
	BusinessVerticalID *uuid.UUID            `json:"business_vertical_id"`
	BusinessVertical   *RBACVerticalResponse `json:"business_vertical,omitempty"`
	Level              int                   `json:"level"`
	IsActive           bool                  `json:"is_active"`
	Permissions        []PermissionResponse  `json:"permissions"`
}

type RBACVerticalResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Code string    `json:"code"`
}

type RBACAssignmentResponse struct {
	ID         uuid.UUID        `json:"id"`
	UserID     uuid.UUID        `json:"user_id"`
	RoleID     uuid.UUID        `json:"role_id"`
	ScopeKey   string           `json:"scope_key"`
	IsActive   bool             `json:"is_active"`
	AssignedAt time.Time        `json:"assigned_at"`
	AssignedBy *uuid.UUID       `json:"assigned_by"`
	Role       RBACRoleResponse `json:"role"`
}

type assignRBACRoleRequest struct {
	RoleID uuid.UUID `json:"role_id"`
}

func writeRBACJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeRBACError(w http.ResponseWriter, status int, message string) {
	writeRBACJSON(w, status, map[string]string{"error": message})
}

func roleResponseFromModel(role models.RBACRole) RBACRoleResponse {
	permissions := make([]PermissionResponse, 0, len(role.Permissions))
	for _, permission := range role.Permissions {
		permissions = append(permissions, PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
		})
	}

	response := RBACRoleResponse{
		ID:                 role.ID,
		Name:               role.Name,
		DisplayName:        role.DisplayName,
		Description:        role.Description,
		ScopeType:          role.ScopeType,
		BusinessVerticalID: role.BusinessVerticalID,
		Level:              role.Level,
		IsActive:           role.IsActive,
		Permissions:        permissions,
	}
	if role.BusinessVertical != nil {
		response.BusinessVertical = &RBACVerticalResponse{
			ID: role.BusinessVertical.ID, Name: role.BusinessVertical.Name, Code: role.BusinessVertical.Code,
		}
	}
	return response
}

func assignmentResponseFromModel(assignment models.UserRoleAssignment) RBACAssignmentResponse {
	return RBACAssignmentResponse{
		ID:         assignment.ID,
		UserID:     assignment.UserID,
		RoleID:     assignment.RoleID,
		ScopeKey:   assignment.ScopeKey,
		IsActive:   assignment.IsActive,
		AssignedAt: assignment.AssignedAt,
		AssignedBy: assignment.AssignedBy,
		Role:       roleResponseFromModel(assignment.Role),
	}
}

func validateRBACRoleRequest(tx *gorm.DB, req *RBACRoleRequest, excludeRoleID *uuid.UUID) ([]models.Permission, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" || req.DisplayName == "" {
		return nil, errors.New("name and display_name are required")
	}
	if len(req.Name) > 50 || len(req.DisplayName) > 100 || len(req.Description) > 255 {
		return nil, errors.New("role fields exceed their maximum length")
	}
	if req.Level < 0 || req.Level > 100 {
		return nil, errors.New("level must be between 0 and 100")
	}

	candidate := models.RBACRole{ScopeType: req.ScopeType, BusinessVerticalID: req.BusinessVerticalID}
	if err := candidate.ValidateScope(); err != nil {
		return nil, err
	}
	if req.ScopeType == models.RoleScopeBusinessVertical {
		var count int64
		if err := tx.Model(&models.BusinessVertical{}).
			Where("id = ? AND is_active = ?", *req.BusinessVerticalID, true).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, errors.New("business vertical is not active or does not exist")
		}
	}

	duplicateQuery := tx.Model(&models.RBACRole{}).Where("LOWER(name) = LOWER(?) AND scope_type = ?", req.Name, req.ScopeType)
	if req.ScopeType == models.RoleScopeGlobal {
		duplicateQuery = duplicateQuery.Where("business_vertical_id IS NULL")
	} else {
		duplicateQuery = duplicateQuery.Where("business_vertical_id = ?", *req.BusinessVerticalID)
	}
	if excludeRoleID != nil {
		duplicateQuery = duplicateQuery.Where("id <> ?", *excludeRoleID)
	}
	var duplicateCount int64
	if err := duplicateQuery.Count(&duplicateCount).Error; err != nil {
		return nil, err
	}
	if duplicateCount > 0 {
		return nil, errors.New("a role with this name already exists in the selected scope")
	}

	permissionIDs := make(map[uuid.UUID]struct{}, len(req.PermissionIDs))
	for _, permissionID := range req.PermissionIDs {
		if permissionID == uuid.Nil {
			return nil, errors.New("permission_ids contains an invalid ID")
		}
		permissionIDs[permissionID] = struct{}{}
	}
	permissions := make([]models.Permission, 0, len(permissionIDs))
	if len(permissionIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(permissionIDs))
		for permissionID := range permissionIDs {
			ids = append(ids, permissionID)
		}
		if err := tx.Where("id IN ?", ids).Find(&permissions).Error; err != nil {
			return nil, err
		}
		if len(permissions) != len(ids) {
			return nil, errors.New("one or more permission_ids do not exist")
		}
	}
	return permissions, nil
}

func loadRBACRole(db *gorm.DB, roleID uuid.UUID) (models.RBACRole, error) {
	var role models.RBACRole
	err := db.Preload("Permissions").Preload("BusinessVertical").First(&role, "id = ?", roleID).Error
	return role, err
}

func replaceRBACRolePermissions(tx *gorm.DB, roleID uuid.UUID, permissions []models.Permission) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&models.RBACRolePermission{}).Error; err != nil {
		return err
	}

	if len(permissions) == 0 {
		return nil
	}

	rows := make([]models.RBACRolePermission, 0, len(permissions))
	now := time.Now().UTC()
	for _, permission := range permissions {
		rows = append(rows, models.RBACRolePermission{
			RoleID:       roleID,
			PermissionID: permission.ID,
			CreatedAt:    now,
		})
	}

	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return err
	}

	var assignedCount int64
	if err := tx.Model(&models.RBACRolePermission{}).Where("role_id = ?", roleID).Count(&assignedCount).Error; err != nil {
		return err
	}
	if assignedCount != int64(len(rows)) {
		return errors.New("failed to persist all permission assignments")
	}

	return nil
}

func authenticatedRBACActorID(r *http.Request) (uuid.UUID, error) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		return uuid.Nil, errors.New("unauthorized")
	}
	actorID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil, errors.New("invalid authenticated user")
	}
	return actorID, nil
}

func invalidateUsersAssignedToRole(db *gorm.DB, roleID uuid.UUID) {
	var userIDs []uuid.UUID
	if err := db.Model(&models.UserRoleAssignment{}).
		Where("role_id = ? AND is_active = ?", roleID, true).
		Distinct("user_id").Pluck("user_id", &userIDs).Error; err == nil {
		for _, userID := range userIDs {
			middleware.InvalidateUserCache(userID.String())
		}
	}
	InvalidateAdminUsersCache()
	InvalidateUnifiedRolesCache()
}

func ListRBACRoles(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	defer cleanup()

	page, limit := 1, 50
	if value, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && value > 0 && value <= 200 {
		limit = value
	}

	query := db.Model(&models.RBACRole{})
	if r.URL.Query().Get("include_inactive") != "true" {
		query = query.Where("is_active = ?", true)
	}
	if scope := strings.TrimSpace(r.URL.Query().Get("scope_type")); scope != "" {
		if scope != string(models.RoleScopeGlobal) && scope != string(models.RoleScopeBusinessVertical) {
			writeRBACError(w, http.StatusBadRequest, "invalid scope_type")
			return
		}
		query = query.Where("scope_type = ?", scope)
	}
	if rawID := strings.TrimSpace(r.URL.Query().Get("business_vertical_id")); rawID != "" {
		businessID, err := uuid.Parse(rawID)
		if err != nil {
			writeRBACError(w, http.StatusBadRequest, "invalid business_vertical_id")
			return
		}
		query = query.Where("business_vertical_id = ?", businessID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeRBACError(w, http.StatusInternalServerError, "failed to count roles")
		return
	}
	var roles []models.RBACRole
	if err := query.Preload("Permissions").Preload("BusinessVertical").
		Order("scope_type, level, display_name").Offset((page - 1) * limit).Limit(limit).Find(&roles).Error; err != nil {
		writeRBACError(w, http.StatusInternalServerError, "failed to load roles")
		return
	}
	data := make([]RBACRoleResponse, 0, len(roles))
	for _, role := range roles {
		data = append(data, roleResponseFromModel(role))
	}
	writeRBACJSON(w, http.StatusOK, map[string]interface{}{"data": data, "page": page, "limit": limit, "total": total})
}

func GetRBACRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	defer cleanup()

	roleID, err := uuid.Parse(mux.Vars(r)["roleId"])
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid role ID")
		return
	}
	role, err := loadRBACRole(db, roleID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeRBACError(w, http.StatusNotFound, "role not found")
		return
	}
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "failed to load role")
		return
	}
	writeRBACJSON(w, http.StatusOK, roleResponseFromModel(role))
}

func CreateRBACRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	defer cleanup()

	actorID, err := authenticatedRBACActorID(r)
	if err != nil {
		writeRBACError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req RBACRoleRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var created models.RBACRole
	err = db.Transaction(func(tx *gorm.DB) error {
		permissions, err := validateRBACRoleRequest(tx, &req, nil)
		if err != nil {
			return err
		}
		active := true
		if req.IsActive != nil {
			active = *req.IsActive
		}
		created = models.RBACRole{
			Name: req.Name, DisplayName: req.DisplayName, Description: req.Description,
			ScopeType: req.ScopeType, BusinessVerticalID: req.BusinessVerticalID,
			Level: req.Level, IsActive: active,
		}
		allowed, err := actorCanAssignRBACRole(tx, actorID, created)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("you cannot create a role at or above your privilege level")
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		return replaceRBACRolePermissions(tx, created.ID, permissions)
	})
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err = loadRBACRole(db, created.ID)
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "role created but could not be reloaded")
		return
	}
	InvalidateUnifiedRolesCache()
	writeRBACJSON(w, http.StatusCreated, roleResponseFromModel(created))
}

func UpdateRBACRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	defer cleanup()

	roleID, err := uuid.Parse(mux.Vars(r)["roleId"])
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid role ID")
		return
	}
	actorID, err := authenticatedRBACActorID(r)
	if err != nil {
		writeRBACError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req RBACRoleRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		var role models.RBACRole
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&role, "id = ?", roleID).Error; err != nil {
			return err
		}
		if role.Name == "super_admin" {
			return errors.New("the super_admin role cannot be modified")
		}
		allowed, err := actorCanAssignRBACRole(tx, actorID, role)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("you cannot modify a role at or above your privilege level")
		}
		permissions, err := validateRBACRoleRequest(tx, &req, &roleID)
		if err != nil {
			return err
		}
		if role.ScopeType != req.ScopeType || !sameOptionalUUID(role.BusinessVerticalID, req.BusinessVerticalID) {
			var activeAssignments int64
			if err := tx.Model(&models.UserRoleAssignment{}).
				Where("role_id = ? AND is_active = ?", roleID, true).
				Count(&activeAssignments).Error; err != nil {
				return err
			}
			if activeAssignments > 0 {
				return errors.New("role scope cannot change while active assignments exist")
			}
		}
		candidate := role
		candidate.Level = req.Level
		allowed, err = actorCanAssignRBACRole(tx, actorID, candidate)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("you cannot raise a role to or above your privilege level")
		}
		role.Name = req.Name
		role.DisplayName = req.DisplayName
		role.Description = req.Description
		role.ScopeType = req.ScopeType
		role.BusinessVerticalID = req.BusinessVerticalID
		role.Level = req.Level
		if req.IsActive != nil {
			role.IsActive = *req.IsActive
		}
		if err := tx.Save(&role).Error; err != nil {
			return err
		}
		if err := replaceRBACRolePermissions(tx, role.ID, permissions); err != nil {
			return err
		}
		newScopeKey, err := role.AssignmentScopeKey()
		if err != nil {
			return err
		}
		return tx.Model(&models.UserRoleAssignment{}).
			Where("role_id = ? AND is_active = ?", roleID, true).
			Update("scope_key", newScopeKey).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeRBACError(w, http.StatusNotFound, "role not found")
		return
	}
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, err.Error())
		return
	}
	role, err := loadRBACRole(db, roleID)
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "role updated but could not be reloaded")
		return
	}
	invalidateUsersAssignedToRole(db, roleID)
	writeRBACJSON(w, http.StatusOK, roleResponseFromModel(role))
}

func sameOptionalUUID(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func DeactivateRBACRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	defer cleanup()

	roleID, err := uuid.Parse(mux.Vars(r)["roleId"])
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid role ID")
		return
	}
	actorID, err := authenticatedRBACActorID(r)
	if err != nil {
		writeRBACError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var affectedUserIDs []uuid.UUID
	err = db.Transaction(func(tx *gorm.DB) error {
		var role models.RBACRole
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&role, "id = ?", roleID).Error; err != nil {
			return err
		}
		if role.Name == "super_admin" {
			return errors.New("the super_admin role cannot be deactivated")
		}
		allowed, err := actorCanAssignRBACRole(tx, actorID, role)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("you cannot deactivate a role at or above your privilege level")
		}
		if err := tx.Model(&models.UserRoleAssignment{}).
			Where("role_id = ? AND is_active = ?", roleID, true).
			Distinct("user_id").Pluck("user_id", &affectedUserIDs).Error; err != nil {
			return err
		}
		result := tx.Model(&models.RBACRole{}).Where("id = ?", roleID).Update("is_active", false)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&models.UserRoleAssignment{}).
			Where("role_id = ? AND is_active = ?", roleID, true).
			Update("is_active", false).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeRBACError(w, http.StatusNotFound, "role not found")
		return
	}
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, userID := range affectedUserIDs {
		middleware.InvalidateUserCache(userID.String())
	}
	InvalidateAdminUsersCache()
	InvalidateUnifiedRolesCache()
	writeRBACJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func ListUserRBACAssignments(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	defer cleanup()

	userID, err := uuid.Parse(mux.Vars(r)["userId"])
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	var assignments []models.UserRoleAssignment
	if err := db.Preload("Role.Permissions").Preload("Role.BusinessVertical").
		Where("user_id = ? AND is_active = ?", userID, true).
		Order("scope_key").Find(&assignments).Error; err != nil {
		writeRBACError(w, http.StatusInternalServerError, "failed to load assignments")
		return
	}
	data := make([]RBACAssignmentResponse, 0, len(assignments))
	for _, assignment := range assignments {
		data = append(data, assignmentResponseFromModel(assignment))
	}
	writeRBACJSON(w, http.StatusOK, data)
}

func actorCanAssignRBACRole(tx *gorm.DB, actorID uuid.UUID, target models.RBACRole) (bool, error) {
	var assignments []models.UserRoleAssignment
	if err := tx.Preload("Role").Where("user_id = ? AND is_active = ?", actorID, true).Find(&assignments).Error; err != nil {
		return false, err
	}
	for _, assignment := range assignments {
		if actorRoleCanManageTarget(assignment.Role, target) {
			return true, nil
		}
	}
	return false, nil
}

func actorRoleCanManageTarget(actor, target models.RBACRole) bool {
	if !actor.IsActive {
		return false
	}
	if actor.ScopeType == models.RoleScopeGlobal {
		return actor.Name == "super_admin" || actor.Level < target.Level
	}
	if target.ScopeType != models.RoleScopeBusinessVertical ||
		!sameOptionalUUID(actor.BusinessVerticalID, target.BusinessVerticalID) {
		return false
	}
	return actor.Level < target.Level
}

func AssignRBACRole(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	defer cleanup()

	targetUserID, err := uuid.Parse(mux.Vars(r)["userId"])
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeRBACError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	actorID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeRBACError(w, http.StatusUnauthorized, "invalid authenticated user")
		return
	}
	var req assignRBACRoleRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || req.RoleID == uuid.Nil {
		writeRBACError(w, http.StatusBadRequest, "role_id is required")
		return
	}

	var assignment models.UserRoleAssignment
	err = db.Transaction(func(tx *gorm.DB) error {
		var userCount int64
		if err := tx.Model(&models.User{}).Where("id = ? AND is_active = ?", targetUserID, true).Count(&userCount).Error; err != nil {
			return err
		}
		if userCount != 1 {
			return errors.New("target user is not active or does not exist")
		}
		var role models.RBACRole
		if err := tx.First(&role, "id = ? AND is_active = ?", req.RoleID, true).Error; err != nil {
			return errors.New("role is not active or does not exist")
		}
		allowed, err := actorCanAssignRBACRole(tx, actorID, role)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("you cannot assign a role at or above your privilege level")
		}
		scopeKey, err := role.AssignmentScopeKey()
		if err != nil {
			return err
		}
		var existing models.UserRoleAssignment
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND scope_key = ? AND is_active = ?", targetUserID, scopeKey, true).
			First(&existing).Error
		if err == nil && existing.RoleID == role.ID {
			assignment = existing
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if err := tx.Model(&existing).Update("is_active", false).Error; err != nil {
				return err
			}
		}
		assignment = models.UserRoleAssignment{
			UserID: targetUserID, RoleID: role.ID, ScopeKey: scopeKey,
			IsActive: true, AssignedAt: time.Now().UTC(), AssignedBy: &actorID,
		}
		return tx.Create(&assignment).Error
	})
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := db.Preload("Role.Permissions").Preload("Role.BusinessVertical").First(&assignment, "id = ?", assignment.ID).Error; err != nil {
		writeRBACError(w, http.StatusInternalServerError, "assignment saved but could not be reloaded")
		return
	}
	middleware.InvalidateUserCache(targetUserID.String())
	InvalidateAdminUsersCache()
	writeRBACJSON(w, http.StatusOK, assignmentResponseFromModel(assignment))
}

func RemoveRBACRoleAssignment(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		writeRBACError(w, http.StatusInternalServerError, "database unavailable")
		return
	}
	defer cleanup()

	targetUserID, err := uuid.Parse(mux.Vars(r)["userId"])
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	assignmentID, err := uuid.Parse(mux.Vars(r)["assignmentId"])
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, "invalid assignment ID")
		return
	}
	actorID, err := authenticatedRBACActorID(r)
	if err != nil {
		writeRBACError(w, http.StatusUnauthorized, err.Error())
		return
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var assignment models.UserRoleAssignment
		if err := tx.Preload("Role").Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&assignment, "id = ? AND user_id = ? AND is_active = ?", assignmentID, targetUserID, true).Error; err != nil {
			return err
		}
		if assignment.UserID == actorID && assignment.Role.ScopeType == models.RoleScopeGlobal {
			return errors.New("you cannot remove your own global role assignment")
		}
		allowed, err := actorCanAssignRBACRole(tx, actorID, assignment.Role)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("you cannot remove a role at or above your privilege level")
		}
		return tx.Model(&assignment).Update("is_active", false).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeRBACError(w, http.StatusNotFound, "active assignment not found")
		return
	}
	if err != nil {
		writeRBACError(w, http.StatusBadRequest, err.Error())
		return
	}
	middleware.InvalidateUserCache(targetUserID.String())
	InvalidateAdminUsersCache()
	writeRBACJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func CurrentUserRBACAssignments(user models.User) []RBACAssignmentResponse {
	responses := make([]RBACAssignmentResponse, 0, len(user.RoleAssignments))
	for _, assignment := range user.RoleAssignments {
		if assignment.IsActive && assignment.Role.IsActive {
			responses = append(responses, assignmentResponseFromModel(assignment))
		}
	}
	return responses
}
