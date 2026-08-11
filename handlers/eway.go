package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gorm.io/gorm/clause"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"

	"github.com/gorilla/mux"
)

func GetAllEways(w http.ResponseWriter, r *http.Request) {
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

	service := models.NewReportService(db, models.Eway{})
	response, err := service.GetReport(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func CreateEway(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	var item models.Eway
	json.NewDecoder(r.Body).Decode(&item)
	user := middleware.GetUser(r)
	item.EnteredBy = user.Name
	db.Create(&item)
	json.NewEncoder(w).Encode(item)
}

func GetEway(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])
	var item models.Eway
	db.First(&item, id)
	json.NewEncoder(w).Encode(item)
}

func UpdateEway(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])
	var item models.Eway
	db.First(&item, id)
	json.NewDecoder(r.Body).Decode(&item)
	db.Save(&item)
	json.NewEncoder(w).Encode(item)
}

func DeleteEway(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])
	db.Delete(&models.Eway{}, id)
	w.WriteHeader(http.StatusNoContent)
}

// BatchContractorReports handles POST /api/v1/contractor/batch
func BatchEwayss(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	var batch []models.Eway
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	user := middleware.GetUser(r)
	for i := range batch {
		batch[i].EnteredBy = user.Name

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
