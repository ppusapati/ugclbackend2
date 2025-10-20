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

// GetAllWaterTankerReportsWithSiteFilter - Enhanced version with site-level filtering
// This replaces the original GetAllWaterTankerReports when site access control is enabled
func GetAllWaterTankerReportsWithSiteFilter(w http.ResponseWriter, r *http.Request) {
	// Get business context (contains permissions, roles, business ID)
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

	// Get site access context
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext == nil {
		http.Error(w, "site access context not found", http.StatusForbidden)
		return
	}

	// Get accessible site codes/names
	var accessibleSiteNames []string
	if len(siteContext.AccessibleSiteIDs) > 0 {
		var sites []models.Site
		if err := config.DB.Where("id IN ?", siteContext.AccessibleSiteIDs).Find(&sites).Error; err != nil {
			http.Error(w, "failed to fetch accessible sites", http.StatusInternalServerError)
			return
		}

		for _, site := range sites {
			accessibleSiteNames = append(accessibleSiteNames, site.Name)
		}
	}

	// If user has no accessible sites, return empty result
	if len(accessibleSiteNames) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]models.Water{})
		return
	}

	// Parse report parameters
	params, err := models.ParseReportParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := params.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Add business and site filters to query
	params.Filters["business_vertical_id"] = businessID.String()

	// Filter by accessible sites only
	service := models.NewReportService(config.DB, models.Water{})

	// Build query with site filter
	query := config.DB.Model(&models.Water{}).
		Where("business_vertical_id = ?", businessID).
		Where("site_name IN ?", accessibleSiteNames)

	// Apply additional filters from params
	for key, value := range params.Filters {
		if key != "business_vertical_id" {
			query = query.Where(key+" = ?", value)
		}
	}

	// Apply sorting
	if params.SortBy != "" {
		order := params.SortBy
		if params.SortOrder == "desc" {
			order += " DESC"
		} else {
			order += " ASC"
		}
		query = query.Order(order)
	}

	// Apply pagination
	var total int64
	query.Count(&total)

	var records []models.Water
	if err := query.
		Offset(params.Offset).
		Limit(params.Limit).
		Find(&records).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"data":       records,
		"total":      total,
		"page":       params.Page,
		"pageSize":   params.Limit,
		"totalPages": (total + int64(params.Limit) - 1) / int64(params.Limit),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateWaterTankerReportWithSiteCheck - Enhanced version with site-level access check
func CreateWaterTankerReportWithSiteCheck(w http.ResponseWriter, r *http.Request) {
	var item models.Water
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user := middleware.GetUser(r)
	if user == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	item.SiteEngineerName = user.Name
	item.SiteEngineerPhone = user.Phone

	// Get business context and set business_vertical_id
	businessContext := middleware.GetUserBusinessContext(r)
	if businessContext != nil {
		if businessID, ok := businessContext["business_id"].(uuid.UUID); ok {
			item.BusinessVerticalID = businessID
		}
	}

	// Verify user has access to the site they're creating a report for
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext == nil {
		http.Error(w, "site access context not found", http.StatusForbidden)
		return
	}

	// Find the site by name
	var site models.Site
	if err := config.DB.Where("name = ? AND business_vertical_id = ?", item.SiteName, item.BusinessVerticalID).First(&site).Error; err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	// Check if user can create in this site
	hasAccess := false
	canCreate := false
	for _, siteID := range siteContext.AccessibleSiteIDs {
		if siteID == site.ID {
			hasAccess = true
			if perm, ok := siteContext.SitePermissions[siteID]; ok {
				canCreate = perm.CanCreate
			}
			break
		}
	}

	if !hasAccess {
		http.Error(w, "no access to this site", http.StatusForbidden)
		return
	}

	if !canCreate {
		http.Error(w, "no create permission for this site", http.StatusForbidden)
		return
	}

	// Create the record
	if err := config.DB.Create(&item).Error; err != nil {
		http.Error(w, "failed to create record", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

// UpdateWaterTankerReportWithSiteCheck - Enhanced version with site-level access check
func UpdateWaterTankerReportWithSiteCheck(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	// Get business context
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

	// Get existing record
	var item models.Water
	result := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item)
	if result.Error != nil {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	// Find the site
	var site models.Site
	if err := config.DB.Where("name = ? AND business_vertical_id = ?", item.SiteName, businessID).First(&site).Error; err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	// Check if user can update in this site
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext == nil {
		http.Error(w, "site access context not found", http.StatusForbidden)
		return
	}

	canUpdate := false
	for _, siteID := range siteContext.AccessibleSiteIDs {
		if siteID == site.ID {
			if perm, ok := siteContext.SitePermissions[siteID]; ok {
				canUpdate = perm.CanUpdate
			}
			break
		}
	}

	if !canUpdate {
		http.Error(w, "no update permission for this site", http.StatusForbidden)
		return
	}

	// Decode the update
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure business_vertical_id cannot be changed
	item.BusinessVerticalID = businessID

	if err := config.DB.Save(&item).Error; err != nil {
		http.Error(w, "failed to update record", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

// DeleteWaterTankerReportWithSiteCheck - Enhanced version with site-level access check
func DeleteWaterTankerReportWithSiteCheck(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	// Get business context
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

	// Get existing record to check site
	var item models.Water
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	// Find the site
	var site models.Site
	if err := config.DB.Where("name = ? AND business_vertical_id = ?", item.SiteName, businessID).First(&site).Error; err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	// Check if user can delete in this site
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext == nil {
		http.Error(w, "site access context not found", http.StatusForbidden)
		return
	}

	canDelete := false
	for _, siteID := range siteContext.AccessibleSiteIDs {
		if siteID == site.ID {
			if perm, ok := siteContext.SitePermissions[siteID]; ok {
				canDelete = perm.CanDelete
			}
			break
		}
	}

	if !canDelete {
		http.Error(w, "no delete permission for this site", http.StatusForbidden)
		return
	}

	// Delete the record
	result := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).Delete(&models.Water{})
	if result.Error != nil {
		http.Error(w, "failed to delete record", http.StatusInternalServerError)
		return
	}
	if result.RowsAffected == 0 {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
