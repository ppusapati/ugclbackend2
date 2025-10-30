package abac

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// AttributeService handles attribute operations
type AttributeService struct {
	db *gorm.DB
}

// NewAttributeService creates a new attribute service instance
func NewAttributeService(db *gorm.DB) *AttributeService {
	return &AttributeService{db: db}
}

// AssignUserAttribute assigns an attribute to a user
func (as *AttributeService) AssignUserAttribute(userID, attributeID, assignedBy uuid.UUID, value string, validUntil *time.Time) error {
	// Check if attribute exists
	var attribute models.Attribute
	if err := as.db.First(&attribute, "id = ? AND is_active = ?", attributeID, true).Error; err != nil {
		return fmt.Errorf("attribute not found: %v", err)
	}

	// Check if user exists
	var user models.User
	if err := as.db.First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("user not found: %v", err)
	}

	// Deactivate existing attribute assignment
	as.db.Model(&models.UserAttribute{}).
		Where("user_id = ? AND attribute_id = ? AND is_active = ?", userID, attributeID, true).
		Update("is_active", false)

	// Create new attribute assignment
	userAttr := models.UserAttribute{
		UserID:      userID,
		AttributeID: attributeID,
		Value:       value,
		IsActive:    true,
		ValidFrom:   time.Now(),
		ValidUntil:  validUntil,
		AssignedBy:  assignedBy,
	}

	if err := as.db.Create(&userAttr).Error; err != nil {
		return fmt.Errorf("failed to assign attribute: %v", err)
	}

	return nil
}

// RemoveUserAttribute removes an attribute from a user
func (as *AttributeService) RemoveUserAttribute(userID, attributeID uuid.UUID) error {
	result := as.db.Model(&models.UserAttribute{}).
		Where("user_id = ? AND attribute_id = ? AND is_active = ?", userID, attributeID, true).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to remove attribute: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("attribute assignment not found")
	}

	return nil
}

// GetUserAttributes retrieves all active attributes for a user
func (as *AttributeService) GetUserAttributes(userID uuid.UUID) (map[string]string, error) {
	return models.GetAllUserAttributes(as.db, userID)
}

// AssignResourceAttribute assigns an attribute to a resource
func (as *AttributeService) AssignResourceAttribute(resourceType string, resourceID, attributeID, assignedBy uuid.UUID, value string, validUntil *time.Time) error {
	// Check if attribute exists
	var attribute models.Attribute
	if err := as.db.First(&attribute, "id = ? AND is_active = ?", attributeID, true).Error; err != nil {
		return fmt.Errorf("attribute not found: %v", err)
	}

	// Deactivate existing attribute assignment
	as.db.Model(&models.ResourceAttribute{}).
		Where("resource_type = ? AND resource_id = ? AND attribute_id = ? AND is_active = ?",
			resourceType, resourceID, attributeID, true).
		Update("is_active", false)

	// Create new attribute assignment
	resourceAttr := models.ResourceAttribute{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		AttributeID:  attributeID,
		Value:        value,
		IsActive:     true,
		ValidFrom:    time.Now(),
		ValidUntil:   validUntil,
		AssignedBy:   assignedBy,
	}

	if err := as.db.Create(&resourceAttr).Error; err != nil {
		return fmt.Errorf("failed to assign attribute: %v", err)
	}

	return nil
}

// RemoveResourceAttribute removes an attribute from a resource
func (as *AttributeService) RemoveResourceAttribute(resourceType string, resourceID, attributeID uuid.UUID) error {
	result := as.db.Model(&models.ResourceAttribute{}).
		Where("resource_type = ? AND resource_id = ? AND attribute_id = ? AND is_active = ?",
			resourceType, resourceID, attributeID, true).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to remove attribute: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("attribute assignment not found")
	}

	return nil
}

