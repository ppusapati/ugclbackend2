package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Diesel represents a DPR diesel‐usage entry.
type Diesel struct {
	ID                 uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID uuid.UUID        `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical   BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	NameOfSite         string           `gorm:"not null" json:"nameOfSite"`
	ToWhom             string         `gorm:"not null" json:"toWhom"`
	Item               string         `gorm:"not null" json:"item"`
	CardNumber         string         `gorm:"not null" json:"cardNumber"`
	VehicleNumber      string         `gorm:"not null" json:"vehicleNumber"`
	QuantityInLiters   string         `gorm:"not null" json:"quantityInLiters"`
	AmountPaid         string         `gorm:"not null" json:"amountPaid"`
	ContractorName     string         `gorm:"not null" json:"contractorName"`
	ContractorPhone    string         `gorm:"not null" json:"contractorPhone"`
	MeterReadingPhotos pq.StringArray `gorm:"type:text[]" json:"meterReadingPhotos"`
	BillPhotos         pq.StringArray `gorm:"type:text[]" json:"billPhotos"`
	PersonFilled       string         `json:"personFilled,omitempty"`
	PersonPhone        string         `json:"personPhone,omitempty"`
	Remarks            *string        `json:"remarks,omitempty"`
	Latitude           float64        `gorm:"not null" json:"latitude"`
	Longitude          float64        `gorm:"not null" json:"longitude"`
	SubmittedAt        JSONTime       `gorm:"not null" json:"submittedAt"`
	CreatedAt          time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt          time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}
