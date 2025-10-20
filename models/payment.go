package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PaymentReport represents a "payment" form submission.
type Payment struct {
	ID                uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID uuid.UUID       `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical  BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	NameOfSite        string           `gorm:"column:name_of_site;not null"         json:"nameOfSite"`
	RequestType       string         `gorm:"column:request_type;not null"        json:"requestType"`
	Purpose           string         `gorm:"column:purpose;not null"             json:"purpose"`
	BeneficiaryName   string         `gorm:"column:beneficiary_name;not null"    json:"beneficiaryName"`
	BillValue         *string        `gorm:"column:bill_value"                   json:"billValue,omitempty"`
	PaymentType       *string        `gorm:"column:payment_type"                 json:"paymentType,omitempty"`
	QuotationFiles    datatypes.JSON `gorm:"column:quotation_files;type:jsonb;not null" json:"quotationFiles"`
	KYVFiles          datatypes.JSON `gorm:"column:kyv_files;      type:jsonb;not null" json:"kyvFiles"`
	Priority          string         `gorm:"column:priority;not null"            json:"priority"`
	DueDate           *JSONTime      `gorm:"column:due_date"                     json:"dueDate,omitempty"`
	Remarks           *string        `gorm:"column:remarks"                      json:"remarks,omitempty"`
	SiteEngineerName  string         `gorm:"column:site_engineer_name;not null"  json:"siteEngineerName"`
	SiteEngineerPhone string         `gorm:"column:site_engineer_phone;not null" json:"siteEngineerPhone"`
	Latitude          float64        `gorm:"column:latitude;not null"            json:"latitude"`
	Longitude         float64        `gorm:"column:longitude;not null"           json:"longitude"`
	SubmittedAt       JSONTime       `gorm:"column:submitted_at;not null"        json:"submittedAt"`

	CreatedAt time.Time      `gorm:"autoCreateTime"  json:"createdAt"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"  json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index"           json:"-"`
}
