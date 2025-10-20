package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DairySite struct {
	ID                 uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID uuid.UUID        `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical   BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	NameOfSite         string           `json:"nameOfSite"`
	TodaysWork        string   `json:"todaysWork"`
	SiteEngineerName  string   `json:"siteEngineerName"`
	SiteEngineerPhone string   `json:"siteEngineerPhone"`
	Latitude          float64  `json:"latitude"`
	Longitude         float64  `json:"longitude"`
	SubmittedAt       JSONTime `json:"submittedAt"`

	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
