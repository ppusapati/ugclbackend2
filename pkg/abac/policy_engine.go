package abac

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// PolicyEngine handles policy evaluation
type PolicyEngine struct {
	db *gorm.DB
}

// NewPolicyEngine creates a new policy engine instance
func NewPolicyEngine(db *gorm.DB) *PolicyEngine {
	return &PolicyEngine{db: db}
}

// EvaluateRequest evaluates a policy request and returns a decision
func (pe *PolicyEngine) EvaluateRequest(req models.PolicyRequest) (*models.PolicyDecision, error) {
	startTime := time.Now()

	// Get all active policies sorted by priority (highest first)
	var policies []models.Policy
	query := pe.db.Where("status = ?", models.PolicyStatusActive).
		Where("valid_from <= ?", time.Now()).
		Where("valid_until IS NULL OR valid_until > ?", time.Now()).
		Order("priority DESC")

	if err := query.Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch policies: %v", err)
	}

	decision := &models.PolicyDecision{
		Allowed:           false,
		Effect:            models.PolicyEffectDeny,
		MatchedPolicies:   make([]uuid.UUID, 0),
		EvaluatedPolicies: len(policies),
		EvaluationTime:    time.Now(),
		Context:           make(map[string]string),
	}

	// Build full context
	context := pe.buildContext(req)

	var denyPolicies []uuid.UUID
	var allowPolicies []uuid.UUID

	// Evaluate each policy
	for _, policy := range policies {
		// Check if policy applies to this action
		if !pe.policyAppliesToAction(policy, req.Action) {
			continue
		}

		// Check if policy applies to this resource
		if !pe.policyAppliesToResource(policy, req.ResourceType) {
			continue
		}

		// Evaluate policy conditions
		matches, err := pe.evaluateConditions(policy.Conditions, context)
		if err != nil {
			// Log error but continue with other policies
			continue
		}

		if matches {
			decision.MatchedPolicies = append(decision.MatchedPolicies, policy.ID)

			if policy.Effect == models.PolicyEffectDeny {
				denyPolicies = append(denyPolicies, policy.ID)
			} else if policy.Effect == models.PolicyEffectAllow {
				allowPolicies = append(allowPolicies, policy.ID)
			}

			// Log evaluation asynchronously
			go pe.logEvaluation(policy.ID, req, policy.Effect, context, time.Since(startTime))
		}
	}

	// Decision logic: Any DENY overrides all ALLOW
	if len(denyPolicies) > 0 {
		decision.Allowed = false
		decision.Effect = models.PolicyEffectDeny
		decision.Reason = fmt.Sprintf("Denied by %d policy(ies)", len(denyPolicies))
	} else if len(allowPolicies) > 0 {
		decision.Allowed = true
		decision.Effect = models.PolicyEffectAllow
		decision.Reason = fmt.Sprintf("Allowed by %d policy(ies)", len(allowPolicies))
	} else {
		// No matching policies - return deny by default (fail-safe)
		decision.Allowed = false
		decision.Effect = models.PolicyEffectDeny
		decision.Reason = "No matching policies found"
	}

	decision.Context = context

	return decision, nil
}

// buildContext creates a complete context map from the request
func (pe *PolicyEngine) buildContext(req models.PolicyRequest) map[string]string {
	context := make(map[string]string)

	// Add user attributes
	for k, v := range req.UserAttributes {
		context[k] = v
	}

	// Add resource attributes
	for k, v := range req.ResourceAttributes {
		context[k] = v
	}

	// Add environment attributes
	for k, v := range req.Environment {
		context[k] = v
	}

	// Add computed attributes
	context["user.id"] = req.UserID.String()
	context["action"] = req.Action
	context["resource.type"] = req.ResourceType
	if req.ResourceID != nil {
		context["resource.id"] = req.ResourceID.String()
	}

	// Add time-based attributes
	now := time.Now()
	context["environment.hour"] = strconv.Itoa(now.Hour())
	context["environment.day_of_week"] = now.Weekday().String()
	context["environment.date"] = now.Format("2006-01-02")
	context["environment.timestamp"] = now.Format(time.RFC3339)

	return context
}

