package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

func buildSystemFields(tableName string, title string, fields []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"table_name":  tableName,
		"form_title":  title,
		"form_fields": fields,
		"db_fields":   []map[string]interface{}{},
		"all_fields":  fields,
	}
}

func getDMSDocumentFields() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "id", "label": "Document ID", "type": "text", "dataType": "text", "source": "system", "column_name": "id"},
		{"id": "title", "label": "Title", "type": "text", "dataType": "text", "source": "system", "column_name": "title"},
		{"id": "description", "label": "Description", "type": "text", "dataType": "text", "source": "system", "column_name": "description"},
		{"id": "file_name", "label": "File Name", "type": "text", "dataType": "text", "source": "system", "column_name": "file_name"},
		{"id": "file_type", "label": "File Type", "type": "text", "dataType": "text", "source": "system", "column_name": "file_type"},
		{"id": "file_size", "label": "File Size", "type": "number", "dataType": "number", "source": "system", "column_name": "file_size"},
		{"id": "status", "label": "Status", "type": "text", "dataType": "text", "source": "system", "column_name": "status"},
		{"id": "current_state", "label": "Workflow State", "type": "text", "dataType": "text", "source": "system", "column_name": "current_state"},
		{"id": "business_vertical_id", "label": "Business Vertical ID", "type": "text", "dataType": "text", "source": "system", "column_name": "business_vertical_id"},
		{"id": "project_id", "label": "Project ID", "type": "text", "dataType": "text", "source": "system", "column_name": "project_id"},
		{"id": "task_id", "label": "Task ID", "type": "text", "dataType": "text", "source": "system", "column_name": "task_id"},
		{"id": "uploaded_by_id", "label": "Uploaded By ID", "type": "text", "dataType": "text", "source": "system", "column_name": "uploaded_by_id"},
		{"id": "download_count", "label": "Download Count", "type": "number", "dataType": "number", "source": "system", "column_name": "download_count"},
		{"id": "view_count", "label": "View Count", "type": "number", "dataType": "number", "source": "system", "column_name": "view_count"},
		{"id": "is_public", "label": "Is Public", "type": "boolean", "dataType": "boolean", "source": "system", "column_name": "is_public"},
		{"id": "created_at", "label": "Created At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "created_at"},
		{"id": "updated_at", "label": "Updated At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "updated_at"},
	}
}

func getPMSProjectFields() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "id", "label": "Project ID", "type": "text", "dataType": "text", "source": "system", "column_name": "id"},
		{"id": "code", "label": "Project Code", "type": "text", "dataType": "text", "source": "system", "column_name": "code"},
		{"id": "name", "label": "Project Name", "type": "text", "dataType": "text", "source": "system", "column_name": "name"},
		{"id": "description", "label": "Description", "type": "text", "dataType": "text", "source": "system", "column_name": "description"},
		{"id": "business_vertical_id", "label": "Business Vertical ID", "type": "text", "dataType": "text", "source": "system", "column_name": "business_vertical_id"},
		{"id": "status", "label": "Status", "type": "text", "dataType": "text", "source": "system", "column_name": "status"},
		{"id": "progress", "label": "Progress", "type": "number", "dataType": "number", "source": "system", "column_name": "progress"},
		{"id": "total_budget", "label": "Total Budget", "type": "number", "dataType": "number", "source": "system", "column_name": "total_budget"},
		{"id": "allocated_budget", "label": "Allocated Budget", "type": "number", "dataType": "number", "source": "system", "column_name": "allocated_budget"},
		{"id": "spent_budget", "label": "Spent Budget", "type": "number", "dataType": "number", "source": "system", "column_name": "spent_budget"},
		{"id": "start_date", "label": "Start Date", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "start_date"},
		{"id": "end_date", "label": "End Date", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "end_date"},
		{"id": "created_by", "label": "Created By", "type": "text", "dataType": "text", "source": "system", "column_name": "created_by"},
		{"id": "created_at", "label": "Created At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "created_at"},
		{"id": "updated_at", "label": "Updated At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "updated_at"},
	}
}

func getPMSTaskFields() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "id", "label": "Task ID", "type": "text", "dataType": "text", "source": "system", "column_name": "id"},
		{"id": "code", "label": "Task Code", "type": "text", "dataType": "text", "source": "system", "column_name": "code"},
		{"id": "title", "label": "Task Title", "type": "text", "dataType": "text", "source": "system", "column_name": "title"},
		{"id": "project_id", "label": "Project ID", "type": "text", "dataType": "text", "source": "system", "column_name": "project_id"},
		{"id": "zone_id", "label": "Zone ID", "type": "text", "dataType": "text", "source": "system", "column_name": "zone_id"},
		{"id": "start_node_id", "label": "Start Node ID", "type": "text", "dataType": "text", "source": "system", "column_name": "start_node_id"},
		{"id": "stop_node_id", "label": "Stop Node ID", "type": "text", "dataType": "text", "source": "system", "column_name": "stop_node_id"},
		{"id": "status", "label": "Task Status", "type": "text", "dataType": "text", "source": "system", "column_name": "status"},
		{"id": "current_state", "label": "Workflow State", "type": "text", "dataType": "text", "source": "system", "column_name": "current_state"},
		{"id": "priority", "label": "Priority", "type": "text", "dataType": "text", "source": "system", "column_name": "priority"},
		{"id": "progress", "label": "Progress", "type": "number", "dataType": "number", "source": "system", "column_name": "progress"},
		{"id": "allocated_budget", "label": "Allocated Budget", "type": "number", "dataType": "number", "source": "system", "column_name": "allocated_budget"},
		{"id": "total_cost", "label": "Total Cost", "type": "number", "dataType": "number", "source": "system", "column_name": "total_cost"},
		{"id": "planned_start_date", "label": "Planned Start", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "planned_start_date"},
		{"id": "planned_end_date", "label": "Planned End", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "planned_end_date"},
		{"id": "actual_start_date", "label": "Actual Start", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "actual_start_date"},
		{"id": "actual_end_date", "label": "Actual End", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "actual_end_date"},
		{"id": "created_by", "label": "Created By", "type": "text", "dataType": "text", "source": "system", "column_name": "created_by"},
		{"id": "created_at", "label": "Created At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "created_at"},
		{"id": "updated_at", "label": "Updated At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "updated_at"},
	}
}

func appendSystemReportTable(tables []map[string]interface{}, title string, tableName string) []map[string]interface{} {
	return append(tables, map[string]interface{}{
		"form_id":              nil,
		"form_code":            "",
		"form_title":           title,
		"table_name":           tableName,
		"schema_name":          "public",
		"module_id":            nil,
		"accessible_verticals": []string{},
		"system":               true,
	})
}

