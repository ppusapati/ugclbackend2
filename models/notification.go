package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// NotificationType defines the type of notification
type NotificationType string

const (
	NotificationTypeWorkflowTransition NotificationType = "workflow_transition"
	NotificationTypeFormSubmission     NotificationType = "form_submission"
	NotificationTypeFormAssignment     NotificationType = "form_assignment"
	NotificationTypeApprovalRequired   NotificationType = "approval_required"
	NotificationTypeApprovalApproved   NotificationType = "approval_approved"
	NotificationTypeApprovalRejected   NotificationType = "approval_rejected"
	NotificationTypeTaskAssigned       NotificationType = "task_assigned"
	NotificationTypeTaskCompleted      NotificationType = "task_completed"
	NotificationTypeSystemAlert        NotificationType = "system_alert"
)

// NotificationChannel defines how notification is delivered
type NotificationChannel string

const (
	NotificationChannelInApp   NotificationChannel = "in_app"
	NotificationChannelEmail   NotificationChannel = "email"
	NotificationChannelSMS     NotificationChannel = "sms"
	NotificationChannelWebPush NotificationChannel = "web_push"
)

// NotificationStatus defines the status of a notification
type NotificationStatus string

const (
	NotificationStatusPending  NotificationStatus = "pending"
	NotificationStatusSent     NotificationStatus = "sent"
	NotificationStatusRead     NotificationStatus = "read"
	NotificationStatusFailed   NotificationStatus = "failed"
	NotificationStatusArchived NotificationStatus = "archived"
)

// NotificationPriority defines the priority level
type NotificationPriority string

const (
	NotificationPriorityLow      NotificationPriority = "low"
	NotificationPriorityNormal   NotificationPriority = "normal"
	NotificationPriorityHigh     NotificationPriority = "high"
	NotificationPriorityCritical NotificationPriority = "critical"
)

// NotificationRecipient represents who should receive the notification
type NotificationRecipient struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	NotificationRuleID uuid.UUID  `gorm:"type:uuid;not null;index" json:"notification_rule_id"`

	// Multi-level targeting
	UserID             *string    `gorm:"size:255;index" json:"user_id,omitempty"`                    // Specific user
	RoleID             *uuid.UUID `gorm:"type:uuid;index" json:"role_id,omitempty"`                   // Global role
	BusinessRoleID     *uuid.UUID `gorm:"type:uuid;index" json:"business_role_id,omitempty"`          // Business-specific role
	PermissionCode     *string    `gorm:"size:100;index" json:"permission_code,omitempty"`            // Users with permission
	AttributeCondition JSONMap    `gorm:"type:jsonb" json:"attribute_condition,omitempty"`            // ABAC condition
	PolicyID           *uuid.UUID `gorm:"type:uuid;index" json:"policy_id,omitempty"`                 // PBAC policy

	// Dynamic recipient resolution
	RecipientType      string     `gorm:"size:50;not null" json:"recipient_type"`                      // user, role, business_role, permission, attribute, policy, submitter, approver

	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Relationships
	NotificationRule *NotificationRule `gorm:"foreignKey:NotificationRuleID" json:"notification_rule,omitempty"`
}

// TableName specifies the table name
func (NotificationRecipient) TableName() string {
	return "notification_recipients"
}

