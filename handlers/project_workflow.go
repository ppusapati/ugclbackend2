package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// ProjectWorkflowHandler handles workflow integration for projects and tasks
type ProjectWorkflowHandler struct {
	db             *gorm.DB
	workflowEngine *WorkflowEngine
}

// NewProjectWorkflowHandler creates a new project workflow handler
func NewProjectWorkflowHandler() *ProjectWorkflowHandler {
	return &ProjectWorkflowHandler{
		db:             config.DB,
		workflowEngine: NewWorkflowEngine(),
	}
}

// SubmitTaskForApproval submits a task for approval workflow
func (h *ProjectWorkflowHandler) SubmitTaskForApproval(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req struct {
		Comment string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Get task
	var task models.Tasks
	if err := h.db.Preload("Project").First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get user info from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	// Check if task has a workflow
	if task.WorkflowID == nil {
		http.Error(w, "Task does not have a workflow configured", http.StatusBadRequest)
		return
	}

	// If task doesn't have a form submission, create one
	if task.FormSubmissionID == nil {
		// Create task data as JSON
		taskData := map[string]interface{}{
			"task_id":       task.ID,
			"code":          task.Code,
			"title":         task.Title,
			"description":   task.Description,
			"start_node_id": task.StartNodeID,
			"stop_node_id":  task.StopNodeID,
			"status":        task.Status,
			"progress":      task.Progress,
			"priority":      task.Priority,
		}
		formDataJSON, _ := json.Marshal(taskData)

		// Create form submission
		submission, err := h.workflowEngine.CreateSubmission(
			"task_approval", // form code (needs to be created)
			task.Project.BusinessVerticalID,
			nil, // no specific site
			json.RawMessage(formDataJSON),
			claims.UserID,
		)

		if err != nil {
			log.Printf("❌ Failed to create form submission: %v", err)
			http.Error(w, "Failed to create workflow submission", http.StatusInternalServerError)
			return
		}

		// Link submission to task
		task.FormSubmissionID = &submission.ID
		task.CurrentState = submission.CurrentState
		h.db.Save(&task)
	}

	// Transition to submitted state
	submission, err := h.workflowEngine.TransitionState(
		*task.FormSubmissionID,
		"submit",
		claims.UserID,
		user.Name,
		"submitter",
		req.Comment,
		map[string]interface{}{
			"task_id": task.ID.String(),
		},
	)

	if err != nil {
		log.Printf("❌ Failed to submit task for approval: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update task state
	task.CurrentState = submission.CurrentState
	h.db.Save(&task)

	log.Printf("✅ Task submitted for approval: %s", taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Task submitted for approval successfully",
		"task":       task,
		"submission": submission,
	})
}

// ApproveTask approves a task
func (h *ProjectWorkflowHandler) ApproveTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req struct {
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Comment == "" {
		http.Error(w, "Comment is required for approval", http.StatusBadRequest)
		return
	}

	// Get task
	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if task.FormSubmissionID == nil {
		http.Error(w, "Task not submitted for approval", http.StatusBadRequest)
		return
	}

	// Get user info from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	// Transition to approved state
	submission, err := h.workflowEngine.TransitionState(
		*task.FormSubmissionID,
		"approve",
		claims.UserID,
		user.Name,
		claims.Role,
		req.Comment,
		map[string]interface{}{
			"task_id": task.ID.String(),
		},
	)

	if err != nil {
		log.Printf("❌ Failed to approve task: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update task state and status
	task.CurrentState = submission.CurrentState
	if task.Status == "pending" || task.Status == "assigned" {
		task.Status = "in-progress"
	}
	h.db.Save(&task)

	log.Printf("✅ Task approved: %s by %s", taskID, user.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Task approved successfully",
		"task":       task,
		"submission": submission,
	})
}

// RejectTask rejects a task
func (h *ProjectWorkflowHandler) RejectTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req struct {
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Comment == "" {
		http.Error(w, "Comment is required for rejection", http.StatusBadRequest)
		return
	}

	// Get task
	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if task.FormSubmissionID == nil {
		http.Error(w, "Task not submitted for approval", http.StatusBadRequest)
		return
	}

	// Get user info from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	// Transition to rejected state
	submission, err := h.workflowEngine.TransitionState(
		*task.FormSubmissionID,
		"reject",
		claims.UserID,
		user.Name,
		claims.Role,
		req.Comment,
		map[string]interface{}{
			"task_id": task.ID.String(),
		},
	)

	if err != nil {
		log.Printf("❌ Failed to reject task: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update task state
	task.CurrentState = submission.CurrentState
	h.db.Save(&task)

	log.Printf("⚠️  Task rejected: %s by %s", taskID, user.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Task rejected",
		"task":       task,
		"submission": submission,
	})
}

// CompleteTask marks a task as completed and submits for final approval
func (h *ProjectWorkflowHandler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req struct {
		Comment        string                 `json:"comment"`
		CompletionData map[string]interface{} `json:"completion_data"`
		ActualEndDate  *string                `json:"actual_end_date"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Get task
	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get user info from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	// If task has workflow, transition to completed state
	if task.FormSubmissionID != nil {
		submission, err := h.workflowEngine.TransitionState(
			*task.FormSubmissionID,
			"complete",
			claims.UserID,
			user.Name,
			"worker",
			req.Comment,
			req.CompletionData,
		)

		if err != nil {
			log.Printf("❌ Failed to complete task: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		task.CurrentState = submission.CurrentState
	}

	// Update task status
	task.Progress = 100
	task.UpdatedBy = claims.UserID
	h.db.Save(&task)

	log.Printf("✅ Task marked as completed: %s", taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Task marked as completed",
		"task":    task,
	})
}

// GetTaskWorkflowHistory retrieves workflow history for a task
func (h *ProjectWorkflowHandler) GetTaskWorkflowHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	// Get task
	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if task.FormSubmissionID == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"history": []interface{}{},
			"message": "No workflow history available",
		})
		return
	}

	// Get workflow transitions
	transitions, err := h.workflowEngine.GetWorkflowHistory(*task.FormSubmissionID)
	if err != nil {
		http.Error(w, "Failed to fetch workflow history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"history": transitions,
		"count":   len(transitions),
	})
}

// GetAvailableTaskActions retrieves available workflow actions for a task
func (h *ProjectWorkflowHandler) GetAvailableTaskActions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	// Get task
	var task models.Tasks
	if err := h.db.Preload("Workflow").First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if task.FormSubmissionID == nil || task.Workflow == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"actions": []interface{}{},
			"message": "No workflow configured for this task",
		})
		return
	}

	// Get submission
	submission, err := h.workflowEngine.GetSubmission(*task.FormSubmissionID)
	if err != nil {
		http.Error(w, "Failed to fetch workflow submission", http.StatusInternalServerError)
		return
	}

	// Get available actions
	actions, err := submission.GetAvailableActions(task.Workflow)
	if err != nil {
		http.Error(w, "Failed to get available actions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_state": task.CurrentState,
		"actions":       actions,
	})
}

// CreateTaskApprovalWorkflow creates the default task approval workflow
func (h *ProjectWorkflowHandler) CreateTaskApprovalWorkflow(w http.ResponseWriter, r *http.Request) {
	// Define states
	states := []models.WorkflowState{
		{Code: "draft", Name: "Draft", Description: "Task is being drafted", Color: "#gray", Icon: "edit"},
		{Code: "submitted", Name: "Submitted", Description: "Submitted for approval", Color: "#blue", Icon: "send"},
		{Code: "approved", Name: "Approved", Description: "Approved by supervisor", Color: "#green", Icon: "check"},
		{Code: "in_progress", Name: "In Progress", Description: "Work is in progress", Color: "#yellow", Icon: "play"},
		{Code: "completed", Name: "Completed", Description: "Work completed, pending verification", Color: "#orange", Icon: "done"},
		{Code: "verified", Name: "Verified", Description: "Completed and verified", Color: "#green", Icon: "verified", IsFinal: true},
		{Code: "rejected", Name: "Rejected", Description: "Rejected", Color: "#red", Icon: "close"},
	}

	// Define transitions
	transitions := []models.WorkflowTransitionDef{
		{From: "draft", To: "submitted", Action: "submit", Label: "Submit for Approval", Permission: "task:submit"},
		{From: "submitted", To: "approved", Action: "approve", Label: "Approve", Permission: "task:approve", RequiresComment: true},
		{From: "submitted", To: "rejected", Action: "reject", Label: "Reject", Permission: "task:approve", RequiresComment: true},
		{From: "rejected", To: "draft", Action: "revise", Label: "Revise", Permission: "task:update"},
		{From: "approved", To: "in_progress", Action: "start", Label: "Start Work", Permission: "task:execute"},
		{From: "in_progress", To: "completed", Action: "complete", Label: "Mark as Completed", Permission: "task:execute"},
		{From: "completed", To: "verified", Action: "verify", Label: "Verify Completion", Permission: "task:verify", RequiresComment: true},
		{From: "completed", To: "in_progress", Action: "return", Label: "Return for Revision", Permission: "task:verify", RequiresComment: true},
	}

	statesJSON, _ := json.Marshal(states)
	transitionsJSON, _ := json.Marshal(transitions)

	// Create workflow definition
	workflow := models.WorkflowDefinition{
		Code:         "task_approval",
		Name:         "Task Approval Workflow",
		Description:  "Standard workflow for task approvals in project management",
		Version:      "1.0.0",
		InitialState: "draft",
		States:       json.RawMessage(statesJSON),
		Transitions:  json.RawMessage(transitionsJSON),
		IsActive:     true,
	}

	if err := h.db.Create(&workflow).Error; err != nil {
		http.Error(w, "Failed to create workflow", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Created task approval workflow: %s", workflow.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Task approval workflow created successfully",
		"workflow": workflow,
	})
}

// AssignWorkflowToTask assigns a workflow to a task
func (h *ProjectWorkflowHandler) AssignWorkflowToTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req struct {
		WorkflowID uuid.UUID `json:"workflow_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Verify workflow exists
	var workflow models.WorkflowDefinition
	if err := h.db.First(&workflow, "id = ? AND is_active = ?", req.WorkflowID, true).Error; err != nil {
		http.Error(w, "Workflow not found or inactive", http.StatusBadRequest)
		return
	}

	// Get task
	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Assign workflow
	task.WorkflowID = &req.WorkflowID
	task.CurrentState = workflow.InitialState

	if err := h.db.Save(&task).Error; err != nil {
		http.Error(w, "Failed to assign workflow", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Assigned workflow %s to task %s", req.WorkflowID, taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Workflow assigned to task successfully",
		"task":    task,
	})
}