func ensureDefaultReportTemplates() error {
	templates := []models.ReportTemplate{
		{
			Code:        "dms_documents_overview",
			Name:        "DMS Documents Overview",
			Description: "Track documents by status, size, ownership, and project/task linkage.",
			Category:    "DMS",
			Icon:        "i-heroicons-document-duplicate-solid",
		},
		{
			Code:        "dms_documents_by_context",
			Name:        "DMS Project-Task Document Matrix",
			Description: "Analyze document counts and visibility across project and task context.",
			Category:    "DMS",
			Icon:        "i-heroicons-table-cells-solid",
		},
		{
			Code:        "pms_projects_health_overview",
			Name:        "PMS Projects Health Overview",
			Description: "Monitor project progress and budget burn across statuses.",
			Category:    "PMS",
			Icon:        "i-heroicons-building-office-2-solid",
		},
		{
			Code:        "pms_tasks_execution_tracker",
			Name:        "PMS Tasks Execution Tracker",
			Description: "Track task workflow, delivery dates, and execution costs.",
			Category:    "PMS",
			Icon:        "i-heroicons-clipboard-document-list-solid",
		},
	}

	templatePayloads := map[string]map[string]interface{}{
		"dms_documents_overview": {
			"report_type": "table",
			"data_sources": []map[string]interface{}{{
				"alias":      "dms_documents",
				"table_name": "documents",
			}},
			"fields": []map[string]interface{}{
				{"field_name": "title", "alias": "Title", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 1},
				{"field_name": "status", "alias": "Status", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 2},
				{"field_name": "current_state", "alias": "Workflow State", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 3},
				{"field_name": "file_type", "alias": "File Type", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 4},
				{"field_name": "file_size", "alias": "File Size", "data_source": "dms_documents", "data_type": "number", "is_visible": true, "order": 5},
				{"field_name": "project_id", "alias": "Project ID", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 6},
				{"field_name": "task_id", "alias": "Task ID", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 7},
				{"field_name": "created_at", "alias": "Uploaded At", "data_source": "dms_documents", "data_type": "datetime", "is_visible": true, "order": 8},
			},
		},
		"dms_documents_by_context": {
			"report_type": "table",
			"data_sources": []map[string]interface{}{{
				"alias":      "dms_documents",
				"table_name": "documents",
			}},
			"fields": []map[string]interface{}{
				{"field_name": "project_id", "alias": "Project ID", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 1},
				{"field_name": "task_id", "alias": "Task ID", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 2},
				{"field_name": "status", "alias": "Status", "data_source": "dms_documents", "data_type": "text", "is_visible": true, "order": 3},
				{"field_name": "view_count", "alias": "View Count", "data_source": "dms_documents", "data_type": "number", "is_visible": true, "order": 4},
				{"field_name": "download_count", "alias": "Download Count", "data_source": "dms_documents", "data_type": "number", "is_visible": true, "order": 5},
				{"field_name": "created_at", "alias": "Created At", "data_source": "dms_documents", "data_type": "datetime", "is_visible": true, "order": 6},
			},
		},
		"pms_projects_health_overview": {
			"report_type": "table",
			"data_sources": []map[string]interface{}{{
				"alias":      "pms_projects",
				"table_name": "projects",
			}},
			"fields": []map[string]interface{}{
				{"field_name": "code", "alias": "Project Code", "data_source": "pms_projects", "data_type": "text", "is_visible": true, "order": 1},
				{"field_name": "name", "alias": "Project Name", "data_source": "pms_projects", "data_type": "text", "is_visible": true, "order": 2},
				{"field_name": "status", "alias": "Status", "data_source": "pms_projects", "data_type": "text", "is_visible": true, "order": 3},
				{"field_name": "progress", "alias": "Progress", "data_source": "pms_projects", "data_type": "number", "is_visible": true, "order": 4},
				{"field_name": "total_budget", "alias": "Total Budget", "data_source": "pms_projects", "data_type": "number", "is_visible": true, "order": 5},
				{"field_name": "spent_budget", "alias": "Spent Budget", "data_source": "pms_projects", "data_type": "number", "is_visible": true, "order": 6},
				{"field_name": "start_date", "alias": "Start Date", "data_source": "pms_projects", "data_type": "datetime", "is_visible": true, "order": 7},
				{"field_name": "end_date", "alias": "End Date", "data_source": "pms_projects", "data_type": "datetime", "is_visible": true, "order": 8},
			},
		},
		"pms_tasks_execution_tracker": {
			"report_type": "table",
			"data_sources": []map[string]interface{}{{
				"alias":      "pms_tasks",
				"table_name": "tasks",
			}},
			"fields": []map[string]interface{}{
				{"field_name": "code", "alias": "Task Code", "data_source": "pms_tasks", "data_type": "text", "is_visible": true, "order": 1},
				{"field_name": "title", "alias": "Task", "data_source": "pms_tasks", "data_type": "text", "is_visible": true, "order": 2},
				{"field_name": "project_id", "alias": "Project ID", "data_source": "pms_tasks", "data_type": "text", "is_visible": true, "order": 3},
				{"field_name": "status", "alias": "Status", "data_source": "pms_tasks", "data_type": "text", "is_visible": true, "order": 4},
				{"field_name": "current_state", "alias": "Workflow State", "data_source": "pms_tasks", "data_type": "text", "is_visible": true, "order": 5},
				{"field_name": "priority", "alias": "Priority", "data_source": "pms_tasks", "data_type": "text", "is_visible": true, "order": 6},
				{"field_name": "progress", "alias": "Progress", "data_source": "pms_tasks", "data_type": "number", "is_visible": true, "order": 7},
				{"field_name": "allocated_budget", "alias": "Allocated Budget", "data_source": "pms_tasks", "data_type": "number", "is_visible": true, "order": 8},
				{"field_name": "total_cost", "alias": "Total Cost", "data_source": "pms_tasks", "data_type": "number", "is_visible": true, "order": 9},
				{"field_name": "planned_end_date", "alias": "Planned End", "data_source": "pms_tasks", "data_type": "datetime", "is_visible": true, "order": 10},
				{"field_name": "actual_end_date", "alias": "Actual End", "data_source": "pms_tasks", "data_type": "datetime", "is_visible": true, "order": 11},
			},
		},
	}

	for _, template := range templates {
		var existing models.ReportTemplate
		err := config.DB.Where("code = ?", template.Code).First(&existing).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}

		payload := templatePayloads[template.Code]
		rawPayload, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}

		template.Template = rawPayload
		template.IsActive = true
		if createErr := config.DB.Create(&template).Error; createErr != nil {
			return createErr
		}
	}

	return nil
}

