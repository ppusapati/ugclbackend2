package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowEngineDedicated handles workflow state transitions using dedicated form tables
type WorkflowEngineDedicated struct {
	db           *gorm.DB
	tableManager *FormTableManager
}

var lookupIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// NewWorkflowEngineDedicated creates a new workflow engine instance that uses dedicated tables
func NewWorkflowEngineDedicated() *WorkflowEngineDedicated {
	return &WorkflowEngineDedicated{
		db:           config.DB,
		tableManager: NewFormTableManager(),
	}
}

// FormSubmissionRecord represents a record in a dedicated form table
type FormSubmissionRecord struct {
	ID                 uuid.UUID                  `json:"id"`
	FormID             uuid.UUID                  `json:"form_id"`
	FormCode           string                     `json:"form_code"`
	BusinessVerticalID uuid.UUID                  `json:"business_vertical_id"`
	SiteID             *uuid.UUID                 `json:"site_id,omitempty"`
	WorkflowID         *uuid.UUID                 `json:"workflow_id,omitempty"`
	CurrentState       string                     `json:"current_state"`
	CreatedBy          string                     `json:"created_by"`
	CreatedAt          time.Time                  `json:"created_at"`
	UpdatedBy          string                     `json:"updated_by,omitempty"`
	UpdatedAt          time.Time                  `json:"updated_at"`
	DeletedBy          string                     `json:"deleted_by,omitempty"`
	DeletedAt          *time.Time                 `json:"deleted_at,omitempty"`
	FormData           map[string]interface{}     `json:"form_data"` // Additional form fields
	Form               *models.AppForm            `json:"form,omitempty"`
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

	log.Printf("🔍 Table %s exists: %v", form.DBTableName, exists)

	if !exists {
		// Check if form has schema or steps defined
		var formSchema map[string]interface{}
		hasSchema := false

		log.Printf("🔍 Checking form_schema: len=%d, value=%s", len(form.FormSchema), string(form.FormSchema))
		log.Printf("🔍 Checking steps: len=%d, value=%s", len(form.Steps), string(form.Steps))

		// Check form_schema
		if len(form.FormSchema) > 0 && string(form.FormSchema) != "{}" {
			if err := json.Unmarshal(form.FormSchema, &formSchema); err == nil {
				if fields, ok := formSchema["fields"].([]interface{}); ok && len(fields) > 0 {
					hasSchema = true
					log.Printf("✅ Found %d fields in form_schema", len(fields))
				}
			}
		}

		// Check steps if no form_schema
		if !hasSchema && len(form.Steps) > 0 && string(form.Steps) != "[]" {
			log.Printf("📋 Extracting fields from steps...")
			fields, err := we.tableManager.ExtractFieldsFromSteps(form.Steps)
			if err == nil && len(fields) > 0 {
				hasSchema = true
				log.Printf("✅ Found %d fields from steps", len(fields))
			} else if err != nil {
				log.Printf("❌ Error extracting fields from steps: %v", err)
			}
		}

		// If no schema or steps, infer from submitted data
		if !hasSchema {
			log.Printf("🔍 Form %s has no schema/steps - inferring from submission data", formCode)
			inferredSchema := we.tableManager.InferSchemaFromData(formData)
			if err := we.tableManager.CreateFormTableWithSchema(&form, inferredSchema); err != nil {
				return nil, fmt.Errorf("failed to create form table with inferred schema: %w", err)
			}
		} else {
			// Use existing schema or steps
			log.Printf("📊 Creating table using existing schema/steps")
			if err := we.tableManager.CreateFormTable(&form); err != nil {
				return nil, fmt.Errorf("failed to create form table: %w", err)
			}
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

	// Resolve reference field values (UUIDs to display names)
	enhancedFormData := we.ResolveFormFieldValues(&form, formData)

	// Insert data into dedicated table
	recordID, err := we.tableManager.InsertFormData(
		form.DBTableName,
		form.ID,
		formCode,
		businessVerticalID,
		siteID,
		form.WorkflowID,
		initialState,
		enhancedFormData,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	log.Printf("✅ Created form submission in %s: %s (state: %s)", form.DBTableName, recordID, initialState)

	// Retrieve and return the created record
	return we.GetSubmissionDedicated(form.DBTableName, recordID)
}

// ResolveFormFieldValues enhances form data by resolving reference fields to display names
// For fields with dataSource: "api" or reference types, this function fetches the display value
// E.g., converts UUID of dairy site to "Malabad Dairy Site"
func (we *WorkflowEngineDedicated) ResolveFormFieldValues(form *models.AppForm, formData map[string]interface{}) map[string]interface{} {
	resolvedData := make(map[string]interface{})
	for k, v := range formData {
		resolvedData[k] = v
	}

	// Extract field definitions from form schema
	fieldDefs := make(map[string]map[string]interface{})

	var formSchema map[string]interface{}
	if err := json.Unmarshal(form.FormSchema, &formSchema); err == nil {
		// Extract from top-level fields
		if fields, ok := formSchema["fields"].([]interface{}); ok {
			for _, f := range fields {
				if fieldMap, ok := f.(map[string]interface{}); ok {
					if id, ok := fieldMap["id"].(string); ok {
						fieldDefs[id] = fieldMap
					}
				}
			}
		}

		// Extract from steps
		if steps, ok := formSchema["steps"].([]interface{}); ok {
			for _, step := range steps {
				if stepMap, ok := step.(map[string]interface{}); ok {
					if stepFields, ok := stepMap["fields"].([]interface{}); ok {
						for _, f := range stepFields {
							if fieldMap, ok := f.(map[string]interface{}); ok {
								if id, ok := fieldMap["id"].(string); ok {
									fieldDefs[id] = fieldMap
								}
							}
						}
					}
				}
			}
		}
	}

	// Resolve each field in the form data
	for fieldID, value := range formData {
		fieldDef, exists := fieldDefs[fieldID]
		if !exists {
			continue
		}

		// Only resolve if the field has a data source (API lookup)
		dataSource, ok := fieldDef["dataSource"].(string)
		if !ok || dataSource != "api" {
			continue
		}

		// Get the display and value fields
		displayField, _ := fieldDef["displayField"].(string)
		if displayField == "" {
			displayField = "name"
		}
		valueField, _ := fieldDef["valueField"].(string)
		if valueField == "" {
			valueField = "id"
		}

		// If value is a string (UUID), try to resolve it
		if strVal, ok := value.(string); ok {
			displayValue := we.resolveReferenceValue(fieldDef, strVal, displayField, valueField)
			if displayValue != "" {
				// Store an object with both UUID and display name
				resolvedData[fieldID] = map[string]interface{}{
					"id":   strVal,       // UUID
					"name": displayValue, // Human-readable name
				}
			}
		}

		// Multi-select reference field support.
		if listVal, ok := value.([]interface{}); ok {
			resolvedList := make([]interface{}, 0, len(listVal))
			for _, item := range listVal {
				itemStr, isString := item.(string)
				if !isString {
					resolvedList = append(resolvedList, item)
					continue
				}
				displayValue := we.resolveReferenceValue(fieldDef, itemStr, displayField, valueField)
				if displayValue == "" {
					resolvedList = append(resolvedList, itemStr)
					continue
				}
				resolvedList = append(resolvedList, map[string]interface{}{
					"id":   itemStr,
					"name": displayValue,
				})
			}
			resolvedData[fieldID] = resolvedList
		}
	}

	return resolvedData
}

// resolveReferenceValue looks up the display value for a reference field
func (we *WorkflowEngineDedicated) resolveReferenceValue(fieldDef map[string]interface{}, valueID string, displayField string, valueField string) string {
	apiEndpoint, ok := fieldDef["apiEndpoint"].(string)
	if !ok || apiEndpoint == "" {
		return ""
	}
	if strings.TrimSpace(valueID) == "" {
		return ""
	}

	safeDisplayField := sanitizeLookupIdentifier(displayField, "name")
	safeValueField := sanitizeLookupIdentifier(valueField, "id")
	normalizedEndpoint := strings.ToLower(strings.TrimSpace(apiEndpoint))

	// Dynamic form lookup endpoints are computed payloads and do not map to one SQL table.
	// Skip direct table resolution to avoid incorrect joins like business_verticals.bg_number.
	if strings.Contains(normalizedEndpoint, "/forms/") && strings.Contains(normalizedEndpoint, "/lookup") {
		return ""
	}

	// First-class resolvers for common reference entities.
	if strings.Contains(normalizedEndpoint, "/sites") {
		return we.resolveByTable("sites", valueID, safeDisplayField, safeValueField, "deleted_at IS NULL")
	}
	if strings.Contains(normalizedEndpoint, "/users") {
		return we.resolveByTable("users", valueID, safeDisplayField, safeValueField, "is_active = TRUE")
	}
	if strings.Contains(normalizedEndpoint, "business") || strings.Contains(normalizedEndpoint, "vertical") {
		return we.resolveByTable("business_verticals", valueID, safeDisplayField, safeValueField, "is_active = TRUE")
	}
	if strings.Contains(normalizedEndpoint, "/modules") {
		return we.resolveByTable("modules", valueID, safeDisplayField, safeValueField, "is_active = TRUE")
	}
	if strings.Contains(normalizedEndpoint, "business-roles") || strings.Contains(normalizedEndpoint, "business_roles") {
		return we.resolveByTable("business_roles", valueID, safeDisplayField, safeValueField, "is_active = TRUE")
	}
	if strings.Contains(normalizedEndpoint, "/roles") {
		if val := we.resolveByTable("roles", valueID, safeDisplayField, safeValueField, "is_active = TRUE"); val != "" {
			return val
		}
		return we.resolveByTable("business_roles", valueID, safeDisplayField, safeValueField, "is_active = TRUE")
	}

	// Generic fallback: infer table from endpoint tail segment.
	if inferredTable := inferTableNameFromEndpoint(normalizedEndpoint); inferredTable != "" {
		if val := we.resolveByTable(inferredTable, valueID, safeDisplayField, safeValueField, ""); val != "" {
			return val
		}
	}

	log.Printf("⚠️  Reference resolution did not match endpoint: %s", apiEndpoint)
	return ""
}

func (we *WorkflowEngineDedicated) resolveByTable(tableName string, valueID string, displayField string, valueField string, extraCondition string) string {
	if strings.TrimSpace(tableName) == "" || strings.TrimSpace(valueID) == "" {
		return ""
	}
	safeTableName := sanitizeLookupIdentifier(tableName, "")
	if safeTableName == "" {
		return ""
	}
	safeDisplayField := sanitizeLookupIdentifier(displayField, "name")
	safeValueField := sanitizeLookupIdentifier(valueField, "id")

	query := we.db.Table(safeTableName).Select(safeDisplayField)
	if safeValueField == "id" {
		query = query.Where("id::text = ?", valueID)
	} else {
		query = query.Where(fmt.Sprintf("%s = ?", safeValueField), valueID)
	}
	if strings.TrimSpace(extraCondition) != "" {
		query = query.Where(extraCondition)
	}

	var resolved string
	if err := query.Limit(1).Scan(&resolved).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(resolved)
}

func sanitizeLookupIdentifier(value string, fallback string) string {
	v := strings.TrimSpace(value)
	if lookupIdentifierPattern.MatchString(v) {
		return v
	}
	if fallback != "" {
		if lookupIdentifierPattern.MatchString(strings.TrimSpace(fallback)) {
			return strings.TrimSpace(fallback)
		}
	}
	return ""
}

func inferTableNameFromEndpoint(endpoint string) string {
	trimmed := strings.Split(strings.TrimSpace(endpoint), "?")[0]
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" || strings.HasPrefix(part, "{") {
			continue
		}
		part = strings.ReplaceAll(part, "-", "_")
		if !lookupIdentifierPattern.MatchString(part) {
			continue
		}
		return part
	}
	return ""
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
		SubmittedBy:        record.CreatedBy,
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

// GetSubmissionsByFormDedicatedPage retrieves submissions for a form from dedicated table using keyset pagination.
func (we *WorkflowEngineDedicated) GetSubmissionsByFormDedicatedPage(
	formCode string,
	businessVerticalID uuid.UUID,
	filters map[string]interface{},
	limit int,
	cursor *submissionsCursor,
) ([]*FormSubmissionRecord, error) {
	var form models.AppForm
	if err := we.db.Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		return nil, fmt.Errorf("form not found: %w", err)
	}

	if form.DBTableName == "" {
		return nil, fmt.Errorf("form %s does not have a dedicated table configured", formCode)
	}

	dataList, err := we.tableManager.GetFormDataListPage(form.DBTableName, businessVerticalID, filters, limit, cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch submissions: %w", err)
	}

	records := make([]*FormSubmissionRecord, 0, len(dataList))
	for _, data := range dataList {
		record := &FormSubmissionRecord{FormData: make(map[string]interface{})}

		if id, ok := data["id"].([]byte); ok {
			record.ID, _ = uuid.FromBytes(id)
		} else if idStr, ok := data["id"].(string); ok {
			if parsedID, parseErr := uuid.Parse(idStr); parseErr == nil {
				record.ID = parsedID
			}
		}
		if createdAt, ok := data["created_at"].(time.Time); ok {
			record.CreatedAt = createdAt
		}
		if formCodeVal, ok := data["form_code"].(string); ok {
			record.FormCode = formCodeVal
		}
		if state, ok := data["current_state"].(string); ok {
			record.CurrentState = state
		}

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

	return fmt.Errorf("invalid transition: action '%s' not allowed from state '%s'", action, record.CurrentState)
}
