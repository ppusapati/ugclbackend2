package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
)

// RegisterProjectRoutes registers all project management routes
// Call this from your main routes setup
func RegisterProjectRoutes(r *mux.Router) {
	// Initialize handlers
	projectHandler := handlers.NewProjectHandler()
	taskHandler := handlers.NewTaskHandler()
	budgetHandler := handlers.NewBudgetHandler()
	roleHandler := handlers.NewProjectRoleHandler()
	workflowHandler := handlers.NewProjectWorkflowHandler()

	// =====================================================
	// Project Management Routes
	// =====================================================

	// Projects
	r.Handle("/admin/projects", middleware.RequirePermission("project:create")(
		http.HandlerFunc(projectHandler.CreateProject))).Methods("POST")
	r.Handle("/projects", middleware.RequirePermission("project:read")(
		http.HandlerFunc(projectHandler.ListProjects))).Methods("GET")
	r.Handle("/projects/{id}", middleware.RequirePermission("project:read")(
		http.HandlerFunc(projectHandler.GetProject))).Methods("GET")
	r.Handle("/projects/{id}", middleware.RequirePermission("project:update")(
		http.HandlerFunc(projectHandler.UpdateProject))).Methods("PUT")
	r.Handle("/projects/{id}", middleware.RequirePermission("project:delete")(
		http.HandlerFunc(projectHandler.DeleteProject))).Methods("DELETE")

	// KMZ Upload
	r.Handle("/projects/{id}/kmz", middleware.RequirePermission("project:update")(
		http.HandlerFunc(projectHandler.UploadKMZ))).Methods("POST")
	r.Handle("/projects/{id}/geojson", middleware.RequirePermission("project:read")(
		http.HandlerFunc(projectHandler.GetProjectGeoJSON))).Methods("GET")

	// Project Zones and Nodes
	r.Handle("/projects/{id}/zones", middleware.RequirePermission("project:read")(
		http.HandlerFunc(projectHandler.GetProjectZones))).Methods("GET")
	r.Handle("/projects/{id}/nodes", middleware.RequirePermission("project:read")(
		http.HandlerFunc(projectHandler.GetProjectNodes))).Methods("GET")

	// Project Statistics
	r.Handle("/projects/{id}/stats", middleware.RequirePermission("project:read")(
		http.HandlerFunc(projectHandler.GetProjectStats))).Methods("GET")

	// =====================================================
	// Task Management Routes
	// =====================================================

	// Tasks
	r.Handle("/api/v1/tasks", middleware.RequirePermission("task:create")(
		http.HandlerFunc(taskHandler.CreateTask))).Methods("POST")
	r.Handle("/api/v1/tasks", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.ListTasks))).Methods("GET")
	r.Handle("/api/v1/tasks/{id}", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.GetTask))).Methods("GET")
	r.Handle("/api/v1/tasks/{id}", middleware.RequirePermission("task:update")(
		http.HandlerFunc(taskHandler.UpdateTask))).Methods("PUT")

	// Task Assignment
	r.Handle("/api/v1/tasks/{id}/assign", middleware.RequirePermission("task:assign")(
		http.HandlerFunc(taskHandler.AssignTask))).Methods("POST")

	// Task Status
	r.Handle("/api/v1/tasks/{id}/status", middleware.RequirePermission("task:update")(
		http.HandlerFunc(taskHandler.UpdateTaskStatus))).Methods("PUT")

	// Task Comments
	r.Handle("/api/v1/tasks/{id}/comments", middleware.RequirePermission("task:comment")(
		http.HandlerFunc(taskHandler.AddTaskComment))).Methods("POST")
	r.Handle("/api/v1/tasks/{id}/comments", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.GetTaskComments))).Methods("GET")

	// Task Audit Log
	r.Handle("/api/v1/tasks/{id}/audit", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.GetTaskAuditLog))).Methods("GET")

	// Workflow Actions
	r.Handle("/api/v1/tasks/{id}/submit", middleware.RequirePermission("task:submit")(
		http.HandlerFunc(workflowHandler.SubmitTaskForApproval))).Methods("POST")
	r.Handle("/api/v1/tasks/{id}/approve", middleware.RequirePermission("task:approve")(
		http.HandlerFunc(workflowHandler.ApproveTask))).Methods("POST")
	r.Handle("/api/v1/tasks/{id}/reject", middleware.RequirePermission("task:approve")(
		http.HandlerFunc(workflowHandler.RejectTask))).Methods("POST")
	r.Handle("/api/v1/tasks/{id}/complete", middleware.RequirePermission("task:execute")(
		http.HandlerFunc(workflowHandler.CompleteTask))).Methods("POST")
	r.Handle("/api/v1/tasks/{id}/workflow/history", middleware.RequirePermission("task:read")(
		http.HandlerFunc(workflowHandler.GetTaskWorkflowHistory))).Methods("GET")
	r.Handle("/api/v1/tasks/{id}/workflow/actions", middleware.RequirePermission("task:read")(
		http.HandlerFunc(workflowHandler.GetAvailableTaskActions))).Methods("GET")
	r.Handle("/api/v1/tasks/{id}/workflow", middleware.RequirePermission("task:update")(
		http.HandlerFunc(workflowHandler.AssignWorkflowToTask))).Methods("POST")

	// =====================================================
	// Budget Management Routes
	// =====================================================

	// Budget Allocations
	r.Handle("/api/v1/budget/allocations", middleware.RequirePermission("budget:allocate")(
		http.HandlerFunc(budgetHandler.CreateBudgetAllocation))).Methods("POST")
	r.Handle("/api/v1/budget/allocations", middleware.RequirePermission("budget:view")(
		http.HandlerFunc(budgetHandler.ListBudgetAllocations))).Methods("GET")
	r.Handle("/api/v1/budget/allocations/{id}", middleware.RequirePermission("budget:view")(
		http.HandlerFunc(budgetHandler.GetBudgetAllocation))).Methods("GET")
	r.Handle("/api/v1/budget/allocations/{id}", middleware.RequirePermission("budget:manage")(
		http.HandlerFunc(budgetHandler.UpdateBudgetAllocation))).Methods("PUT")
	r.Handle("/api/v1/budget/allocations/{id}", middleware.RequirePermission("budget:manage")(
		http.HandlerFunc(budgetHandler.DeleteBudgetAllocation))).Methods("DELETE")

	// Budget Approval
	r.Handle("/api/v1/budget/allocations/{id}/approve", middleware.RequirePermission("budget:manage")(
		http.HandlerFunc(budgetHandler.ApproveBudgetAllocation))).Methods("POST")

	// Budget Summaries
	r.Handle("/api/v1/budget/projects/{id}/summary", middleware.RequirePermission("budget:view")(
		http.HandlerFunc(budgetHandler.GetProjectBudgetSummary))).Methods("GET")
	r.Handle("/api/v1/budget/tasks/{id}/summary", middleware.RequirePermission("budget:view")(
		http.HandlerFunc(budgetHandler.GetTaskBudgetSummary))).Methods("GET")

	// =====================================================
	// Project Roles & Permissions Routes
	// =====================================================

	// Roles
	r.Handle("/api/v1/project-roles", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(roleHandler.CreateRole))).Methods("POST")
	r.Handle("/api/v1/project-roles", middleware.RequirePermission("project:read")(
		http.HandlerFunc(roleHandler.ListRoles))).Methods("GET")
	r.Handle("/api/v1/project-roles/{id}", middleware.RequirePermission("project:read")(
		http.HandlerFunc(roleHandler.GetRole))).Methods("GET")
	r.Handle("/api/v1/project-roles/{id}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(roleHandler.UpdateRole))).Methods("PUT")
	r.Handle("/api/v1/project-roles/{id}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(roleHandler.DeleteRole))).Methods("DELETE")

	// Role Assignments
	r.Handle("/api/v1/project-roles/assign", middleware.RequirePermission("user:assign")(
		http.HandlerFunc(roleHandler.AssignRoleToUser))).Methods("POST")
	r.Handle("/api/v1/project-roles/assignments/{id}", middleware.RequirePermission("user:assign")(
		http.HandlerFunc(roleHandler.RevokeRoleFromUser))).Methods("DELETE")
	r.Handle("/api/v1/project-roles/assignments", middleware.RequirePermission("project:read")(
		http.HandlerFunc(roleHandler.GetUserProjectRoles))).Methods("GET")

	// Permission Checking
	r.Handle("/api/v1/project-roles/check-permission", middleware.RequirePermission("project:read")(
		http.HandlerFunc(roleHandler.CheckUserPermission))).Methods("GET")
	r.Handle("/api/v1/project-roles/permissions", middleware.RequirePermission("project:read")(
		http.HandlerFunc(roleHandler.GetAvailablePermissions))).Methods("GET")

	// Project Users
	r.Handle("/api/v1/projects/{id}/users", middleware.RequirePermission("project:read")(
		http.HandlerFunc(roleHandler.GetProjectUsers))).Methods("GET")

	// =====================================================
	// Workflow Management Routes
	// =====================================================

	// Create default workflows
	r.Handle("/api/v1/workflows/task-approval", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(workflowHandler.CreateTaskApprovalWorkflow))).Methods("POST")
}
