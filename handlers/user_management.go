package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

type updateUserReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	RoleID   string `json:"role_id"`
	IsActive *bool  `json:"is_active"`
}

// UpdateUser allows admins to update user information
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// Parse UUID
	id, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	var req updateUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Get existing user
	var user models.User
	if err := config.DB.First(&user, "id = ?", id).Error; err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Update fields
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.RoleID != "" {
		roleID, err := uuid.Parse(req.RoleID)
		if err != nil {
			http.Error(w, "invalid role ID", http.StatusBadRequest)
			return
		}
		user.RoleID = &roleID
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	// Save changes
	if err := config.DB.Save(&user).Error; err != nil {
		http.Error(w, "failed to update user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated user (without password hash)
	user.PasswordHash = ""
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// DeleteUser allows admins to soft delete users
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// Parse UUID
	id, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Check if user exists
	var user models.User
	if err := config.DB.First(&user, "id = ?", id).Error; err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Prevent self-deletion
	currentUser := middleware.GetClaims(r)
	if currentUser.UserID == userID {
		http.Error(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Soft delete (set IsActive to false)
	user.IsActive = false
	if err := config.DB.Save(&user).Error; err != nil {
		http.Error(w, "failed to delete user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type changePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword allows users to change their own password
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Get current user
	claims := middleware.GetClaims(r)
	var user models.User
	if err := config.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "error hashing password", http.StatusInternalServerError)
		return
	}

	// Update password
	user.PasswordHash = string(hash)
	if err := config.DB.Save(&user).Error; err != nil {
		http.Error(w, "failed to update password: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "password updated successfully"})
}

func GetbyID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// Parse UUID
	id, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Get user
	var user models.User
	if err := config.DB.First(&user, "id = ?", id).Error; err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Return user (without password hash)
	user.PasswordHash = ""
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
