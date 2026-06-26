package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowEngine handles workflow state transitions
type WorkflowEngine struct {
	db *gorm.DB
}

// NewWorkflowEngine creates a new workflow engine instance
func NewWorkflowEngine() *WorkflowEngine {
	return &WorkflowEngine{
		db: config.DB,
	}
}

// CreateSubmission creates a new form submission with initial workflow state
func (we *WorkflowEngine) CreateSubmission(
	formCode string,
	businessVerticalID uuid.UUID,
	siteID *uuid.UUID,
	formData json.RawMessage,
	latitude *float64,
	longitude *float64,
	userID string,
) (*models.FormSubmission, error) {
	// Get the form definition
	var form models.AppForm
	if err := we.db.Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		return nil, fmt.Errorf("form not found: %w", err)
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

	// Resolve reference field UUIDs into readable display objects where supported
	// so downstream reporting can show human-friendly values.
	enhancedFormData := formData
	if len(formData) > 0 && string(formData) != "null" {
		var formDataMap map[string]interface{}
		if err := json.Unmarshal(formData, &formDataMap); err == nil {
			resolvedMap := NewWorkflowEngineDedicated().ResolveFormFieldValues(&form, formDataMap)
			if resolvedJSON, marshalErr := json.Marshal(resolvedMap); marshalErr == nil {
				enhancedFormData = resolvedJSON
			}
		}
	}

	// Create submission
	submission := &models.FormSubmission{
		FormCode:           formCode,
		FormID:             form.ID,
		BusinessVerticalID: businessVerticalID,
		SiteID:             siteID,
		WorkflowID:         form.WorkflowID,
		CurrentState:       initialState,
		FormData:           enhancedFormData,
		SubmittedBy:        userID,
		SubmittedAt:        time.Now(),
		Latitude:           latitude,
		Longitude:          longitude,
		LastModifiedBy:     userID,
		LastModifiedAt:     time.Now(),
		Version:            1,
	}

	if err := we.db.Create(submission).Error; err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	log.Printf("✅ Created form submission: %s (state: %s)", submission.ID, submission.CurrentState)
	return submission, nil
}

// TransitionState performs a workflow state transition
func (we *WorkflowEngine) TransitionState(
	submissionID uuid.UUID,
	action string,
	actorID string,
	actorName string,
	actorRole string,
	comment string,
	metadata map[string]interface{},
) (*models.FormSubmission, error) {
	// Get the submission with its workflow
	var submission models.FormSubmission
	if err := we.db.Preload("Form").Preload("Workflow").First(&submission, "id = ?", submissionID).Error; err != nil {
		return nil, fmt.Errorf("submission not found: %w", err)
	}

	// Get workflow definition
	if submission.Workflow == nil {
		return nil, errors.New("no workflow defined for this form")
	}

	// Parse workflow transitions
	var transitions []models.WorkflowTransitionDef
	if err := json.Unmarshal(submission.Workflow.Transitions, &transitions); err != nil {
		return nil, fmt.Errorf("invalid workflow configuration: %w", err)
	}

	// Find the matching transition
	var targetTransition *models.WorkflowTransitionDef
	for _, t := range transitions {
		if t.From == submission.CurrentState && t.Action == action {
			targetTransition = &t
			break
		}
	}

	if targetTransition == nil {
		return nil, fmt.Errorf("invalid transition: action '%s' not allowed from state '%s'", action, submission.CurrentState)
	}

	// Validate required comment
	if targetTransition.RequiresComment && comment == "" {
		return nil, errors.New("comment is required for this action")
	}

	// Store previous state
	previousState := submission.CurrentState

	// Begin transaction
	tx := we.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update submission state
	submission.CurrentState = targetTransition.To
	submission.LastModifiedBy = actorID
	submission.LastModifiedAt = time.Now()
	submission.Version++

	if err := tx.Save(&submission).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update submission: %w", err)
	}

	// Create transition record
	metadataJSON, _ := json.Marshal(metadata)
	transition := models.WorkflowTransition{
		SubmissionID:   submissionID,
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

	log.Printf("✅ Transitioned submission %s: %s -> %s (action: %s, actor: %s)",
		submissionID, previousState, targetTransition.To, action, actorName)

	// Process notifications (after transaction commit)
	// Reload submission with relationships for notification context
	we.db.Preload("Form").Preload("Workflow").Preload("BusinessVertical").First(&submission, submissionID)

	notifService := NewNotificationService()
	if err := notifService.ProcessTransitionNotifications(&submission, &transition, submission.Workflow, targetTransition, actorName); err != nil {
		log.Printf("⚠️  Failed to process notifications: %v", err)
		// Don't fail the transition if notifications fail
	}

	return &submission, nil
}