// CreateReportDefinition creates a new report
func CreateReportDefinition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code               string          `json:"code"`
		Name               string          `json:"name"`
		Description        string          `json:"description"`
		Category           string          `json:"category"`
		ReportType         string          `json:"report_type"`
		DataSources        json.RawMessage `json:"data_sources"`
		Fields             json.RawMessage `json:"fields"`
		Filters            json.RawMessage `json:"filters"`
		Groupings          json.RawMessage `json:"groupings"`
		Aggregations       json.RawMessage `json:"aggregations"`
		Sorting            json.RawMessage `json:"sorting"`
		Calculations       json.RawMessage `json:"calculations"`
		ChartType          string          `json:"chart_type"`
		ChartConfig        json.RawMessage `json:"chart_config"`
		Layout             json.RawMessage `json:"layout"`
		BusinessVerticalID uuid.UUID       `json:"business_vertical_id"`
		IsPublic           bool            `json:"is_public"`
		AllowedRoles       []string        `json:"allowed_roles"`
		AllowedUsers       []string        `json:"allowed_users"`
		ExportFormats      []string        `json:"export_formats"`
		Tags               []string        `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	businessID := req.BusinessVerticalID
	if businessID == uuid.Nil {
		businessID = middleware.GetCurrentBusinessID(r)
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "report name is required", http.StatusBadRequest)
		return
	}

	if len(req.DataSources) == 0 || string(req.DataSources) == "null" {
		http.Error(w, "data_sources is required", http.StatusBadRequest)
		return
	}

	if len(req.Fields) == 0 || string(req.Fields) == "null" {
		http.Error(w, "fields is required", http.StatusBadRequest)
		return
	}

	if err := ensureReportViewsForDataSources(config.DB, req.DataSources); err != nil {
		http.Error(w, fmt.Sprintf("invalid report data_sources: %v", err), http.StatusBadRequest)
		return
	}

	if businessID == uuid.Nil {
		http.Error(w, "business_vertical_id is required (send in body or X-Business-ID/X-Business-Code header)", http.StatusBadRequest)
		return
	}

	report := &models.ReportDefinition{
		Code:               req.Code,
		Name:               req.Name,
		Description:        req.Description,
		Category:           req.Category,
		ReportType:         req.ReportType,
		DataSources:        req.DataSources,
		Fields:             req.Fields,
		Filters:            req.Filters,
		Groupings:          req.Groupings,
		Aggregations:       req.Aggregations,
		Sorting:            req.Sorting,
		Calculations:       req.Calculations,
		ChartType:          req.ChartType,
		ChartConfig:        req.ChartConfig,
		Layout:             req.Layout,
		BusinessVerticalID: businessID,
		IsPublic:           req.IsPublic,
		AllowedRoles:       req.AllowedRoles,
		AllowedUsers:       req.AllowedUsers,
		ExportFormats:      req.ExportFormats,
		Tags:               req.Tags,
		IsActive:           true,
		CreatedBy:          claims.UserID,
	}

	if err := config.DB.Create(report).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to create report: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Report created successfully",
		"report":  report,
	})
}

// GetReportDefinitions retrieves all reports for a business vertical
func GetReportDefinitions(w http.ResponseWriter, r *http.Request) {
	businessVerticalID := r.URL.Query().Get("business_vertical_id")
	category := r.URL.Query().Get("category")
	reportType := r.URL.Query().Get("report_type")

	query := config.DB.Where("deleted_at IS NULL")

	if businessVerticalID != "" {
		query = query.Where("business_vertical_id = ?", businessVerticalID)
	}

	if category != "" {
		query = query.Where("category = ?", category)
	}

	if reportType != "" {
		query = query.Where("report_type = ?", reportType)
	}

	var all []models.ReportDefinition
	if err := query.Order("created_at DESC").Find(&all).Error; err != nil {
		http.Error(w, "Failed to fetch reports", http.StatusInternalServerError)
		return
	}

	// Filter to only reports the requesting user may view.
	reports := make([]models.ReportDefinition, 0, len(all))
	for i := range all {
		if canViewReport(r, &all[i]) {
			reports = append(reports, all[i])
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"reports": reports,
		"count":   len(reports),
	})
}

// GetReportDefinition retrieves a single report by ID
func GetReportDefinition(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]

	var report models.ReportDefinition
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", reportID).First(&report).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	if !canViewReport(r, &report) {
		reportAccessDenied(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"report": report})
}

// UpdateReportDefinition updates an existing report
func UpdateReportDefinition(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]
	claims := middleware.GetClaims(r)

	var report models.ReportDefinition
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", reportID).First(&report).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	if !canModifyReport(r, &report) {
		reportAccessDenied(w)
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update allowed fields
	req["updated_by"] = claims.UserID
	req["updated_at"] = time.Now()

	tx := config.DB.Begin()
	if tx.Error != nil {
		http.Error(w, "Failed to update report", http.StatusInternalServerError)
		return
	}

	if err := tx.Model(&report).Updates(req).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to update report", http.StatusInternalServerError)
		return
	}

	if err := tx.Where("id = ? AND deleted_at IS NULL", reportID).First(&report).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Report not found after update", http.StatusNotFound)
		return
	}

	if err := ensureReportViewsForDataSources(tx, report.DataSources); err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("failed to sync report views: %v", err), http.StatusBadRequest)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to update report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Report updated successfully",
		"report":  report,
	})
}

// DeleteReportDefinition soft deletes a report
func DeleteReportDefinition(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]

	var report models.ReportDefinition
	if err := config.DB.Where("id = ?", reportID).First(&report).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	if !canModifyReport(r, &report) {
		reportAccessDenied(w)
		return
	}

	now := time.Now()
	if err := config.DB.Model(&report).Update("deleted_at", now).Error; err != nil {
		http.Error(w, "Failed to delete report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Report deleted successfully"})
}

// ExecuteReport executes a report and returns the results
func ExecuteReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]
	claims := middleware.GetClaims(r)

	var report models.ReportDefinition
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", reportID).First(&report).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	if !canViewReport(r, &report) {
		reportAccessDenied(w)
		return
	}

	// Parse runtime filters from request
	var req struct {
		Filters []models.ReportFilter `json:"filters"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := ensureReportViewsForDataSources(config.DB, report.DataSources); err != nil {
		http.Error(w, fmt.Sprintf("failed to sync report views: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute report
	engine := NewReportEngine()
	result, err := engine.ExecuteReport(&report, req.Filters, claims.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"report": report,
		"result": result,
	})
}

// GetSubmissionWorkflowHistory returns the full workflow lifecycle for a single form submission.
// This is used by the report viewer's timeline drill-down feature.
// GET /api/v1/reports/submissions/{submissionId}/workflow-history
func GetSubmissionWorkflowHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

	// Fetch full transition history ordered chronologically.
	var transitions []models.WorkflowTransition
	if err := config.DB.
		Where("submission_id = ?", submissionID).
		Order("transitioned_at ASC").
		Find(&transitions).Error; err != nil {
		http.Error(w, "failed to fetch workflow history", http.StatusInternalServerError)
		return
	}

	// Also fetch the current submission state for the header.
	var submission models.FormSubmission
	submissionFound := config.DB.
		Select("id", "form_code", "current_state", "submitted_by", "submitted_at").
		First(&submission, "id = ? AND deleted_at IS NULL", submissionID).Error == nil

	resp := map[string]interface{}{
		"history": transitions,
		"count":   len(transitions),
	}
	if submissionFound {
		resp["submission"] = map[string]interface{}{
			"id":            submission.ID,
			"form_code":     submission.FormCode,
			"current_state": submission.CurrentState,
			"submitted_by":  submission.SubmittedBy,
			"submitted_at":  submission.SubmittedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// sanitizeColumnName converts a raw string to a safe SQL column name
// using the same rules as getColumnDefinition in form_table_manager.go.
// GetFormTableFields retrieves field definitions for a form identified by its code or db_table_name.
// Form submission data is stored as JSONB in form_submissions.form_data keyed by field ID.
// column_name returned for each field is the field.id — the JSONB key used for extraction at query time.
func GetFormTableFields(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableName := vars["table_name"]
	normalizedTable := strings.ToLower(strings.TrimSpace(tableName))

	if normalizedTable == "attendance_sessions" {
		attendanceSessionFields := []map[string]interface{}{
			{"id": "id", "label": "Session ID", "type": "text", "dataType": "text", "source": "system", "column_name": "id"},
			{"id": "user_id", "label": "User ID", "type": "text", "dataType": "text", "source": "system", "column_name": "user_id"},
			{"id": "site_id", "label": "Site ID", "type": "text", "dataType": "text", "source": "system", "column_name": "site_id"},
			{"id": "business_vertical_id", "label": "Business Vertical ID", "type": "text", "dataType": "text", "source": "system", "column_name": "business_vertical_id"},
			{"id": "status", "label": "Status", "type": "text", "dataType": "text", "source": "system", "column_name": "status"},
			{"id": "validation_status", "label": "Validation Status", "type": "text", "dataType": "text", "source": "system", "column_name": "validation_status"},
			{"id": "validation_method", "label": "Validation Method", "type": "text", "dataType": "text", "source": "system", "column_name": "validation_method"},
			{"id": "check_in_at", "label": "Check In At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "check_in_at"},
			{"id": "check_out_at", "label": "Check Out At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "check_out_at"},
			{"id": "last_seen_at", "label": "Last Seen At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "last_seen_at"},
			{"id": "check_in_latitude", "label": "Check In Latitude", "type": "number", "dataType": "number", "source": "system", "column_name": "check_in_latitude"},
			{"id": "check_in_longitude", "label": "Check In Longitude", "type": "number", "dataType": "number", "source": "system", "column_name": "check_in_longitude"},
			{"id": "check_out_latitude", "label": "Check Out Latitude", "type": "number", "dataType": "number", "source": "system", "column_name": "check_out_latitude"},
			{"id": "check_out_longitude", "label": "Check Out Longitude", "type": "number", "dataType": "number", "source": "system", "column_name": "check_out_longitude"},
			{"id": "last_latitude", "label": "Last Latitude", "type": "number", "dataType": "number", "source": "system", "column_name": "last_latitude"},
			{"id": "last_longitude", "label": "Last Longitude", "type": "number", "dataType": "number", "source": "system", "column_name": "last_longitude"},
			{"id": "last_accuracy", "label": "Last Accuracy", "type": "number", "dataType": "number", "source": "system", "column_name": "last_accuracy"},
			{"id": "device_id", "label": "Device ID", "type": "text", "dataType": "text", "source": "system", "column_name": "device_id"},
			{"id": "anomaly_flags", "label": "Anomaly Flags", "type": "json", "dataType": "json", "source": "system", "column_name": "anomaly_flags"},
			{"id": "metadata", "label": "Metadata", "type": "json", "dataType": "json", "source": "system", "column_name": "metadata"},
			{"id": "created_at", "label": "Created At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "created_at"},
			{"id": "updated_at", "label": "Updated At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "updated_at"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"table_name":  "attendance_sessions",
			"form_title":  "Attendance Sessions",
			"form_fields": attendanceSessionFields,
			"db_fields":   []map[string]interface{}{},
			"all_fields":  attendanceSessionFields,
		})
		return
	}

	if normalizedTable == "attendance_events" {
		attendanceEventFields := []map[string]interface{}{
			{"id": "id", "label": "Event ID", "type": "text", "dataType": "text", "source": "system", "column_name": "id"},
			{"id": "session_id", "label": "Session ID", "type": "text", "dataType": "text", "source": "system", "column_name": "session_id"},
			{"id": "user_id", "label": "User ID", "type": "text", "dataType": "text", "source": "system", "column_name": "user_id"},
			{"id": "site_id", "label": "Site ID", "type": "text", "dataType": "text", "source": "system", "column_name": "site_id"},
			{"id": "business_vertical_id", "label": "Business Vertical ID", "type": "text", "dataType": "text", "source": "system", "column_name": "business_vertical_id"},
			{"id": "event_type", "label": "Event Type", "type": "text", "dataType": "text", "source": "system", "column_name": "event_type"},
			{"id": "event_time", "label": "Event Time", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "event_time"},
			{"id": "latitude", "label": "Latitude", "type": "number", "dataType": "number", "source": "system", "column_name": "latitude"},
			{"id": "longitude", "label": "Longitude", "type": "number", "dataType": "number", "source": "system", "column_name": "longitude"},
			{"id": "accuracy", "label": "Accuracy", "type": "number", "dataType": "number", "source": "system", "column_name": "accuracy"},
			{"id": "device_id", "label": "Device ID", "type": "text", "dataType": "text", "source": "system", "column_name": "device_id"},
			{"id": "validation_status", "label": "Validation Status", "type": "text", "dataType": "text", "source": "system", "column_name": "validation_status"},
			{"id": "validation_method", "label": "Validation Method", "type": "text", "dataType": "text", "source": "system", "column_name": "validation_method"},
			{"id": "anomaly_flags", "label": "Anomaly Flags", "type": "json", "dataType": "json", "source": "system", "column_name": "anomaly_flags"},
			{"id": "is_mock_location", "label": "Mock Location", "type": "boolean", "dataType": "boolean", "source": "system", "column_name": "is_mock_location"},
			{"id": "is_gps_enabled", "label": "GPS Enabled", "type": "boolean", "dataType": "boolean", "source": "system", "column_name": "is_gps_enabled"},
			{"id": "app_state", "label": "App State", "type": "text", "dataType": "text", "source": "system", "column_name": "app_state"},
			{"id": "network_status", "label": "Network Status", "type": "text", "dataType": "text", "source": "system", "column_name": "network_status"},
			{"id": "battery_level", "label": "Battery Level", "type": "number", "dataType": "number", "source": "system", "column_name": "battery_level"},
			{"id": "server_received_at", "label": "Server Received At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "server_received_at"},
			{"id": "created_at", "label": "Created At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "created_at"},
			{"id": "updated_at", "label": "Updated At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "updated_at"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"table_name":  "attendance_events",
			"form_title":  "Attendance Events",
			"form_fields": attendanceEventFields,
			"db_fields":   []map[string]interface{}{},
			"all_fields":  attendanceEventFields,
		})
		return
	}

	// Special system tables: return their columns directly without form schema lookup.
	if normalizedTable == "workflow_transitions" {
		wtFields := []map[string]interface{}{
			{"id": "id", "label": "Transition ID", "type": "text", "dataType": "text", "source": "system", "column_name": "id"},
			{"id": "submission_id", "label": "Submission ID", "type": "text", "dataType": "text", "source": "system", "column_name": "submission_id"},
			{"id": "from_state", "label": "From State", "type": "text", "dataType": "text", "source": "system", "column_name": "from_state"},
			{"id": "to_state", "label": "To State", "type": "text", "dataType": "text", "source": "system", "column_name": "to_state"},
			{"id": "action", "label": "Action", "type": "text", "dataType": "text", "source": "system", "column_name": "action"},
			{"id": "actor_id", "label": "Actor ID", "type": "text", "dataType": "text", "source": "system", "column_name": "actor_id"},
			{"id": "actor_name", "label": "Actor Name", "type": "text", "dataType": "text", "source": "system", "column_name": "actor_name"},
			{"id": "actor_role", "label": "Actor Role", "type": "text", "dataType": "text", "source": "system", "column_name": "actor_role"},
			{"id": "comment", "label": "Comment", "type": "text", "dataType": "text", "source": "system", "column_name": "comment"},
			{"id": "transitioned_at", "label": "Transitioned At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "transitioned_at"},
			{"id": "created_at", "label": "Created At", "type": "datetime", "dataType": "datetime", "source": "system", "column_name": "created_at"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"table_name":  "workflow_transitions",
			"form_title":  "Workflow Audit Trail",
			"form_fields": wtFields,
			"db_fields":   []map[string]interface{}{},
			"all_fields":  wtFields,
		})
		return
	}

	if normalizedTable == "documents" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildSystemFields("documents", "DMS Documents", getDMSDocumentFields()))
		return
	}

	if normalizedTable == "projects" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildSystemFields("projects", "PMS Projects", getPMSProjectFields()))
		return
	}

	if normalizedTable == "tasks" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildSystemFields("tasks", "PMS Tasks", getPMSTaskFields()))
		return
	}

	// Find the form by db_table_name OR code (table_name may be a form code for forms without a dedicated table).
	var form models.AppForm
	var formFields []map[string]interface{}
	var formTitle string

	dbErr := config.DB.
		Where("is_active = ? AND (LOWER(db_table_name) = LOWER(?) OR LOWER(code) = LOWER(?))", true, tableName, tableName).
		Select("id", "code", "title", "form_schema", "steps", "core_fields").
		First(&form).Error

	if dbErr == nil && form.ID != uuid.Nil {
		for _, field := range buildFormFieldList(form) {
			id, _ := field["id"].(string)
			name, _ := field["name"].(string)
			columnName := strings.TrimSpace(id)
			if normalizedName := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"), "-", "_")); normalizedName != "" && sqlIdentifierPattern.MatchString(normalizedName) {
				// Prefer semantic schema names like end_time when available so reports match the generated view.
				columnName = normalizedName
			} else if strings.TrimSpace(id) != "" && sqlIdentifierPattern.MatchString(strings.TrimSpace(id)) {
				columnName = strings.TrimSpace(id)
			}
			field["column_name"] = columnName
			formFields = append(formFields, field)
		}
		formFields = append(formFields, inferMissingFormFieldsFromSubmissions(form.ID, formFields)...)
		fmt.Printf("[REPORT BUILDER] Loaded %d form fields for %s\n", len(formFields), form.Code)
		formTitle = form.Title
	}

	// Metadata fields are direct columns on form_submissions (or resolved via JOIN in the engine).
	metadataFields := []map[string]interface{}{
		// _submission_id is a hidden field used by the frontend to identify rows for workflow timeline drill-down.
		{"id": "submission_id", "type": "text", "label": "Submission ID", "dataType": "text", "source": "metadata", "column_name": "submission_id", "hidden": true},
		{"id": "submitted_at", "type": "datetime", "label": "Submitted At", "dataType": "datetime", "source": "metadata", "column_name": "submitted_at"},
		{"id": "submitted_by_name", "type": "text", "label": "Submitted By", "dataType": "text", "source": "metadata", "column_name": "submitted_by_name"},
		{"id": "current_state", "type": "text", "label": "Status", "dataType": "text", "source": "metadata", "column_name": "current_state"},
		{"id": "site_name", "type": "text", "label": "Site Name", "dataType": "text", "source": "metadata", "column_name": "site_name"},
		{"id": "business_vertical_name", "type": "text", "label": "Business Vertical", "dataType": "text", "source": "metadata", "column_name": "business_vertical_name"},
		{"id": "form_code", "type": "text", "label": "Form Code", "dataType": "text", "source": "metadata", "column_name": "form_code"},
		// Workflow fields — resolved via LATERAL join on workflow_transitions in the report engine.
		{"id": "wf_last_action", "type": "text", "label": "Last Action", "dataType": "text", "source": "workflow", "column_name": "wf_last_action"},
		{"id": "wf_last_action_by", "type": "text", "label": "Action By", "dataType": "text", "source": "workflow", "column_name": "wf_last_action_by"},
		{"id": "wf_last_action_at", "type": "datetime", "label": "Action At", "dataType": "datetime", "source": "workflow", "column_name": "wf_last_action_at"},
		{"id": "wf_last_comment", "type": "text", "label": "Action Comment", "dataType": "text", "source": "workflow", "column_name": "wf_last_comment"},
	}
	combinedFields := append([]map[string]interface{}{}, formFields...)
	combinedFields = append(combinedFields, metadataFields...)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"table_name":  tableName,
		"form_title":  formTitle,
		"form_fields": combinedFields,
		"db_fields":   []map[string]interface{}{},
		"all_fields":  combinedFields,
	})
}

func inferMissingFormFieldsFromSubmissions(formID uuid.UUID, existingFields []map[string]interface{}) []map[string]interface{} {
	seen := make(map[string]struct{}, len(existingFields))
	for _, field := range existingFields {
		id, _ := field["id"].(string)
		if id != "" {
			seen[id] = struct{}{}
		}
	}

	var submissions []models.FormSubmission
	if err := config.DB.
		Where("form_id = ?", formID).
		Order("submitted_at DESC").
		Limit(100).
		Find(&submissions).Error; err != nil {
		return nil
	}

	baseFields := map[string]struct{}{
		"id": {}, "created_by": {}, "created_at": {}, "updated_by": {}, "updated_at": {}, "deleted_by": {}, "deleted_at": {},
		"business_vertical_id": {}, "site_id": {}, "workflow_id": {}, "current_state": {}, "form_id": {}, "form_code": {},
	}

	missingFields := make([]map[string]interface{}, 0)
	for _, submission := range submissions {
		var formData map[string]interface{}
		if err := json.Unmarshal(submission.FormData, &formData); err != nil {
			continue
		}

		for key, value := range formData {
			if key == "" {
				continue
			}
			if _, skip := baseFields[key]; skip {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}

			fieldType := inferReportFieldTypeFromValue(value)
			missingFields = append(missingFields, map[string]interface{}{
				"id":          key,
				"column_name": key,
				"label":       humanizeReportFieldLabel(key),
				"source":      "submission_inferred",
				"type":        fieldType,
				"dataType":    fieldType,
			})
			seen[key] = struct{}{}
		}
	}

	return missingFields
}

func inferReportFieldTypeFromValue(value interface{}) string {
	switch v := value.(type) {
	case bool:
		return "checkbox"
	case int, int8, int16, int32, int64, float32, float64:
		return "number"
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return "text"
		}
		if len(trimmed) == 10 && trimmed[4] == '-' && trimmed[7] == '-' {
			return "date"
		}
		if strings.Contains(trimmed, "T") && strings.Contains(trimmed, ":") {
			return "datetime"
		}
		return "text"
	case []interface{}, map[string]interface{}:
		return "json"
	default:
		return "text"
	}
}

