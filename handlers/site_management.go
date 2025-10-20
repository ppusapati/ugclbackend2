package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// GetAllSites returns all sites for a business vertical
func GetAllSites(w http.ResponseWriter, r *http.Request) {
	businessContext := middleware.GetUserBusinessContext(r)
	if businessContext == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := businessContext["business_id"].(uuid.UUID)
	if !ok {
		http.Error(w, "invalid business context", http.StatusInternalServerError)
		return
	}

	var sites []models.Site
	if err := config.DB.Where("business_vertical_id = ? AND is_active = ?", businessID, true).Find(&sites).Error; err != nil {
		http.Error(w, "failed to fetch sites", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sites)
}

// GetUserSites returns all sites the current user has access to
func GetUserSites(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	businessContext := middleware.GetUserBusinessContext(r)
	if businessContext == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := businessContext["business_id"].(uuid.UUID)
	if !ok {
		http.Error(w, "invalid business context", http.StatusInternalServerError)
		return
	}

	// Get sites with user's access information
	type SiteWithAccess struct {
		models.Site
		CanRead   bool `json:"canRead"`
		CanCreate bool `json:"canCreate"`
		CanUpdate bool `json:"canUpdate"`
		CanDelete bool `json:"canDelete"`
	}

	var result []SiteWithAccess
	err := config.DB.Table("sites").
		Select("sites.*, user_site_accesses.can_read, user_site_accesses.can_create, user_site_accesses.can_update, user_site_accesses.can_delete").
		Joins("JOIN user_site_accesses ON user_site_accesses.site_id = sites.id").
		Where("user_site_accesses.user_id = ? AND sites.business_vertical_id = ? AND sites.is_active = ?",
			user.ID, businessID, true).
		Scan(&result).Error

	if err != nil {
		http.Error(w, "failed to fetch user sites", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// AssignUserSiteAccessRequest represents the request body for assigning site access
type AssignUserSiteAccessRequest struct {
	UserID    uuid.UUID `json:"userId"`
	SiteID    uuid.UUID `json:"siteId"`
	CanRead   bool      `json:"canRead"`
	CanCreate bool      `json:"canCreate"`
	CanUpdate bool      `json:"canUpdate"`
	CanDelete bool      `json:"canDelete"`
}

// AssignUserSiteAccess assigns or updates a user's access to a site
// Only business admins or users with business_manage_users permission can do this
func AssignUserSiteAccess(w http.ResponseWriter, r *http.Request) {
	var req AssignUserSiteAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	currentUser := middleware.GetUser(r)
	if currentUser == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Verify the site exists and belongs to the current business vertical
	businessContext := middleware.GetUserBusinessContext(r)
	if businessContext == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := businessContext["business_id"].(uuid.UUID)
	if !ok {
		http.Error(w, "invalid business context", http.StatusInternalServerError)
		return
	}

	var site models.Site
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", req.SiteID, businessID).First(&site).Error; err != nil {
		http.Error(w, "site not found or does not belong to this business", http.StatusNotFound)
		return
	}

	// Check if access already exists
	var existing models.UserSiteAccess
	err := config.DB.Where("user_id = ? AND site_id = ?", req.UserID, req.SiteID).First(&existing).Error

	if err != nil {
		// Create new access
		access := models.UserSiteAccess{
			UserID:     req.UserID,
			SiteID:     req.SiteID,
			CanRead:    req.CanRead,
			CanCreate:  req.CanCreate,
			CanUpdate:  req.CanUpdate,
			CanDelete:  req.CanDelete,
			AssignedBy: &currentUser.ID,
		}

		if err := config.DB.Create(&access).Error; err != nil {
			http.Error(w, "failed to create site access", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(access)
	} else {
		// Update existing access
		existing.CanRead = req.CanRead
		existing.CanCreate = req.CanCreate
		existing.CanUpdate = req.CanUpdate
		existing.CanDelete = req.CanDelete
		existing.AssignedBy = &currentUser.ID

		if err := config.DB.Save(&existing).Error; err != nil {
			http.Error(w, "failed to update site access", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(existing)
	}
}

// RevokeUserSiteAccess removes a user's access to a site
func RevokeUserSiteAccess(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	accessID := vars["accessId"]

	if err := config.DB.Delete(&models.UserSiteAccess{}, "id = ?", accessID).Error; err != nil {
		http.Error(w, "failed to revoke site access", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetSiteUsers returns all users with access to a specific site
func GetSiteUsers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	siteID := vars["siteId"]

	type UserAccess struct {
		UserID    uuid.UUID `json:"userId"`
		Name      string    `json:"name"`
		Phone     string    `json:"phone"`
		CanRead   bool      `json:"canRead"`
		CanCreate bool      `json:"canCreate"`
		CanUpdate bool      `json:"canUpdate"`
		CanDelete bool      `json:"canDelete"`
	}

	var users []UserAccess
	err := config.DB.Table("user_site_accesses").
		Select("users.id as user_id, users.name, users.phone, user_site_accesses.can_read, user_site_accesses.can_create, user_site_accesses.can_update, user_site_accesses.can_delete").
		Joins("JOIN users ON users.id = user_site_accesses.user_id").
		Where("user_site_accesses.site_id = ?", siteID).
		Scan(&users).Error

	if err != nil {
		http.Error(w, "failed to fetch site users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
