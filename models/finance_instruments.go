package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BankGuarantee captures a business-scoped bank guarantee lifecycle record.
type BankGuarantee struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	BusinessVerticalID uuid.UUID         `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`

	ProjectID *uuid.UUID `gorm:"type:uuid;index" json:"project_id,omitempty"`

	GuaranteeNumber string  `gorm:"size:100;not null;index" json:"guarantee_number"`
	IssuingBank     string  `gorm:"size:255;not null" json:"issuing_bank"`
	BeneficiaryName string  `gorm:"size:255;not null" json:"beneficiary_name"`
	GuaranteeType   string  `gorm:"size:100;not null" json:"guarantee_type"`
	Amount          float64 `gorm:"type:decimal(15,2);not null" json:"amount"`
	Currency        string  `gorm:"size:10;not null;default:'INR'" json:"currency"`

	IssueDate  *time.Time `json:"issue_date,omitempty"`
	ExpiryDate *time.Time `gorm:"index" json:"expiry_date,omitempty"`
	ClaimDate  *time.Time `json:"claim_date,omitempty"`

	Status  string `gorm:"size:50;not null;default:'draft';index" json:"status"`
	Remarks string `gorm:"type:text" json:"remarks,omitempty"`

	ApprovalRequestID *uuid.UUID `gorm:"type:uuid;index" json:"approval_request_id,omitempty"`

	CreatedBy string         `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy string         `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (bg *BankGuarantee) BeforeCreate(tx *gorm.DB) (err error) {
	if bg.ID == uuid.Nil {
		bg.ID = uuid.New()
	}
	return nil
}

func (BankGuarantee) TableName() string {
	return "bank_guarantees"
}

// LetterOfCredit captures LC lifecycle and financial details.
type LetterOfCredit struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	BusinessVerticalID uuid.UUID         `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`

	ProjectID *uuid.UUID `gorm:"type:uuid;index" json:"project_id,omitempty"`

	LCNumber        string  `gorm:"size:100;not null;index" json:"lc_number"`
	ApplicantBank   string  `gorm:"size:255;not null" json:"applicant_bank"`
	BeneficiaryName string  `gorm:"size:255;not null" json:"beneficiary_name"`
	LCType          string  `gorm:"size:100;not null" json:"lc_type"`
	Amount          float64 `gorm:"type:decimal(15,2);not null" json:"amount"`
	UtilizedAmount  float64 `gorm:"type:decimal(15,2);default:0" json:"utilized_amount"`
	Currency        string  `gorm:"size:10;not null;default:'INR'" json:"currency"`

	IssueDate    *time.Time `json:"issue_date,omitempty"`
	MaturityDate *time.Time `gorm:"index" json:"maturity_date,omitempty"`

	Status  string `gorm:"size:50;not null;default:'draft';index" json:"status"`
	Remarks string `gorm:"type:text" json:"remarks,omitempty"`

	ApprovalRequestID *uuid.UUID `gorm:"type:uuid;index" json:"approval_request_id,omitempty"`

	CreatedBy string         `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy string         `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (lc *LetterOfCredit) BeforeCreate(tx *gorm.DB) (err error) {
	if lc.ID == uuid.Nil {
		lc.ID = uuid.New()
	}
	return nil
}

func (LetterOfCredit) TableName() string {
	return "letters_of_credit"
}

// InsurancePolicy stores core policy and renewal details.
type InsurancePolicy struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	BusinessVerticalID uuid.UUID         `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`

	ProjectID *uuid.UUID `gorm:"type:uuid;index" json:"project_id,omitempty"`

	PolicyNumber  string  `gorm:"size:100;not null;index" json:"policy_number"`
	ProviderName  string  `gorm:"size:255;not null" json:"provider_name"`
	CoverageType  string  `gorm:"size:100;not null" json:"coverage_type"`
	SumInsured    float64 `gorm:"type:decimal(15,2);not null" json:"sum_insured"`
	PremiumAmount float64 `gorm:"type:decimal(15,2);not null" json:"premium_amount"`
	Currency      string  `gorm:"size:10;not null;default:'INR'" json:"currency"`

	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `gorm:"index" json:"end_date,omitempty"`
	RenewalDate *time.Time `gorm:"index" json:"renewal_date,omitempty"`

	Status  string `gorm:"size:50;not null;default:'draft';index" json:"status"`
	Remarks string `gorm:"type:text" json:"remarks,omitempty"`

	ApprovalRequestID *uuid.UUID `gorm:"type:uuid;index" json:"approval_request_id,omitempty"`

	CreatedBy string         `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy string         `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ip *InsurancePolicy) BeforeCreate(tx *gorm.DB) (err error) {
	if ip.ID == uuid.Nil {
		ip.ID = uuid.New()
	}
	return nil
}

func (InsurancePolicy) TableName() string {
	return "insurance_policies"
}

// InsuranceClaim tracks claims and settlements under insurance policies.
type InsuranceClaim struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	BusinessVerticalID uuid.UUID         `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`

	PolicyID       uuid.UUID        `gorm:"type:uuid;not null;index" json:"policy_id"`
	Policy         *InsurancePolicy `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
	ClaimNumber    string           `gorm:"size:100;not null;index" json:"claim_number"`
	ClaimAmount    float64          `gorm:"type:decimal(15,2);not null" json:"claim_amount"`
	ApprovedAmount float64          `gorm:"type:decimal(15,2);default:0" json:"approved_amount"`

	IncidentDate *time.Time `json:"incident_date,omitempty"`
	ClaimDate    *time.Time `json:"claim_date,omitempty"`
	SettledDate  *time.Time `json:"settled_date,omitempty"`

	Status  string `gorm:"size:50;not null;default:'filed';index" json:"status"`
	Remarks string `gorm:"type:text" json:"remarks,omitempty"`

	ApprovalRequestID *uuid.UUID `gorm:"type:uuid;index" json:"approval_request_id,omitempty"`

	CreatedBy string         `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy string         `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ic *InsuranceClaim) BeforeCreate(tx *gorm.DB) (err error) {
	if ic.ID == uuid.Nil {
		ic.ID = uuid.New()
	}
	return nil
}

func (InsuranceClaim) TableName() string {
	return "insurance_claims"
}
