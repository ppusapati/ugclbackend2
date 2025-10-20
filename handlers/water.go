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
	params, err := models.ParseReportParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := params.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	service := models.NewReportService(config.DB, models.Water{})
	// Add business filter to query
	params.Filters["business_vertical_id"] = businessID.String()
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
	json.NewDecoder(r.Body).Decode(&item)
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

	config.DB.Create(&item)
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

	json.NewDecoder(r.Body).Decode(&item)
	// Ensure business_vertical_id cannot be changed
	item.BusinessVerticalID = businessID
	config.DB.Save(&item)
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
