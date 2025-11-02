package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WorkflowDefinition represents a workflow configuration
type WorkflowDefinition struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code         string    `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	Description  string    `gorm:"type:text" json:"description,omitempty"`
	Version      string    `gorm:"size:50;not null;default:'1.0.0'" json:"version"`
	InitialState string    `gorm:"size:50;not null;default:'draft'" json:"initial_state"`

	// Workflow configuration stored as JSONB
	States      json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"states"`
	Transitions json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"transitions"`

	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for WorkflowDefinition
func (WorkflowDefinition) TableName() string {
	return "workflow_definitions"
}

// WorkflowState represents a state in the workflow
type WorkflowState struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"` // For UI display
	Icon        string `json:"icon,omitempty"`
	IsFinal     bool   `json:"is_final"` // Terminal state (no further transitions)
}

// WorkflowTransitionDef represents a state transition definition
type WorkflowTransitionDef struct {
	From            string                      `json:"from"`
	To              string                      `json:"to"`
	Action          string                      `json:"action"`
	Label           string                      `json:"label,omitempty"`
	Permission      string                      `json:"permission,omitempty"`
	RequiresComment bool                        `json:"requires_comment,omitempty"`

	// Notification configuration
	Notifications   []TransitionNotification    `json:"notifications,omitempty"`
}

// TransitionNotification defines notification config for a transition
type TransitionNotification struct {
	// Recipients - supports multiple targeting strategies
	Recipients      []NotificationRecipientDef `json:"recipients"`

	// Content
	TitleTemplate   string                     `json:"title_template"`
	BodyTemplate    string                     `json:"body_template"`
	Priority        string                     `json:"priority,omitempty"`        // low, normal, high, critical

	// Delivery
	Channels        []string                   `json:"channels,omitempty"`        // in_app, email, sms, web_push

	// Conditions (optional - send only if condition met)
	Condition       map[string]interface{}     `json:"condition,omitempty"`
}

// NotificationRecipientDef defines who receives the notification
type NotificationRecipientDef struct {
	Type            string                 `json:"type"`                     // user, role, business_role, permission, attribute, policy, submitter, approver, field_value

	// Type-specific values
	Value           string                 `json:"value,omitempty"`          // For user (user_id), role (role_name), permission (perm_code), field_value (field_name)
	RoleID          string                 `json:"role_id,omitempty"`        // For role targeting
	BusinessRoleID  string                 `json:"business_role_id,omitempty"` // For business_role targeting
	PermissionCode  string                 `json:"permission_code,omitempty"`  // For permission targeting
	AttributeQuery  map[string]interface{} `json:"attribute_query,omitempty"` // For ABAC targeting
	PolicyID        string                 `json:"policy_id,omitempty"`      // For PBAC targeting
}