// UpdateSubmissionData updates the form data of a submission (only in draft state)
func (we *WorkflowEngine) UpdateSubmissionData(
	submissionID uuid.UUID,
	formData json.RawMessage,
	latitude *float64,
	longitude *float64,
	userID string,
) (*models.FormSubmission, error) {
	var submission models.FormSubmission
	if err := we.db.First(&submission, "id = ?", submissionID).Error; err != nil {
		return nil, fmt.Errorf("submission not found: %w", err)
	}

	// Only allow updates in draft state
	if submission.CurrentState != "draft" {
		return nil, fmt.Errorf("cannot update submission in state '%s' - only draft submissions can be edited", submission.CurrentState)
	}

	// Resolve reference field UUIDs into readable display objects where supported.
	enhancedFormData := formData
	if len(formData) > 0 && string(formData) != "null" {
		var form models.AppForm
		if err := we.db.Where("id = ?", submission.FormID).First(&form).Error; err == nil {
			var formDataMap map[string]interface{}
			if err := json.Unmarshal(formData, &formDataMap); err == nil {
				resolvedMap := NewWorkflowEngineDedicated().ResolveFormFieldValues(&form, formDataMap)
				if resolvedJSON, marshalErr := json.Marshal(resolvedMap); marshalErr == nil {
					enhancedFormData = resolvedJSON
				}
			}
		}
	}

	// Update data
	submission.FormData = enhancedFormData
	submission.Latitude = latitude
	submission.Longitude = longitude
	submission.LastModifiedBy = userID
	submission.LastModifiedAt = time.Now()
	submission.Version++

	if err := we.db.Save(&submission).Error; err != nil {
		return nil, fmt.Errorf("failed to update submission: %w", err)
	}

	log.Printf("✅ Updated submission data: %s", submissionID)
	return &submission, nil
}

// GetSubmission retrieves a submission by ID with workflow details
func (we *WorkflowEngine) GetSubmission(submissionID uuid.UUID) (*models.FormSubmission, error) {
	var submission models.FormSubmission
	if err := we.db.
		Preload("Form").
		Preload("Workflow").
		Preload("BusinessVertical").
		Preload("Transitions", func(db *gorm.DB) *gorm.DB {
			return db.Order("transitioned_at DESC")
		}).
		First(&submission, "id = ?", submissionID).Error; err != nil {
		return nil, fmt.Errorf("submission not found: %w", err)
	}

	return &submission, nil
}

// GetSubmissionsByForm retrieves all submissions for a specific form
func (we *WorkflowEngine) GetSubmissionsByForm(
	formCode string,
	businessVerticalID uuid.UUID,
	filters map[string]interface{},
) ([]models.FormSubmission, error) {
	query := we.db.
		Preload("Form").
		Preload("Workflow").
		Where("form_code = ? AND business_vertical_id = ?", formCode, businessVerticalID)

	// Apply state filter if provided
	if state, ok := filters["state"].(string); ok && state != "" {
		query = query.Where("current_state = ?", state)
	}

	// Apply site filter if provided
	if siteID, ok := filters["site_id"].(uuid.UUID); ok {
		query = query.Where("site_id = ?", siteID)
	}

	// Apply user filter if provided
	if userID, ok := filters["submitted_by"].(string); ok && userID != "" {
		query = query.Where("submitted_by = ?", userID)
	}

	var submissions []models.FormSubmission
	if err := query.Order("submitted_at DESC").Find(&submissions).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch submissions: %w", err)
	}

	return submissions, nil
}