// NotificationRule defines when and how to send notifications
type NotificationRule struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code        string     `gorm:"size:100;uniqueIndex;not null" json:"code"`
	Name        string     `gorm:"size:200;not null" json:"name"`
	Description string     `gorm:"type:text" json:"description,omitempty"`

	// Workflow integration
	WorkflowID      *uuid.UUID       `gorm:"type:uuid;index" json:"workflow_id,omitempty"`              // null = global rule
	TriggerOnStates StringArray      `gorm:"type:jsonb;default:'[]'" json:"trigger_on_states"`          // States that trigger notification
	TriggerOnActions StringArray     `gorm:"type:jsonb;default:'[]'" json:"trigger_on_actions"`         // Actions that trigger notification

	// Notification content
	Priority        NotificationPriority `gorm:"size:20;default:'normal'" json:"priority"`
	Channels        ChannelArray         `gorm:"type:jsonb;default:'[\"in_app\"]'" json:"channels"`        // Delivery channels
	TitleTemplate   string               `gorm:"size:500;not null" json:"title_template"`                  // Template with variables
	BodyTemplate    string               `gorm:"type:text;not null" json:"body_template"`                  // Template with variables
	ActionURL       string               `gorm:"size:500" json:"action_url,omitempty"`                     // Deep link URL

	// Email specific (if email channel enabled)
	EmailSubject    string     `gorm:"size:500" json:"email_subject,omitempty"`
	EmailTemplate   string     `gorm:"type:text" json:"email_template,omitempty"`

	// SMS specific (if SMS channel enabled)
	SMSTemplate     string     `gorm:"size:500" json:"sms_template,omitempty"`

	// Conditions
	Conditions      JSONMap    `gorm:"type:jsonb" json:"conditions,omitempty"`                          // Additional conditions to evaluate

	// Settings
	IsActive        bool       `gorm:"default:true" json:"is_active"`
	BatchInterval   int        `gorm:"default:0" json:"batch_interval_minutes"`                         // 0 = immediate, >0 = batch notifications
	DeduplicateKey  string     `gorm:"size:200" json:"deduplicate_key,omitempty"`                       // Prevent duplicate notifications

	// Metadata
	CreatedBy       string     `gorm:"size:255" json:"created_by,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Relationships
	Workflow   *WorkflowDefinition     `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	Recipients []NotificationRecipient `gorm:"foreignKey:NotificationRuleID" json:"recipients,omitempty"`
}

// TableName specifies the table name
func (NotificationRule) TableName() string {
	return "notification_rules"
}

// Notification represents an actual notification instance sent to a user
type Notification struct {
	ID               uuid.UUID            `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`

	// Rule reference
	NotificationRuleID *uuid.UUID         `gorm:"type:uuid;index" json:"notification_rule_id,omitempty"`
	NotificationRule   *NotificationRule  `gorm:"foreignKey:NotificationRuleID" json:"notification_rule,omitempty"`

	// Recipient
	UserID           string               `gorm:"size:255;not null;index" json:"user_id"`
	User             *User                `gorm:"foreignKey:UserID" json:"user,omitempty"`

	// Content
	Type             NotificationType     `gorm:"size:50;not null;index" json:"type"`
	Priority         NotificationPriority `gorm:"size:20;default:'normal'" json:"priority"`
	Title            string               `gorm:"size:500;not null" json:"title"`
	Body             string               `gorm:"type:text;not null" json:"body"`
	ActionURL        string               `gorm:"size:500" json:"action_url,omitempty"`

	// Context (what triggered this notification)
	SubmissionID     *uuid.UUID           `gorm:"type:uuid;index" json:"submission_id,omitempty"`
	WorkflowID       *uuid.UUID           `gorm:"type:uuid;index" json:"workflow_id,omitempty"`
	TransitionID     *uuid.UUID           `gorm:"type:uuid;index" json:"transition_id,omitempty"`
	FormCode         string               `gorm:"size:50;index" json:"form_code,omitempty"`
	BusinessVerticalID *uuid.UUID         `gorm:"type:uuid;index" json:"business_vertical_id,omitempty"`

	// Additional context data
	Metadata         JSONMap              `gorm:"type:jsonb" json:"metadata,omitempty"`

	// Delivery status
	Status           NotificationStatus   `gorm:"size:20;default:'pending';index" json:"status"`
	Channel          NotificationChannel  `gorm:"size:20;default:'in_app'" json:"channel"`
	SentAt           *time.Time           `json:"sent_at,omitempty"`
	ReadAt           *time.Time           `json:"read_at,omitempty"`
	ArchivedAt       *time.Time           `json:"archived_at,omitempty"`
	FailedReason     string               `gorm:"type:text" json:"failed_reason,omitempty"`

	// Grouping (for batching similar notifications)
	GroupKey         string               `gorm:"size:200;index" json:"group_key,omitempty"`

	// Timestamps
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`

	// Relationships
	Submission       *FormSubmission      `gorm:"foreignKey:SubmissionID" json:"submission,omitempty"`
	Workflow         *WorkflowDefinition  `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	Transition       *WorkflowTransition  `gorm:"foreignKey:TransitionID" json:"transition,omitempty"`
	BusinessVertical *BusinessVertical    `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`
}

// TableName specifies the table name
func (Notification) TableName() string {
	return "notifications"
}

