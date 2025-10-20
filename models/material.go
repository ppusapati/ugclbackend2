package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MaterialIndent represents a "Material Indent" form submission.
type Material struct {
	ID                     uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID     uuid.UUID        `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical       BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	NameOfSite             string           `gorm:"not null" json:"nameOfSite"`
	MaterialOrService      datatypes.JSON `gorm:"type:jsonb;not null" json:"materialOrService"` // e.g. ["Material","Service"]
	Description            string         `gorm:"not null" json:"description"`
	QtyRequiredNow         string         `gorm:"not null" json:"qtyRequiredNow"`
	EstimatedCost          *string        `json:"estimatedCost,omitempty"`
	QuotationPhotos        datatypes.JSON `gorm:"type:jsonb;not null" json:"quotationPhotos"` // e.g. ["file1.png","file2.jpg"]
	QtyPresentStock        *string        `json:"qtyPresentStock,omitempty"`
	ExpectedCompletionDate *JSONTime      `json:"expectedCompletionDate,omitempty"`
	Priority               *string        `json:"priority,omitempty"` // e.g. "one week"
	DueDate                JSONTime       `gorm:"not null" json:"dueDate"`
	SiteEngineerName       string         `gorm:"not null" json:"siteEngineerName"`
	PhoneNumber            string         `gorm:"not null" json:"phoneNumber"`
	Latitude               float64        `gorm:"not null" json:"latitude"`
	Longitude              float64        `gorm:"not null" json:"longitude"`
	SubmittedAt            JSONTime       `gorm:"not null" json:"submittedAt"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
