package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// FinanceApprovalStatus defines approval states for finance instruments.
type FinanceApprovalStatus string

const (
	FinanceApprovalPending  FinanceApprovalStatus = "pending"
	FinanceApprovalApproved FinanceApprovalStatus = "approved"
	FinanceApprovalRejected FinanceApprovalStatus = "rejected"
)

// FinanceApprovalRequest stores maker-checker approval requests for BG/LC/Insurance entities.
type FinanceApprovalRequest struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	BusinessVerticalID uuid.UUID         `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`

	EntityType string    `gorm:"size:50;not null;index" json:"entity_type"` // bank_guarantee, letter_of_credit, insurance_policy, insurance_claim
	EntityID   uuid.UUID `gorm:"type:uuid;not null;index" json:"entity_id"`

	RequestType       string                `gorm:"size:80;not null;index" json:"request_type"`
	Status            FinanceApprovalStatus `gorm:"size:20;not null;default:'pending';index" json:"status"`
	RequestedBy       string                `gorm:"size:255;not null" json:"requested_by"`
	Notes             string                `gorm:"type:text" json:"notes,omitempty"`
	RequiredApprovals int                   `gorm:"default:1" json:"required_approvals"`
	ReceivedApprovals int                   `gorm:"default:0" json:"received_approvals"`
	Metadata          datatypes.JSON        `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy string     `gorm:"size:255" json:"resolved_by,omitempty"`
}

func (far *FinanceApprovalRequest) BeforeCreate(tx *gorm.DB) (err error) {
	if far.ID == uuid.Nil {
		far.ID = uuid.New()
	}
	return nil
}

func (FinanceApprovalRequest) TableName() string {
	return "finance_approval_requests"
}

// FinanceApproval stores individual approve/reject actions against a request.
type FinanceApproval struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	RequestID uuid.UUID               `gorm:"type:uuid;not null;index" json:"request_id"`
	Request   *FinanceApprovalRequest `gorm:"foreignKey:RequestID" json:"request,omitempty"`

	ApproverID string                `gorm:"size:255;not null" json:"approver_id"`
	Status     FinanceApprovalStatus `gorm:"size:20;not null" json:"status"`
	Comments   string                `gorm:"type:text" json:"comments,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

func (fa *FinanceApproval) BeforeCreate(tx *gorm.DB) (err error) {
	if fa.ID == uuid.Nil {
		fa.ID = uuid.New()
	}
	return nil
}

func (FinanceApproval) TableName() string {
	return "finance_approvals"
}
