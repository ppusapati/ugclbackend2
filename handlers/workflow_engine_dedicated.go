package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowEngineDedicated handles workflow state transitions using dedicated form tables
type WorkflowEngineDedicated struct {
	db          *gorm.DB
	tableManager *FormTableManager
}

// NewWorkflowEngineDedicated creates a new workflow engine instance that uses dedicated tables
func NewWorkflowEngineDedicated() *WorkflowEngineDedicated {
	return &WorkflowEngineDedicated{
		db:          config.DB,
		tableManager: NewFormTableManager(),
	}
}

// FormSubmissionRecord represents a record in a dedicated form table
type FormSubmissionRecord struct {
	ID                 uuid.UUID               `json:"id"`
	FormID             uuid.UUID               `json:"form_id"`
	FormCode           string                  `json:"form_code"`
	BusinessVerticalID uuid.UUID               `json:"business_vertical_id"`
	SiteID             *uuid.UUID              `json:"site_id,omitempty"`
	WorkflowID         *uuid.UUID              `json:"workflow_id,omitempty"`
	CurrentState       string                  `json:"current_state"`
	CreatedBy          string                  `json:"created_by"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedBy          string                  `json:"updated_by,omitempty"`
	UpdatedAt          time.Time               `json:"updated_at"`
	DeletedBy          string                  `json:"deleted_by,omitempty"`
	DeletedAt          *time.Time              `json:"deleted_at,omitempty"`
	FormData           map[string]interface{}  `json:"form_data"` // Additional form fields
	Form               *models.AppForm         `json:"form,omitempty"`
	Workflow           *models.WorkflowDefinition `json:"workflow,omitempty"`
}

// CreateSubmissionDedicated creates a new form submission in the dedicated form table
func (we *WorkflowEngineDedicated) CreateSubmissionDedicated(
	formCode string,
	businessVerticalID uuid.UUID,
	siteID *uuid.UUID,
	formData map[string]interface{},
	userID string,
) (*FormSubmissionRecord, error) {
	// Get the form definition
	var form models.AppForm
	if err := we.db.Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		return nil, fmt.Errorf("form not found: %w", err)
	}

	// Validate that form has a dedicated table
	if form.DBTableName == "" {
		return nil, fmt.Errorf("form %s does not have a dedicated table configured", formCode)
	}

	// Check if table exists, create if not
	exists, err := we.tableManager.TableExists(form.DBTableName)
	if err != nil {
		return nil, fmt.Errorf("failed to check table existence: %w", err)
	}
	if !exists {
		if err := we.tableManager.CreateFormTable(&form); err != nil {
			return nil, fmt.Errorf("failed to create form table: %w", err)
		}
	}

	// Get workflow definition if specified
	var workflowDef *models.WorkflowDefinition
	if form.WorkflowID != nil {
		workflowDef = &models.WorkflowDefinition{}
		if err := we.db.First(workflowDef, "id = ? AND is_active = ?", form.WorkflowID, true).Error; err != nil {
			log.Printf("⚠️  Workflow not found for form %s: %v", formCode, err)
			workflowDef = nil
		}
	}

	// Determine initial state
	initialState := form.InitialState
	if initialState == "" {
		initialState = "draft"
	}
	if workflowDef != nil && workflowDef.InitialState != "" {
		initialState = workflowDef.InitialState
	}

	// Insert data into dedicated table
	recordID, err := we.tableManager.InsertFormData(
		form.DBTableName,
		form.ID,
		formCode,
		businessVerticalID,
		siteID,
		form.WorkflowID,
		initialState,
		formData,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	log.Printf("✅ Created form submission in %s: %s (state: %s)", form.DBTableName, recordID, initialState)

	// Retrieve and return the created record
	return we.GetSubmissionDedicated(form.DBTableName, recordID)
}

// TransitionStateDedicated performs a workflow state transition for a dedicated table record
func (we *WorkflowEngineDedicated) TransitionStateDedicated(
	formCode string,
	recordID uuid.UUID,
	action string,
	actorID string,
	actorName string,
	actorRole string,
	comment string,
	metadata map[string]interface{},
) (*FormSubmissionRecord, error) {
	// Get the form definition
	var form models.AppForm
	if err := we.db.Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		return nil, fmt.Errorf("form not found: %w", err)
	}

	if form.DBTableName == "" {
		return nil, fmt.Errorf("form %s does not have a dedicated table configured", formCode)
	}

	// Get the submission record
	record, err := we.GetSubmissionDedicated(form.DBTableName, recordID)
	if err != nil {
		return nil, fmt.Errorf("submission not found: %w", err)
	}

	// Get workflow definition
	if record.WorkflowID == nil {
		return nil, errors.New("no workflow defined for this form")
	}

	var workflowDef models.WorkflowDefinition
	if err := we.db.First(&workflowDef, "id = ?", record.WorkflowID).Error; err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	// Parse workflow transitions
	var transitions []models.WorkflowTransitionDef
	if err := json.Unmarshal(workflowDef.Transitions, &transitions); err != nil {
		return nil, fmt.Errorf("invalid workflow configuration: %w", err)
	}

	// Find the matching transition
	var targetTransition *models.WorkflowTransitionDef
	for _, t := range transitions {
		if t.From == record.CurrentState && t.Action == action {
			targetTransition = &t
			break
		}
	}

	if targetTransition == nil {
		return nil, fmt.Errorf("invalid transition: action '%s' not allowed from state '%s'", action, record.CurrentState)
	}

	// Validate required comment
	if targetTransition.RequiresComment && comment == "" {
		return nil, errors.New("comment is required for this action")
	}

	// Store previous state
	previousState := record.CurrentState

	// Begin transaction
	tx := we.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update workflow state in the dedicated table
	if err := we.tableManager.UpdateWorkflowState(form.DBTableName, recordID, targetTransition.To, actorID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update submission state: %w", err)
	}

	// Create transition record in workflow_transitions table
	metadataJSON, _ := json.Marshal(metadata)
	transition := models.WorkflowTransition{
		SubmissionID:   recordID, // Use record ID as submission ID
		FromState:      previousState,
		ToState:        targetTransition.To,
		Action:         action,
		ActorID:        actorID,
		ActorName:      actorName,
		ActorRole:      actorRole,
		Comment:        comment,
		Metadata:       metadataJSON,
		TransitionedAt: time.Now(),
	}

	if err := tx.Create(&transition).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create transition record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✅ Transitioned submission %s in %s: %s -> %s (action: %s, actor: %s)",
		recordID, form.DBTableName, previousState, targetTransition.To, action, actorName)

	// Process notifications (if configured)
	// Note: You may need to adapt notification processing for dedicated tables
	notifService := NewNotificationService()
	// Create a temporary FormSubmission-like structure for notification processing
	tempSubmission := &models.FormSubmission{
		ID:                 recordID,
		FormCode:           formCode,
		FormID:             form.ID,
		BusinessVerticalID: record.BusinessVerticalID,
		CurrentState:       targetTransition.To,
		Form:               &form,
		Workflow:           &workflowDef,
	}

	if err := notifService.ProcessTransitionNotifications(tempSubmission, &transition, &workflowDef, targetTransition, actorName); err != nil {
		log.Printf("⚠️  Failed to process notifications: %v", err)
		// Don't fail the transition if notifications fail
	}

	// Retrieve and return updated record
	return we.GetSubmissionDedicated(form.DBTableName, recordID)
}

// UpdateSubmissionDataDedicated updates the form data in the dedicated table
func (we *WorkflowEngineDedicated) UpdateSubmissionDataDedicated(
	formCode string,
	recordID uuid.UUID,
	formData map[string]interface{},
	userID string,
) (*FormSubmissionRecord, error) {
	// Get the form definition
	var form models.AppForm
	if err := we.db.Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		return nil, fmt.Errorf("form not found: %w", err)
	}

	if form.DBTableName == "" {
		return nil, fmt.Errorf("form %s does not have a dedicated table configured", formCode)
	}

	// Get current record
	record, err := we.GetSubmissionDedicated(form.DBTableName, recordID)
	if err != nil {
		return nil, fmt.Errorf("submission not found: %w", err)
	}

	// Only allow updates in draft state
	if record.CurrentState != "draft" {
		return nil, fmt.Errorf("cannot update submission in state '%s' - only draft submissions can be edited", record.CurrentState)
	}

	// Update data in dedicated table
	if err := we.tableManager.UpdateFormData(form.DBTableName, recordID, formData, userID); err != nil {
		return nil, fmt.Errorf("failed to update submission: %w", err)
	}

	log.Printf("✅ Updated submission data in %s: %s", form.DBTableName, recordID)

	// Retrieve and return updated record
	return we.GetSubmissionDedicated(form.DBTableName, recordID)
}

// GetSubmissionDedicated retrieves a submission by ID from the dedicated table
func (we *WorkflowEngineDedicated) GetSubmissionDedicated(tableName string, recordID uuid.UUID) (*FormSubmissionRecord, error) {
	data, err := we.tableManager.GetFormData(tableName, recordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get submission: %w", err)
	}

	// Convert to FormSubmissionRecord
	record := &FormSubmissionRecord{
		FormData: make(map[string]interface{}),
	}

	// Extract base fields
	if id, ok := data["id"].([]byte); ok {
		record.ID, _ = uuid.FromBytes(id)
	}
	if formID, ok := data["form_id"].([]byte); ok {
		record.FormID, _ = uuid.FromBytes(formID)
	}
	if formCode, ok := data["form_code"].(string); ok {
		record.FormCode = formCode
	}
	if bizID, ok := data["business_vertical_id"].([]byte); ok {
		record.BusinessVerticalID, _ = uuid.FromBytes(bizID)
	}
	if state, ok := data["current_state"].(string); ok {
		record.CurrentState = state
	}
	if createdBy, ok := data["created_by"].(string); ok {
		record.CreatedBy = createdBy
	}
	if createdAt, ok := data["created_at"].(time.Time); ok {
		record.CreatedAt = createdAt
	}

	// Store all other fields as form data
	baseFields := map[string]bool{
		"id": true, "form_id": true, "form_code": true, "business_vertical_id": true,
		"site_id": true, "workflow_id": true, "current_state": true,
		"created_by": true, "created_at": true, "updated_by": true, "updated_at": true,
		"deleted_by": true, "deleted_at": true,
	}

	for key, val := range data {
		if !baseFields[key] {
			record.FormData[key] = val
		}
	}

	return record, nil
}

// GetSubmissionsByFormDedicated retrieves all submissions for a specific form from dedicated table
func (we *WorkflowEngineDedicated) GetSubmissionsByFormDedicated(
	formCode string,
	businessVerticalID uuid.UUID,
	filters map[string]interface{},
) ([]*FormSubmissionRecord, error) {
	// Get the form definition
	var form models.AppForm
	if err := we.db.Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		return nil, fmt.Errorf("form not found: %w", err)
	}

	if form.DBTableName == "" {
		return nil, fmt.Errorf("form %s does not have a dedicated table configured", formCode)
	}

	// Get data from dedicated table
	dataList, err := we.tableManager.GetFormDataList(form.DBTableName, businessVerticalID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch submissions: %w", err)
	}

	// Convert to records
	records := make([]*FormSubmissionRecord, 0, len(dataList))
	for _, data := range dataList {
		record := &FormSubmissionRecord{
			FormData: make(map[string]interface{}),
		}

		// Extract base fields (similar to GetSubmissionDedicated)
		if id, ok := data["id"].([]byte); ok {
			record.ID, _ = uuid.FromBytes(id)
		}
		if formCode, ok := data["form_code"].(string); ok {
			record.FormCode = formCode
		}
		if state, ok := data["current_state"].(string); ok {
			record.CurrentState = state
		}

		// Store other fields as form data
		baseFields := map[string]bool{
			"id": true, "form_id": true, "form_code": true, "business_vertical_id": true,
			"site_id": true, "workflow_id": true, "current_state": true,
			"created_by": true, "created_at": true, "updated_by": true, "updated_at": true,
			"deleted_by": true, "deleted_at": true,
		}

		for key, val := range data {
			if !baseFields[key] {
				record.FormData[key] = val
			}
		}

		records = append(records, record)
	}

	return records, nil
}

// DeleteSubmissionDedicated soft deletes a submission from the dedicated table
func (we *WorkflowEngineDedicated) DeleteSubmissionDedicated(
	formCode string,
	recordID uuid.UUID,
	userID string,
) error {
	// Get the form definition
	var form models.AppForm
	if err := we.db.Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	if form.DBTableName == "" {
		return fmt.Errorf("form %s does not have a dedicated table configured", formCode)
	}

	// Soft delete from dedicated table
	if err := we.tableManager.SoftDeleteFormData(form.DBTableName, recordID, userID); err != nil {
		return fmt.Errorf("failed to delete submission: %w", err)
	}

	log.Printf("✅ Deleted submission in %s: %s", form.DBTableName, recordID)
	return nil
}

// GetWorkflowHistoryDedicated retrieves the complete transition history (from workflow_transitions)
func (we *WorkflowEngineDedicated) GetWorkflowHistoryDedicated(recordID uuid.UUID) ([]models.WorkflowTransition, error) {
	var transitions []models.WorkflowTransition
	if err := we.db.
		Where("submission_id = ?", recordID).
		Order("transitioned_at ASC").
		Find(&transitions).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch workflow history: %w", err)
	}

	return transitions, nil
}

// ValidateTransitionDedicated checks if a transition is valid without executing it
func (we *WorkflowEngineDedicated) ValidateTransitionDedicated(
	formCode string,
	recordID uuid.UUID,
	action string,
	userPermissions []string,
) error {
	// Get the form definition
	var form models.AppForm
	if err := we.db.Preload("Workflow").Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	if form.DBTableName == "" {
		return fmt.Errorf("form %s does not have a dedicated table configured", formCode)
	}

	// Get current record
	record, err := we.GetSubmissionDedicated(form.DBTableName, recordID)
	if err != nil {
		return fmt.Errorf("submission not found: %w", err)
	}

	if record.WorkflowID == nil {
		return errors.New("no workflow defined for this form")
	}

	var workflowDef models.WorkflowDefinition
	if err := we.db.First(&workflowDef, "id = ?", record.WorkflowID).Error; err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	// Parse transitions
	var transitions []models.WorkflowTransitionDef
	if err := json.Unmarshal(workflowDef.Transitions, &transitions); err != nil {
		return fmt.Errorf("invalid workflow configuration: %w", err)
	}

	// Find matching transition
	for _, t := range transitions {
		if t.From == record.CurrentState && t.Action == action {
			// Check permission if required
			if t.Permission != "" {
				hasPermission := false
				for _, perm := range userPermissions {
					if perm == t.Permission || perm == "admin_all" {
						hasPermission = true
						break
					}
				}
				if !hasPermission {
					return fmt.Errorf("insufficient permissions: requires '%s'", t.Permission)
				}
			}
			return nil // Valid transition
		}
	}

	return fmt.Errorf("invalid transition: action '%s' not allowed from state '%s'", action, record.CurrentState)
}
