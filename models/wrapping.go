package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WrappingReport represents one "wrapping" form submission.
type Wrapping struct {
	ID                 uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID uuid.UUID        `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical   BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	YardName           string           `gorm:"column:yard_name;not null"              json:"yardName"`
	ContractorName    string         `gorm:"column:contractor_name;not null"        json:"contractorName"`
	Activity          string         `gorm:"column:activity;not null"               json:"activity"`
	PipeNo            string         `gorm:"column:pipe_no;not null"                json:"pipeNo"`
	LengthOfPipe      string         `gorm:"column:length_of_pipe;not null"         json:"lengthOfPipe"`
	SquareMeters      string         `gorm:"column:square_meters;not null"          json:"squareMeters"`
	Photos            datatypes.JSON `gorm:"column:photos;type:jsonb;not null"      json:"photos"` // JSON array of filenames/URLs
	Remarks           *string        `gorm:"column:remarks"                         json:"remarks,omitempty"`
	SiteEngineerName  string         `gorm:"column:site_engineer_name;not null"     json:"siteEngineerName"`
	SiteEngineerPhone string         `gorm:"column:site_engineer_phone;not null"    json:"siteEngineerPhone"`
	Latitude          float64        `gorm:"column:latitude;not null"               json:"latitude"`
	Longitude         float64        `gorm:"column:longitude;not null"              json:"longitude"`
	SubmittedAt       JSONTime       `gorm:"column:submitted_at;not null"           json:"submittedAt"`

	CreatedAt time.Time      `gorm:"autoCreateTime"                         json:"createdAt"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"                         json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                  json:"-"`
}
