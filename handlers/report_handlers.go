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

	// Special system tables: return their columns directly without form schema lookup.
	if strings.ToLower(tableName) == "workflow_transitions" {
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

	// Always append the workflow audit trail as a system data source.
	tables = append(tables, map[string]interface{}{
		"form_id":              nil,
		"form_code":            "",
		"form_title":           "Workflow Audit Trail",
		"table_name":           "workflow_transitions",
		"schema_name":          "public",
		"module_id":            nil,
		"accessible_verticals": []string{},
		"system":               true,
	})
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
