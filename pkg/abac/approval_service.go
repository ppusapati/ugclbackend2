package abac

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// ApprovalService handles policy approval workflows
type ApprovalService struct {
	db *gorm.DB
}

// NewApprovalService creates a new approval service instance
func NewApprovalService(db *gorm.DB) *ApprovalService {
	return &ApprovalService{db: db}
}

// CreateApprovalRequest creates a new approval request
func (as *ApprovalService) CreateApprovalRequest(policyID uuid.UUID, requestType string, requestedBy uuid.UUID, notes string, changesProposed models.JSONMap) (*models.PolicyApprovalRequest, error) {
	// Get applicable workflow
	var workflow models.PolicyApprovalWorkflow
	if err := as.db.Where("request_type = ? AND is_active = ?", requestType, true).
		Order("priority DESC").
		First(&workflow).Error; err != nil {
		// No workflow found - use default (1 approval required)
		workflow.RequiredApprovals = 1
	}

	request := &models.PolicyApprovalRequest{
		PolicyID:          policyID,
		RequestType:       requestType,
		Status:            models.ApprovalStatusPending,
		RequestedBy:       requestedBy,
		RequestNotes:      notes,
		RequiredApprovals: workflow.RequiredApprovals,
		ReceivedApprovals: 0,
		ChangesProposed:   changesProposed,
	}

	if err := as.db.Create(request).Error; err != nil {
		return nil, fmt.Errorf("failed to create approval request: %v", err)
	}

	return request, nil
}

// ApproveRequest approves an approval request
func (as *ApprovalService) ApproveRequest(requestID, approverID uuid.UUID, comments string) (*models.PolicyApprovalRequest, error) {
	var request models.PolicyApprovalRequest
	if err := as.db.Preload("Approvals").First(&request, "id = ?", requestID).Error; err != nil {
		return nil, fmt.Errorf("approval request not found: %v", err)
	}

	if request.Status != models.ApprovalStatusPending {
		return nil, fmt.Errorf("request is not pending (status: %s)", request.Status)
	}

	// Check if user already approved
	for _, approval := range request.Approvals {
		if approval.ApproverID == approverID {
			return nil, fmt.Errorf("user has already %s this request", approval.Status)
		}
	}

	// Create approval record
	approval := models.PolicyApproval{
		RequestID:  requestID,
		ApproverID: approverID,
		Status:     models.ApprovalStatusApproved,
		Comments:   comments,
	}

	if err := as.db.Create(&approval).Error; err != nil {
		return nil, fmt.Errorf("failed to create approval: %v", err)
	}

	// Update request
	request.ReceivedApprovals++

	// Check if approval is complete
	if request.IsApprovalComplete() {
		request.Status = models.ApprovalStatusApproved
		now := time.Now()
		request.ResolvedAt = &now
		request.ResolvedBy = &approverID

		// Execute the approved action
		if err := as.executeApprovedAction(&request); err != nil {
			return nil, fmt.Errorf("failed to execute approved action: %v", err)
		}
	}

	if err := as.db.Save(&request).Error; err != nil {
		return nil, fmt.Errorf("failed to update request: %v", err)
	}

	return &request, nil
}

// RejectRequest rejects an approval request
func (as *ApprovalService) RejectRequest(requestID, approverID uuid.UUID, comments string) (*models.PolicyApprovalRequest, error) {
	var request models.PolicyApprovalRequest
	if err := as.db.First(&request, "id = ?", requestID).Error; err != nil {
		return nil, fmt.Errorf("approval request not found: %v", err)
	}

	if request.Status != models.ApprovalStatusPending {
		return nil, fmt.Errorf("request is not pending (status: %s)", request.Status)
	}

	// Create rejection record
	approval := models.PolicyApproval{
		RequestID:  requestID,
		ApproverID: approverID,
		Status:     models.ApprovalStatusRejected,
		Comments:   comments,
	}

	if err := as.db.Create(&approval).Error; err != nil {
		return nil, fmt.Errorf("failed to create rejection: %v", err)
	}

	// Update request status
	request.Status = models.ApprovalStatusRejected
	now := time.Now()
	request.ResolvedAt = &now
	request.ResolvedBy = &approverID

	if err := as.db.Save(&request).Error; err != nil {
		return nil, fmt.Errorf("failed to update request: %v", err)
	}

	return &request, nil
}

// executeApprovedAction executes the action after approval
func (as *ApprovalService) executeApprovedAction(request *models.PolicyApprovalRequest) error {
	switch request.RequestType {
	case "activate":
		return as.db.Model(&models.Policy{}).
			Where("id = ?", request.PolicyID).
			Update("status", models.PolicyStatusActive).Error

	case "deactivate":
		return as.db.Model(&models.Policy{}).
			Where("id = ?", request.PolicyID).
			Update("status", models.PolicyStatusInactive).Error

	case "delete":
		return as.db.Delete(&models.Policy{}, "id = ?", request.PolicyID).Error

	case "create", "update":
		// Apply the proposed changes
		if request.ChangesProposed != nil {
			return as.db.Model(&models.Policy{}).
				Where("id = ?", request.PolicyID).
				Updates(request.ChangesProposed).Error
		}
		return nil

	default:
		return fmt.Errorf("unknown request type: %s", request.RequestType)
	}
}