// MarkAsRead marks the notification as read
func (n *Notification) MarkAsRead() {
	now := time.Now()
	n.ReadAt = &now
	n.Status = NotificationStatusRead
}

// MarkAsSent marks the notification as sent
func (n *Notification) MarkAsSent() {
	now := time.Now()
	n.SentAt = &now
	n.Status = NotificationStatusSent
}

// MarkAsFailed marks the notification as failed
func (n *Notification) MarkAsFailed(reason string) {
	n.Status = NotificationStatusFailed
	n.FailedReason = reason
}

// ChannelArray is a custom type for notification channels
type ChannelArray []NotificationChannel

// Scan implements the sql.Scanner interface
func (c *ChannelArray) Scan(value interface{}) error {
	if value == nil {
		*c = []NotificationChannel{NotificationChannelInApp}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		*c = []NotificationChannel{NotificationChannelInApp}
		return nil
	}

	var channels []string
	if err := json.Unmarshal(bytes, &channels); err != nil {
		return err
	}

	result := make([]NotificationChannel, len(channels))
	for i, ch := range channels {
		result[i] = NotificationChannel(ch)
	}
	*c = result
	return nil
}

// Value implements the driver.Valuer interface
func (c ChannelArray) Value() (driver.Value, error) {
	if c == nil {
		c = []NotificationChannel{NotificationChannelInApp}
	}

	channels := make([]string, len(c))
	for i, ch := range c {
		channels[i] = string(ch)
	}
	return json.Marshal(channels)
}

// GormDataType defines the data type for GORM
func (ChannelArray) GormDataType() string {
	return "jsonb"
}

// NotificationPreference stores user notification preferences
type NotificationPreference struct {
	ID                  uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID              string    `gorm:"size:255;not null;uniqueIndex" json:"user_id"`

	// Channel preferences
	EnableInApp         bool      `gorm:"default:true" json:"enable_in_app"`
	EnableEmail         bool      `gorm:"default:true" json:"enable_email"`
	EnableSMS           bool      `gorm:"default:false" json:"enable_sms"`
	EnableWebPush       bool      `gorm:"default:true" json:"enable_web_push"`

	// Type preferences (can disable specific types)
	DisabledTypes       StringArray `gorm:"type:jsonb;default:'[]'" json:"disabled_types"`

	// Quiet hours
	QuietHoursEnabled   bool        `gorm:"default:false" json:"quiet_hours_enabled"`
	QuietHoursStart     string      `gorm:"size:5" json:"quiet_hours_start,omitempty"` // HH:MM format
	QuietHoursEnd       string      `gorm:"size:5" json:"quiet_hours_end,omitempty"`   // HH:MM format

	// Digest settings
	DigestEnabled       bool        `gorm:"default:false" json:"digest_enabled"`
	DigestFrequency     string      `gorm:"size:20" json:"digest_frequency,omitempty"` // daily, weekly

	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`

	// Relationships
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName specifies the table name
func (NotificationPreference) TableName() string {
	return "notification_preferences"
}

// NotificationDTO represents the API response format
type NotificationDTO struct {
	ID           uuid.UUID            `json:"id"`
	Type         NotificationType     `json:"type"`
	Priority     NotificationPriority `json:"priority"`
	Title        string               `json:"title"`
	Body         string               `json:"body"`
	ActionURL    string               `json:"action_url,omitempty"`
	Status       NotificationStatus   `json:"status"`
	IsRead       bool                 `json:"is_read"`
	SubmissionID *uuid.UUID           `json:"submission_id,omitempty"`
	FormCode     string               `json:"form_code,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
	ReadAt       *time.Time           `json:"read_at,omitempty"`
}

// ToDTO converts Notification to DTO
func (n *Notification) ToDTO() NotificationDTO {
	return NotificationDTO{
		ID:           n.ID,
		Type:         n.Type,
		Priority:     n.Priority,
		Title:        n.Title,
		Body:         n.Body,
		ActionURL:    n.ActionURL,
		Status:       n.Status,
		IsRead:       n.ReadAt != nil,
		SubmissionID: n.SubmissionID,
		FormCode:     n.FormCode,
		Metadata:     n.Metadata,
		CreatedAt:    n.CreatedAt,
		ReadAt:       n.ReadAt,
	}
}
