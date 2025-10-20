package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WaterReport represents one "water" form submission.
type Water struct {
	ID                  uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID  uuid.UUID        `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical    BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	SiteName            string           `gorm:"column:site_name;not null"              json:"siteName"`
	Purpose             string         `gorm:"column:purpose;not null"                json:"purpose"`
	PlaceOfSupply       *string        `gorm:"column:place_of_supply"                 json:"placeOfSupply,omitempty"`
	TankerVehicleNumber string         `gorm:"column:tanker_vehicle_number;not null"  json:"tankerVehicleNumber"`
	CapacityInLiters    string         `gorm:"column:capacity_in_liters;not null"     json:"capacityInLiters"`
	RatePerUnit         *string        `gorm:"column:rate_per_unit"                   json:"ratePerUnit,omitempty"`
	Photos              datatypes.JSON `gorm:"column:photos;type:jsonb;not null"      json:"photos"`
	SupplierName        string         `gorm:"column:supplier_name;not null"          json:"supplierName"`
	SupplierPhone       string         `gorm:"column:supplier_phone;not null"         json:"supplierPhone"`
	SiteEngineerName    string         `gorm:"column:site_engineer_name;not null"     json:"siteEngineerName"`
	SiteEngineerPhone   string         `gorm:"column:site_engineer_phone;not null"    json:"siteEngineerPhone"`
	Latitude            float64        `gorm:"column:latitude;not null"               json:"latitude"`
	Longitude           float64        `gorm:"column:longitude;not null"              json:"longitude"`
	SubmittedAt         JSONTime       `gorm:"column:submitted_at;not null"           json:"submittedAt"`

	CreatedAt time.Time      `gorm:"autoCreateTime"                         json:"createdAt"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"                         json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                  json:"-"`
}
