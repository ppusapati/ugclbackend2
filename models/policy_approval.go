package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PolicyApprovalStatus defines the approval status
type PolicyApprovalStatus string

const (
	ApprovalStatusPending  PolicyApprovalStatus = "pending"
	ApprovalStatusApproved PolicyApprovalStatus = "approved"
	ApprovalStatusRejected PolicyApprovalStatus = "rejected"
	ApprovalStatusRevoked  PolicyApprovalStatus = "revoked"
)

// PolicyVersion stores version history of policies
type PolicyVersion struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	PolicyID    uuid.UUID `gorm:"type:uuid;not null;index" json:"policy_id"`
	Version     int       `gorm:"not null" json:"version"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	DisplayName string    `gorm:"size:200;not null" json:"display_name"`
	Description string    `gorm:"type:text" json:"description"`
	Effect      PolicyEffect `gorm:"size:10;not null" json:"effect"`
	Priority    int       `gorm:"default:0" json:"priority"`
	Conditions  JSONMap   `gorm:"type:jsonb;not null" json:"conditions"`
	Actions     JSONArray `gorm:"type:jsonb" json:"actions"`
	Resources   JSONArray `gorm:"type:jsonb" json:"resources"`
	Metadata    JSONMap   `gorm:"type:jsonb" json:"metadata"`
	CreatedBy   uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	ChangeNotes string    `gorm:"type:text" json:"change_notes"` // What changed in this version

	// Relationships
	Policy *Policy `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
}

// PolicyApprovalRequest represents a request for policy approval
type PolicyApprovalRequest struct {
	ID                 uuid.UUID            `gorm:"type:uuid;primaryKey" json:"id"`
	PolicyID           uuid.UUID            `gorm:"type:uuid;not null;index" json:"policy_id"`
	PolicyVersionID    *uuid.UUID           `gorm:"type:uuid;index" json:"policy_version_id"`
	RequestType        string               `gorm:"size:50;not null" json:"request_type"` // create, update, activate, deactivate, delete
	Status             PolicyApprovalStatus `gorm:"size:20;default:'pending'" json:"status"`
	RequestedBy        uuid.UUID            `gorm:"type:uuid;not null" json:"requested_by"`
	RequestNotes       string               `gorm:"type:text" json:"request_notes"`
	RequiredApprovals  int                  `gorm:"default:1" json:"required_approvals"`  // Number of approvals needed
	ReceivedApprovals  int                  `gorm:"default:0" json:"received_approvals"`  // Number of approvals received
	ChangesProposed    JSONMap              `gorm:"type:jsonb" json:"changes_proposed"`   // What changes are requested
	CreatedAt          time.Time            `json:"created_at"`
	ResolvedAt         *time.Time           `json:"resolved_at"`
	ResolvedBy         *uuid.UUID           `gorm:"type:uuid" json:"resolved_by"`

	// Relationships
	Policy          *Policy                  `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
	PolicyVersion   *PolicyVersion           `gorm:"foreignKey:PolicyVersionID" json:"policy_version,omitempty"`
	Approvals       []PolicyApproval         `gorm:"foreignKey:RequestID" json:"approvals,omitempty"`
}

// PolicyApproval represents an individual approval/rejection
type PolicyApproval struct {
	ID         uuid.UUID            `gorm:"type:uuid;primaryKey" json:"id"`
	RequestID  uuid.UUID            `gorm:"type:uuid;not null;index" json:"request_id"`
	ApproverID uuid.UUID            `gorm:"type:uuid;not null" json:"approver_id"`
	Status     PolicyApprovalStatus `gorm:"size:20;not null" json:"status"` // approved or rejected
	Comments   string               `gorm:"type:text" json:"comments"`
	CreatedAt  time.Time            `json:"created_at"`

	// Relationships
	Request  *PolicyApprovalRequest `gorm:"foreignKey:RequestID" json:"request,omitempty"`
	Approver *User                  `gorm:"foreignKey:ApproverID" json:"approver,omitempty"`
}

// PolicyChangeLog tracks all changes to policies
type PolicyChangeLog struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	PolicyID    uuid.UUID `gorm:"type:uuid;not null;index" json:"policy_id"`
	VersionID   *uuid.UUID `gorm:"type:uuid" json:"version_id"`
	Action      string    `gorm:"size:50;not null" json:"action"` // created, updated, activated, deactivated, deleted
	ChangedBy   uuid.UUID `gorm:"type:uuid;not null" json:"changed_by"`
	ChangesJSON JSONMap   `gorm:"type:jsonb" json:"changes"` // What changed
	Reason      string    `gorm:"type:text" json:"reason"`
	CreatedAt   time.Time `json:"created_at"`

	// Relationships
	Policy  *Policy        `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
	Version *PolicyVersion `gorm:"foreignKey:VersionID" json:"version,omitempty"`
	User    *User          `gorm:"foreignKey:ChangedBy" json:"user,omitempty"`
}

