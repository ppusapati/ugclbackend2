package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// StockReport represents one "stock" form submission.
type Stock struct {
	ID                     uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID     uuid.UUID        `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical       BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	InOut                  string           `gorm:"column:in_out;not null"               json:"inOut"`
	YardName               string         `gorm:"column:yard_name;not null"            json:"yardName"`
	InvoiceDate            JSONTime       `gorm:"column:invoice_date;not null"         json:"invoiceDate"`
	CompanyName            string         `gorm:"column:company_name;not null"         json:"companyName"`
	ItemDescription        string         `gorm:"column:item_description;not null"     json:"itemDescription"`
	SpecialItemDescription string         `gorm:"column:special_item_description;not null" json:"specialItemDescription"`
	PipeDia                string         `gorm:"column:pipe_dia;not null"             json:"pipeDia"`
	TotalLength            string         `gorm:"column:total_length;not null"         json:"totalLength"`
	ItemQuantity           string         `gorm:"column:item_quantity;not null"        json:"itemQuantity"`
	SpecialsDetail         *string        `gorm:"column:specials_detail"               json:"specialsDetail,omitempty"`
	DefectiveMaterial      *string        `gorm:"column:defective_material"            json:"defectiveMaterial,omitempty"`
	DefectivePhotos        datatypes.JSON `gorm:"column:defective_photos;type:jsonb;not null" json:"defectivePhotos"`
	ContractorName         string         `gorm:"column:contractor_name;not null"      json:"contractorName"`
	LabelNumber            string         `gorm:"column:label_number;not null"         json:"labelNumber"`
	VehicleNumber          string         `gorm:"column:vehicle_number;not null"       json:"vehicleNumber"`
	Remarks                *string        `gorm:"column:remarks"                       json:"remarks,omitempty"`
	YardInchargeName       string         `gorm:"column:yard_incharge_name;not null"   json:"yardInchargeName"`
	YardInchargePhone      string         `gorm:"column:yard_incharge_phone;not null"  json:"yardInchargePhone"`
	ChallanFiles           datatypes.JSON `gorm:"column:challan_files;type:jsonb;not null" json:"challanFiles"`

	Latitude    float64  `gorm:"column:latitude;not null"             json:"latitude"`
	Longitude   float64  `gorm:"column:longitude;not null"            json:"longitude"`
	SubmittedAt JSONTime `gorm:"column:submitted_at;not null"         json:"submittedAt"`

	CreatedAt time.Time      `gorm:"autoCreateTime"                       json:"createdAt"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"                       json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                json:"-"`
}
