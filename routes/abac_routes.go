package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
)

// RegisterABACRoutes registers ABAC and Policy management routes
func RegisterABACRoutes(api *mux.Router) {
	// Policy Management Routes
	policyRouter := api.PathPrefix("/policies").Subrouter()

	// List and create policies
	policyRouter.Handle("", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.ListPolicies))).Methods("GET")
	policyRouter.Handle("", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.CreatePolicy))).Methods("POST")

	// Policy statistics
	policyRouter.Handle("/statistics", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.GetPolicyStatistics))).Methods("GET")

	// Policy evaluation endpoint (any authenticated user can test policies)
	policyRouter.Handle("/evaluate", http.HandlerFunc(handlers.EvaluatePolicyRequest)).Methods("POST")

	// Individual policy operations
	policyRouter.Handle("/{id}", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.GetPolicy))).Methods("GET")
	policyRouter.Handle("/{id}", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.UpdatePolicy))).Methods("PUT")
	policyRouter.Handle("/{id}", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.DeletePolicy))).Methods("DELETE")

	// Policy status management
	policyRouter.Handle("/{id}/activate", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.ActivatePolicy))).Methods("POST")
	policyRouter.Handle("/{id}/deactivate", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.DeactivatePolicy))).Methods("POST")

	// Test policy
	policyRouter.Handle("/{id}/test", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.TestPolicy))).Methods("POST")

	// Clone policy
	policyRouter.Handle("/{id}/clone", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.ClonePolicy))).Methods("POST")

	// Policy evaluation history
	policyRouter.Handle("/{id}/evaluations", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.GetPolicyEvaluations))).Methods("GET")

	// Policy version history and change logs
	policyRouter.Handle("/{id}/versions", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.GetPolicyVersions))).Methods("GET")
	policyRouter.Handle("/{id}/changelog", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.GetPolicyChangeLogs))).Methods("GET")

	// Attribute Management Routes
	attributeRouter := api.PathPrefix("/attributes").Subrouter()

	// List and create attributes
	attributeRouter.Handle("", middleware.RequirePermission("manage_attributes")(http.HandlerFunc(handlers.ListAttributes))).Methods("GET")
	attributeRouter.Handle("", middleware.RequirePermission("manage_attributes")(http.HandlerFunc(handlers.CreateAttribute))).Methods("POST")

	// Individual attribute operations
	attributeRouter.Handle("/{id}", middleware.RequirePermission("manage_attributes")(http.HandlerFunc(handlers.UpdateAttribute))).Methods("PUT")
	attributeRouter.Handle("/{id}", middleware.RequirePermission("manage_attributes")(http.HandlerFunc(handlers.DeleteAttribute))).Methods("DELETE")

	// User Attribute Management
	userAttrRouter := api.PathPrefix("/users").Subrouter()

	// Get user attributes
	userAttrRouter.Handle("/{user_id}/attributes", http.HandlerFunc(handlers.GetUserAttributes)).Methods("GET")

	// Assign/remove user attributes
	userAttrRouter.Handle("/{user_id}/attributes", middleware.RequirePermission("manage_user_attributes")(http.HandlerFunc(handlers.AssignUserAttribute))).Methods("POST")
	userAttrRouter.Handle("/{user_id}/attributes/bulk", middleware.RequirePermission("manage_user_attributes")(http.HandlerFunc(handlers.BulkAssignUserAttributes))).Methods("POST")
	userAttrRouter.Handle("/{user_id}/attributes/{attribute_id}", middleware.RequirePermission("manage_user_attributes")(http.HandlerFunc(handlers.RemoveUserAttribute))).Methods("DELETE")

	// User attribute history
	userAttrRouter.Handle("/{user_id}/attributes/{attribute_id}/history", middleware.RequirePermission("manage_user_attributes")(http.HandlerFunc(handlers.GetUserAttributeHistory))).Methods("GET")

	// Resource Attribute Management
	resourceAttrRouter := api.PathPrefix("/resources").Subrouter()

	// Get resource attributes
	resourceAttrRouter.Handle("/{resource_type}/{resource_id}/attributes", http.HandlerFunc(handlers.GetResourceAttributes)).Methods("GET")

	// Assign/remove resource attributes
	resourceAttrRouter.Handle("/attributes", middleware.RequirePermission("manage_resource_attributes")(http.HandlerFunc(handlers.AssignResourceAttribute))).Methods("POST")
	resourceAttrRouter.Handle("/{resource_type}/{resource_id}/attributes/{attribute_id}", middleware.RequirePermission("manage_resource_attributes")(http.HandlerFunc(handlers.RemoveResourceAttribute))).Methods("DELETE")

	// Policy Approval Workflow Routes
	approvalRouter := api.PathPrefix("/approvals").Subrouter()

	// Approval requests
	approvalRouter.Handle("/requests", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.CreateApprovalRequest))).Methods("POST")
	approvalRouter.Handle("/requests/pending", http.HandlerFunc(handlers.GetPendingApprovals)).Methods("GET")
	approvalRouter.Handle("/requests/my-pending", http.HandlerFunc(handlers.GetMyPendingApprovals)).Methods("GET")
	approvalRouter.Handle("/requests/{id}", http.HandlerFunc(handlers.GetApprovalRequest)).Methods("GET")
	approvalRouter.Handle("/requests/{id}/approve", http.HandlerFunc(handlers.ApproveRequest)).Methods("POST")
	approvalRouter.Handle("/requests/{id}/reject", http.HandlerFunc(handlers.RejectRequest)).Methods("POST")

	// Approval workflows
	approvalRouter.Handle("/workflows", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.GetWorkflows))).Methods("GET")
	approvalRouter.Handle("/workflows", middleware.RequirePermission("manage_policies")(http.HandlerFunc(handlers.CreateWorkflow))).Methods("POST")
}