// GetResourceAttributes retrieves all active attributes for a resource
func (as *AttributeService) GetResourceAttributes(resourceType string, resourceID uuid.UUID) (map[string]string, error) {
	return models.GetAllResourceAttributes(as.db, resourceType, resourceID)
}

// CreateAttribute creates a new attribute definition
func (as *AttributeService) CreateAttribute(attr models.Attribute) (*models.Attribute, error) {
	// Check for duplicate
	var existing models.Attribute
	if err := as.db.Where("name = ?", attr.Name).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("attribute with name '%s' already exists", attr.Name)
	}

	if err := as.db.Create(&attr).Error; err != nil {
		return nil, fmt.Errorf("failed to create attribute: %v", err)
	}

	return &attr, nil
}

// UpdateAttribute updates an existing attribute
func (as *AttributeService) UpdateAttribute(id uuid.UUID, updates map[string]interface{}) (*models.Attribute, error) {
	var attr models.Attribute
	if err := as.db.First(&attr, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("attribute not found: %v", err)
	}

	if err := as.db.Model(&attr).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update attribute: %v", err)
	}

	return &attr, nil
}

// DeleteAttribute soft deletes an attribute
func (as *AttributeService) DeleteAttribute(id uuid.UUID) error {
	result := as.db.Model(&models.Attribute{}).
		Where("id = ?", id).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to delete attribute: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("attribute not found")
	}

	return nil
}

// GetAttributeByName retrieves an attribute by its name
func (as *AttributeService) GetAttributeByName(name string) (*models.Attribute, error) {
	var attr models.Attribute
	if err := as.db.Where("name = ? AND is_active = ?", name, true).First(&attr).Error; err != nil {
		return nil, fmt.Errorf("attribute not found: %v", err)
	}
	return &attr, nil
}

// ListAttributes lists all attributes with optional filtering
func (as *AttributeService) ListAttributes(attrType *models.AttributeType, isActive *bool) ([]models.Attribute, error) {
	var attributes []models.Attribute
	query := as.db

	if attrType != nil {
		query = query.Where("type = ?", *attrType)
	}

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	if err := query.Order("type, name").Find(&attributes).Error; err != nil {
		return nil, fmt.Errorf("failed to list attributes: %v", err)
	}

	return attributes, nil
}

// BulkAssignUserAttributes assigns multiple attributes to a user at once
func (as *AttributeService) BulkAssignUserAttributes(userID, assignedBy uuid.UUID, attributes map[string]string) error {
	for attrName, value := range attributes {
		attr, err := as.GetAttributeByName(attrName)
		if err != nil {
			return fmt.Errorf("attribute '%s' not found: %v", attrName, err)
		}

		if err := as.AssignUserAttribute(userID, attr.ID, assignedBy, value, nil); err != nil {
			return fmt.Errorf("failed to assign attribute '%s': %v", attrName, err)
		}
	}

	return nil
}

// BulkAssignResourceAttributes assigns multiple attributes to a resource at once
func (as *AttributeService) BulkAssignResourceAttributes(resourceType string, resourceID, assignedBy uuid.UUID, attributes map[string]string) error {
	for attrName, value := range attributes {
		attr, err := as.GetAttributeByName(attrName)
		if err != nil {
			return fmt.Errorf("attribute '%s' not found: %v", attrName, err)
		}

		if err := as.AssignResourceAttribute(resourceType, resourceID, attr.ID, assignedBy, value, nil); err != nil {
			return fmt.Errorf("failed to assign attribute '%s': %v", attrName, err)
		}
	}

	return nil
}

// GetUserAttributeHistory retrieves the history of attribute assignments for a user
func (as *AttributeService) GetUserAttributeHistory(userID, attributeID uuid.UUID) ([]models.UserAttribute, error) {
	var history []models.UserAttribute
	if err := as.db.Preload("Attribute").
		Where("user_id = ? AND attribute_id = ?", userID, attributeID).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get attribute history: %v", err)
	}

	return history, nil
}
