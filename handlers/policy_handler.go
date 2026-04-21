package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/sync/singleflight"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/pkg/abac"
)

const (
	policyListCacheTTL  = 10 * time.Minute
	policyStatsCacheTTL = 10 * time.Minute
)

type policyResponseCacheEntry struct {
	payload   []byte
	expiresAt time.Time
}

type policyResponseCache struct {
	mu      sync.Mutex
	entries map[string]policyResponseCacheEntry
}

var (
	policyListCache  = &policyResponseCache{entries: make(map[string]policyResponseCacheEntry)}
	policyStatsCache = &policyResponseCache{entries: make(map[string]policyResponseCacheEntry)}

	policyListLoadGroup  singleflight.Group
	policyStatsLoadGroup singleflight.Group
)

func (c *policyResponseCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return entry.payload, true
}

func (c *policyResponseCache) set(key string, payload []byte, ttl time.Duration) {
	c.mu.Lock()
	c.entries[key] = policyResponseCacheEntry{payload: payload, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

func (c *policyResponseCache) invalidate() {
	c.mu.Lock()
	clear(c.entries)
	c.mu.Unlock()
}

func invalidatePolicyCaches() {
	policyListCache.invalidate()
	policyStatsCache.invalidate()
}

// CreatePolicy creates a new policy
func CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var policy models.Policy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get current user
	userIDStr := middleware.GetUserID(r)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}
	policy.CreatedBy = userID

	// Create policy
	policyService := abac.NewPolicyService(config.DB)
	createdPolicy, err := policyService.CreatePolicy(policy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	invalidatePolicyCaches()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdPolicy)
}

// UpdatePolicy updates an existing policy
func UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	var updates models.Policy
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get current user
	userIDStr := middleware.GetUserID(r)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}
	updates.UpdatedBy = &userID

	// Update policy
	policyService := abac.NewPolicyService(config.DB)
	updatedPolicy, err := policyService.UpdatePolicy(policyID, updates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	invalidatePolicyCaches()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedPolicy)
}

// DeletePolicy deletes a policy
func DeletePolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	policyService := abac.NewPolicyService(config.DB)
	if err := policyService.DeletePolicy(policyID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	invalidatePolicyCaches()

	w.WriteHeader(http.StatusNoContent)
}

// GetPolicy retrieves a single policy
func GetPolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	policyService := abac.NewPolicyService(config.DB)
	policy, err := policyService.GetPolicy(policyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

// ListPolicies lists all policies with pagination and filtering
func ListPolicies(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	statusStr := r.URL.Query().Get("status")
	businessIDStr := r.URL.Query().Get("business_vertical_id")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	var status *models.PolicyStatus
	if statusStr != "" {
		s := models.PolicyStatus(statusStr)
		status = &s
	}

	var businessVerticalID *uuid.UUID
	if businessIDStr != "" {
		if id, err := uuid.Parse(businessIDStr); err == nil {
			businessVerticalID = &id
		}
	}

	policyService := abac.NewPolicyService(config.DB)
	statusKey := ""
	if status != nil {
		statusKey = string(*status)
	}
	businessKey := ""
	if businessVerticalID != nil {
		businessKey = businessVerticalID.String()
	}
	cacheKey := statusKey + "|" + businessKey + "|" + strconv.Itoa(limit) + "|" + strconv.Itoa(offset)

	if payload, ok := policyListCache.get(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
		return
	}

	loaded, err, _ := policyListLoadGroup.Do(cacheKey, func() (interface{}, error) {
		if payload, ok := policyListCache.get(cacheKey); ok {
			return payload, nil
		}

		policies, total, listErr := policyService.ListPolicies(status, businessVerticalID, limit, offset)
		if listErr != nil {
			return nil, listErr
		}

		response := map[string]interface{}{
			"policies": policies,
			"total":    total,
			"limit":    limit,
			"offset":   offset,
		}

		payload, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			return nil, marshalErr
		}
		policyListCache.set(cacheKey, payload, policyListCacheTTL)
		return payload, nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(loaded.([]byte))
}

// ActivatePolicy activates a policy
func ActivatePolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	userIDStr := middleware.GetUserID(r)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}
	policyService := abac.NewPolicyService(config.DB)
	if err := policyService.ActivatePolicy(policyID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	invalidatePolicyCaches()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Policy activated successfully"})
}

// DeactivatePolicy deactivates a policy
func DeactivatePolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	userIDStr := middleware.GetUserID(r)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}
	policyService := abac.NewPolicyService(config.DB)
	if err := policyService.DeactivatePolicy(policyID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	invalidatePolicyCaches()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Policy deactivated successfully"})
}

// TestPolicy tests a policy against a request
func TestPolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	var req models.PolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	policyService := abac.NewPolicyService(config.DB)
	decision, err := policyService.TestPolicy(policyID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decision)
}

// ClonePolicy clones an existing policy
func ClonePolicy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	var req struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userIDStr := middleware.GetUserID(r)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}
	policyService := abac.NewPolicyService(config.DB)
	clonedPolicy, err := policyService.ClonePolicy(policyID, userID, req.NewName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	invalidatePolicyCaches()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(clonedPolicy)
}

// GetPolicyEvaluations retrieves evaluation history for a policy
func GetPolicyEvaluations(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	policyService := abac.NewPolicyService(config.DB)
	evaluations, total, err := policyService.GetPolicyEvaluations(policyID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"evaluations": evaluations,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetPolicyStatistics returns policy statistics
func GetPolicyStatistics(w http.ResponseWriter, r *http.Request) {
	policyService := abac.NewPolicyService(config.DB)
	const cacheKey = "all"

	if payload, ok := policyStatsCache.get(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
		return
	}

	loaded, err, _ := policyStatsLoadGroup.Do(cacheKey, func() (interface{}, error) {
		if payload, ok := policyStatsCache.get(cacheKey); ok {
			return payload, nil
		}

		stats, statsErr := policyService.GetPolicyStatistics()
		if statsErr != nil {
			return nil, statsErr
		}

		payload, marshalErr := json.Marshal(stats)
		if marshalErr != nil {
			return nil, marshalErr
		}
		policyStatsCache.set(cacheKey, payload, policyStatsCacheTTL)
		return payload, nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(loaded.([]byte))
}

// EvaluatePolicyRequest evaluates an authorization request against all policies
func EvaluatePolicyRequest(w http.ResponseWriter, r *http.Request) {
	var req models.PolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	policyEngine := abac.NewPolicyEngine(config.DB)
	decision, err := policyEngine.EvaluateRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decision)
}