func humanizeReportFieldLabel(value string) string {
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), "_", " "), "-", " "))
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func buildFormFieldList(form models.AppForm) []map[string]interface{} {
	fieldList := []map[string]interface{}{}
	seen := make(map[string]struct{})
	appendNestedFields := func(field map[string]interface{}, source string, visit func(map[string]interface{}, string)) {
		for _, key := range []string{"fields", "children", "sections", "components"} {
			rawItems, ok := field[key].([]interface{})
			if !ok {
				continue
			}
			for _, rawItem := range rawItems {
				child, ok := rawItem.(map[string]interface{})
				if ok {
					visit(child, source)
				}
			}
		}

		rawColumns, ok := field["columns"].([]interface{})
		if !ok {
			return
		}
		for _, rawColumn := range rawColumns {
			column, ok := rawColumn.(map[string]interface{})
			if !ok {
				continue
			}
			if childFields, ok := column["fields"].([]interface{}); ok {
				for _, rawChildField := range childFields {
					childField, ok := rawChildField.(map[string]interface{})
					if ok {
						visit(childField, source)
					}
				}
			}
		}
	}

	var visitField func(map[string]interface{}, string)
	appendField := func(field map[string]interface{}, source string) {
		id, _ := field["id"].(string)
		if id == "" {
			appendNestedFields(field, source, visitField)
			return
		}
		if _, exists := seen[id]; exists {
			appendNestedFields(field, source, visitField)
			return
		}
		seen[id] = struct{}{}
		// Include "name" if present — it is used as the DB column name by getColumnDefinition.
		// The handler cross-references this against information_schema to set column_name definitively.
		entry := map[string]interface{}{
			"id":       field["id"],
			"type":     field["type"],
			"label":    field["label"],
			"dataType": field["dataType"],
			"source":   source,
		}
		if name, ok := field["name"].(string); ok && name != "" {
			entry["name"] = name
		}
		fieldList = append(fieldList, entry)
		appendNestedFields(field, source, visitField)
	}
	visitField = appendField

	var formSchema map[string]interface{}
	if len(form.FormSchema) > 0 && string(form.FormSchema) != "{}" {
		if err := json.Unmarshal(form.FormSchema, &formSchema); err == nil {
			if fields, ok := formSchema["fields"].([]interface{}); ok {
				for _, raw := range fields {
					if field, ok := raw.(map[string]interface{}); ok {
						appendField(field, "form")
					}
				}
			}
			if steps, ok := formSchema["steps"].([]interface{}); ok {
				for _, rawStep := range steps {
					if step, ok := rawStep.(map[string]interface{}); ok {
						if stepFields, ok := step["fields"].([]interface{}); ok {
							for _, rawField := range stepFields {
								if field, ok := rawField.(map[string]interface{}); ok {
									appendField(field, "form")
								}
							}
						}
					}
				}
			}
		}
	}

	if len(form.Steps) > 0 && string(form.Steps) != "[]" {
		var steps []map[string]interface{}
		if err := json.Unmarshal(form.Steps, &steps); err == nil {
			for _, step := range steps {
				if stepFields, ok := step["fields"].([]interface{}); ok {
					for _, rawField := range stepFields {
						if field, ok := rawField.(map[string]interface{}); ok {
							appendField(field, "form")
						}
					}
				}
			}
		}
	}

	if len(fieldList) == 0 {
		var coreFields []map[string]interface{}
		if err := json.Unmarshal(form.CoreFields, &coreFields); err == nil {
			for _, field := range coreFields {
				appendField(field, "core")
			}
		}
	}

	return fieldList
}

