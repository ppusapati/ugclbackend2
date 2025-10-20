package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DprSite represents a DPR Site form submission.
type DprSite struct {
	ID                                    uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BusinessVerticalID                    uuid.UUID        `gorm:"type:uuid;index;not null" json:"businessVerticalId"`
	BusinessVertical                      BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	NameOfSite                            string           `gorm:"not null" json:"nameOfSite"`
	LabelNumber                           string         `gorm:"not null" json:"labelNumber"`
	ClassOfPipes                          string         `gorm:"not null" json:"classOfPipes"`
	MaterialOfPipe                        string         `gorm:"not null" json:"materialOfPipe"`
	PipeDia                               string         `gorm:"not null" json:"pipeDia"`
	TypeOfWorks                           string         `gorm:"not null" json:"typeOfWorks"`
	ChainageFrom                          string         `gorm:"not null" json:"chainageFrom"`
	ChainageTo                            string         `gorm:"not null" json:"chainageTo"`
	ActualMetersLaidOnDay                 string         `gorm:"not null" json:"actualMetersLaidOnDay"`
	Width                                 string         `gorm:"not null" json:"width"`
	LineType                              string         `gorm:"not null" json:"lineType"`
	UploadWorkingSitePhoto                string         `gorm:"not null" json:"uploadWorkingSitePhoto"`
	DiaOfSpecialsReceived                 *string        `json:"diaOfSpecialsReceived,omitempty"`
	PipeSpecialsQuantityReceived          *string        `json:"pipeSpecialsQuantityReceived,omitempty"`
	AnyOtherMaterialsReceived             *string        `json:"anyOtherMaterialsReceived,omitempty"`
	DieselIssuedInLitres                  string         `gorm:"not null" json:"dieselIssuedInLitres"`
	AmountInRs                            string         `gorm:"not null" json:"amountInRs"`
	CardNumber                            string         `gorm:"not null" json:"cardNumber"`
	UploadTheDieselBillPhoto              string         `gorm:"not null" json:"uploadTheDieselBillPhoto"`
	Remarks                               *string        `json:"remarks,omitempty"`
	NameOfContractor                      string         `gorm:"not null" json:"nameOfContractor"`
	PhoneNumberOfContractor               string         `gorm:"not null" json:"phoneNumberOfContractor"`
	NameOfSiteEngineer                    string         `gorm:"not null" json:"nameOfSiteEngineer"`
	PhoneNumberOfSiteEngineer             string         `gorm:"not null" json:"phoneNumberOfSiteEngineer"`
	NameOfSupervisor                      *string        `json:"nameOfSupervisor,omitempty"`
	InformationEnteredBy                  string         `gorm:"not null" json:"informationEnteredBy"`
	PhoneNumberOfInformationEnteredPerson string         `json:"phoneNumberOfInformationEnteredPerson,omitempty"`
	Latitude                              float64        `gorm:"not null" json:"latitude"`
	Longitude                             float64        `gorm:"not null" json:"longitude"`
	SubmittedAt                           JSONTime       `gorm:"not null" json:"submittedAt"`
	CreatedAt                             time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt                             time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt                             gorm.DeletedAt `gorm:"index" json:"-"`
}

func (d DprSite) TableName() string {
	return "dpr_sites"
}
