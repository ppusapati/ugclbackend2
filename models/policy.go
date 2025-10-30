package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PolicyEffect defines whether policy allows or denies access
type PolicyEffect string

const (
	PolicyEffectAllow PolicyEffect = "ALLOW"
	PolicyEffectDeny  PolicyEffect = "DENY"
)

// PolicyStatus defines the current status of the policy
type PolicyStatus string

const (
	PolicyStatusActive   PolicyStatus = "active"
	PolicyStatusInactive PolicyStatus = "inactive"
	PolicyStatusDraft    PolicyStatus = "draft"
	PolicyStatusArchived PolicyStatus = "archived"
)

// Policy represents a complete access control policy
type Policy struct {
	ID                 uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	Name               string       `gorm:"size:200;not null" json:"name"`
	DisplayName        string       `gorm:"size:200;not null" json:"display_name"`
	Description        string       `gorm:"type:text" json:"description"`
	Effect             PolicyEffect `gorm:"size:10;not null" json:"effect"`           // ALLOW or DENY
	Priority           int          `gorm:"default:0" json:"priority"`                // Higher priority evaluated first
	Status             PolicyStatus `gorm:"size:20;default:'draft'" json:"status"`
	BusinessVerticalID *uuid.UUID   `gorm:"type:uuid;index" json:"business_vertical_id"` // null = global policy
	Conditions         JSONMap      `gorm:"type:jsonb;not null" json:"conditions"`    // Complex condition tree
	Actions            JSONArray    `gorm:"type:jsonb" json:"actions"`                // List of actions this policy applies to
	Resources          JSONArray    `gorm:"type:jsonb" json:"resources"`              // List of resource patterns
	Metadata           JSONMap      `gorm:"type:jsonb" json:"metadata"`               // Additional metadata
	ValidFrom          time.Time    `gorm:"default:CURRENT_TIMESTAMP" json:"valid_from"`
	ValidUntil         *time.Time   `json:"valid_until"`
	CreatedBy          uuid.UUID    `gorm:"type:uuid;not null" json:"created_by"`
	UpdatedBy          *uuid.UUID   `gorm:"type:uuid" json:"updated_by"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`

	// Relationships
	BusinessVertical *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`
	Rules            []PolicyRule      `gorm:"foreignKey:PolicyID" json:"rules,omitempty"`
}

// PolicyRule represents individual rules within a policy
type PolicyRule struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	PolicyID    uuid.UUID `gorm:"type:uuid;not null;index" json:"policy_id"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Condition   JSONMap   `gorm:"type:jsonb;not null" json:"condition"` // Single condition
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	Order       int       `gorm:"default:0" json:"order"` // Evaluation order within policy
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	Policy *Policy `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
}

// PolicyEvaluation stores the results of policy evaluations for audit
type PolicyEvaluation struct {
	ID                 uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	PolicyID           uuid.UUID    `gorm:"type:uuid;not null;index" json:"policy_id"`
	UserID             uuid.UUID    `gorm:"type:uuid;not null;index" json:"user_id"`
	ResourceType       string       `gorm:"size:50;index" json:"resource_type"`
	ResourceID         *uuid.UUID   `gorm:"type:uuid;index" json:"resource_id"`
	Action             string       `gorm:"size:100;not null" json:"action"`
	Effect             PolicyEffect `gorm:"size:10;not null" json:"effect"` // Final decision
	Context            JSONMap      `gorm:"type:jsonb" json:"context"`      // Context at evaluation time
	MatchedConditions  JSONArray    `gorm:"type:jsonb" json:"matched_conditions"`
	EvaluationTime     time.Time    `gorm:"default:CURRENT_TIMESTAMP;index" json:"evaluation_time"`
	IPAddress          string       `gorm:"size:50" json:"ip_address"`
	UserAgent          string       `gorm:"size:500" json:"user_agent"`
	RequestPath        string       `gorm:"size:500" json:"request_path"`
	EvaluationDuration int          `gorm:"default:0" json:"evaluation_duration_ms"` // Duration in milliseconds

	// Relationships
	Policy *Policy `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
	User   *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// JSONMap type for JSONB fields
type JSONMap map[string]interface{}

// Scan implements sql.Scanner interface for JSONMap
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Value implements driver.Valuer interface for JSONMap
func (j JSONMap) Value() (interface{}, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

// JSONArray type for JSONB array fields
type JSONArray []interface{}

// Scan implements sql.Scanner interface for JSONArray
func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONArray, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Value implements driver.Valuer interface for JSONArray
func (j JSONArray) Value() (interface{}, error) {
	if j == nil {
		return "[]", nil
	}
	return json.Marshal(j)
}

func (p *Policy) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return
}

func (pr *PolicyRule) BeforeCreate(tx *gorm.DB) (err error) {
	if pr.ID == uuid.Nil {
		pr.ID = uuid.New()
	}
	return
}

func (pe *PolicyEvaluation) BeforeCreate(tx *gorm.DB) (err error) {
	if pe.ID == uuid.Nil {
		pe.ID = uuid.New()
	}
	return
}

// IsActive checks if policy is currently active
func (p *Policy) IsActive() bool {
	now := time.Now()
	if p.Status != PolicyStatusActive {
		return false
	}
	if p.ValidFrom.After(now) {
		return false
	}
	if p.ValidUntil != nil && p.ValidUntil.Before(now) {
		return false
	}
	return true
}

// Condition represents a single condition in a policy
type Condition struct {
	Attribute string      `json:"attribute"` // e.g., "user.department", "resource.sensitivity"
	Operator  string      `json:"operator"`  // e.g., "=", "!=", ">", "<", "IN", "CONTAINS", "MATCHES"
	Value     interface{} `json:"value"`     // The value to compare against
}

// ConditionTree represents complex conditions with logical operators
type ConditionTree struct {
	Operator   string          `json:"operator"`   // AND, OR, NOT
	Conditions []Condition     `json:"conditions"` // Leaf conditions
	Children   []ConditionTree `json:"children"`   // Nested condition trees
}

// PolicyRequest represents a request for policy evaluation
type PolicyRequest struct {
	UserID             uuid.UUID         `json:"user_id"`
	Action             string            `json:"action"`
	ResourceType       string            `json:"resource_type"`
	ResourceID         *uuid.UUID        `json:"resource_id"`
	UserAttributes     map[string]string `json:"user_attributes"`
	ResourceAttributes map[string]string `json:"resource_attributes"`
	Environment        map[string]string `json:"environment"`
}

// PolicyDecision represents the result of policy evaluation
type PolicyDecision struct {
	Allowed           bool              `json:"allowed"`
	Effect            PolicyEffect      `json:"effect"`
	MatchedPolicies   []uuid.UUID       `json:"matched_policies"`
	Reason            string            `json:"reason"`
	EvaluationTime    time.Time         `json:"evaluation_time"`
	EvaluatedPolicies int               `json:"evaluated_policies"`
	Context           map[string]string `json:"context"`
}