// PolicyApprovalWorkflow defines workflow rules for policy changes
type PolicyApprovalWorkflow struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name              string    `gorm:"size:200;not null;unique" json:"name"`
	Description       string    `gorm:"type:text" json:"description"`
	RequestType       string    `gorm:"size:50;not null" json:"request_type"` // create, update, activate, etc.
	RequiredApprovals int       `gorm:"default:1" json:"required_approvals"`
	ApproverRoles     JSONArray `gorm:"type:jsonb" json:"approver_roles"` // List of role names that can approve
	Conditions        JSONMap   `gorm:"type:jsonb" json:"conditions"`     // Conditions when this workflow applies
	IsActive          bool      `gorm:"default:true" json:"is_active"`
	Priority          int       `gorm:"default:0" json:"priority"` // Higher priority workflows evaluated first
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (pv *PolicyVersion) BeforeCreate(tx *gorm.DB) (err error) {
	if pv.ID == uuid.Nil {
		pv.ID = uuid.New()
	}
	return
}

func (par *PolicyApprovalRequest) BeforeCreate(tx *gorm.DB) (err error) {
	if par.ID == uuid.Nil {
		par.ID = uuid.New()
	}
	return
}

func (pa *PolicyApproval) BeforeCreate(tx *gorm.DB) (err error) {
	if pa.ID == uuid.Nil {
		pa.ID = uuid.New()
	}
	return
}

func (pcl *PolicyChangeLog) BeforeCreate(tx *gorm.DB) (err error) {
	if pcl.ID == uuid.Nil {
		pcl.ID = uuid.New()
	}
	return
}

func (paw *PolicyApprovalWorkflow) BeforeCreate(tx *gorm.DB) (err error) {
	if paw.ID == uuid.Nil {
		paw.ID = uuid.New()
	}
	return
}

// IsApprovalComplete checks if approval request has enough approvals
func (par *PolicyApprovalRequest) IsApprovalComplete() bool {
	return par.ReceivedApprovals >= par.RequiredApprovals
}

// CanUserApprove checks if a user can approve this request
func (par *PolicyApprovalRequest) CanUserApprove(userID uuid.UUID, userRoles []string, db *gorm.DB) bool {
	// Check if user already approved/rejected
	var existingApproval PolicyApproval
	if err := db.Where("request_id = ? AND approver_id = ?", par.ID, userID).First(&existingApproval).Error; err == nil {
		return false // Already approved/rejected
	}

	// Get applicable workflow
	var workflow PolicyApprovalWorkflow
	if err := db.Where("request_type = ? AND is_active = ?", par.RequestType, true).
		Order("priority DESC").
		First(&workflow).Error; err != nil {
		return false // No workflow found
	}

	// Check if user's role is in approver_roles
	if workflow.ApproverRoles != nil {
		approverRoles := make([]string, 0)
		for _, role := range workflow.ApproverRoles {
			if roleStr, ok := role.(string); ok {
				approverRoles = append(approverRoles, roleStr)
			}
		}

		for _, userRole := range userRoles {
			for _, approverRole := range approverRoles {
				if userRole == approverRole {
					return true
				}
			}
		}
	}

	return false
}