// FormSubmission represents a submitted form instance with workflow state
type FormSubmission struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`

	// Form reference
	FormCode         string     `gorm:"size:50;not null;index" json:"form_code"`
	FormID           uuid.UUID  `gorm:"type:uuid;not null;index" json:"form_id"`
	Form             *AppForm   `gorm:"foreignKey:FormID" json:"form,omitempty"`

	// Business context
	BusinessVerticalID uuid.UUID       `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`

	// Site context (optional - for site-specific forms)
	SiteID           *uuid.UUID `gorm:"type:uuid;index" json:"site_id,omitempty"`

	// Workflow state
	WorkflowID       *uuid.UUID          `gorm:"type:uuid;index" json:"workflow_id,omitempty"`
	Workflow         *WorkflowDefinition `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	CurrentState     string              `gorm:"size:50;not null;default:'draft';index" json:"current_state"`

	// Form data (submitted field values)
	FormData         json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"form_data"`

	// Metadata
	Version          int       `gorm:"default:1" json:"version"`
	SubmittedBy      string    `gorm:"size:255;not null" json:"submitted_by"`
	SubmittedAt      time.Time `json:"submitted_at"`
	LastModifiedBy   string    `gorm:"size:255" json:"last_modified_by,omitempty"`
	LastModifiedAt   time.Time `json:"last_modified_at,omitempty"`

	// Audit trail
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Transitions      []WorkflowTransition `gorm:"foreignKey:SubmissionID" json:"transitions,omitempty"`
}

// TableName specifies the table name for FormSubmission
func (FormSubmission) TableName() string {
	return "form_submissions"
}

// WorkflowTransition represents a state transition event (audit trail)
type WorkflowTransition struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`

	// Submission reference
	SubmissionID uuid.UUID       `gorm:"type:uuid;not null;index" json:"submission_id"`
	Submission   *FormSubmission `gorm:"foreignKey:SubmissionID" json:"submission,omitempty"`

	// Transition details
	FromState    string    `gorm:"size:50;not null" json:"from_state"`
	ToState      string    `gorm:"size:50;not null" json:"to_state"`
	Action       string    `gorm:"size:50;not null" json:"action"`

	// Actor information
	ActorID      string    `gorm:"size:255;not null" json:"actor_id"`
	ActorName    string    `gorm:"size:255" json:"actor_name,omitempty"`
	ActorRole    string    `gorm:"size:100" json:"actor_role,omitempty"`

	// Additional context
	Comment      string          `gorm:"type:text" json:"comment,omitempty"`
	Metadata     json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	// Timestamp
	TransitionedAt time.Time `gorm:"not null;index" json:"transitioned_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName specifies the table name for WorkflowTransition
func (WorkflowTransition) TableName() string {
	return "workflow_transitions"
}

// FormSubmissionMetadata is a custom type for storing additional metadata
type FormSubmissionMetadata map[string]interface{}

// Scan implements the sql.Scanner interface
func (m *FormSubmissionMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = make(FormSubmissionMetadata)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		*m = make(FormSubmissionMetadata)
		return nil
	}

	return json.Unmarshal(bytes, m)
}

// Value implements the driver.Valuer interface
func (m FormSubmissionMetadata) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(make(map[string]interface{}))
	}
	return json.Marshal(m)
}

// GormDataType defines the data type for GORM
func (FormSubmissionMetadata) GormDataType() string {
	return "jsonb"
}

// WorkflowAction represents an available action for a submission
type WorkflowAction struct {
	Action          string `json:"action"`
	Label           string `json:"label"`
	ToState         string `json:"to_state"`
	RequiresComment bool   `json:"requires_comment"`
	Permission      string `json:"permission,omitempty"`
}

// GetAvailableActions returns available workflow actions for current state
func (s *FormSubmission) GetAvailableActions(workflowDef *WorkflowDefinition) ([]WorkflowAction, error) {
	var actions []WorkflowAction

	if workflowDef == nil {
		return actions, nil
	}

	// Parse transitions
	var transitions []WorkflowTransitionDef
	if err := json.Unmarshal(workflowDef.Transitions, &transitions); err != nil {
		return nil, err
	}

	// Find applicable transitions
	for _, t := range transitions {
		if t.From == s.CurrentState {
			action := WorkflowAction{
				Action:          t.Action,
				Label:           t.Label,
				ToState:         t.To,
				RequiresComment: t.RequiresComment,
				Permission:      t.Permission,
			}

			// Set default label if not specified
			if action.Label == "" {
				action.Label = t.Action
			}

			actions = append(actions, action)
		}
	}

	return actions, nil
}

// CanTransition checks if a transition is allowed
func (s *FormSubmission) CanTransition(action string, workflowDef *WorkflowDefinition) bool {
	actions, err := s.GetAvailableActions(workflowDef)
	if err != nil {
		return false
	}

	for _, a := range actions {
		if a.Action == action {
			return true
		}
	}

	return false
}

// FormSubmissionDTO represents the response structure for form submissions
type FormSubmissionDTO struct {
	ID                 uuid.UUID       `json:"id"`
	FormCode           string          `json:"form_code"`
	FormTitle          string          `json:"form_title,omitempty"`
	BusinessVerticalID uuid.UUID       `json:"business_vertical_id"`
	SiteID             *uuid.UUID      `json:"site_id,omitempty"`
	CurrentState       string          `json:"current_state"`
	FormData           json.RawMessage `json:"form_data"`
	SubmittedBy        string          `json:"submitted_by"`
	SubmittedAt        time.Time       `json:"submitted_at"`
	LastModifiedBy     string          `json:"last_modified_by,omitempty"`
	LastModifiedAt     time.Time       `json:"last_modified_at,omitempty"`
	AvailableActions   []WorkflowAction `json:"available_actions,omitempty"`
}

// ToDTO converts FormSubmission to DTO
func (s *FormSubmission) ToDTO(workflowDef *WorkflowDefinition) FormSubmissionDTO {
	dto := FormSubmissionDTO{
		ID:                 s.ID,
		FormCode:           s.FormCode,
		BusinessVerticalID: s.BusinessVerticalID,
		SiteID:             s.SiteID,
		CurrentState:       s.CurrentState,
		FormData:           s.FormData,
		SubmittedBy:        s.SubmittedBy,
		SubmittedAt:        s.SubmittedAt,
		LastModifiedBy:     s.LastModifiedBy,
		LastModifiedAt:     s.LastModifiedAt,
	}

	if s.Form != nil {
		dto.FormTitle = s.Form.Title
	}

	if workflowDef != nil {
		actions, _ := s.GetAvailableActions(workflowDef)
		dto.AvailableActions = actions
	}

	return dto
}