// evaluateConditions evaluates a condition tree
func (pe *PolicyEngine) evaluateConditions(conditions models.JSONMap, context map[string]string) (bool, error) {
	// Check for logical operators
	if andConditions, ok := conditions["AND"].([]interface{}); ok {
		return pe.evaluateAND(andConditions, context)
	}

	if orConditions, ok := conditions["OR"].([]interface{}); ok {
		return pe.evaluateOR(orConditions, context)
	}

	if notCondition, ok := conditions["NOT"].(map[string]interface{}); ok {
		result, err := pe.evaluateConditions(notCondition, context)
		return !result, err
	}

	// Single condition
	return pe.evaluateCondition(conditions, context)
}

// evaluateAND evaluates AND logic
func (pe *PolicyEngine) evaluateAND(conditions []interface{}, context map[string]string) (bool, error) {
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		result, err := pe.evaluateConditions(condMap, context)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil // Short-circuit on first false
		}
	}
	return true, nil
}

// evaluateOR evaluates OR logic
func (pe *PolicyEngine) evaluateOR(conditions []interface{}, context map[string]string) (bool, error) {
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		result, err := pe.evaluateConditions(condMap, context)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil // Short-circuit on first true
		}
	}
	return false, nil
}

// evaluateCondition evaluates a single condition
func (pe *PolicyEngine) evaluateCondition(condition models.JSONMap, context map[string]string) (bool, error) {
	attribute, ok := condition["attribute"].(string)
	if !ok {
		return false, fmt.Errorf("missing attribute in condition")
	}

	operator, ok := condition["operator"].(string)
	if !ok {
		return false, fmt.Errorf("missing operator in condition")
	}

	value := condition["value"]

	// Get actual value from context
	actualValue, exists := context[attribute]
	if !exists {
		// Attribute not found in context - treat as false
		return false, nil
	}

	// Resolve template variables in value
	if valueStr, ok := value.(string); ok && strings.HasPrefix(valueStr, "{{") && strings.HasSuffix(valueStr, "}}") {
		templateKey := strings.TrimSpace(valueStr[2 : len(valueStr)-2])
		if resolvedValue, exists := context[templateKey]; exists {
			value = resolvedValue
		} else {
			return false, nil
		}
	}

	// Evaluate based on operator
	return pe.evaluateOperator(actualValue, operator, value)
}

