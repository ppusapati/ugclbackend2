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
	phase1Handler := handlers.NewProjectPhase1Handler()

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

	// Phase 1 - WBS and planning controls
	r.Handle("/projects/{id}/wbs-nodes", middleware.RequirePermission("project:wbs_manage")(
		http.HandlerFunc(phase1Handler.CreateWBSNode))).Methods("POST")
	r.Handle("/projects/{id}/wbs-nodes", middleware.RequirePermission("project:wbs_read")(
		http.HandlerFunc(phase1Handler.ListWBSNodes))).Methods("GET")

	r.Handle("/projects/{id}/task-dependencies", middleware.RequirePermission("task:dependency_manage")(
		http.HandlerFunc(phase1Handler.CreateTaskDependency))).Methods("POST")
	r.Handle("/projects/{id}/task-dependencies", middleware.RequirePermission("task:dependency_read")(
		http.HandlerFunc(phase1Handler.ListTaskDependencies))).Methods("GET")

	// Phase 1 - BOQ and measurement book
	r.Handle("/projects/{id}/boq-items", middleware.RequirePermission("project:boq_manage")(
		http.HandlerFunc(phase1Handler.CreateBOQItem))).Methods("POST")
	r.Handle("/projects/{id}/boq-items", middleware.RequirePermission("project:boq_read")(
		http.HandlerFunc(phase1Handler.ListBOQItems))).Methods("GET")

	r.Handle("/projects/{id}/mb-entries", middleware.RequirePermission("project:mb_manage")(
		http.HandlerFunc(phase1Handler.CreateMBEntry))).Methods("POST")
	r.Handle("/projects/{id}/mb-entries", middleware.RequirePermission("project:mb_read")(
		http.HandlerFunc(phase1Handler.ListMBEntries))).Methods("GET")

	// Phase 1 - Running account billing
	r.Handle("/projects/{id}/ra-bills", middleware.RequirePermission("project:billing_manage")(
		http.HandlerFunc(phase1Handler.CreateRABill))).Methods("POST")
	r.Handle("/projects/{id}/ra-bills", middleware.RequirePermission("project:billing_read")(
		http.HandlerFunc(phase1Handler.ListRABills))).Methods("GET")
	r.Handle("/projects/{id}/ra-bills/{billId}", middleware.RequirePermission("project:billing_read")(
		http.HandlerFunc(phase1Handler.GetRABill))).Methods("GET")
	r.Handle("/projects/{id}/ra-bills/{billId}/lines", middleware.RequirePermission("project:billing_manage")(
		http.HandlerFunc(phase1Handler.AddRABillLine))).Methods("POST")
	r.Handle("/projects/{id}/ra-bills/{billId}/submit", middleware.RequirePermission("project:billing_submit")(
		http.HandlerFunc(phase1Handler.SubmitRABill))).Methods("POST")
	r.Handle("/projects/{id}/ra-bills/{billId}/approve", middleware.RequirePermission("project:billing_approve")(
		http.HandlerFunc(phase1Handler.ApproveRABill))).Methods("POST")
	r.Handle("/projects/{id}/ra-bills/{billId}/reject", middleware.RequirePermission("project:billing_approve")(
		http.HandlerFunc(phase1Handler.RejectRABill))).Methods("POST")
	r.Handle("/projects/{id}/ra-bills/{billId}/pay", middleware.RequirePermission("project:billing_pay")(
		http.HandlerFunc(phase1Handler.MarkRABillPaid))).Methods("POST")

	// =====================================================
	// Task Management Routes
	// =====================================================

	// Tasks (project management domain)
	r.Handle("/project-tasks", middleware.RequirePermission("task:create")(
		http.HandlerFunc(taskHandler.CreateTask))).Methods("POST")
	r.Handle("/project-tasks", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.ListTasks))).Methods("GET")
	r.Handle("/project-tasks/{id}", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.GetTask))).Methods("GET")
	r.Handle("/project-tasks/{id}", middleware.RequirePermission("task:update")(
		http.HandlerFunc(taskHandler.UpdateTask))).Methods("PUT")

	// Task Assignment
	r.Handle("/project-tasks/{id}/assign", middleware.RequirePermission("task:assign")(
		http.HandlerFunc(taskHandler.AssignTask))).Methods("POST")

	// Task Status
	r.Handle("/project-tasks/{id}/status", middleware.RequirePermission("task:update")(
		http.HandlerFunc(taskHandler.UpdateTaskStatus))).Methods("PUT")

	// Task Comments
	r.Handle("/project-tasks/{id}/comments", middleware.RequirePermission("task:comment")(
		http.HandlerFunc(taskHandler.AddTaskComment))).Methods("POST")
	r.Handle("/project-tasks/{id}/comments", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.GetTaskComments))).Methods("GET")
	r.Handle("/project-tasks/{id}/attachments", middleware.RequirePermission("task:update")(
		http.HandlerFunc(taskHandler.AddTaskAttachment))).Methods("POST")
	r.Handle("/project-tasks/{id}/attachments", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.GetTaskAttachments))).Methods("GET")

	// Task Audit Log
	r.Handle("/project-tasks/{id}/audit", middleware.RequirePermission("task:read")(
		http.HandlerFunc(taskHandler.GetTaskAuditLog))).Methods("GET")

	// Workflow Actions
	r.Handle("/project-tasks/{id}/submit", middleware.RequirePermission("task:submit")(
		http.HandlerFunc(workflowHandler.SubmitTaskForApproval))).Methods("POST")
	r.Handle("/project-tasks/{id}/approve", middleware.RequirePermission("task:approve")(
		http.HandlerFunc(workflowHandler.ApproveTask))).Methods("POST")
	r.Handle("/project-tasks/{id}/reject", middleware.RequirePermission("task:approve")(
		http.HandlerFunc(workflowHandler.RejectTask))).Methods("POST")
	r.Handle("/project-tasks/{id}/complete", middleware.RequirePermission("task:execute")(
		http.HandlerFunc(workflowHandler.CompleteTask))).Methods("POST")
	r.Handle("/project-tasks/{id}/workflow/history", middleware.RequirePermission("task:read")(
		http.HandlerFunc(workflowHandler.GetTaskWorkflowHistory))).Methods("GET")
	r.Handle("/project-tasks/{id}/workflow/actions", middleware.RequirePermission("task:read")(
		http.HandlerFunc(workflowHandler.GetAvailableTaskActions))).Methods("GET")
	r.Handle("/project-tasks/{id}/workflow", middleware.RequirePermission("task:update")(
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
