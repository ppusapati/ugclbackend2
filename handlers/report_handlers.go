package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

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
		BusinessVerticalID: req.BusinessVerticalID,
		IsPublic:           req.IsPublic,
		AllowedRoles:       req.AllowedRoles,
		AllowedUsers:       req.AllowedUsers,
		ExportFormats:      req.ExportFormats,
		Tags:               req.Tags,
		IsActive:           true,
		CreatedBy:          claims.UserID,
	}

	if err := config.DB.Create(report).Error; err != nil {
		http.Error(w, "Failed to create report", http.StatusInternalServerError)
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

	var reports []models.ReportDefinition
	if err := query.Order("created_at DESC").Find(&reports).Error; err != nil {
		http.Error(w, "Failed to fetch reports", http.StatusInternalServerError)
		return
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

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update allowed fields
	req["updated_by"] = claims.UserID
	req["updated_at"] = time.Now()

	if err := config.DB.Model(&report).Updates(req).Error; err != nil {
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

	// Parse runtime filters from request
	var req struct {
		Filters []models.ReportFilter `json:"filters"`
	}
	json.NewDecoder(r.Body).Decode(&req)

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

// GetFormTableFields retrieves all fields from a form table
func GetFormTableFields(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableName := vars["table_name"]

	engine := NewReportEngine()
	schema, err := engine.GetFormTableSchema(tableName)
	if err != nil {
		http.Error(w, "Failed to fetch table schema", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"table_name": tableName,
		"fields":     schema,
	})
}

// GetAvailableFormTables retrieves all form tables available for reporting
func GetAvailableFormTables(w http.ResponseWriter, r *http.Request) {
	var forms []models.AppForm
	if err := config.DB.Where("is_active = ? AND db_table_name IS NOT NULL AND db_table_name != ''", true).
		Select("id", "code", "title", "db_table_name", "module_id").
		Find(&forms).Error; err != nil {
		http.Error(w, "Failed to fetch forms", http.StatusInternalServerError)
		return
	}

	tables := []map[string]interface{}{}
	for _, form := range forms {
		tables = append(tables, map[string]interface{}{
			"form_id":    form.ID,
			"form_code":  form.Code,
			"form_title": form.Title,
			"table_name": form.DBTableName,
			"module_id":  form.ModuleID,
		})
	}
	fmt.Println(tables)
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
