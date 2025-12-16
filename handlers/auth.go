// handlers/auth.go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

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
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
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
		if strings.Contains(err.Error(), "duplicate key") {
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
	fmt.Println("Login request received")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var u models.User
	if err := config.DB.Preload("RoleModel").Where("phone = ?", req.Phone).First(&u).Error; err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Determine role name for token
	roleName := "user" // default
	if u.RoleModel != nil {
		roleName = u.RoleModel.Name
	}

	token, err := middleware.GenerateToken(u.ID.String(), roleName, u.Name, u.Phone)
	if err != nil {
		http.Error(w, "couldn't create token", http.StatusInternalServerError)
		return
	}
	u.PasswordHash = "" // don't leak password hash

	// Check if user is super admin
	isSuperAdmin := roleName == "super_admin"

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

func GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// 1) Extract token
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "Missing Bearer token", http.StatusUnauthorized)
		return
	}
	tokenString := strings.TrimPrefix(auth, "Bearer ")

	// 2) Parse & validate
	secret := []byte(os.Getenv("JWT_SECRET"))
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	claims := token.Claims.(*models.JWTClaims)

	// 3) Load user with all relationships
	var user models.User
	if err := config.DB.
		Preload("RoleModel.Permissions").
		Preload("UserBusinessRoles.BusinessRole.Permissions").
		Preload("UserBusinessRoles.BusinessRole.BusinessVertical").
		First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// 4) Collect permissions from all sources
	permissions := []string{}
	businessRoles := []map[string]interface{}{}

	// Check for Super Admin wildcard first
	if user.RoleModel != nil && user.RoleModel.Name == "super_admin" {
		permissions = append(permissions, "*:*:*")
	} else {
		// Collect from global role
		if user.RoleModel != nil {
			for _, perm := range user.RoleModel.Permissions {
				permissions = append(permissions, perm.Name)
			}
		}

		// Collect from business roles
		permMap := make(map[string]bool) // To avoid duplicates
		for _, p := range permissions {
			permMap[p] = true
		}

		for _, ubr := range user.UserBusinessRoles {
			if ubr.IsActive && ubr.BusinessRole.ID != uuid.Nil {
				// Add permissions from this business role
				for _, perm := range ubr.BusinessRole.Permissions {
					if !permMap[perm.Name] {
						permissions = append(permissions, perm.Name)
						permMap[perm.Name] = true
					}
				}

				// Build business role info for frontend
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
	offset := (page - 1) * limit

	var users []models.User
	if err := config.DB.
		Preload("RoleModel").
		Where("is_active = ?", true).
		Limit(limit).
		Offset(offset).
		Find(&users).Error; err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var total int64
	if err := config.DB.
		Model(&models.User{}).
		Where("is_active = ?", true).
		Count(&total).Error; err != nil {
		http.Error(w, "DB count error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type userOut struct {
		ID         uuid.UUID  `json:"id"`
		Name       string     `json:"name"`
		Email      string     `json:"email"`
		Phone      string     `json:"phone"`
		RoleID     *uuid.UUID `json:"role_id"`
		GlobalRole string     `json:"global_role"`
		IsActive   bool       `json:"is_active"`
	}

	out := make([]userOut, len(users))
	for i, u := range users {
		globalRoleName := ""
		if u.RoleModel != nil {
			globalRoleName = u.RoleModel.Name
		}
		out[i] = userOut{
			ID:         u.ID,
			Name:       u.Name,
			Email:      u.Email,
			Phone:      u.Phone,
			RoleID:     u.RoleID,
			GlobalRole: globalRoleName,
			IsActive:   u.IsActive,
		}
	}

	response := map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  out,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
