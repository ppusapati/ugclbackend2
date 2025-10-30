package masters

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// GetAllSites returns all sites irrespective of business vertical (Admin only)
func GetAllSites(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	limit := 100 // Default limit for sites

	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	offset := (page - 1) * limit

	// Get total count of all active sites
	var total int64
	config.DB.Model(&models.Site{}).Where("sites.is_active = ?", true).Count(&total)

	// Use JOIN to get sites with business vertical name in a single query
	type SiteWithBusinessVertical struct {
		models.Site
		BusinessVerticalName string `json:"business_vertical_name"`
		BusinessVerticalCode string `json:"business_vertical_code"`
	}

	var sites []SiteWithBusinessVertical
	err := config.DB.Table("sites").
		Select("sites.*, business_verticals.name as business_vertical_name, business_verticals.code as business_vertical_code").
		Joins("LEFT JOIN business_verticals ON sites.business_vertical_id = business_verticals.id").
		Where("sites.is_active = ?", true).
		Order("sites.created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&sites).Error

	if err != nil {
		http.Error(w, "failed to fetch sites", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  sites,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetSiteByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	siteID := vars["siteId"]
	var site models.Site
	if err := config.DB.Where("id = ? AND is_active = ?", siteID, true).First(&site).Error; err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(site)
}

func CreateSite(w http.ResponseWriter, r *http.Request) {
	var site models.Site
	fmt.Println(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&site); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	fmt.Println(site)
	if err := config.DB.Create(&site).Error; err != nil {
		fmt.Println(err)
		http.Error(w, "failed to create site", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(site)

}

// GetBusinessSites returns all sites for a specific business vertical
func GetBusinessSites(w http.ResponseWriter, r *http.Request) {
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

	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	limit := 100 // Default limit for sites

	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	offset := (page - 1) * limit

	// Get total count for this business
	var total int64
	config.DB.Model(&models.Site{}).Where("business_vertical_id = ? AND is_active = ?", businessID, true).Count(&total)

	// Get paginated sites for this business
	var sites []models.Site
	if err := config.DB.Where("business_vertical_id = ? AND is_active = ?", businessID, true).
		Limit(limit).
		Offset(offset).
		Find(&sites).Error; err != nil {
		http.Error(w, "failed to fetch sites", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  sites,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUserSites returns all sites the current user has access to
func GetUserSites(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

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

	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	limit := 100 // Default limit for sites

	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	offset := (page - 1) * limit

	// Get total count
	var total int64
	config.DB.Table("user_site_accesses").
		Joins("JOIN sites ON sites.id = user_site_accesses.site_id").
		Where("user_site_accesses.user_id = ? AND sites.business_vertical_id = ? AND sites.is_active = ?",
			user.ID, businessID, true).
		Count(&total)

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
		Limit(limit).
		Offset(offset).
		Scan(&result).Error

	if err != nil {
		http.Error(w, "failed to fetch user sites", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
// Only business admins or users with site:manage_access permission can do this
func AssignUserSiteAccess(w http.ResponseWriter, r *http.Request) {
	var req AssignUserSiteAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get current user ID from JWT claims
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "user claims not found", http.StatusUnauthorized)
		return
	}

	// Parse current user ID
	currentUserID, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user ID in claims", http.StatusInternalServerError)
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
	err = config.DB.Where("user_id = ? AND site_id = ?", req.UserID, req.SiteID).First(&existing).Error

	if err != nil {
		// Create new access
		access := models.UserSiteAccess{
			UserID:     req.UserID,
			SiteID:     req.SiteID,
			CanRead:    req.CanRead,
			CanCreate:  req.CanCreate,
			CanUpdate:  req.CanUpdate,
			CanDelete:  req.CanDelete,
			AssignedBy: &currentUserID,
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
		existing.AssignedBy = &currentUserID

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

	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	limit := 100 // Default limit for users

	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	offset := (page - 1) * limit

	// Get total count
	var total int64
	config.DB.Table("user_site_accesses").
		Where("user_site_accesses.site_id = ?", siteID).
		Count(&total)

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
		Limit(limit).
		Offset(offset).
		Scan(&users).Error

	if err != nil {
		http.Error(w, "failed to fetch site users", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  users,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
