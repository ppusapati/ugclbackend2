package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AttributeType defines the category of attribute
type AttributeType string

const (
	AttributeTypeUser        AttributeType = "user"        // User attributes (department, clearance_level, etc.)
	AttributeTypeResource    AttributeType = "resource"    // Resource attributes (sensitivity, owner, etc.)
	AttributeTypeEnvironment AttributeType = "environment" // Environmental attributes (time, location, IP, etc.)
	AttributeTypeAction      AttributeType = "action"      // Action attributes (operation_type, risk_level, etc.)
)

// AttributeDataType defines the data type of attribute value
type AttributeDataType string

const (
	DataTypeString   AttributeDataType = "string"
	DataTypeInteger  AttributeDataType = "integer"
	DataTypeFloat    AttributeDataType = "float"
	DataTypeBoolean  AttributeDataType = "boolean"
	DataTypeDateTime AttributeDataType = "datetime"
	DataTypeJSON     AttributeDataType = "json"
	DataTypeArray    AttributeDataType = "array"
)

// Attribute defines a specific attribute that can be used in policies
type Attribute struct {
	ID          uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string            `gorm:"size:100;uniqueIndex;not null" json:"name"` // e.g., "user.department", "resource.sensitivity"
	DisplayName string            `gorm:"size:100;not null" json:"display_name"`     // e.g., "User Department"
	Description string            `gorm:"size:500" json:"description"`
	Type        AttributeType     `gorm:"size:50;not null" json:"type"`           // user, resource, environment, action
	DataType    AttributeDataType `gorm:"size:50;not null" json:"data_type"`      // string, integer, boolean, etc.
	IsSystem    bool              `gorm:"default:false" json:"is_system"`         // System-defined or user-defined
	IsActive    bool              `gorm:"default:true" json:"is_active"`
	Metadata    JSONMap           `gorm:"type:jsonb" json:"metadata"`             // Additional configuration (allowed_values, validation_rules, etc.)
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// UserAttribute stores user-specific attribute values
type UserAttribute struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index:idx_user_attr" json:"user_id"`
	AttributeID uuid.UUID `gorm:"type:uuid;not null;index:idx_user_attr" json:"attribute_id"`
	Value       string    `gorm:"type:text;not null" json:"value"` // Stored as string, parsed based on attribute.DataType
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	ValidFrom   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"valid_from"`
	ValidUntil  *time.Time `json:"valid_until"` // Optional expiration
	AssignedBy  uuid.UUID `gorm:"type:uuid" json:"assigned_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Attribute Attribute `gorm:"foreignKey:AttributeID" json:"attribute,omitempty"`
}

// ResourceAttribute stores resource-specific attribute values
type ResourceAttribute struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ResourceType string    `gorm:"size:50;not null;index:idx_resource_attr" json:"resource_type"` // e.g., "project", "report", "site"
	ResourceID   uuid.UUID `gorm:"type:uuid;not null;index:idx_resource_attr" json:"resource_id"`
	AttributeID  uuid.UUID `gorm:"type:uuid;not null;index:idx_resource_attr" json:"attribute_id"`
	Value        string    `gorm:"type:text;not null" json:"value"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	ValidFrom    time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"valid_from"`
	ValidUntil   *time.Time `json:"valid_until"`
	AssignedBy   uuid.UUID `gorm:"type:uuid" json:"assigned_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships
	Attribute Attribute `gorm:"foreignKey:AttributeID" json:"attribute,omitempty"`
}

func (a *Attribute) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return
}

func (ua *UserAttribute) BeforeCreate(tx *gorm.DB) (err error) {
	if ua.ID == uuid.Nil {
		ua.ID = uuid.New()
	}
	return
}

func (ra *ResourceAttribute) BeforeCreate(tx *gorm.DB) (err error) {
	if ra.ID == uuid.Nil {
		ra.ID = uuid.New()
	}
	return
}

// GetUserAttributeValue retrieves a specific attribute value for a user
func GetUserAttributeValue(db *gorm.DB, userID uuid.UUID, attributeName string) (string, bool) {
	var userAttr UserAttribute
	err := db.Joins("JOIN attributes ON attributes.id = user_attributes.attribute_id").
		Where("user_attributes.user_id = ? AND attributes.name = ? AND user_attributes.is_active = ?", userID, attributeName, true).
		Where("user_attributes.valid_from <= ? AND (user_attributes.valid_until IS NULL OR user_attributes.valid_until > ?)", time.Now(), time.Now()).
		First(&userAttr).Error

	if err != nil {
		return "", false
	}
	return userAttr.Value, true
}

// GetResourceAttributeValue retrieves a specific attribute value for a resource
func GetResourceAttributeValue(db *gorm.DB, resourceType string, resourceID uuid.UUID, attributeName string) (string, bool) {
	var resourceAttr ResourceAttribute
	err := db.Joins("JOIN attributes ON attributes.id = resource_attributes.attribute_id").
		Where("resource_attributes.resource_type = ? AND resource_attributes.resource_id = ? AND attributes.name = ? AND resource_attributes.is_active = ?",
			resourceType, resourceID, attributeName, true).
		Where("resource_attributes.valid_from <= ? AND (resource_attributes.valid_until IS NULL OR resource_attributes.valid_until > ?)", time.Now(), time.Now()).
		First(&resourceAttr).Error

	if err != nil {
		return "", false
	}
	return resourceAttr.Value, true
}

// GetAllUserAttributes retrieves all active attributes for a user
func GetAllUserAttributes(db *gorm.DB, userID uuid.UUID) (map[string]string, error) {
	var userAttrs []UserAttribute
	err := db.Preload("Attribute").
		Where("user_id = ? AND is_active = ?", userID, true).
		Where("valid_from <= ? AND (valid_until IS NULL OR valid_until > ?)", time.Now(), time.Now()).
		Find(&userAttrs).Error

	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, attr := range userAttrs {
		result[attr.Attribute.Name] = attr.Value
	}
	return result, nil
}

// GetAllResourceAttributes retrieves all active attributes for a resource
func GetAllResourceAttributes(db *gorm.DB, resourceType string, resourceID uuid.UUID) (map[string]string, error) {
	var resourceAttrs []ResourceAttribute
	err := db.Preload("Attribute").
		Where("resource_type = ? AND resource_id = ? AND is_active = ?", resourceType, resourceID, true).
		Where("valid_from <= ? AND (valid_until IS NULL OR valid_until > ?)", time.Now(), time.Now()).
		Find(&resourceAttrs).Error

	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, attr := range resourceAttrs {
		result[attr.Attribute.Name] = attr.Value
	}
	return result, nil
}