// BuildFormFieldMap creates a map of field ID -> field definition for quick lookup
func buildFormFieldMap(form models.AppForm) map[string]map[string]interface{} {
	fieldMap := make(map[string]map[string]interface{})

	for _, field := range buildFormFieldList(form) {
		if id, ok := field["id"].(string); ok && id != "" {
			fieldMap[id] = field
		}
	}

	return fieldMap
}

// GetAvailableFormTables retrieves all form tables available for reporting.
// Optional query params:
//   - module_id: restrict to a single module UUID
//   - business_vertical_id / vertical_id: restrict by form accessible_verticals tokens (UUID/code)
func GetAvailableFormTables(w http.ResponseWriter, r *http.Request) {
	moduleID := strings.TrimSpace(r.URL.Query().Get("module_id"))
	verticalToken := strings.TrimSpace(r.URL.Query().Get("business_vertical_id"))
	if verticalToken == "" {
		verticalToken = strings.TrimSpace(r.URL.Query().Get("vertical_id"))
	}

	query := config.DB.Model(&models.AppForm{}).Where("is_active = ?", true)

	if moduleID != "" {
		moduleUUID, err := uuid.Parse(moduleID)
		if err != nil {
			http.Error(w, "invalid module_id", http.StatusBadRequest)
			return
		}
		query = query.Where("module_id = ?", moduleUUID)
	}

	if verticalToken != "" {
		candidateTokens := map[string]struct{}{
			verticalToken:                  {},
			strings.ToUpper(verticalToken): {},
		}

		if verticalUUID, err := uuid.Parse(verticalToken); err == nil {
			var matched models.BusinessVertical
			if err := config.DB.Select("id", "code").Where("id = ?", verticalUUID).First(&matched).Error; err == nil {
				candidateTokens[matched.ID.String()] = struct{}{}
				if strings.TrimSpace(matched.Code) != "" {
					candidateTokens[matched.Code] = struct{}{}
					candidateTokens[strings.ToUpper(matched.Code)] = struct{}{}
				}
			}
		} else {
			var matched []models.BusinessVertical
			if err := config.DB.Select("id", "code").Where("LOWER(code) = LOWER(?)", verticalToken).Find(&matched).Error; err == nil {
				for _, v := range matched {
					candidateTokens[v.ID.String()] = struct{}{}
					if strings.TrimSpace(v.Code) != "" {
						candidateTokens[v.Code] = struct{}{}
						candidateTokens[strings.ToUpper(v.Code)] = struct{}{}
					}
				}
			}
		}

		filterConditions := []string{"accessible_verticals = '[]'::jsonb"}
		filterArgs := make([]interface{}, 0, len(candidateTokens))
		for token := range candidateTokens {
			if strings.TrimSpace(token) == "" {
				continue
			}
			filterConditions = append(filterConditions, "accessible_verticals @> ?")
			filterArgs = append(filterArgs, `["`+token+`"]`)
		}
		query = query.Where(strings.Join(filterConditions, " OR "), filterArgs...)
	}

	var forms []models.AppForm
	if err := query.
		Select("id", "code", "title", "db_table_name", "module_id", "accessible_verticals", "form_schema", "steps", "core_fields").
		Find(&forms).Error; err != nil {
		http.Error(w, "Failed to fetch forms", http.StatusInternalServerError)
		return
	}

	tables := []map[string]interface{}{}
	for _, form := range forms {
		// Only include forms that have at least one field defined in their schema.
		fields := buildFormFieldList(form)
		if len(fields) == 0 {
			continue
		}
		tables = append(tables, map[string]interface{}{
			"form_id":    form.ID,
			"form_code":  form.Code,
			"form_title": form.Title,
			// table_name is the identifier used to call GetFormTableFields.
			// We use form.Code so the fields API can always find the form regardless of db_table_name.
			"table_name":           form.Code,
			"schema_name":          "",
			"module_id":            form.ModuleID,
			"accessible_verticals": form.AccessibleVerticals,
		})
	}

	// Always append system data sources.
	tables = appendSystemReportTable(tables, "Workflow Audit Trail", "workflow_transitions")
	tables = appendSystemReportTable(tables, "DMS Documents", "documents")
	tables = appendSystemReportTable(tables, "PMS Projects", "projects")
	tables = appendSystemReportTable(tables, "PMS Tasks", "tasks")

	// Attendance system data sources for attendance-specific analytics reports.
	tables = appendSystemReportTable(tables, "Attendance Sessions", "attendance_sessions")
	tables = appendSystemReportTable(tables, "Attendance Events", "attendance_events")
	fmt.Printf("[REPORT BUILDER] available forms=%d (module_id=%s, vertical=%s)\n", len(tables), moduleID, verticalToken)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tables": tables,
		"count":  len(tables),
	})
}

