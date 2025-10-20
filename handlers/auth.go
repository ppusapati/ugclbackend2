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
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Role     string `json:"role"`
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
		Role:         req.Role,
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
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Phone string    `json:"phone"`
	Role  string    `json:"role"`
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	fmt.Println("Login request received")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var u models.User
	if err := config.DB.Where("phone = ?", req.Phone).First(&u).Error; err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := middleware.GenerateToken(u.ID.String(), u.Role, u.Name, u.Phone)
	if err != nil {
		http.Error(w, "couldn't create token", http.StatusInternalServerError)
		return
	}
	u.PasswordHash = "" // don't leak password hash
	out := loginResp{
		Token: token,
		User: userPayload{
			ID:    u.ID,
			Name:  u.Name,
			Email: u.Email,
			Phone: u.Phone,
			Role:  u.Role,
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

	// 3) Fetch user record
	var user models.User
	if err := config.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// 4) Return only the fields you need
	resp := map[string]interface{}{
		"id":    user.ID,
		"name":  user.Name,
		"phone": user.Phone,
		"email": user.Email,
		"roles": user.Role,
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
		Where("is_active = ?", true).
		Where("role <> ?", "Super Admin").
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
		Where("role <> ?", "Super Admin").
		Count(&total).Error; err != nil {
		http.Error(w, "DB count error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type userOut struct {
		ID    uuid.UUID `json:"id"`
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Phone string    `json:"phone"`
		Role  string    `json:"role"`
	}

	out := make([]userOut, len(users))
	for i, u := range users {
		out[i] = userOut{
			ID:    u.ID,
			Name:  u.Name,
			Email: u.Email,
			Phone: u.Phone,
			Role:  u.Role,
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
