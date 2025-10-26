package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm/clause"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

func GetAllWaterTankerReports(w http.ResponseWriter, r *http.Request) {
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

	// Get site access context for filtering
	siteContext := middleware.GetSiteAccessContext(r)
	var accessibleSiteNames []string

	// If site context exists, filter by accessible sites
	if siteContext != nil && len(siteContext.AccessibleSiteIDs) > 0 {
		// Optimized: Select only names instead of loading full site objects
		config.DB.Model(&models.Site{}).
			Select("name").
			Where("id IN ?", siteContext.AccessibleSiteIDs).
			Pluck("name", &accessibleSiteNames)
	}

	params, err := models.ParseReportParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := params.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Add business filter to query
	params.Filters["business_vertical_id"] = businessID.String()

	// If site filtering is enabled and user has specific sites, add site name filter
	if len(accessibleSiteNames) > 0 {
		// Create site filter string for the report service
		// The report service expects filters as map[string]interface{}
		// For IN queries, we'll need to handle this differently

		// Build custom query with site filtering
		jsonToDB, err := models.BuildJSONtoDBColumnMap(config.DB, models.Water{})
		if err != nil {
			http.Error(w, "failed to build column mapping", http.StatusInternalServerError)
			return
		}

		query := config.DB.Model(&models.Water{}).
			Where("business_vertical_id = ?", businessID).
			Where("site_name IN ?", accessibleSiteNames)

		// Apply date filters
		if params.HasDateFilter() {
			if params.FromDate != "" && params.ToDate != "" {
				query = query.Where(params.DateColumn+" BETWEEN ? AND ?", params.FromDate, params.ToDate)
			} else if params.FromDate != "" {
				query = query.Where(params.DateColumn+" >= ?", params.FromDate)
			} else if params.ToDate != "" {
				query = query.Where(params.DateColumn+" <= ?", params.ToDate)
			}
		}

		// Apply additional filters from params
		for jsonField, value := range params.Filters {
			if jsonField != "business_vertical_id" {
				if dbCol, ok := jsonToDB[jsonField]; ok {
					query = query.Where(dbCol+" = ?", value)
				} else {
					query = query.Where(jsonField+" = ?", value)
				}
			}
		}

		// Get total count
		var total int64
		query.Count(&total)

		// Apply pagination
		var records []models.Water
		if err := query.
			Offset(params.GetOffset()).
			Limit(params.Limit).
			Find(&records).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"data":       records,
			"total":      total,
			"page":       params.Page,
			"limit":      params.Limit,
			"totalPages": (total + int64(params.Limit) - 1) / int64(params.Limit),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// No site filtering - use original service method
	service := models.NewReportService(config.DB, models.Water{})
	response, err := service.GetReport(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func CreateWaterTankerReport(w http.ResponseWriter, r *http.Request) {
	var item models.Water
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user := middleware.GetUser(r)
	item.SiteEngineerName = user.Name
	item.SiteEngineerPhone = user.Phone

	// Get business context and set business_vertical_id
	businessContext := middleware.GetUserBusinessContext(r)
	if businessContext != nil {
		if businessID, ok := businessContext["business_id"].(uuid.UUID); ok {
			item.BusinessVerticalID = businessID
		}
	}

	// Check site access if site context is available
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext != nil {
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
	}

	if err := config.DB.Create(&item).Error; err != nil {
		http.Error(w, "failed to create record", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func GetWaterTankerReport(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	// Get business context to ensure user can only access their business data
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

	var item models.Water
	result := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item)
	if result.Error != nil {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	// Check site access if site context is available
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext != nil {
		// Find the site
		var site models.Site
		if err := config.DB.Where("name = ? AND business_vertical_id = ?", item.SiteName, businessID).First(&site).Error; err != nil {
			http.Error(w, "site not found", http.StatusNotFound)
			return
		}

		// Check if user has access to this site
		hasAccess := false
		for _, siteID := range siteContext.AccessibleSiteIDs {
			if siteID == site.ID {
				hasAccess = true
				break
			}
		}

		if !hasAccess {
			http.Error(w, "no access to this site", http.StatusForbidden)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func UpdateWaterTankerReport(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	// Get business context to ensure user can only update their business data
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

	var item models.Water
	result := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item)
	if result.Error != nil {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	// Check site access if site context is available
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext != nil {
		// Find the site
		var site models.Site
		if err := config.DB.Where("name = ? AND business_vertical_id = ?", item.SiteName, businessID).First(&site).Error; err != nil {
			http.Error(w, "site not found", http.StatusNotFound)
			return
		}

		// Check if user can update in this site
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
	}

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

func DeleteWaterTankerReport(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	// Get business context to ensure user can only delete their business data
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

	// Check site access if site context is available
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext != nil {
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
	}

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

func BatchWaterReports(w http.ResponseWriter, r *http.Request) {
	var batch []models.Water
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	user := middleware.GetUser(r)

	// Get business context
	businessContext := middleware.GetUserBusinessContext(r)
	var businessID uuid.UUID
	if businessContext != nil {
		if bID, ok := businessContext["business_id"].(uuid.UUID); ok {
			businessID = bID
		}
	}

	// Check site access if site context is available
	siteContext := middleware.GetSiteAccessContext(r)
	if siteContext != nil {
		// Build map of accessible site names for quick lookup
		var sites []models.Site
		if err := config.DB.Where("id IN ?", siteContext.AccessibleSiteIDs).Find(&sites).Error; err != nil {
			http.Error(w, "failed to fetch accessible sites", http.StatusInternalServerError)
			return
		}

		siteAccessMap := make(map[string]bool)
		for _, site := range sites {
			// Check if user has create permission for this site
			if perm, ok := siteContext.SitePermissions[site.ID]; ok && perm.CanCreate {
				siteAccessMap[site.Name] = true
			}
		}

		// Validate all batch items have site access
		for i := range batch {
			if !siteAccessMap[batch[i].SiteName] {
				http.Error(w, "no create permission for site: "+batch[i].SiteName, http.StatusForbidden)
				return
			}
		}
	}

	for i := range batch {
		batch[i].SiteEngineerName = user.Name
		batch[i].SiteEngineerPhone = user.Phone
		// Set business_vertical_id for each item
		if businessID != uuid.Nil {
			batch[i].BusinessVerticalID = businessID
		}
	}

	if err := config.DB.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).
		Create(&batch).Error; err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