// evaluateOperator evaluates a comparison operator
func (pe *PolicyEngine) evaluateOperator(actual string, operator string, expected interface{}) (bool, error) {
	switch strings.ToUpper(operator) {
	case "=", "==", "EQUALS":
		return actual == fmt.Sprintf("%v", expected), nil

	case "!=", "NOT_EQUALS":
		return actual != fmt.Sprintf("%v", expected), nil

	case ">", "GREATER_THAN":
		actualNum, err1 := strconv.ParseFloat(actual, 64)
		expectedNum, err2 := pe.toFloat64(expected)
		if err1 != nil || err2 != nil {
			return false, nil
		}
		return actualNum > expectedNum, nil

	case "<", "LESS_THAN":
		actualNum, err1 := strconv.ParseFloat(actual, 64)
		expectedNum, err2 := pe.toFloat64(expected)
		if err1 != nil || err2 != nil {
			return false, nil
		}
		return actualNum < expectedNum, nil

	case ">=", "GREATER_THAN_OR_EQUAL":
		actualNum, err1 := strconv.ParseFloat(actual, 64)
		expectedNum, err2 := pe.toFloat64(expected)
		if err1 != nil || err2 != nil {
			return false, nil
		}
		return actualNum >= expectedNum, nil

	case "<=", "LESS_THAN_OR_EQUAL":
		actualNum, err1 := strconv.ParseFloat(actual, 64)
		expectedNum, err2 := pe.toFloat64(expected)
		if err1 != nil || err2 != nil {
			return false, nil
		}
		return actualNum <= expectedNum, nil

	case "IN":
		expectedArray, ok := expected.([]interface{})
		if !ok {
			// Try to convert to array
			if arr, ok := expected.([]string); ok {
				for _, v := range arr {
					if actual == v {
						return true, nil
					}
				}
				return false, nil
			}
			return false, nil
		}
		for _, v := range expectedArray {
			if actual == fmt.Sprintf("%v", v) {
				return true, nil
			}
		}
		return false, nil

	case "NOT_IN":
		result, err := pe.evaluateOperator(actual, "IN", expected)
		return !result, err

	case "CONTAINS":
		return strings.Contains(actual, fmt.Sprintf("%v", expected)), nil

	case "STARTS_WITH":
		return strings.HasPrefix(actual, fmt.Sprintf("%v", expected)), nil

	case "ENDS_WITH":
		return strings.HasSuffix(actual, fmt.Sprintf("%v", expected)), nil

	case "MATCHES":
		pattern := fmt.Sprintf("%v", expected)
		matched, err := regexp.MatchString(pattern, actual)
		return matched, err

	case "BETWEEN":
		expectedArray, ok := expected.([]interface{})
		if !ok || len(expectedArray) != 2 {
			return false, fmt.Errorf("BETWEEN requires array of 2 values")
		}
		actualNum, err := strconv.ParseFloat(actual, 64)
		if err != nil {
			return false, nil
		}
		min, err1 := pe.toFloat64(expectedArray[0])
		max, err2 := pe.toFloat64(expectedArray[1])
		if err1 != nil || err2 != nil {
			return false, nil
		}
		return actualNum >= min && actualNum <= max, nil

	case "NOT_BETWEEN":
		result, err := pe.evaluateOperator(actual, "BETWEEN", expected)
		return !result, err

	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

// toFloat64 converts various types to float64
func (pe *PolicyEngine) toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert to float64: %v", value)
	}
}

// policyAppliesToAction checks if policy applies to the given action
func (pe *PolicyEngine) policyAppliesToAction(policy models.Policy, action string) bool {
	if policy.Actions == nil || len(policy.Actions) == 0 {
		return true // No action restriction
	}

	for _, a := range policy.Actions {
		actionStr := fmt.Sprintf("%v", a)
		if actionStr == "*" || actionStr == action {
			return true
		}
		// Support wildcards like "project:*"
		if strings.HasSuffix(actionStr, "*") {
			prefix := strings.TrimSuffix(actionStr, "*")
			if strings.HasPrefix(action, prefix) {
				return true
			}
		}
	}

	return false
}

// policyAppliesToResource checks if policy applies to the given resource
func (pe *PolicyEngine) policyAppliesToResource(policy models.Policy, resourceType string) bool {
	if policy.Resources == nil || len(policy.Resources) == 0 {
		return true // No resource restriction
	}

	for _, r := range policy.Resources {
		resourceStr := fmt.Sprintf("%v", r)
		if resourceStr == "*" || resourceStr == resourceType {
			return true
		}
	}

	return false
}

// logEvaluation logs policy evaluation for audit
func (pe *PolicyEngine) logEvaluation(policyID uuid.UUID, req models.PolicyRequest, effect models.PolicyEffect, context map[string]string, duration time.Duration) {
	// Convert map[string]string to models.JSONMap (map[string]interface{})
	jsonContext := make(models.JSONMap)
	for k, v := range context {
		jsonContext[k] = v
	}

	evaluation := models.PolicyEvaluation{
		PolicyID:           policyID,
		UserID:             req.UserID,
		ResourceType:       req.ResourceType,
		ResourceID:         req.ResourceID,
		Action:             req.Action,
		Effect:             effect,
		Context:            jsonContext,
		EvaluationTime:     time.Now(),
		EvaluationDuration: int(duration.Milliseconds()),
	}

	// Store in database (ignore errors for async logging)
	pe.db.Create(&evaluation)
}