// CloneReport duplicates an existing report
func CloneReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]
	claims := middleware.GetClaims(r)

	var originalReport models.ReportDefinition
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", reportID).First(&originalReport).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	// Create new report as a copy
	newReport := models.ReportDefinition{
		Code:               fmt.Sprintf("%s_copy_%d", originalReport.Code, time.Now().Unix()),
		Name:               fmt.Sprintf("%s (Copy)", originalReport.Name),
		Description:        originalReport.Description,
		Category:           originalReport.Category,
		ReportType:         originalReport.ReportType,
		DataSources:        originalReport.DataSources,
		Fields:             originalReport.Fields,
		Filters:            originalReport.Filters,
		Groupings:          originalReport.Groupings,
		Aggregations:       originalReport.Aggregations,
		Sorting:            originalReport.Sorting,
		Calculations:       originalReport.Calculations,
		ChartType:          originalReport.ChartType,
		ChartConfig:        originalReport.ChartConfig,
		Layout:             originalReport.Layout,
		BusinessVerticalID: originalReport.BusinessVerticalID,
		IsPublic:           false,
		AllowedRoles:       originalReport.AllowedRoles,
		ExportFormats:      originalReport.ExportFormats,
		Tags:               originalReport.Tags,
		IsActive:           true,
		CreatedBy:          claims.UserID,
	}

	if err := config.DB.Create(&newReport).Error; err != nil {
		http.Error(w, "Failed to clone report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Report cloned successfully",
		"report":  newReport,
	})
}

