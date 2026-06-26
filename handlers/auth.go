// handlers/auth.go
package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/singleflight"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"
)

const adminUsersCacheTTL = 10 * time.Minute
const userRegistrationBcryptCost = 12

type adminUsersCacheEntry struct {
	payload   []byte
	expiresAt time.Time
}

type adminUsersCacheStore struct {
	mu      sync.Mutex // get() deletes expired entries so always needs the write lock; Mutex is correct.
	entries map[string]adminUsersCacheEntry
}

var adminUsersCache = &adminUsersCacheStore{entries: make(map[string]adminUsersCacheEntry)}
var adminUsersLoadGroup singleflight.Group

func adminUsersCacheKey(page, limit int) string {
	return strconv.Itoa(page) + ":" + strconv.Itoa(limit)
}

func (c *adminUsersCacheStore) get(key string) ([]byte, bool) {
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

func (c *adminUsersCacheStore) set(key string, payload []byte) {
	c.mu.Lock()
	c.entries[key] = adminUsersCacheEntry{payload: payload, expiresAt: time.Now().Add(adminUsersCacheTTL)}
	c.mu.Unlock()
}

func InvalidateAdminUsersCache() {
	adminUsersCache.mu.Lock()
	clear(adminUsersCache.entries)
	adminUsersCache.mu.Unlock()
}

type loginPayload struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type registerReq struct {
	Name     string     `json:"name"`
	Email    string     `json:"email"`
	Phone    string     `json:"phone"`
	Password string     `json:"password"`
	RoleID   *uuid.UUID `json:"role_id"` // Global role ID (optional)
}

func Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	// hash pw
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), userRegistrationBcryptCost)
	if err != nil {
		http.Error(w, "error hashing password", http.StatusInternalServerError)
		return
	}
	u := models.User{
		Name:         req.Name,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
		RoleID:       req.RoleID,
	}
	if err := config.DB.Create(&u).Error; err != nil {
		if utils.IsUniqueViolation(err) {
			http.Error(w, "username already taken", http.StatusConflict)
		} else {
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusCreated)
}

type loginReq struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type loginResp struct {
	Token string      `json:"token"`
	User  userPayload `json:"user"`
}
type userPayload struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Phone        string     `json:"phone"`
	RoleID       *uuid.UUID `json:"role_id"`
	Role         string     `json:"role"`
	IsSuperAdmin bool       `json:"is_super_admin"`
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Keep login lookup minimal and index-friendly: avoid implicit ORDER BY from First().
	var u models.User
	if err := config.DB.
		Select("id", "name", "email", "phone", "password_hash", "role_id").
		Where("phone = ?", req.Phone).
		Take(&u).Error; err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Determine role name for token
	roleName := "user" // default
	if u.RoleID != nil {
		var role models.Role
		if err := config.DB.Select("name").Where("id = ?", *u.RoleID).Take(&role).Error; err == nil {
			roleName = role.Name
		}
	}

	token, err := middleware.GenerateToken(u.ID.String(), roleName, u.Name, u.Phone)
	if err != nil {
		http.Error(w, "couldn't create token", http.StatusInternalServerError)
		return
	}
	u.PasswordHash = "" // don't leak password hash

	// Check if user is super admin
	isSuperAdmin := roleName == "super_admin"

	loginEvent := models.UserLoginEvent{
		UserID:    u.ID,
		LoginAt:   time.Now().UTC(),
		IPAddress: clientIPFromRequest(r),
		UserAgent: strings.TrimSpace(r.UserAgent()),
	}
	if err := config.DB.Create(&loginEvent).Error; err != nil {
		http.Error(w, "failed to persist login audit event", http.StatusInternalServerError)
		return
	}

	out := loginResp{
		Token: token,
		User: userPayload{
			ID:           u.ID,
			Name:         u.Name,
			Email:        u.Email,
			Phone:        u.Phone,
			RoleID:       u.RoleID,
			Role:         roleName,
			IsSuperAdmin: isSuperAdmin,
		},
	}
	json.NewEncoder(w).Encode(out)
}

