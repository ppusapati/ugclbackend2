package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// TaskHandler handles task management operations
type TaskHandler struct {
	db             *gorm.DB
	workflowEngine *WorkflowEngine
}

// NewTaskHandler creates a new task handler
func NewTaskHandler() *TaskHandler {
	return &TaskHandler{
		db:             config.DB,
		workflowEngine: NewWorkflowEngine(),
	}
}

// CreateTaskRequest represents the request to create a task
type CreateTaskRequest struct {
	Code             string                 `json:"code"`
	Title            string                 `json:"title"`
	Description      string                 `json:"description"`
	ProjectID        uuid.UUID              `json:"project_id"`
	ZoneID           *uuid.UUID             `json:"zone_id"`
	StartNodeID      uuid.UUID              `json:"start_node_id"`
	StopNodeID       uuid.UUID              `json:"stop_node_id"`
	PlannedStartDate *time.Time             `json:"planned_start_date"`
	PlannedEndDate   *time.Time             `json:"planned_end_date"`
	AllocatedBudget  float64                `json:"allocated_budget"`
	Priority         string                 `json:"priority"`
	WorkflowID       *uuid.UUID             `json:"workflow_id"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// UpdateTaskRequest represents the request to update a task
type UpdateTaskRequest struct {
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	PlannedStartDate *time.Time `json:"planned_start_date"`
	PlannedEndDate   *time.Time `json:"planned_end_date"`
	AllocatedBudget  float64    `json:"allocated_budget"`
	Status           string     `json:"status"`
	Progress         float64    `json:"progress"`
	Priority         string     `json:"priority"`
	LaborCost        float64    `json:"labor_cost"`
	MaterialCost     float64    `json:"material_cost"`
	EquipmentCost    float64    `json:"equipment_cost"`
	OtherCost        float64    `json:"other_cost"`
}

// AssignTaskRequest represents the request to assign users to a task
type AssignTaskRequest struct {
	Assignments []TaskAssignmentData `json:"assignments"`
}

// TaskAssignmentData represents an assignment
type TaskAssignmentData struct {
	UserID     string     `json:"user_id"`
	UserName   string     `json:"user_name"`
	UserType   string     `json:"user_type"` // employee, contractor, supervisor
	Role       string     `json:"role"`      // worker, supervisor, manager, approver
	StartDate  *time.Time `json:"start_date"`
	EndDate    *time.Time `json:"end_date"`
	CanEdit    bool       `json:"can_edit"`
	CanApprove bool       `json:"can_approve"`
	Notes      string     `json:"notes"`
}

// UpdateTaskStatusRequest represents the request to update task status
type UpdateTaskStatusRequest struct {
	Status          string     `json:"status"`
	Comment         string     `json:"comment"`
	ActualStartDate *time.Time `json:"actual_start_date"`
	ActualEndDate   *time.Time `json:"actual_end_date"`
}

// CreateTask creates a new task
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Code == "" || req.Title == "" {
		http.Error(w, "Code and Title are required", http.StatusBadRequest)
		return
	}

	// Validate nodes exist and belong to project
	var startNode, stopNode models.Node
	if err := h.db.First(&startNode, "id = ? AND project_id = ?", req.StartNodeID, req.ProjectID).Error; err != nil {
		http.Error(w, "Invalid start node", http.StatusBadRequest)
		return
	}
	if err := h.db.First(&stopNode, "id = ? AND project_id = ?", req.StopNodeID, req.ProjectID).Error; err != nil {
		http.Error(w, "Invalid stop node", http.StatusBadRequest)
		return
	}

	// Get user from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	// Prepare metadata
	metadataJSON, _ := json.Marshal(req.Metadata)

	// Create task
	task := models.Tasks{
		Code:             req.Code,
		Title:            req.Title,
		Description:      req.Description,
		ProjectID:        req.ProjectID,
		ZoneID:           req.ZoneID,
		StartNodeID:      req.StartNodeID,
		StopNodeID:       req.StopNodeID,
		PlannedStartDate: req.PlannedStartDate,
		PlannedEndDate:   req.PlannedEndDate,
		AllocatedBudget:  req.AllocatedBudget,
		Priority:         req.Priority,
		WorkflowID:       req.WorkflowID,
		Status:           "pending",
		Progress:         0,
		Metadata:         json.RawMessage(metadataJSON),
		CreatedBy:        claims.UserID,
	}

	// Set default priority if not provided
	if task.Priority == "" {
		task.Priority = "medium"
	}

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&task).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to create task: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// Update node statuses to allocated
	tx.Model(&models.Node{}).Where("id IN ?", []uuid.UUID{req.StartNodeID, req.StopNodeID}).Update("status", "allocated")

	// Create audit log
	auditLog := models.TaskAuditLog{
		TaskID:          task.ID,
		Action:          "created",
		PerformedBy:     claims.UserID,
		PerformedByName: user.Name,
		PerformedAt:     time.Now(),
	}
	tx.Create(&auditLog)

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Created task: %s (ID: %s)", task.Title, task.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Task created successfully",
		"task":    task,
	})
}

// AssignTask assigns users to a task
func (h *TaskHandler) AssignTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req AssignTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Assignments) == 0 {
		http.Error(w, "At least one assignment is required", http.StatusBadRequest)
		return
	}

	// Get task
	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get user from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create assignments
	now := time.Now()
	for _, assignmentData := range req.Assignments {
		assignment := models.TaskAssignment{
			TaskID:     task.ID,
			UserID:     assignmentData.UserID,
			UserName:   assignmentData.UserName,
			UserType:   assignmentData.UserType,
			Role:       assignmentData.Role,
			AssignedBy: claims.UserID,
			AssignedAt: now,
			StartDate:  assignmentData.StartDate,
			EndDate:    assignmentData.EndDate,
			Status:     "active",
			IsActive:   true,
			CanEdit:    assignmentData.CanEdit,
			CanApprove: assignmentData.CanApprove,
			Notes:      assignmentData.Notes,
		}

		if err := tx.Create(&assignment).Error; err != nil {
			tx.Rollback()
			log.Printf("❌ Failed to create assignment: %v", err)
			http.Error(w, "Failed to create assignments", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditLog := models.TaskAuditLog{
			TaskID:          task.ID,
			Action:          "assigned",
			PerformedBy:     claims.UserID,
			PerformedByName: user.Name,
			NewValue:        fmt.Sprintf("%s (%s) as %s", assignmentData.UserName, assignmentData.UserID, assignmentData.Role),
			PerformedAt:     time.Now(),
		}
		tx.Create(&auditLog)
	}

	// Update task status to assigned if it was pending
	if task.Status == "pending" {
		task.Status = "assigned"
		task.UpdatedBy = claims.UserID
		tx.Save(&task)

		// Create status change audit log
		auditLog := models.TaskAuditLog{
			TaskID:          task.ID,
			Action:          "status_changed",
			Field:           "status",
			OldValue:        "pending",
			NewValue:        "assigned",
			PerformedBy:     claims.UserID,
			PerformedByName: user.Name,
			PerformedAt:     time.Now(),
		}
		tx.Create(&auditLog)
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Assigned %d users to task: %s", len(req.Assignments), taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":           "Task assigned successfully",
		"assignments_count": len(req.Assignments),
	})
}

// UpdateTaskStatus updates the task status
func (h *TaskHandler) UpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req UpdateTaskStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Status == "" {
		http.Error(w, "Status is required", http.StatusBadRequest)
		return
	}

	// Get task
	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get user from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	oldStatus := task.Status

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update task
	task.Status = req.Status
	task.UpdatedBy = claims.UserID

	// Update dates based on status
	if req.Status == "in-progress" && task.ActualStartDate == nil {
		now := time.Now()
		task.ActualStartDate = &now
		if req.ActualStartDate != nil {
			task.ActualStartDate = req.ActualStartDate
		}
	}
	if req.Status == "completed" && task.ActualEndDate == nil {
		now := time.Now()
		task.ActualEndDate = &now
		if req.ActualEndDate != nil {
			task.ActualEndDate = req.ActualEndDate
		}
		task.Progress = 100
	}

	if err := tx.Save(&task).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	// Update node statuses
	if req.Status == "in-progress" {
		tx.Model(&models.Node{}).Where("id IN ?", []uuid.UUID{task.StartNodeID, task.StopNodeID}).Update("status", "in-progress")
	} else if req.Status == "completed" {
		tx.Model(&models.Node{}).Where("id IN ?", []uuid.UUID{task.StartNodeID, task.StopNodeID}).Update("status", "completed")
	}

	// Create audit log
	auditLog := models.TaskAuditLog{
		TaskID:          task.ID,
		Action:          "status_changed",
		Field:           "status",
		OldValue:        oldStatus,
		NewValue:        req.Status,
		Comment:         req.Comment,
		PerformedBy:     claims.UserID,
		PerformedByName: user.Name,
		PerformedAt:     time.Now(),
	}
	tx.Create(&auditLog)

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated task status: %s -> %s (Task: %s)", oldStatus, req.Status, taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Task status updated successfully",
		"task":    task,
	})
}

// GetTask retrieves a task by ID
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var task models.Task
	if err := h.db.
		Preload("Project").
		Preload("Zone").
		Preload("StartNode").
		Preload("StopNode").
		Preload("Assignments").
		Preload("Comments").
		Preload("Attachments").
		First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// ListTasks lists all tasks with filters
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	var tasks []models.Task

	query := h.db.
		Preload("Project").
		Preload("StartNode").
		Preload("StopNode")

	// Apply filters
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if priority := r.URL.Query().Get("priority"); priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if assignedTo := r.URL.Query().Get("assigned_to"); assignedTo != "" {
		query = query.Joins("JOIN task_assignments ON task_assignments.task_id = tasks.id").
			Where("task_assignments.user_id = ? AND task_assignments.is_active = ?", assignedTo, true)
	}

	if err := query.Order("created_at DESC").Find(&tasks).Error; err != nil {
		http.Error(w, "Failed to fetch tasks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// UpdateTask updates a task
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get user from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	// Track changes for audit
	changes := []models.TaskAuditLog{}

	// Update fields and track changes
	if req.Title != "" && req.Title != task.Title {
		changes = append(changes, models.TaskAuditLog{
			TaskID: task.ID, Action: "updated", Field: "title",
			OldValue: task.Title, NewValue: req.Title,
			PerformedBy: claims.UserID, PerformedByName: user.Name, PerformedAt: time.Now(),
		})
		task.Title = req.Title
	}
	if req.Description != "" && req.Description != task.Description {
		task.Description = req.Description
	}
	if req.PlannedStartDate != nil {
		task.PlannedStartDate = req.PlannedStartDate
	}
	if req.PlannedEndDate != nil {
		task.PlannedEndDate = req.PlannedEndDate
	}
	if req.AllocatedBudget > 0 {
		task.AllocatedBudget = req.AllocatedBudget
	}
	if req.Priority != "" {
		task.Priority = req.Priority
	}
	if req.Progress >= 0 {
		task.Progress = req.Progress
	}

	// Update costs and recalculate total
	if req.LaborCost >= 0 {
		task.LaborCost = req.LaborCost
	}
	if req.MaterialCost >= 0 {
		task.MaterialCost = req.MaterialCost
	}
	if req.EquipmentCost >= 0 {
		task.EquipmentCost = req.EquipmentCost
	}
	if req.OtherCost >= 0 {
		task.OtherCost = req.OtherCost
	}
	task.TotalCost = task.LaborCost + task.MaterialCost + task.EquipmentCost + task.OtherCost

	task.UpdatedBy = claims.UserID

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Save(&task).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	// Create audit logs
	for _, change := range changes {
		tx.Create(&change)
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated task: %s", taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Task updated successfully",
		"task":    task,
	})
}

// GetTaskAuditLog retrieves the audit log for a task
func (h *TaskHandler) GetTaskAuditLog(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var auditLogs []models.TaskAuditLog
	if err := h.db.Where("task_id = ?", taskID).Order("performed_at DESC").Find(&auditLogs).Error; err != nil {
		http.Error(w, "Failed to fetch audit logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"audit_logs": auditLogs,
		"count":      len(auditLogs),
	})
}

// AddTaskComment adds a comment to a task
func (h *TaskHandler) AddTaskComment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var req struct {
		Comment     string     `json:"comment"`
		CommentType string     `json:"comment_type"`
		ParentID    *uuid.UUID `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Comment == "" {
		http.Error(w, "Comment is required", http.StatusBadRequest)
		return
	}

	// Verify task exists
	var task models.Task
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get user from context
	claims := middleware.GetClaims(r)
	user := middleware.GetUser(r)

	comment := models.TaskComment{
		TaskID:      task.ID,
		Comment:     req.Comment,
		CommentType: req.CommentType,
		AuthorID:    claims.UserID,
		AuthorName:  user.Name,
		ParentID:    req.ParentID,
	}

	if comment.CommentType == "" {
		comment.CommentType = "general"
	}

	if err := h.db.Create(&comment).Error; err != nil {
		http.Error(w, "Failed to add comment", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Added comment to task: %s", taskID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Comment added successfully",
		"comment": comment,
	})
}

// GetTaskComments retrieves comments for a task
func (h *TaskHandler) GetTaskComments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var comments []models.TaskComment
	if err := h.db.Where("task_id = ?", taskID).Order("created_at DESC").Find(&comments).Error; err != nil {
		http.Error(w, "Failed to fetch comments", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"comments": comments,
		"count":    len(comments),
	})
}