// ToggleFavoriteReport marks/unmarks a report as favorite
func ToggleFavoriteReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]

	var report models.ReportDefinition
	if err := config.DB.Where("id = ?", reportID).First(&report).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	report.IsFavorite = !report.IsFavorite
	if err := config.DB.Save(&report).Error; err != nil {
		http.Error(w, "Failed to update favorite status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     "Favorite status updated",
		"is_favorite": report.IsFavorite,
	})
}

// GetReportExecutionHistory retrieves execution history for a report
func GetReportExecutionHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	var executions []models.ReportExecution
	if err := config.DB.Where("report_id = ?", reportID).
		Order("started_at DESC").
		Limit(limit).
		Find(&executions).Error; err != nil {
		http.Error(w, "Failed to fetch execution history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"executions": executions,
		"count":      len(executions),
	})
}

// Dashboard handlers

// CreateDashboard creates a new dashboard
func CreateDashboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code               string          `json:"code"`
		Name               string          `json:"name"`
		Description        string          `json:"description"`
		BusinessVerticalID uuid.UUID       `json:"business_vertical_id"`
		Layout             json.RawMessage `json:"layout"`
		IsPublic           bool            `json:"is_public"`
		AllowedRoles       []string        `json:"allowed_roles"`
		IsDefault          bool            `json:"is_default"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	claims := middleware.GetClaims(r)

	dashboard := &models.Dashboard{
		Code:               req.Code,
		Name:               req.Name,
		Description:        req.Description,
		BusinessVerticalID: req.BusinessVerticalID,
		Layout:             req.Layout,
		IsPublic:           req.IsPublic,
		AllowedRoles:       req.AllowedRoles,
		IsDefault:          req.IsDefault,
		IsActive:           true,
		CreatedBy:          claims.UserID,
	}

	if err := config.DB.Create(dashboard).Error; err != nil {
		http.Error(w, "Failed to create dashboard", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Dashboard created successfully",
		"dashboard": dashboard,
	})
}

// GetDashboards retrieves all dashboards
func GetDashboards(w http.ResponseWriter, r *http.Request) {
	businessVerticalID := r.URL.Query().Get("business_vertical_id")

	query := config.DB.Preload("Widgets").Preload("Widgets.Report").Where("deleted_at IS NULL")

	if businessVerticalID != "" {
		query = query.Where("business_vertical_id = ?", businessVerticalID)
	}

	var dashboards []models.Dashboard
	if err := query.Order("created_at DESC").Find(&dashboards).Error; err != nil {
		http.Error(w, "Failed to fetch dashboards", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"dashboards": dashboards,
		"count":      len(dashboards),
	})
}

// GetDashboard retrieves a single dashboard with all widgets
func GetDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dashboardID := vars["id"]

	var dashboard models.Dashboard
	if err := config.DB.Preload("Widgets").Preload("Widgets.Report").
		Where("id = ? AND deleted_at IS NULL", dashboardID).
		First(&dashboard).Error; err != nil {
		http.Error(w, "Dashboard not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"dashboard": dashboard})
}

// DeleteDashboard soft deletes a dashboard and removes related widgets
func DeleteDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dashboardID := vars["id"]

	var dashboard models.Dashboard
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", dashboardID).First(&dashboard).Error; err != nil {
		http.Error(w, "Dashboard not found", http.StatusNotFound)
		return
	}

	if dashboard.IsDefault {
		http.Error(w, "Default dashboard cannot be deleted", http.StatusBadRequest)
		return
	}

	now := time.Now()
	tx := config.DB.Begin()
	if tx.Error != nil {
		http.Error(w, "Failed to delete dashboard", http.StatusInternalServerError)
		return
	}

	if err := tx.Model(&models.ReportWidget{}).Where("dashboard_id = ?", dashboard.ID).Delete(&models.ReportWidget{}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete dashboard widgets", http.StatusInternalServerError)
		return
	}

	if err := tx.Model(&dashboard).Update("deleted_at", now).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete dashboard", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to delete dashboard", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Dashboard deleted successfully"})
}

// AddWidgetToDashboard adds a report widget to a dashboard
func AddWidgetToDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dashboardID := vars["id"]

	var req struct {
		ReportID    uuid.UUID       `json:"report_id"`
		Title       string          `json:"title"`
		Position    json.RawMessage `json:"position"`
		RefreshRate int             `json:"refresh_rate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dashboardUUID, _ := uuid.Parse(dashboardID)

	widget := &models.ReportWidget{
		DashboardID: dashboardUUID,
		ReportID:    req.ReportID,
		Title:       req.Title,
		Position:    req.Position,
		RefreshRate: req.RefreshRate,
	}

	if err := config.DB.Create(widget).Error; err != nil {
		http.Error(w, "Failed to add widget", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Widget added successfully",
		"widget":  widget,
	})
}

