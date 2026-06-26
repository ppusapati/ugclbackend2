package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/middleware"
)

var workflowEngineDedicated *WorkflowEngineDedicated

// getWorkflowEngineDedicated returns the dedicated workflow engine instance
func getWorkflowEngineDedicated() *WorkflowEngineDedicated {
	if workflowEngineDedicated == nil {
		workflowEngineDedicated = NewWorkflowEngineDedicated()
	}
	return workflowEngineDedicated
}

// CreateFormSubmissionDedicated creates a new form submission in dedicated table
// POST /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated
func CreateFormSubmissionDedicated(w http.ResponseWriter, r *http.Request) {
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

	// Parse request body
	var req struct {
		FormData map[string]interface{} `json:"form_data"`
		SiteID   *uuid.UUID             `json:"site_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("📝 Creating form submission in dedicated table: %s for business: %s, user: %s", formCode, businessCode, claims.UserID)

	// Create submission in dedicated table
	record, err := getWorkflowEngineDedicated().CreateSubmissionDedicated(
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

	log.Printf("✅ Created submission: %s (state: %s)", record.ID, record.CurrentState)
	triggerDedicatedFormSubmissionWebhook(record)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "form submission created successfully",
		"submission": record,
	})
}

// GetFormSubmissionsDedicated retrieves all submissions for a form from dedicated table
// GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated
func GetFormSubmissionsDedicated(w http.ResponseWriter, r *http.Request) {
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
		filters["current_state"] = state
	}
	if siteID := r.URL.Query().Get("site_id"); siteID != "" {
		if id, err := uuid.Parse(siteID); err == nil {
			filters["site_id"] = id
		}
	}
	if r.URL.Query().Get("my_submissions") == "true" {
		filters["created_by"] = claims.UserID
	}

	cursorRaw := strings.TrimSpace(r.URL.Query().Get("cursor"))
	limitRaw := strings.TrimSpace(r.URL.Query().Get("limit"))
	rawMode := r.URL.Query().Get("pagination")
	rawLegacy := r.URL.Query().Get("legacy_pagination")
	usePagination := shouldUseSubmissionCursorMode(rawMode, rawLegacy, cursorRaw, limitRaw)

	var cursor *submissionsCursor
	if usePagination {
		var parseErr error
		cursor, parseErr = decodeSubmissionsCursor(cursorRaw)
		if parseErr != nil {
			http.Error(w, "invalid cursor", http.StatusBadRequest)
			return
		}
	}

	pageSize := defaultSubmissionPageSize
	if usePagination {
		parsedLimit, parseErr := parseSubmissionPageSize(limitRaw)
		if parseErr != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		pageSize = parsedLimit
	}

	var records []*FormSubmissionRecord
	var err error
	if usePagination {
		records, err = getWorkflowEngineDedicated().GetSubmissionsByFormDedicatedPage(formCode, businessID, filters, pageSize+1, cursor)
	} else {
		records, err = getWorkflowEngineDedicated().GetSubmissionsByFormDedicated(formCode, businessID, filters)
	}
	if err != nil {
		log.Printf("❌ Error fetching submissions: %v", err)
		http.Error(w, "failed to fetch submissions", http.StatusInternalServerError)
		return
	}

	hasMore := false
	nextCursor := ""
	if usePagination && len(records) > pageSize {
		hasMore = true
		records = records[:pageSize]
	}
	if usePagination && hasMore && len(records) > 0 {
		last := records[len(records)-1]
		nextCursor = encodeSubmissionsCursor(last.CreatedAt, last.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"submissions": records,
		"count":       len(records),
	}
	if usePagination {
		response["limit"] = pageSize
		response["has_more"] = hasMore
		response["next_cursor"] = nextCursor
	}
	json.NewEncoder(w).Encode(response)
}

// GetFormSubmissionDedicated retrieves a single submission by ID from dedicated table
// GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated/{submissionId}
func GetFormSubmissionDedicated(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

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

	// Get form to find table name
	record, err := getWorkflowEngineDedicated().GetSubmissionDedicated(formCode, submissionID)
	if err != nil {
		log.Printf("❌ Error fetching submission: %v", err)
		http.Error(w, "submission not found", http.StatusNotFound)
		return
	}

	// Verify business context
	if record.BusinessVerticalID != businessID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Get workflow history
	history, _ := getWorkflowEngineDedicated().GetWorkflowHistoryDedicated(submissionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"submission": record,
		"history":    history,
	})
}

// UpdateFormSubmissionDedicated updates a draft submission's data in dedicated table
// PUT /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated/{submissionId}
func UpdateFormSubmissionDedicated(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

	var req struct {
		FormData map[string]interface{} `json:"form_data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	record, err := getWorkflowEngineDedicated().UpdateSubmissionDataDedicated(formCode, submissionID, req.FormData, claims.UserID)
	if err != nil {
		log.Printf("❌ Error updating submission: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("✅ Updated submission: %s", submissionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "submission updated successfully",
		"submission": record,
	})
}

// TransitionFormSubmissionDedicated performs a workflow state transition on dedicated table record
// POST /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated/{submissionId}/transition
func TransitionFormSubmissionDedicated(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]
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

	// Use merged global + business-context permissions for transition authorization.
	userPermissions := middleware.GetEffectivePermissions(r)

	// Validate transition
	if err := getWorkflowEngineDedicated().ValidateTransitionDedicated(formCode, submissionID, req.Action, userPermissions); err != nil {
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
	record, err := getWorkflowEngineDedicated().TransitionStateDedicated(
		formCode,
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

	log.Printf("✅ Transitioned submission %s: action=%s, new_state=%s", submissionID, req.Action, record.CurrentState)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "transition successful",
		"submission":    record,
		"current_state": record.CurrentState,
	})
}

// DeleteFormSubmissionDedicated soft deletes a submission from dedicated table
// DELETE /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated/{submissionId}
func DeleteFormSubmissionDedicated(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]
	submissionIDStr := vars["submissionId"]

	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		http.Error(w, "invalid submission ID", http.StatusBadRequest)
		return
	}

	if err := getWorkflowEngineDedicated().DeleteSubmissionDedicated(formCode, submissionID, claims.UserID); err != nil {
		log.Printf("❌ Error deleting submission: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Deleted submission: %s", submissionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "submission deleted successfully",
	})
}
