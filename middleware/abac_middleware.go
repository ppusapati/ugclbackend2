package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/pkg/abac"
)

// RequireABACPolicy evaluates ABAC policies for authorization
// This middleware should be used AFTER RequirePermission for hybrid RBAC+ABAC
func RequireABACPolicy(action string, resourceType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)

			// Super admin bypass
			if claims.Role == "super_admin" {
				next.ServeHTTP(w, r)
				return
			}

			// Parse user ID
			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				http.Error(w, "Invalid user ID", http.StatusUnauthorized)
				return
			}

			// Get resource ID from request if available
			var resourceID *uuid.UUID
			resourceIDStr := r.URL.Query().Get("resource_id")
			if resourceIDStr != "" {
				if rid, err := uuid.Parse(resourceIDStr); err == nil {
					resourceID = &rid
				}
			}

			// Build policy request
			policyReq := models.PolicyRequest{
				UserID:             userID,
				Action:             action,
				ResourceType:       resourceType,
				ResourceID:         resourceID,
				UserAttributes:     make(map[string]string),
				ResourceAttributes: make(map[string]string),
				Environment:        make(map[string]string),
			}

			// Get user attributes
			attributeService := abac.NewAttributeService(config.DB)
			userAttrs, err := attributeService.GetUserAttributes(userID)
			if err == nil {
				policyReq.UserAttributes = userAttrs
			}

			// Add user role to attributes
			policyReq.UserAttributes["user.role"] = claims.Role

			// Get resource attributes if resource ID is provided
			if resourceID != nil {
				resourceAttrs, err := attributeService.GetResourceAttributes(resourceType, *resourceID)
				if err == nil {
					policyReq.ResourceAttributes = resourceAttrs
				}
			}

			// Add environment attributes
			policyReq.Environment["environment.ip_address"] = getClientIP(r)
			policyReq.Environment["environment.user_agent"] = r.UserAgent()

			// Evaluate policies
			policyEngine := abac.NewPolicyEngine(config.DB)
			decision, err := policyEngine.EvaluateRequest(policyReq)
			if err != nil {
				http.Error(w, fmt.Sprintf("Policy evaluation error: %v", err), http.StatusInternalServerError)
				return
			}

			// Check decision
			if !decision.Allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":  "Access denied by policy",
					"reason": decision.Reason,
					"effect": decision.Effect,
				})
				return
			}

			// Policy allows - proceed
			next.ServeHTTP(w, r)
		})
	}
}

// RequireHybridAuth combines RBAC and ABAC authorization
// First checks RBAC permission, then evaluates ABAC policies
func RequireHybridAuth(permission string, action string, resourceType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Chain RBAC and ABAC middlewares
		rbacHandler := RequirePermission(permission)(next)
		abacHandler := RequireABACPolicy(action, resourceType)(rbacHandler)
		return abacHandler
	}
}

// CheckPolicyDecision is a helper function to check policy decision for programmatic use
func CheckPolicyDecision(userID uuid.UUID, action string, resourceType string, resourceID *uuid.UUID) (*models.PolicyDecision, error) {
	// Build policy request
	policyReq := models.PolicyRequest{
		UserID:             userID,
		Action:             action,
		ResourceType:       resourceType,
		ResourceID:         resourceID,
		UserAttributes:     make(map[string]string),
		ResourceAttributes: make(map[string]string),
		Environment:        make(map[string]string),
	}

	// Get user attributes
	attributeService := abac.NewAttributeService(config.DB)
	userAttrs, err := attributeService.GetUserAttributes(userID)
	if err == nil {
		policyReq.UserAttributes = userAttrs
	}

	// Get resource attributes if resource ID is provided
	if resourceID != nil {
		resourceAttrs, err := attributeService.GetResourceAttributes(resourceType, *resourceID)
		if err == nil {
			policyReq.ResourceAttributes = resourceAttrs
		}
	}

	// Evaluate policies
	policyEngine := abac.NewPolicyEngine(config.DB)
	return policyEngine.EvaluateRequest(policyReq)
}