// RemoveWidgetFromDashboard removes a widget from a dashboard
func RemoveWidgetFromDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	widgetID := vars["widget_id"]

	if err := config.DB.Delete(&models.ReportWidget{}, "id = ?", widgetID).Error; err != nil {
		http.Error(w, "Failed to remove widget", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Widget removed successfully"})
}

// GetReportTemplates retrieves all report templates
func GetReportTemplates(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	if err := ensureDefaultReportTemplates(); err != nil {
		http.Error(w, "Failed to initialize default templates", http.StatusInternalServerError)
		return
	}

	query := config.DB.Where("is_active = ?", true)

	if category != "" {
		query = query.Where("category = ?", category)
	}

	var templates []models.ReportTemplate
	if err := query.Order("usage_count DESC, name ASC").Find(&templates).Error; err != nil {
		http.Error(w, "Failed to fetch templates", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	})
}

// CreateReportFromTemplate creates a new report from a template
func CreateReportFromTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateID := vars["template_id"]
	claims := middleware.GetClaims(r)

	var template models.ReportTemplate
	if err := config.DB.Where("id = ?", templateID).First(&template).Error; err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	var req struct {
		Name               string                 `json:"name"`
		BusinessVerticalID uuid.UUID              `json:"business_vertical_id"`
		Customizations     map[string]interface{} `json:"customizations"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Parse template
	var templateData map[string]interface{}
	if err := json.Unmarshal(template.Template, &templateData); err != nil {
		http.Error(w, "Invalid template", http.StatusInternalServerError)
		return
	}

	// Apply customizations
	for key, value := range req.Customizations {
		templateData[key] = value
	}

	// Create report from template
	report := &models.ReportDefinition{
		Code:               fmt.Sprintf("%s_%d", template.Code, time.Now().Unix()),
		Name:               req.Name,
		Category:           template.Category,
		BusinessVerticalID: req.BusinessVerticalID,
		IsActive:           true,
		CreatedBy:          claims.UserID,
	}

	// Marshal back to set fields
	if reportType, ok := templateData["report_type"].(string); ok {
		report.ReportType = reportType
	}

	if dataSources, ok := templateData["data_sources"]; ok {
		report.DataSources, _ = json.Marshal(dataSources)
	}

	if fields, ok := templateData["fields"]; ok {
		report.Fields, _ = json.Marshal(fields)
	}

	if err := config.DB.Create(report).Error; err != nil {
		http.Error(w, "Failed to create report from template", http.StatusInternalServerError)
		return
	}

	// Increment usage count
	config.DB.Model(&template).UpdateColumn("usage_count", template.UsageCount+1)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Report created from template successfully",
		"report":  report,
	})
}

// GetReportAvailableRoles returns global roles that can be used for report sharing.
// Requires report:read permission (enforced at route level). Does NOT require manage_roles.
func GetReportAvailableRoles(w http.ResponseWriter, r *http.Request) {
	var roles []models.Role
	if err := config.DB.
		Where("is_active = ? AND is_global = ?", true, true).
		Select("id", "name", "description").
		Order("name ASC").
		Find(&roles).Error; err != nil {
		http.Error(w, "Failed to fetch roles", http.StatusInternalServerError)
		return
	}

	type roleItem struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	items := make([]roleItem, len(roles))
	for i, role := range roles {
		items[i] = roleItem{
			ID:          role.ID.String(),
			Name:        role.Name,
			Description: role.Description,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"roles": items,
		"count": len(items),
	})
}
