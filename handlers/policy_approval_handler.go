package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/pkg/abac"
)

// CreateApprovalRequest creates a new policy approval request
func CreateApprovalRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PolicyID        string          `json:"policy_id"`
		RequestType     string          `json:"request_type"` // create, update, activate, deactivate, delete
		Notes           string          `json:"notes"`
		ChangesProposed models.JSONMap  `json:"changes_proposed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	policyID, err := uuid.Parse(req.PolicyID)
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

	approvalService := abac.NewApprovalService(config.DB)
	request, err := approvalService.CreateApprovalRequest(policyID, req.RequestType, userID, req.Notes, req.ChangesProposed)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(request)
}

// ApproveRequest approves a policy approval request
func ApproveRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Comments string `json:"comments"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	userIDStr := middleware.GetUserID(r)
	approverID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}

	approvalService := abac.NewApprovalService(config.DB)
	request, err := approvalService.ApproveRequest(requestID, approverID, req.Comments)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(request)
}

// RejectRequest rejects a policy approval request
func RejectRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Comments string `json:"comments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userIDStr := middleware.GetUserID(r)
	approverID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}

	approvalService := abac.NewApprovalService(config.DB)
	request, err := approvalService.RejectRequest(requestID, approverID, req.Comments)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(request)
}

// GetPendingApprovals gets all pending approval requests
func GetPendingApprovals(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

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

	approvalService := abac.NewApprovalService(config.DB)
	requests, total, err := approvalService.GetPendingApprovals(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"requests": requests,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetMyPendingApprovals gets pending approvals for current user
func GetMyPendingApprovals(w http.ResponseWriter, r *http.Request) {
	userIDStr := middleware.GetUserID(r)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusUnauthorized)
		return
	}

	// Get user roles
	user := middleware.GetUser(r)
	userRoles := []string{}
	if user.RoleModel != nil {
		userRoles = append(userRoles, user.RoleModel.Name)
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

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

	approvalService := abac.NewApprovalService(config.DB)
	requests, total, err := approvalService.GetUserPendingApprovals(userID, userRoles, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"requests": requests,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetApprovalRequest gets a specific approval request
func GetApprovalRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	approvalService := abac.NewApprovalService(config.DB)
	request, err := approvalService.GetApprovalRequest(requestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(request)
}

// GetPolicyVersions gets all versions of a policy
func GetPolicyVersions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	policyID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	approvalService := abac.NewApprovalService(config.DB)
	versions, err := approvalService.GetPolicyVersions(policyID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}

// GetPolicyChangeLogs gets change history for a policy
func GetPolicyChangeLogs(w http.ResponseWriter, r *http.Request) {
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

	approvalService := abac.NewApprovalService(config.DB)
	logs, total, err := approvalService.GetPolicyChangeLogs(policyID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateWorkflow creates a new approval workflow
func CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var workflow models.PolicyApprovalWorkflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	approvalService := abac.NewApprovalService(config.DB)
	if err := approvalService.CreateWorkflow(&workflow); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(workflow)
}

// GetWorkflows gets all approval workflows
func GetWorkflows(w http.ResponseWriter, r *http.Request) {
	approvalService := abac.NewApprovalService(config.DB)
	workflows, err := approvalService.GetWorkflows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflows)
}
