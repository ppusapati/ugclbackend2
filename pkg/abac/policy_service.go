package abac

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// PolicyService handles policy management operations
type PolicyService struct {
	db *gorm.DB
}

// NewPolicyService creates a new policy service instance
func NewPolicyService(db *gorm.DB) *PolicyService {
	return &PolicyService{db: db}
}

// CreatePolicy creates a new policy
func (ps *PolicyService) CreatePolicy(policy models.Policy) (*models.Policy, error) {
	// Check for duplicate name
	var existing models.Policy
	if err := ps.db.Where("name = ?", policy.Name).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("policy with name '%s' already exists", policy.Name)
	}

	// Validate conditions
	if err := ps.validateConditions(policy.Conditions); err != nil {
		return nil, fmt.Errorf("invalid conditions: %v", err)
	}

	// Set defaults
	if policy.Status == "" {
		policy.Status = models.PolicyStatusDraft
	}
	if policy.ValidFrom.IsZero() {
		policy.ValidFrom = time.Now()
	}

	if err := ps.db.Create(&policy).Error; err != nil {
		return nil, fmt.Errorf("failed to create policy: %v", err)
	}

	return &policy, nil
}

// UpdatePolicy updates an existing policy
func (ps *PolicyService) UpdatePolicy(id uuid.UUID, updates models.Policy) (*models.Policy, error) {
	var policy models.Policy
	if err := ps.db.First(&policy, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("policy not found: %v", err)
	}

	// Validate conditions if being updated
	if updates.Conditions != nil {
		if err := ps.validateConditions(updates.Conditions); err != nil {
			return nil, fmt.Errorf("invalid conditions: %v", err)
		}
	}

	// Update fields
	if updates.DisplayName != "" {
		policy.DisplayName = updates.DisplayName
	}
	if updates.Description != "" {
		policy.Description = updates.Description
	}
	if updates.Effect != "" {
		policy.Effect = updates.Effect
	}
	if updates.Priority != 0 {
		policy.Priority = updates.Priority
	}
	if updates.Status != "" {
		policy.Status = updates.Status
	}
	if updates.Conditions != nil {
		policy.Conditions = updates.Conditions
	}
	if updates.Actions != nil {
		policy.Actions = updates.Actions
	}
	if updates.Resources != nil {
		policy.Resources = updates.Resources
	}
	if updates.ValidUntil != nil {
		policy.ValidUntil = updates.ValidUntil
	}

	policy.UpdatedBy = updates.UpdatedBy

	if err := ps.db.Save(&policy).Error; err != nil {
		return nil, fmt.Errorf("failed to update policy: %v", err)
	}

	return &policy, nil
}

// DeletePolicy deletes a policy
func (ps *PolicyService) DeletePolicy(id uuid.UUID) error {
	result := ps.db.Delete(&models.Policy{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete policy: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("policy not found")
	}
	return nil
}

// GetPolicy retrieves a policy by ID
func (ps *PolicyService) GetPolicy(id uuid.UUID) (*models.Policy, error) {
	var policy models.Policy
	if err := ps.db.Preload("BusinessVertical").First(&policy, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("policy not found: %v", err)
	}
	return &policy, nil
}

// ListPolicies lists policies with optional filtering
func (ps *PolicyService) ListPolicies(status *models.PolicyStatus, businessVerticalID *uuid.UUID, limit, offset int) ([]models.Policy, int64, error) {
	var policies []models.Policy
	var total int64

	query := ps.db.Model(&models.Policy{})

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if businessVerticalID != nil {
		query = query.Where("business_vertical_id = ?", *businessVerticalID)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count policies: %v", err)
	}

	// Get paginated results
	if err := query.Preload("BusinessVertical").
		Order("priority DESC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&policies).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list policies: %v", err)
	}

	return policies, total, nil
}

// ActivatePolicy activates a policy
func (ps *PolicyService) ActivatePolicy(id, updatedBy uuid.UUID) error {
	var policy models.Policy
	if err := ps.db.First(&policy, "id = ?", id).Error; err != nil {
		return fmt.Errorf("policy not found: %v", err)
	}

	policy.Status = models.PolicyStatusActive
	policy.UpdatedBy = &updatedBy

	if err := ps.db.Save(&policy).Error; err != nil {
		return fmt.Errorf("failed to activate policy: %v", err)
	}

	return nil
}

// DeactivatePolicy deactivates a policy
func (ps *PolicyService) DeactivatePolicy(id, updatedBy uuid.UUID) error {
	var policy models.Policy
	if err := ps.db.First(&policy, "id = ?", id).Error; err != nil {
		return fmt.Errorf("policy not found: %v", err)
	}

	policy.Status = models.PolicyStatusInactive
	policy.UpdatedBy = &updatedBy

	if err := ps.db.Save(&policy).Error; err != nil {
		return fmt.Errorf("failed to deactivate policy: %v", err)
	}

	return nil
}

// TestPolicy tests a policy against a given request without storing the evaluation
func (ps *PolicyService) TestPolicy(policyID uuid.UUID, req models.PolicyRequest) (*models.PolicyDecision, error) {
	var policy models.Policy
	if err := ps.db.First(&policy, "id = ?", policyID).Error; err != nil {
		return nil, fmt.Errorf("policy not found: %v", err)
	}

	engine := NewPolicyEngine(ps.db)
	context := engine.buildContext(req)

	matches, err := engine.evaluateConditions(policy.Conditions, context)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate policy: %v", err)
	}

	decision := &models.PolicyDecision{
		Allowed:           matches && policy.Effect == models.PolicyEffectAllow,
		Effect:            policy.Effect,
		MatchedPolicies:   []uuid.UUID{},
		EvaluationTime:    time.Now(),
		EvaluatedPolicies: 1,
		Context:           context,
	}

	if matches {
		decision.MatchedPolicies = append(decision.MatchedPolicies, policy.ID)
		decision.Reason = fmt.Sprintf("Policy '%s' matched", policy.DisplayName)
	} else {
		decision.Reason = fmt.Sprintf("Policy '%s' did not match", policy.DisplayName)
	}

	return decision, nil
}

