package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// GetBusinessSiteReports returns site reports filtered by business vertical
func GetBusinessSiteReports(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
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

	params.Filters["businessVerticalId"] = businessID.String()

	service := models.NewReportService(config.DB, models.DprSite{})
	response, err := service.GetReport(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateBusinessSiteReport creates a site report within business context
func CreateBusinessSiteReport(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var report models.DprSite
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	report.BusinessVerticalID = businessID
	user := middleware.GetUser(r)
	if report.InformationEnteredBy == "" {
		report.InformationEnteredBy = user.Name
	}
	if report.PhoneNumberOfInformationEnteredPerson == "" {
		report.PhoneNumberOfInformationEnteredPerson = user.Phone
	}

	if err := config.DB.Create(&report).Error; err != nil {
		http.Error(w, "failed to create site report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(report)
}

// GetBusinessMaterials returns materials filtered by business vertical
func GetBusinessMaterials(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
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

	params.Filters["businessVerticalId"] = businessID.String()

	service := models.NewReportService(config.DB, models.Material{})
	response, err := service.GetReport(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateBusinessMaterial creates a material within business context
func CreateBusinessMaterial(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var item models.Material
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	item.BusinessVerticalID = businessID
	user := middleware.GetUser(r)
	if item.SiteEngineerName == "" {
		item.SiteEngineerName = user.Name
	}
	if item.PhoneNumber == "" {
		item.PhoneNumber = user.Phone
	}

	if err := config.DB.Create(&item).Error; err != nil {
		http.Error(w, "failed to create material report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

// GetBusinessAnalytics returns analytics for a specific business vertical
func GetBusinessAnalytics(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	context := middleware.GetUserBusinessContext(r)

	now := time.Now()
	startCurrentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	startPreviousMonth := startCurrentMonth.AddDate(0, -1, 0)

	var totalSiteReports int64
	var totalMaterials int64
	var activeSites int64
	var activeUsers int64
	var currentMonthSiteReports int64
	var previousMonthSiteReports int64
	var currentMonthMaterials int64
	var previousMonthMaterials int64

	config.DB.Model(&models.DprSite{}).Where("business_vertical_id = ?", businessID).Count(&totalSiteReports)
	config.DB.Model(&models.Material{}).Where("business_vertical_id = ?", businessID).Count(&totalMaterials)
	config.DB.Model(&models.Site{}).Where("business_vertical_id = ? AND is_active = ?", businessID, true).Count(&activeSites)

	config.DB.Table("user_business_roles").
		Joins("JOIN business_roles ON business_roles.id = user_business_roles.business_role_id").
		Where("business_roles.business_vertical_id = ? AND user_business_roles.is_active = ?", businessID, true).
		Distinct("user_business_roles.user_id").
		Count(&activeUsers)

	config.DB.Model(&models.DprSite{}).
		Where("business_vertical_id = ? AND created_at >= ?", businessID, startCurrentMonth).
		Count(&currentMonthSiteReports)
	config.DB.Model(&models.DprSite{}).
		Where("business_vertical_id = ? AND created_at >= ? AND created_at < ?", businessID, startPreviousMonth, startCurrentMonth).
		Count(&previousMonthSiteReports)

	config.DB.Model(&models.Material{}).
		Where("business_vertical_id = ? AND created_at >= ?", businessID, startCurrentMonth).
		Count(&currentMonthMaterials)
	config.DB.Model(&models.Material{}).
		Where("business_vertical_id = ? AND created_at >= ? AND created_at < ?", businessID, startPreviousMonth, startCurrentMonth).
		Count(&previousMonthMaterials)

	currentMonthTotal := currentMonthSiteReports + currentMonthMaterials
	previousMonthTotal := previousMonthSiteReports + previousMonthMaterials

	monthlyGrowth := float64(0)
	if previousMonthTotal > 0 {
		monthlyGrowth = (float64(currentMonthTotal-previousMonthTotal) / float64(previousMonthTotal)) * 100
	}

	response := map[string]interface{}{
		"message":      "Business analytics",
		"business_id":  businessID,
		"user_context": context,
		"analytics": map[string]interface{}{
			"total_reports":      totalSiteReports + totalMaterials,
			"total_site_reports": totalSiteReports,
			"total_materials":    totalMaterials,
			"active_users":       activeUsers,
			"active_sites":       activeSites,
			"monthly_growth":     monthlyGrowth,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Solar Farm specific handlers
func GetSolarGeneration(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	
	response := map[string]interface{}{
		"message":     "Solar generation data",
		"business_id": businessID,
		"data": map[string]interface{}{
			"current_generation": "1250 kW",
			"daily_total":       "28.5 MWh",
			"efficiency":        "94.2%",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetSolarPanels(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	
	response := map[string]interface{}{
		"message":     "Solar panel information",
		"business_id": businessID,
		"data": []map[string]interface{}{
			{"panel_id": "SP001", "status": "active", "efficiency": "95.1%"},
			{"panel_id": "SP002", "status": "maintenance", "efficiency": "0%"},
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetSolarMaintenance(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	
	response := map[string]interface{}{
		"message":     "Solar maintenance records",
		"business_id": businessID,
		"data": []map[string]interface{}{
			{"task": "Panel cleaning", "status": "scheduled", "date": "2025-10-16"},
			{"task": "Inverter check", "status": "completed", "date": "2025-10-14"},
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Water Works specific handlers
func GetWaterConsumption(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	
	response := map[string]interface{}{
		"message":     "Water consumption data",
		"business_id": businessID,
		"data": map[string]interface{}{
			"daily_consumption":   "2.5M liters",
			"peak_hour_usage":    "150K liters/hour",
			"efficiency_rating":   "87.3%",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetWaterSupply(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	
	response := map[string]interface{}{
		"message":     "Water supply information",
		"business_id": businessID,
		"data": map[string]interface{}{
			"reservoir_level": "78%",
			"pump_status":     "operational",
			"pressure":        "4.2 bar",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetWaterQuality(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	
	response := map[string]interface{}{
		"message":     "Water quality reports",
		"business_id": businessID,
		"data": []map[string]interface{}{
			{"parameter": "pH", "value": "7.2", "status": "normal"},
			{"parameter": "Chlorine", "value": "0.8 ppm", "status": "normal"},
			{"parameter": "Turbidity", "value": "0.3 NTU", "status": "excellent"},
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}