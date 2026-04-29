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

	// Report read/write subrouters with permission guards
	reportRead := api.PathPrefix("").Subrouter()
	reportRead.Use(middleware.RequirePermission("report:read"))

	reportWrite := api.PathPrefix("").Subrouter()
	reportWrite.Use(middleware.RequireAnyPermission([]string{"report:create", "report:read"}))

	reportExport := api.PathPrefix("").Subrouter()
	reportExport.Use(middleware.RequirePermission("report:export"))

	dashboardRead := api.PathPrefix("").Subrouter()
	dashboardRead.Use(middleware.RequirePermission("dashboard:view"))

	// Report Definitions – writes require report:read (creator must also be able to read)
	reportRead.HandleFunc("/reports/definitions", handlers.GetReportDefinitions).Methods("GET")
	reportRead.HandleFunc("/reports/definitions/{id}", handlers.GetReportDefinition).Methods("GET")
	reportRead.HandleFunc("/reports/definitions/{id}/clone", handlers.CloneReport).Methods("POST")
	reportRead.HandleFunc("/reports/definitions/{id}/favorite", handlers.ToggleFavoriteReport).Methods("POST")
	reportRead.HandleFunc("/reports/definitions/{id}", handlers.UpdateReportDefinition).Methods("PUT")
	reportRead.HandleFunc("/reports/definitions/{id}", handlers.DeleteReportDefinition).Methods("DELETE")
	reportRead.HandleFunc("/reports/definitions", handlers.CreateReportDefinition).Methods("POST")

	// Report Execution
	reportRead.HandleFunc("/reports/definitions/{id}/execute", handlers.ExecuteReport).Methods("POST")
	reportRead.HandleFunc("/reports/definitions/{id}/history", handlers.GetReportExecutionHistory).Methods("GET")

	// Report Export – requires report:export on top of JWT
	reportExport.HandleFunc("/reports/definitions/{id}/export/excel", handlers.ExportReportToExcel).Methods("GET")
	reportExport.HandleFunc("/reports/definitions/{id}/export/csv", handlers.ExportReportToCSV).Methods("GET")
	reportExport.HandleFunc("/reports/definitions/{id}/export/pdf", handlers.ExportReportToPDF).Methods("GET")

	// Form Table Schema Discovery – anyone with report:read can discover schemas
	reportRead.HandleFunc("/reports/forms/tables", handlers.GetAvailableFormTables).Methods("GET")
	reportRead.HandleFunc("/reports/forms/tables/{table_name}/fields", handlers.GetFormTableFields).Methods("GET")

	// Workflow lifecycle drill-down from report viewer (no extra permission beyond report:read)
	reportRead.HandleFunc("/reports/submissions/{submissionId}/workflow-history", handlers.GetSubmissionWorkflowHistory).Methods("GET")

	// Roles available for report sharing (lightweight list, no manage_roles required)
	reportRead.HandleFunc("/reports/available-roles", handlers.GetReportAvailableRoles).Methods("GET")

	// Dashboards
	dashboardRead.HandleFunc("/dashboards", handlers.CreateDashboard).Methods("POST")
	dashboardRead.HandleFunc("/dashboards", handlers.GetDashboards).Methods("GET")
	dashboardRead.HandleFunc("/dashboards/{id}", handlers.GetDashboard).Methods("GET")
	dashboardRead.HandleFunc("/dashboards/{id}", handlers.DeleteDashboard).Methods("DELETE")
	dashboardRead.HandleFunc("/dashboards/{id}/execute", handlers.ExecuteDashboard).Methods("POST")
	dashboardRead.HandleFunc("/dashboards/{id}/widgets", handlers.AddWidgetToDashboard).Methods("POST")
	dashboardRead.HandleFunc("/dashboards/{id}/widgets/{widget_id}", handlers.RemoveWidgetFromDashboard).Methods("DELETE")

	// Report Templates
	reportRead.HandleFunc("/report-templates", handlers.GetReportTemplates).Methods("GET")
	reportWrite.HandleFunc("/report-templates", handlers.CreateReportTemplate).Methods("POST")
	reportWrite.HandleFunc("/report-templates/{template_id}", handlers.UpdateReportTemplate).Methods("PUT")
	reportRead.HandleFunc("/report-templates/{template_id}/create", handlers.CreateReportFromTemplate).Methods("POST")

	// Scheduled Reports – requires report:read; schedule-mutating actions additionally require report:export
	reportRead.HandleFunc("/scheduled-reports", getScheduledReportsHandler).Methods("GET")
	reportExport.HandleFunc("/scheduled-reports/{id}/schedule", scheduleReportHandler).Methods("POST")
	reportExport.HandleFunc("/scheduled-reports/{id}/unschedule", unscheduleReportHandler).Methods("POST")
	reportExport.HandleFunc("/scheduled-reports/{id}/execute-now", executeReportNowHandler).Methods("POST")
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