// GetSubmissionsByFormPage retrieves submissions for a form using keyset pagination.
func (we *WorkflowEngine) GetSubmissionsByFormPage(
	formCode string,
	businessVerticalID uuid.UUID,
	filters map[string]interface{},
	limit int,
	cursor *submissionsCursor,
) ([]models.FormSubmission, error) {
	if limit <= 0 {
		limit = defaultSubmissionPageSize
	}

	query := we.db.
		Preload("Form").
		Preload("Workflow").
		Where("form_code = ? AND business_vertical_id = ?", formCode, businessVerticalID)

	if state, ok := filters["state"].(string); ok && state != "" {
		query = query.Where("current_state = ?", state)
	}

	if siteID, ok := filters["site_id"].(uuid.UUID); ok {
		query = query.Where("site_id = ?", siteID)
	}

	if userID, ok := filters["submitted_by"].(string); ok && userID != "" {
		query = query.Where("submitted_by = ?", userID)
	}

	if cursor != nil {
		query = query.Where("(submitted_at, id) < (?, ?)", cursor.Timestamp.UTC(), cursor.ID)
	}

	var submissions []models.FormSubmission
	if err := query.Order("submitted_at DESC, id DESC").Limit(limit).Find(&submissions).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch submissions: %w", err)
	}

	return submissions, nil
}

// GetWorkflowHistory retrieves the complete transition history for a submission
func (we *WorkflowEngine) GetWorkflowHistory(submissionID uuid.UUID) ([]models.WorkflowTransition, error) {
	var transitions []models.WorkflowTransition
	if err := we.db.
		Where("submission_id = ?", submissionID).
		Order("transitioned_at ASC").
		Find(&transitions).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch workflow history: %w", err)
	}

	return transitions, nil
}

// ValidateTransition checks if a transition is valid without executing it
func (we *WorkflowEngine) ValidateTransition(
	submissionID uuid.UUID,
	action string,
	userPermissions []string,
) error {
	var submission models.FormSubmission
	if err := we.db.Preload("Workflow").First(&submission, "id = ?", submissionID).Error; err != nil {
		return fmt.Errorf("submission not found: %w", err)
	}

	if submission.Workflow == nil {
		return errors.New("no workflow defined for this form")
	}

	// Parse transitions
	var transitions []models.WorkflowTransitionDef
	if err := json.Unmarshal(submission.Workflow.Transitions, &transitions); err != nil {
		return fmt.Errorf("invalid workflow configuration: %w", err)
	}

	// Find matching transition
	for _, t := range transitions {
		if t.From == submission.CurrentState && t.Action == action {
			// Check permission if required
			if t.Permission != "" {
				hasPermission := false
				for _, perm := range userPermissions {
					if perm == "admin_all" || perm == "*:*:*" || utils.MatchesPermission(perm, t.Permission) {
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

	return fmt.Errorf("invalid transition: action '%s' not allowed from state '%s'", action, submission.CurrentState)
}

// GetWorkflowStats returns statistics about submissions in different states
func (we *WorkflowEngine) GetWorkflowStats(formCode string, businessVerticalID uuid.UUID) (map[string]int64, error) {
	type StateCount struct {
		State string
		Count int64
	}

	var results []StateCount
	if err := we.db.Model(&models.FormSubmission{}).
		Select("current_state as state, count(*) as count").
		Where("form_code = ? AND business_vertical_id = ?", formCode, businessVerticalID).
		Group("current_state").
		Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch stats: %w", err)
	}

	stats := make(map[string]int64)
	for _, r := range results {
		stats[r.State] = r.Count
	}

	return stats, nil
}
