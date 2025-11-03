package routes

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
)

// RegisterReportRoutes registers all report builder routes using Mux
func RegisterReportRoutes(r *mux.Router) {
	// Report Builder API v1 - Protected routes
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.JWTMiddleware)

	// Report Definitions
	api.HandleFunc("/reports/definitions", handlers.CreateReportDefinition).Methods("POST")
	api.HandleFunc("/reports/definitions", handlers.GetReportDefinitions).Methods("GET")
	api.HandleFunc("/reports/definitions/{id}", handlers.GetReportDefinition).Methods("GET")
	api.HandleFunc("/reports/definitions/{id}", handlers.UpdateReportDefinition).Methods("PUT")
	api.HandleFunc("/reports/definitions/{id}", handlers.DeleteReportDefinition).Methods("DELETE")
	api.HandleFunc("/reports/definitions/{id}/clone", handlers.CloneReport).Methods("POST")
	api.HandleFunc("/reports/definitions/{id}/favorite", handlers.ToggleFavoriteReport).Methods("POST")

	// Report Execution
	api.HandleFunc("/reports/definitions/{id}/execute", handlers.ExecuteReport).Methods("POST")
	api.HandleFunc("/reports/definitions/{id}/history", handlers.GetReportExecutionHistory).Methods("GET")

	// Report Export
	api.HandleFunc("/reports/definitions/{id}/export/excel", handlers.ExportReportToExcel).Methods("GET")
	api.HandleFunc("/reports/definitions/{id}/export/csv", handlers.ExportReportToCSV).Methods("GET")
	api.HandleFunc("/reports/definitions/{id}/export/pdf", handlers.ExportReportToPDF).Methods("GET")

	// Form Table Schema Discovery
	api.HandleFunc("/reports/forms/tables", handlers.GetAvailableFormTables).Methods("GET")
	api.HandleFunc("/reports/forms/tables/{table_name}/fields", handlers.GetFormTableFields).Methods("GET")

	// Dashboards
	api.HandleFunc("/dashboards", handlers.CreateDashboard).Methods("POST")
	api.HandleFunc("/dashboards", handlers.GetDashboards).Methods("GET")
	api.HandleFunc("/dashboards/{id}", handlers.GetDashboard).Methods("GET")
	api.HandleFunc("/dashboards/{id}/widgets", handlers.AddWidgetToDashboard).Methods("POST")
	api.HandleFunc("/dashboards/{id}/widgets/{widget_id}", handlers.RemoveWidgetFromDashboard).Methods("DELETE")

	// Report Templates
	api.HandleFunc("/report-templates", handlers.GetReportTemplates).Methods("GET")
	api.HandleFunc("/report-templates/{template_id}/create", handlers.CreateReportFromTemplate).Methods("POST")

	// Scheduled Reports (Admin only - add permission middleware if needed)
	api.HandleFunc("/scheduled-reports", getScheduledReportsHandler).Methods("GET")
	api.HandleFunc("/scheduled-reports/{id}/schedule", scheduleReportHandler).Methods("POST")
	api.HandleFunc("/scheduled-reports/{id}/unschedule", unscheduleReportHandler).Methods("POST")
	api.HandleFunc("/scheduled-reports/{id}/execute-now", executeReportNowHandler).Methods("POST")
}

// Handler wrappers for scheduler

func getScheduledReportsHandler(w http.ResponseWriter, r *http.Request) {
	scheduler := handlers.NewReportScheduler()
	reports, err := scheduler.GetScheduledReports()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"reports": reports,
		"count":   len(reports),
	})
}

func scheduleReportHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]

	var req struct {
		Frequency     string   `json:"frequency"`
		Time          string   `json:"time"`
		DayOfWeek     int      `json:"day_of_week"`
		DayOfMonth    int      `json:"day_of_month"`
		Timezone      string   `json:"timezone"`
		Recipients    []string `json:"recipients"`
		ExportFormats []string `json:"export_formats"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	scheduler := handlers.NewReportScheduler()
	reportUUID, _ := uuid.Parse(reportID)

	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	err := scheduler.ScheduleReport(
		reportUUID,
		req.Frequency,
		req.Time,
		req.DayOfWeek,
		req.DayOfMonth,
		req.Timezone,
		req.Recipients,
		req.ExportFormats,
	)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Report scheduled successfully",
	})
}

func unscheduleReportHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]

	scheduler := handlers.NewReportScheduler()
	reportUUID, _ := uuid.Parse(reportID)

	err := scheduler.UnscheduleReport(reportUUID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Report unscheduled successfully",
	})
}

func executeReportNowHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	scheduler := handlers.NewReportScheduler()
	reportUUID, _ := uuid.Parse(reportID)

	err := scheduler.ExecuteReportNow(reportUUID, userID.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Report executed successfully",
	})
}
