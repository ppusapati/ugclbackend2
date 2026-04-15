package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

var workflowEngine *WorkflowEngine

// getWorkflowEngine returns the workflow engine instance, initializing it if needed
func getWorkflowEngine() *WorkflowEngine {
	if workflowEngine == nil {
		workflowEngine = NewWorkflowEngine()
	}
	return workflowEngine
}

// SubmitFormRequest represents the request body for form submission
type SubmitFormRequest struct {
	FormData json.RawMessage `json:"form_data"`
	SiteID   *uuid.UUID      `json:"site_id,omitempty"`
}

// TransitionRequest represents the request body for workflow transitions
type TransitionRequest struct {
	Action   string                 `json:"action"`
	Comment  string                 `json:"comment,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CreateFormSubmission creates a new form submission
// POST /api/v1/business/{businessCode}/forms/{formCode}/submissions
func CreateFormSubmission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]
	businessCode := vars["businessCode"]

	// Get business vertical ID from context
	context := middleware.GetUserBusinessContext(r)
	if context == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := context["business_id"].(uuid.UUID)
	if !ok {
		http.Error(w, "invalid business context", http.StatusInternalServerError)
		return
	}

	var req SubmitFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("📝 Creating form submission: %s for business: %s, user: %s", formCode, businessCode, claims.UserID)

	// Create submission
	submission, err := getWorkflowEngine().CreateSubmission(
		formCode,
		businessID,
		req.SiteID,
		req.FormData,
		claims.UserID,
	)
	if err != nil {
		log.Printf("❌ Error creating submission: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Created submission: %s (state: %s)", submission.ID, submission.CurrentState)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "form submission created successfully",
		"submission": submission.ToDTO(submission.Workflow),
	})
}

// GetFormSubmissions retrieves all submissions for a form
// GET /api/v1/business/{businessCode}/forms/{formCode}/submissions
func GetFormSubmissions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	// Get business vertical ID from context
	context := middleware.GetUserBusinessContext(r)
	if context == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := context["business_id"].(uuid.UUID)
	if !ok {
		http.Error(w, "invalid business context", http.StatusInternalServerError)
		return
	}

	// Parse query parameters
	filters := make(map[string]interface{})
	if state := r.URL.Query().Get("state"); state != "" {
		filters["state"] = state
	}
	if siteID := r.URL.Query().Get("site_id"); siteID != "" {
		if id, err := uuid.Parse(siteID); err == nil {
			filters["site_id"] = id
		}
	}
	if r.URL.Query().Get("my_submissions") == "true" {
		filters["submitted_by"] = claims.UserID
	}

	submissions, err := getWorkflowEngine().GetSubmissionsByForm(formCode, businessID, filters)
	if err != nil {
		log.Printf("❌ Error fetching submissions: %v", err)
		http.Error(w, "failed to fetch submissions", http.StatusInternalServerError)
		return
	}

	// Convert to DTOs
	dtos := make([]models.FormSubmissionDTO, len(submissions))
	includeResolved := strings.EqualFold(r.URL.Query().Get("include_resolved"), "true")
	resolvedItems := make([]map[string]interface{}, 0, len(submissions))
	for i, sub := range submissions {
		dtos[i] = sub.ToDTO(sub.Workflow)
		if includeResolved {
			fieldIndex := buildFieldSchemaIndex(&sub)
			resolvedFields, resolvedFormData := resolveSubmissionFormData(&sub, fieldIndex)
			resolvedItems = append(resolvedItems, map[string]interface{}{
				"submission":         dtos[i],
				"resolved_fields":    resolvedFields,
				"resolved_form_data": resolvedFormData,
			})
		}
	}

	response := map[string]interface{}{
		"submissions": dtos,
		"count":       len(dtos),
	}
	if includeResolved {
		response["resolved_submissions"] = resolvedItems
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetFormSubmission retrieves a single submission by ID
// GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}
func GetFormSubmission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

	submission, err := getWorkflowEngine().GetSubmission(submissionID)
	if err != nil {
		log.Printf("❌ Error fetching submission: %v", err)
		http.Error(w, "submission not found", http.StatusNotFound)
		return
	}

	// Verify business context
	context := middleware.GetUserBusinessContext(r)
	if context == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := context["business_id"].(uuid.UUID)
	if !ok || submission.BusinessVerticalID != businessID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"submission": submission.ToDTO(submission.Workflow),
		"history":    submission.Transitions,
	})
}

type formFieldSchema struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Label        string            `json:"label"`
	Options      []formFieldOption `json:"options"`
	DataSource   string            `json:"dataSource"`
	APIEndpoint  string            `json:"apiEndpoint"`
	DisplayField string            `json:"displayField"`
	ValueField   string            `json:"valueField"`
}

type formFieldOption struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

type formStepSchema struct {
	Fields []formFieldSchema `json:"fields"`
}

type resolvedFieldValue struct {
	FieldID      string      `json:"field_id"`
	Label        string      `json:"label"`
	Type         string      `json:"type,omitempty"`
	RawValue     interface{} `json:"raw_value"`
	DisplayValue interface{} `json:"display_value"`
	Resolved     bool        `json:"resolved"`
}

// GetResolvedFormSubmission returns submission data enriched with field labels
// and best-effort display value resolution for select fields.
// GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}/resolved
func GetResolvedFormSubmission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

	submission, err := getWorkflowEngine().GetSubmission(submissionID)
	if err != nil {
		log.Printf("Error fetching submission: %v", err)
		http.Error(w, "submission not found", http.StatusNotFound)
		return
	}

	context := middleware.GetUserBusinessContext(r)
	if context == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := context["business_id"].(uuid.UUID)
	if !ok || submission.BusinessVerticalID != businessID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	fieldIndex := buildFieldSchemaIndex(submission)
	resolvedFields, resolvedFormData := resolveSubmissionFormData(submission, fieldIndex)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"submission":         submission.ToDTO(submission.Workflow),
		"history":            submission.Transitions,
		"resolved_fields":    resolvedFields,
		"resolved_form_data": resolvedFormData,
	})
}

func buildFieldSchemaIndex(submission *models.FormSubmission) map[string]formFieldSchema {
	index := make(map[string]formFieldSchema)
	if submission == nil || submission.Form == nil || len(submission.Form.Steps) == 0 {
		return index
	}

	var steps []formStepSchema
	if err := json.Unmarshal(submission.Form.Steps, &steps); err != nil {
		return index
	}

	for _, step := range steps {
		for _, field := range step.Fields {
			if field.ID == "" {
				continue
			}
			index[field.ID] = field
		}
	}

	return index
}

func resolveSubmissionFormData(submission *models.FormSubmission, fieldIndex map[string]formFieldSchema) ([]resolvedFieldValue, map[string]interface{}) {
	resolved := make([]resolvedFieldValue, 0)
	resolvedMap := make(map[string]interface{})

	if submission == nil || len(submission.FormData) == 0 {
		return resolved, resolvedMap
	}

	var data map[string]interface{}
	if err := json.Unmarshal(submission.FormData, &data); err != nil {
		return resolved, resolvedMap
	}

	for fieldID, rawValue := range data {
		schema, hasSchema := fieldIndex[fieldID]
		label := fieldID
		fieldType := ""
		if hasSchema {
			if schema.Label != "" {
				label = schema.Label
			}
			fieldType = schema.Type
		}

		displayValue := rawValue
		isResolved := false

		if hasSchema {
			if optionLabel, ok := resolveFromStaticOptions(schema.Options, rawValue); ok {
				displayValue = optionLabel
				isResolved = true
			} else if siteName, ok := resolveFromSiteReference(schema, rawValue); ok {
				displayValue = siteName
				isResolved = true
			}
		}

		resolved = append(resolved, resolvedFieldValue{
			FieldID:      fieldID,
			Label:        label,
			Type:         fieldType,
			RawValue:     rawValue,
			DisplayValue: displayValue,
			Resolved:     isResolved,
		})
		resolvedMap[label] = displayValue
	}

	return resolved, resolvedMap
}

func resolveFromStaticOptions(options []formFieldOption, rawValue interface{}) (string, bool) {
	for _, option := range options {
		if fmt.Sprint(option.Value) == fmt.Sprint(rawValue) {
			return option.Label, true
		}
	}
	return "", false
}

func resolveFromSiteReference(schema formFieldSchema, rawValue interface{}) (string, bool) {
	if schema.DataSource != "api" || schema.APIEndpoint == "" {
		return "", false
	}

	if !strings.Contains(strings.ToLower(schema.APIEndpoint), "/sites") {
		return "", false
	}

	idStr, ok := rawValue.(string)
	if !ok || idStr == "" {
		return "", false
	}

	siteID, err := uuid.Parse(idStr)
	if err != nil {
		return "", false
	}

	var site models.Site
	if err := config.DB.Select("id", "name").First(&site, "id = ?", siteID).Error; err != nil {
		return "", false
	}

	return site.Name, true
}

// UpdateFormSubmission updates a draft submission's data
// PUT /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}
func UpdateFormSubmission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

	var req SubmitFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	submission, err := getWorkflowEngine().UpdateSubmissionData(submissionID, req.FormData, claims.UserID)
	if err != nil {
		log.Printf("❌ Error updating submission: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("✅ Updated submission: %s", submissionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "submission updated successfully",
		"submission": submission.ToDTO(submission.Workflow),
	})
}

// TransitionFormSubmission performs a workflow state transition
// POST /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}/transition
func TransitionFormSubmission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

	var req TransitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get user permissions
	userPermissions := middleware.GetUserPermissions(r)

	// Validate transition
	if err := getWorkflowEngine().ValidateTransition(submissionID, req.Action, userPermissions); err != nil {
		log.Printf("❌ Transition validation failed: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Get user role name
	userRole := ""
	if user.RoleModel != nil {
		userRole = user.RoleModel.Name
	}

	// Perform transition
	submission, err := getWorkflowEngine().TransitionState(
		submissionID,
		req.Action,
		claims.UserID,
		user.Name,
		userRole,
		req.Comment,
		req.Metadata,
	)
	if err != nil {
		log.Printf("❌ Error transitioning submission: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Transitioned submission %s: action=%s, new_state=%s", submissionID, req.Action, submission.CurrentState)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "transition successful",
		"submission":    submission.ToDTO(submission.Workflow),
		"current_state": submission.CurrentState,
	})
}

// GetWorkflowHistory retrieves the complete transition history
// GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}/history
func GetWorkflowHistory(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

	history, err := getWorkflowEngine().GetWorkflowHistory(submissionID)
	if err != nil {
		log.Printf("❌ Error fetching history: %v", err)
		http.Error(w, "failed to fetch history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// GetWorkflowStats returns statistics about form submissions
// GET /api/v1/business/{businessCode}/forms/{formCode}/stats
func GetWorkflowStats(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	context := middleware.GetUserBusinessContext(r)
	if context == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := context["business_id"].(uuid.UUID)
	if !ok {
		http.Error(w, "invalid business context", http.StatusInternalServerError)
		return
	}

	stats, err := getWorkflowEngine().GetWorkflowStats(formCode, businessID)
	if err != nil {
		log.Printf("❌ Error fetching stats: %v", err)
		http.Error(w, "failed to fetch stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"form_code": formCode,
		"stats":     stats,
	})
}

// ============================================================================
// ADMIN ENDPOINTS - Workflow Management
// ============================================================================

// CreateWorkflowDefinition creates a new workflow definition (admin only)
// POST /api/v1/admin/workflows
func CreateWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var workflow models.WorkflowDefinition
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		log.Printf("❌ Error decoding workflow request: %v", err)
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if workflow.Code == "" {
		http.Error(w, "workflow code is required", http.StatusBadRequest)
		return
	}
	if workflow.Name == "" {
		http.Error(w, "workflow name is required", http.StatusBadRequest)
		return
	}

	log.Printf("📝 Creating workflow: code=%s, name=%s, states=%d bytes, transitions=%d bytes",
		workflow.Code, workflow.Name, len(workflow.States), len(workflow.Transitions))

	if err := getWorkflowEngine().db.Create(&workflow).Error; err != nil {
		log.Printf("❌ Error creating workflow in DB: %v", err)
		http.Error(w, "failed to create workflow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Created workflow: %s (ID: %s)", workflow.Code, workflow.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "workflow created successfully",
		"workflow": workflow,
	})
}

// GetAllWorkflows retrieves all workflow definitions (admin only)
// GET /api/v1/admin/workflows
func GetAllWorkflows(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var workflows []models.WorkflowDefinition
	if err := getWorkflowEngine().db.Find(&workflows).Error; err != nil {
		http.Error(w, "failed to fetch workflows", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workflows": workflows,
		"count":     len(workflows),
	})
}

func UpdateWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	workflowIdStr := vars["workflowId"]
	// Fetch the existing workflow definition
	var workflow models.WorkflowDefinition
	if err := getWorkflowEngine().db.First(&workflow, "id = ?", workflowIdStr).Error; err != nil {
		http.Error(w, "failed to fetch workflow", http.StatusInternalServerError)
		return
	}

	// Update the workflow definition with the new data
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := getWorkflowEngine().db.Save(&workflow).Error; err != nil {
		http.Error(w, "failed to update workflow", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "workflow updated successfully",
		"workflow": workflow,
	})
}

func DeleteWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	workflowIdStr := vars["workflowId"]

	// Delete the workflow definition
	if err := getWorkflowEngine().db.Delete(&models.WorkflowDefinition{}, "id = ?", workflowIdStr).Error; err != nil {
		http.Error(w, "failed to delete workflow", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