// GetPolicyEvaluations retrieves evaluation history for a policy
func (ps *PolicyService) GetPolicyEvaluations(policyID uuid.UUID, limit, offset int) ([]models.PolicyEvaluation, int64, error) {
	var evaluations []models.PolicyEvaluation
	var total int64

	query := ps.db.Model(&models.PolicyEvaluation{}).Where("policy_id = ?", policyID)

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count evaluations: %v", err)
	}

	// Get paginated results
	if err := query.Preload("User").Preload("Policy").
		Order("evaluation_time DESC").
		Limit(limit).
		Offset(offset).
		Find(&evaluations).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list evaluations: %v", err)
	}

	return evaluations, total, nil
}

// GetUserPolicyEvaluations retrieves evaluation history for a user
func (ps *PolicyService) GetUserPolicyEvaluations(userID uuid.UUID, limit, offset int) ([]models.PolicyEvaluation, int64, error) {
	var evaluations []models.PolicyEvaluation
	var total int64

	query := ps.db.Model(&models.PolicyEvaluation{}).Where("user_id = ?", userID)

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count evaluations: %v", err)
	}

	// Get paginated results
	if err := query.Preload("Policy").
		Order("evaluation_time DESC").
		Limit(limit).
		Offset(offset).
		Find(&evaluations).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list evaluations: %v", err)
	}

	return evaluations, total, nil
}

// ClonePolicy creates a copy of an existing policy
func (ps *PolicyService) ClonePolicy(id, createdBy uuid.UUID, newName string) (*models.Policy, error) {
	var original models.Policy
	if err := ps.db.First(&original, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("policy not found: %v", err)
	}

	clone := models.Policy{
		Name:               newName,
		DisplayName:        original.DisplayName + " (Copy)",
		Description:        original.Description,
		Effect:             original.Effect,
		Priority:           original.Priority,
		Status:             models.PolicyStatusDraft,
		BusinessVerticalID: original.BusinessVerticalID,
		Conditions:         original.Conditions,
		Actions:            original.Actions,
		Resources:          original.Resources,
		Metadata:           original.Metadata,
		ValidFrom:          time.Now(),
		CreatedBy:          createdBy,
	}

	if err := ps.db.Create(&clone).Error; err != nil {
		return nil, fmt.Errorf("failed to clone policy: %v", err)
	}

	return &clone, nil
}

// validateConditions validates policy conditions structure
func (ps *PolicyService) validateConditions(conditions models.JSONMap) error {
	if conditions == nil || len(conditions) == 0 {
		return fmt.Errorf("conditions cannot be empty")
	}

	// Check for logical operators
	if _, hasAnd := conditions["AND"]; hasAnd {
		return ps.validateLogicalConditions(conditions["AND"])
	}
	if _, hasOr := conditions["OR"]; hasOr {
		return ps.validateLogicalConditions(conditions["OR"])
	}
	if _, hasNot := conditions["NOT"]; hasNot {
		return ps.validateConditions(conditions["NOT"].(map[string]interface{}))
	}

	// Validate single condition
	return ps.validateSingleCondition(conditions)
}

// validateLogicalConditions validates logical operator conditions
func (ps *PolicyService) validateLogicalConditions(conditions interface{}) error {
	condArray, ok := conditions.([]interface{})
	if !ok {
		return fmt.Errorf("logical operator conditions must be an array")
	}

	if len(condArray) == 0 {
		return fmt.Errorf("logical operator must have at least one condition")
	}

	for _, cond := range condArray {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			return fmt.Errorf("condition must be an object")
		}
		if err := ps.validateConditions(condMap); err != nil {
			return err
		}
	}

	return nil
}

// validateSingleCondition validates a single condition
func (ps *PolicyService) validateSingleCondition(condition models.JSONMap) error {
	if _, ok := condition["attribute"]; !ok {
		return fmt.Errorf("condition must have 'attribute' field")
	}

	if _, ok := condition["operator"]; !ok {
		return fmt.Errorf("condition must have 'operator' field")
	}

	if _, ok := condition["value"]; !ok {
		return fmt.Errorf("condition must have 'value' field")
	}

	return nil
}

// GetPolicyStatistics returns statistics about policies
func (ps *PolicyService) GetPolicyStatistics() (map[string]interface{}, error) {
	var stats = make(map[string]interface{})

	// Count by status
	var statusCounts []struct {
		Status string
		Count  int64
	}
	if err := ps.db.Model(&models.Policy{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}
	stats["by_status"] = statusCounts

	// Count by effect
	var effectCounts []struct {
		Effect string
		Count  int64
	}
	if err := ps.db.Model(&models.Policy{}).
		Select("effect, count(*) as count").
		Group("effect").
		Scan(&effectCounts).Error; err != nil {
		return nil, err
	}
	stats["by_effect"] = effectCounts

	// Total evaluations
	var totalEvaluations int64
	ps.db.Model(&models.PolicyEvaluation{}).Count(&totalEvaluations)
	stats["total_evaluations"] = totalEvaluations

	// Recent evaluations (last 24 hours)
	var recentEvaluations int64
	ps.db.Model(&models.PolicyEvaluation{}).
		Where("evaluation_time > ?", time.Now().Add(-24*time.Hour)).
		Count(&recentEvaluations)
	stats["evaluations_last_24h"] = recentEvaluations

	return stats, nil
}