func clientIPFromRequest(r *http.Request) string {
	xForwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xForwardedFor != "" {
		parts := strings.Split(xForwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	xRealIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xRealIP != "" {
		return xRealIP
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	authService := middleware.NewAuthService()
	userCtx, err := authService.LoadUserContext(r)
	if err != nil || userCtx == nil || userCtx.User == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	user := userCtx.User

	// Build effective permission list without triggering a second auth context load.
	permissions := []string{}
	permissionSet := make(map[string]struct{})
	appendPermission := func(permission string) {
		if permission == "" {
			return
		}
		if _, exists := permissionSet[permission]; exists {
			return
		}
		permissionSet[permission] = struct{}{}
		permissions = append(permissions, permission)
	}

	for _, perm := range userCtx.GlobalPermissions {
		appendPermission(perm)
	}

	if userCtx.BusinessContext != nil {
		for _, perm := range userCtx.BusinessContext.Permissions {
			appendPermission(perm)
		}
	}

	if userCtx.IsSuperAdmin {
		appendPermission("*:*:*")
	}

	businessRoles := []map[string]interface{}{}
	for _, ubr := range user.UserBusinessRoles {
		if ubr.IsActive && ubr.BusinessRole.ID != uuid.Nil {
			businessRoles = append(businessRoles, map[string]interface{}{
				"role_id":       ubr.BusinessRole.ID,
				"role_name":     ubr.BusinessRole.DisplayName,
				"vertical_id":   ubr.BusinessRole.BusinessVerticalID,
				"vertical_name": ubr.BusinessRole.BusinessVertical.Name,
				"vertical_code": ubr.BusinessRole.BusinessVertical.Code,
				"level":         ubr.BusinessRole.Level,
			})
		}
	}

	// 5) Return enhanced user info
	var globalRoleName string
	if user.RoleModel != nil {
		globalRoleName = user.RoleModel.Name
	}

	resp := map[string]interface{}{
		"id":             user.ID,
		"name":           user.Name,
		"phone":          user.Phone,
		"email":          user.Email,
		"role_id":        user.RoleID,
		"global_role":    globalRoleName,
		"permissions":    permissions,
		"business_roles": businessRoles,
	}
	json.NewEncoder(w).Encode(resp)
}

func GetAllUsers(w http.ResponseWriter, r *http.Request) {
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
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit
	cacheKey := adminUsersCacheKey(page, limit)

	if payload, ok := adminUsersCache.get(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
		return
	}

	loaded, err, _ := adminUsersLoadGroup.Do(cacheKey, func() (interface{}, error) {
		if payload, ok := adminUsersCache.get(cacheKey); ok {
			return payload, nil
		}

		var users []models.User
		if err := config.DB.
			Preload("RoleModel").
			Preload("BusinessVertical").
			Preload("UserBusinessRoles", "is_active = ?", true).
			Preload("UserBusinessRoles.BusinessRole", "is_active = ?", true).
			Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
			Where("is_active = ?", true).
			Limit(limit).
			Offset(offset).
			Find(&users).Error; err != nil {
			return nil, err
		}

		var total int64
		if err := config.DB.
			Model(&models.User{}).
			Where("is_active = ?", true).
			Count(&total).Error; err != nil {
			return nil, err
		}

		out := make([]adminUserOut, len(users))
		for i, u := range users {
			out[i] = buildAdminUserResponse(u)
		}

		response := map[string]interface{}{
			"total": total,
			"page":  page,
			"limit": limit,
			"data":  out,
		}
		payload, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			return nil, marshalErr
		}

		adminUsersCache.set(cacheKey, payload)
		return payload, nil
	})
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(loaded.([]byte))
}

// TestAuthEndpoint provides information about the current user's authentication status
func TestAuthEndpoint(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	user := middleware.GetUser(r)
	permissions := middleware.GetUserPermissions(r)

	response := map[string]interface{}{
		"authenticated": true,
		"user_id":       claims.UserID,
		"name":          claims.Name,
		"phone":         claims.Phone,
		"role":          claims.Role,
		"user_details":  user,
		"permissions":   permissions,
		"token_valid":   true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// TestPermissionEndpoint tests if user has specific permissions
func TestPermissionEndpoint(w http.ResponseWriter, r *http.Request) {
	permission := r.URL.Query().Get("permission")
	if permission == "" {
		http.Error(w, "permission parameter required", http.StatusBadRequest)
		return
	}

	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	permissions := middleware.GetUserPermissions(r)
	hasPermission := false
	for _, p := range permissions {
		if p == permission {
			hasPermission = true
			break
		}
	}

	response := map[string]interface{}{
		"user_id":         claims.UserID,
		"role":            claims.Role,
		"permission":      permission,
		"has_permission":  hasPermission,
		"all_permissions": permissions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
