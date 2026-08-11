package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"gorm.io/gorm/clause"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

func GetAllSiteEngineerReports(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	params, err := models.ParseReportParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := params.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	service := models.NewReportService(db, models.DprSite{})
	response, err := service.GetReport(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// func GetAllSiteEngineerReports(w http.ResponseWriter, r *http.Request) {
// 	pageStr := r.URL.Query().Get("page")
// 	limitStr := r.URL.Query().Get("limit")
// 	fieldsStr := r.URL.Query().Get("fields")
// 	fromDate := r.URL.Query().Get("fromDate")
// 	toDate := r.URL.Query().Get("toDate")
// 	contractor := r.URL.Query().Get("contractor")
// 	site := r.URL.Query().Get("site")
// 	page := 1
// 	limit := 10

// 	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
// 		page = p
// 	}
// 	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
// 		limit = l
// 	}
// 	offset := (page - 1) * limit

// 	// Build mapping
// 	jsonToDB, err := models.BuildJSONtoDBColumnMap(config.DB, models.DprSite{})
// 	if err != nil {
// 		http.Error(w, "schema error: "+err.Error(), 500)
// 		return
// 	}
// 	db := config.DB.Model(&models.DprSite{})
// 	// Parse fields
// 	var dbFields []string
// 	if fieldsStr != "" {
// 		for _, f := range strings.Split(fieldsStr, ",") {
// 			if dbCol, ok := jsonToDB[f]; ok {
// 				dbFields = append(dbFields, dbCol)
// 			}
// 		}
// 	}

// 	if len(dbFields) > 0 {
// 		db = db.Select(dbFields)
// 	}
// 	dateCol := "created_at" // This matches your DB column for the CreatedAt field

// 	if fromDate != "" && toDate != "" {
// 		db = db.Where(dateCol+" BETWEEN ? AND ?", fromDate, toDate)
// 	} else if fromDate != "" {
// 		db = db.Where(dateCol+" >= ?", fromDate)
// 	} else if toDate != "" {
// 		db = db.Where(dateCol+" <= ?", toDate)
// 	}
// 	if contractor != "" {
// 		db = db.Where("name_of_contractor = ?", contractor)
// 	}
// 	fmt.Println(site)
// 	if site != "" {
// 		db = db.Where("name_of_site = ?", site)
// 	}
// 	rows, err := db.Select(dbFields).Limit(limit).Offset(offset).Rows()
// 	if err != nil {
// 		http.Error(w, "DB fetch error: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer rows.Close()

// 	var results []map[string]interface{}
// 	columns, _ := rows.Columns()
// 	for rows.Next() {
// 		// Create a slice of empty interfaces for the scan target
// 		values := make([]interface{}, len(columns))
// 		valuePtrs := make([]interface{}, len(columns))
// 		for i := range columns {
// 			valuePtrs[i] = &values[i]
// 		}
// 		if err := rows.Scan(valuePtrs...); err != nil {
// 			http.Error(w, "Row scan error: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		rowMap := make(map[string]interface{})
// 		for i, col := range columns {
// 			// Use the JSON field name in the output (for API consistency)
// 			jsonField := col // default to DB column
// 			for k, v := range jsonToDB {
// 				if v == col {
// 					jsonField = k
// 					break
// 				}
// 			}
// 			rowMap[jsonField] = values[i]
// 		}
// 		results = append(results, rowMap)
// 	}

// 	countDB := config.DB.Model(&models.DprSite{})
// 	if fromDate != "" && toDate != "" {
// 		countDB = countDB.Where(dateCol+" BETWEEN ? AND ?", fromDate, toDate)
// 	} else if fromDate != "" {
// 		countDB = countDB.Where(dateCol+" >= ?", fromDate)
// 	} else if toDate != "" {
// 		countDB = countDB.Where(dateCol+" <= ?", toDate)
// 	} else if contractor != "" {
// 		countDB = countDB.Where("name_of_contractor = ?", contractor)
// 	} else if site != "" {
// 		countDB = countDB.Where("name_of_site = ?", site)
// 	}

// 	var total int64
// 	if err := countDB.
// 		Count(&total).Error; err != nil {
// 		http.Error(w, "DB count error: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	response := map[string]interface{}{
// 		"total": total,
// 		"page":  page,
// 		"limit": limit,
// 		"data":  results,
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(response)
// }

// GetAllSiteEngineerReports - Refactored version with ReportParams
// func GetAllSiteEngineerReports(w http.ResponseWriter, r *http.Request) {
// 	// Parse and validate parameters
// 	params, err := models.ParseReportParams(r)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Parameter error: %s", err.Error()), http.StatusBadRequest)
// 		return
// 	}

// 	if err := params.Validate(); err != nil {
// 		http.Error(w, fmt.Sprintf("Validation error: %s", err.Error()), http.StatusBadRequest)
// 		return
// 	}

// 	// Build JSON to DB column mapping
// 	jsonToDB, err := models.BuildJSONtoDBColumnMap(config.DB, models.DprSite{})
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Schema error: %s", err.Error()), http.StatusInternalServerError)
// 		return
// 	}

// 	// Fetch data using the parameters
// 	response, err := models.FetchReportData(params, jsonToDB, &models.DprSite{})
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Database error: %s", err.Error()), http.StatusInternalServerError)
// 		return
// 	}

// 	// Return JSON response
// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(response); err != nil {
// 		http.Error(w, fmt.Sprintf("JSON encoding error: %s", err.Error()), http.StatusInternalServerError)
// 		return
// 	}
// }

func CreateSiteEngineerReport(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	var report models.DprSite
	json.NewDecoder(r.Body).Decode(&report)
	user := middleware.GetUser(r)
	report.InformationEnteredBy = user.Name
	report.PhoneNumberOfInformationEnteredPerson = user.Phone

	db.Create(&report)
	json.NewEncoder(w).Encode(report)
}

func GetSiteEngineerReport(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])
	var report models.DprSite
	db.First(&report, id)
	json.NewEncoder(w).Encode(report)
}

func UpdateSiteEngineerReport(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])
	var report models.DprSite
	db.First(&report, id)
	json.NewDecoder(r.Body).Decode(&report)
	db.Save(&report)
	json.NewEncoder(w).Encode(report)
}

func DeleteSiteEngineerReport(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])
	db.Delete(&models.DprSite{}, id)
	w.WriteHeader(http.StatusNoContent)
}

// BatchContractorReports handles POST /api/v1/contractor/batch
func BatchDprSites(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	var batch []models.DprSite
	// fmt.Println(json.NewDecoder(r.Body).Decode(&batch))
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	user := middleware.GetUser(r)
	for i := range batch {
		batch[i].InformationEnteredBy = user.Name
		batch[i].PhoneNumberOfInformationEnteredPerson = user.Phone
	}
	if err := db.
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
