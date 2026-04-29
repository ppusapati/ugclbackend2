package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WebhookEvent represents the type of event that triggers a webhook
type WebhookEventType string

const (
	EventCreate WebhookEventType = "CREATE"
	EventUpdate WebhookEventType = "UPDATE"
	EventFormSubmitted WebhookEventType = "form.submitted"
)

// WebhookStatus represents the status of a webhook subscription
type WebhookStatus string

const (
	StatusActive   WebhookStatus = "ACTIVE"
	StatusInactive WebhookStatus = "INACTIVE"
	StatusFailed   WebhookStatus = "FAILED"
)

// Webhook represents a webhook subscription
type Webhook struct {
	ID            uint                        `gorm:"primaryKey" json:"id"`
	BusinessID    uuid.UUID                   `gorm:"type:uuid;index" json:"business_id"`
	URL           string                      `gorm:"type:text" json:"url"`
	Events        datatypes.JSONSlice[string] `gorm:"type:jsonb" json:"events"`
	ResourceTypes datatypes.JSONSlice[string] `gorm:"type:jsonb" json:"resource_types"` // e.g., ["User", "Site", "Report"]
	Secret        string                      `gorm:"type:text" json:"secret"`          // For HMAC signature
	Headers       datatypes.JSONMap           `gorm:"type:jsonb" json:"headers"`        // Custom headers to send
	Status        WebhookStatus               `gorm:"type:varchar(20)" json:"status"`
	MaxRetries    int                         `gorm:"default:5" json:"max_retries"`
	RetryInterval int                         `gorm:"default:300" json:"retry_interval"` // In seconds
	IsActive      bool                        `gorm:"default:true;index" json:"is_active"`
	CreatedAt     time.Time                   `json:"created_at"`
	UpdatedAt     time.Time                   `json:"updated_at"`
	DeletedAt     gorm.DeletedAt              `gorm:"index" json:"deleted_at,omitempty"`
}

// WebhookDelivery represents a single webhook delivery attempt
type WebhookDelivery struct {
	ID           uint              `gorm:"primaryKey" json:"id"`
	WebhookID    uint              `gorm:"index" json:"webhook_id"`
	Webhook      *Webhook          `json:"webhook,omitempty"`
	EventType    WebhookEventType  `gorm:"type:varchar(20)" json:"event_type"`
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id"`
	Payload      datatypes.JSONMap `gorm:"type:jsonb" json:"payload"`
	Status       string            `gorm:"type:varchar(20)" json:"status"` // PENDING, SENT, FAILED, SUCCESS
	HTTPStatus   int               `json:"http_status"`
	Response     string            `gorm:"type:text" json:"response"`
	Error        string            `gorm:"type:text" json:"error"`
	Attempt      int               `gorm:"default:1" json:"attempt"`
	MaxAttempts  int               `gorm:"default:5" json:"max_attempts"`
	NextRetryAt  *time.Time        `json:"next_retry_at"`
	SentAt       *time.Time        `json:"sent_at"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// WebhookLog represents a detailed log for audit purposes
type WebhookLog struct {
	ID           uint              `gorm:"primaryKey" json:"id"`
	WebhookID    uint              `gorm:"index" json:"webhook_id"`
	DeliveryID   uint              `gorm:"index" json:"delivery_id"`
	EventType    WebhookEventType  `gorm:"type:varchar(20)" json:"event_type"`
	ResourceType string            `json:"resource_type"`
	Action       string            `gorm:"type:varchar(100)" json:"action"` // SENT, RETRY, FAILED, SUCCESS
	Payload      datatypes.JSONMap `gorm:"type:jsonb" json:"payload"`
	Response     string            `gorm:"type:text" json:"response"`
	Error        string            `gorm:"type:text" json:"error"`
	CreatedAt    time.Time         `json:"created_at"`
}

// WebhookPayload represents the data sent to webhook consumers
type WebhookPayload struct {
	ID           string                 `json:"id"`
	Event        WebhookEventType       `json:"event"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Data         map[string]interface{} `json:"data"`
	Timestamp    time.Time              `json:"timestamp"`
	BusinessID   uuid.UUID              `json:"business_id"`
	Version      string                 `json:"version"`
}

// NewWebhookPayload creates a new webhook payload
func NewWebhookPayload(eventType WebhookEventType, resourceType string, resourceID string, businessID uuid.UUID, data map[string]interface{}) *WebhookPayload {
	return &WebhookPayload{
		ID:           uuid.NewString(),
		Event:        eventType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Data:         data,
		Timestamp:    time.Now().UTC(),
		BusinessID:   businessID,
		Version:      "1.0",
	}
}

// MarshalJSON implements custom JSON marshaling for WebhookPayload
func (wp *WebhookPayload) MarshalJSON() ([]byte, error) {
	type Alias WebhookPayload
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Timestamp: wp.Timestamp.Format(time.RFC3339),
		Alias:     (*Alias)(wp),
	})
}

// TableName specifies the table name for Webhook model
func (Webhook) TableName() string {
	return "webhooks"
}

// TableName specifies the table name for WebhookDelivery model
func (WebhookDelivery) TableName() string {
	return "webhook_deliveries"
}

// TableName specifies the table name for WebhookLog model
func (WebhookLog) TableName() string {
	return "webhook_logs"
}

// Value implements driver.Valuer for WebhookEventType
func (w WebhookEventType) Value() (driver.Value, error) {
	return string(w), nil
}

// Scan implements sql.Scanner for WebhookEventType
func (w *WebhookEventType) Scan(value interface{}) error {
	*w = WebhookEventType(value.(string))
	return nil
}