// GetPendingApprovals gets all pending approval requests
func (as *ApprovalService) GetPendingApprovals(limit, offset int) ([]models.PolicyApprovalRequest, int64, error) {
	var requests []models.PolicyApprovalRequest
	var total int64

	query := as.db.Model(&models.PolicyApprovalRequest{}).
		Where("status = ?", models.ApprovalStatusPending)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Preload("Policy").Preload("Approvals").Preload("Approvals.Approver").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&requests).Error; err != nil {
		return nil, 0, err
	}

	return requests, total, nil
}

// GetUserPendingApprovals gets pending approvals that a user can approve
func (as *ApprovalService) GetUserPendingApprovals(userID uuid.UUID, userRoles []string, limit, offset int) ([]models.PolicyApprovalRequest, int64, error) {
	// Get all pending requests
	allRequests, _, err := as.GetPendingApprovals(1000, 0) // Get all pending
	if err != nil {
		return nil, 0, err
	}

	// Filter requests user can approve
	canApprove := make([]models.PolicyApprovalRequest, 0)
	for _, request := range allRequests {
		if request.CanUserApprove(userID, userRoles, as.db) {
			canApprove = append(canApprove, request)
		}
	}

	// Apply pagination
	total := int64(len(canApprove))
	start := offset
	end := offset + limit
	if start > len(canApprove) {
		return []models.PolicyApprovalRequest{}, total, nil
	}
	if end > len(canApprove) {
		end = len(canApprove)
	}

	return canApprove[start:end], total, nil
}

// GetApprovalRequest gets a specific approval request
func (as *ApprovalService) GetApprovalRequest(requestID uuid.UUID) (*models.PolicyApprovalRequest, error) {
	var request models.PolicyApprovalRequest
	if err := as.db.Preload("Policy").Preload("Approvals").Preload("Approvals.Approver").
		First(&request, "id = ?", requestID).Error; err != nil {
		return nil, err
	}
	return &request, nil
}

// CreatePolicyVersion creates a version snapshot of a policy
func (as *ApprovalService) CreatePolicyVersion(policy *models.Policy, changeNotes string) (*models.PolicyVersion, error) {
	// Get the latest version number
	var latestVersion models.PolicyVersion
	var versionNum int = 1

	if err := as.db.Where("policy_id = ?", policy.ID).
		Order("version DESC").
		First(&latestVersion).Error; err == nil {
		versionNum = latestVersion.Version + 1
	}

	version := &models.PolicyVersion{
		PolicyID:    policy.ID,
		Version:     versionNum,
		Name:        policy.Name,
		DisplayName: policy.DisplayName,
		Description: policy.Description,
		Effect:      policy.Effect,
		Priority:    policy.Priority,
		Conditions:  policy.Conditions,
		Actions:     policy.Actions,
		Resources:   policy.Resources,
		Metadata:    policy.Metadata,
		CreatedBy:   policy.CreatedBy,
		ChangeNotes: changeNotes,
	}

	if err := as.db.Create(version).Error; err != nil {
		return nil, fmt.Errorf("failed to create policy version: %v", err)
	}

	return version, nil
}

// GetPolicyVersions gets all versions of a policy
func (as *ApprovalService) GetPolicyVersions(policyID uuid.UUID) ([]models.PolicyVersion, error) {
	var versions []models.PolicyVersion
	if err := as.db.Where("policy_id = ?", policyID).
		Order("version DESC").
		Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// LogPolicyChange logs a change to a policy
func (as *ApprovalService) LogPolicyChange(policyID uuid.UUID, action string, changedBy uuid.UUID, changes models.JSONMap, reason string) error {
	changeLog := models.PolicyChangeLog{
		PolicyID:    policyID,
		Action:      action,
		ChangedBy:   changedBy,
		ChangesJSON: changes,
		Reason:      reason,
	}

	return as.db.Create(&changeLog).Error
}

// GetPolicyChangeLogs gets change history for a policy
func (as *ApprovalService) GetPolicyChangeLogs(policyID uuid.UUID, limit, offset int) ([]models.PolicyChangeLog, int64, error) {
	var logs []models.PolicyChangeLog
	var total int64

	query := as.db.Model(&models.PolicyChangeLog{}).Where("policy_id = ?", policyID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Preload("User").Preload("Version").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// CreateWorkflow creates a new approval workflow
func (as *ApprovalService) CreateWorkflow(workflow *models.PolicyApprovalWorkflow) error {
	return as.db.Create(workflow).Error
}

// GetWorkflows gets all approval workflows
func (as *ApprovalService) GetWorkflows() ([]models.PolicyApprovalWorkflow, error) {
	var workflows []models.PolicyApprovalWorkflow
	if err := as.db.Order("priority DESC").Find(&workflows).Error; err != nil {
		return nil, err
	}
	return workflows, nil
}
