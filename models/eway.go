package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Eway represents a submitted E-Way Bill form.
type Eway struct {
	ID                     uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID     uuid.UUID        `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical       BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	BillNo                 string           `gorm:"not null" json:"billNo"`
	GeneratedDate          JSONTime  `gorm:"not null" json:"generatedDate"`
	GeneratedBy            string    `gorm:"not null" json:"generatedBy"`
	ValidUpto              *JSONTime `json:"validUpto,omitempty"`
	Mode                   *string   `json:"mode,omitempty"`
	Type                   *string   `json:"type,omitempty"`
	DocumentDetails        *string   `json:"documentDetails,omitempty"`
	DispatchFrom           string    `gorm:"not null" json:"dispatchFrom"`
	DispatchPincode        string    `gorm:"not null" json:"dispatchPincode"`
	ShipToAddress          *string   `json:"shipToAddress,omitempty"`
	ShipToPincode          string    `gorm:"not null" json:"shipToPincode"`
	ProductName            string    `gorm:"not null" json:"productName"`
	SpecialItemDescription *string   `json:"specialItemDescription,omitempty"`
	PipeDia                *string   `json:"pipeDia,omitempty"`
	UOM                    *string   `json:"uom,omitempty"`
	Quantity               string    `gorm:"not null" json:"quantity"`
	HSNCode                *string   `json:"hsnCode,omitempty"`
	TaxableAmount          *string   `json:"taxableAmount,omitempty"`
	TransporterIDName      *string   `json:"transporterIdName,omitempty"`
	TransporterDocNo       *string   `json:"transporterDocNo,omitempty"`
	DocumentDate           *JSONTime `json:"documentDate,omitempty"`
	VehicleNo              *string   `json:"vehicleNo,omitempty"`
	EnteredBy              string    `gorm:"not null" json:"enteredBy"`
	EnteredDate            JSONTime  `gorm:"not null" json:"enteredDate"`
	Remarks                *string   `json:"remarks,omitempty"`
	Latitude               float64   `gorm:"not null" json:"latitude"`
	Longitude              float64   `gorm:"not null" json:"longitude"`
	SubmittedAt            JSONTime  `gorm:"not null" json:"submittedAt"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
