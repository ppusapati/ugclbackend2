package handlers

import (
	"log"
	"net/http"
	"encoding/json"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// BudgetHandler handles budget management operations
type BudgetHandler struct {
	db *gorm.DB
}

// NewBudgetHandler creates a new budget handler
func NewBudgetHandler() *BudgetHandler {
	return &BudgetHandler{
		db: config.DB,
	}
}

// CreateBudgetAllocationRequest represents the request to create a budget allocation
type CreateBudgetAllocationRequest struct {
	ProjectID      *uuid.UUID `json:"project_id"`
	TaskID         *uuid.UUID `json:"task_id"`
	Category       string     `json:"category"`
	Description    string     `json:"description"`
	PlannedAmount  float64    `json:"planned_amount"`
	Currency       string     `json:"currency"`
	AllocationDate *time.Time `json:"allocation_date"`
	StartDate      *time.Time `json:"start_date"`
	EndDate        *time.Time `json:"end_date"`
	Notes          string     `json:"notes"`
}

// UpdateBudgetAllocationRequest represents the request to update a budget allocation
type UpdateBudgetAllocationRequest struct {
	PlannedAmount float64 `json:"planned_amount"`
	ActualAmount  float64 `json:"actual_amount"`
	Status        string  `json:"status"`
	Notes         string  `json:"notes"`
}

// ApproveBudgetRequest represents the request to approve a budget allocation
type ApproveBudgetRequest struct {
	ApprovalComment string `json:"approval_comment"`
}

// CreateBudgetAllocation creates a new budget allocation
func (h *BudgetHandler) CreateBudgetAllocation(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetAllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate that either project_id or task_id is provided (not both)
	if (req.ProjectID == nil && req.TaskID == nil) || (req.ProjectID != nil && req.TaskID != nil) {
		http.Error(w, "Provide either project_id or task_id, not both", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Category == "" || req.PlannedAmount <= 0 {
		http.Error(w, "Category and PlannedAmount are required", http.StatusBadRequest)
		return
	}

	// Validate category
	validCategories := []string{"labor", "material", "equipment", "overhead", "contingency"}
	isValidCategory := false
	for _, cat := range validCategories {
		if req.Category == cat {
			isValidCategory = true
			break
		}
	}
	if !isValidCategory {
		http.Error(w, "Invalid category. Must be one of: labor, material, equipment, overhead, contingency", http.StatusBadRequest)
		return
	}

	// Validate project or task exists
	if req.ProjectID != nil {
		var project models.Project
		if err := h.db.First(&project, "id = ?", req.ProjectID).Error; err != nil {
			http.Error(w, "Project not found", http.StatusBadRequest)
			return
		}
	}
	if req.TaskID != nil {
		var task models.Tasks
		if err := h.db.First(&task, "id = ?", req.TaskID).Error; err != nil {
			http.Error(w, "Task not found", http.StatusBadRequest)
			return
		}
	}

	// Get user from context
	claims := middleware.GetClaims(r)

	// Set default allocation date if not provided
	allocationDate := time.Now()
	if req.AllocationDate != nil {
		allocationDate = *req.AllocationDate
	}

	// Set default currency
	currency := "INR"
	if req.Currency != "" {
		currency = req.Currency
	}

	// Create budget allocation
	allocation := models.BudgetAllocation{
		ProjectID:      req.ProjectID,
		TaskID:         req.TaskID,
		Category:       req.Category,
		Description:    req.Description,
		PlannedAmount:  req.PlannedAmount,
		ActualAmount:   0,
		Currency:       currency,
		AllocationDate: allocationDate,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		Status:         "allocated",
		Notes:          req.Notes,
		CreatedBy:      claims.UserID,
	}

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&allocation).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to create budget allocation: %v", err)
		http.Error(w, "Failed to create budget allocation", http.StatusInternalServerError)
		return
	}

	// Update project or task allocated budget
	if req.ProjectID != nil {
		tx.Model(&models.Project{}).
			Where("id = ?", req.ProjectID).
			Update("allocated_budget", gorm.Expr("allocated_budget + ?", req.PlannedAmount))
	} else if req.TaskID != nil {
		tx.Model(&models.Tasks{}).
			Where("id = ?", req.TaskID).
			Update("allocated_budget", gorm.Expr("allocated_budget + ?", req.PlannedAmount))
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Created budget allocation: %s (Amount: %.2f)", allocation.ID, allocation.PlannedAmount)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Budget allocation created successfully",
		"allocation": allocation,
	})
}

// GetBudgetAllocation retrieves a budget allocation by ID
func (h *BudgetHandler) GetBudgetAllocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	allocationID := vars["id"]

	var allocation models.BudgetAllocation
	if err := h.db.
		Preload("Project").
		Preload("Task").
		First(&allocation, "id = ?", allocationID).Error; err != nil {
		http.Error(w, "Budget allocation not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allocation)
}

// ListBudgetAllocations lists all budget allocations with filters
func (h *BudgetHandler) ListBudgetAllocations(w http.ResponseWriter, r *http.Request) {
	var allocations []models.BudgetAllocation

	query := h.db.Preload("Project").Preload("Task")

	// Apply filters
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if taskID := r.URL.Query().Get("task_id"); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if category := r.URL.Query().Get("category"); category != "" {
		query = query.Where("category = ?", category)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("allocation_date DESC").Find(&allocations).Error; err != nil {
		http.Error(w, "Failed to fetch budget allocations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"allocations": allocations,
		"count":       len(allocations),
	})
}

// UpdateBudgetAllocation updates a budget allocation
func (h *BudgetHandler) UpdateBudgetAllocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	allocationID := vars["id"]

	var req UpdateBudgetAllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var allocation models.BudgetAllocation
	if err := h.db.First(&allocation, "id = ?", allocationID).Error; err != nil {
		http.Error(w, "Budget allocation not found", http.StatusNotFound)
		return
	}

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	oldPlannedAmount := allocation.PlannedAmount
	oldActualAmount := allocation.ActualAmount

	// Update fields
	if req.PlannedAmount > 0 {
		allocation.PlannedAmount = req.PlannedAmount
	}
	if req.ActualAmount >= 0 {
		allocation.ActualAmount = req.ActualAmount
	}
	if req.Status != "" {
		allocation.Status = req.Status
	}
	if req.Notes != "" {
		allocation.Notes = req.Notes
	}

	if err := tx.Save(&allocation).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to update budget allocation", http.StatusInternalServerError)
		return
	}

	// Update project or task budgets if amounts changed
	if req.PlannedAmount > 0 && oldPlannedAmount != req.PlannedAmount {
		diff := req.PlannedAmount - oldPlannedAmount
		if allocation.ProjectID != nil {
			tx.Model(&models.Project{}).
				Where("id = ?", allocation.ProjectID).
				Update("allocated_budget", gorm.Expr("allocated_budget + ?", diff))
		} else if allocation.TaskID != nil {
			tx.Model(&models.Tasks{}).
				Where("id = ?", allocation.TaskID).
				Update("allocated_budget", gorm.Expr("allocated_budget + ?", diff))
		}
	}

	if req.ActualAmount >= 0 && oldActualAmount != req.ActualAmount {
		diff := req.ActualAmount - oldActualAmount
		if allocation.ProjectID != nil {
			tx.Model(&models.Project{}).
				Where("id = ?", allocation.ProjectID).
				Update("spent_budget", gorm.Expr("spent_budget + ?", diff))
		} else if allocation.TaskID != nil {
			// Update task costs based on category
			updateField := ""
			switch allocation.Category {
			case "labor":
				updateField = "labor_cost"
			case "material":
				updateField = "material_cost"
			case "equipment":
				updateField = "equipment_cost"
			default:
				updateField = "other_cost"
			}
			tx.Model(&models.Tasks{}).
				Where("id = ?", allocation.TaskID).
				Update(updateField, gorm.Expr(updateField+" + ?", diff))

			// Recalculate total cost
			tx.Model(&models.Tasks{}).
				Where("id = ?", allocation.TaskID).
				Update("total_cost", gorm.Expr("labor_cost + material_cost + equipment_cost + other_cost"))
		}
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated budget allocation: %s", allocationID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Budget allocation updated successfully",
		"allocation": allocation,
	})
}

// ApproveBudgetAllocation approves a budget allocation
func (h *BudgetHandler) ApproveBudgetAllocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	allocationID := vars["id"]

	var req ApproveBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var allocation models.BudgetAllocation
	if err := h.db.First(&allocation, "id = ?", allocationID).Error; err != nil {
		http.Error(w, "Budget allocation not found", http.StatusNotFound)
		return
	}

	// Get user from context
	claims := middleware.GetClaims(r)

	now := time.Now()
	allocation.ApprovedBy = claims.UserID
	allocation.ApprovedAt = &now
	if req.ApprovalComment != "" {
		allocation.Notes = allocation.Notes + "\n[Approval] " + req.ApprovalComment
	}

	if err := h.db.Save(&allocation).Error; err != nil {
		http.Error(w, "Failed to approve budget allocation", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Approved budget allocation: %s by %s", allocationID, claims.UserID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Budget allocation approved successfully",
		"allocation": allocation,
	})
}

// GetProjectBudgetSummary retrieves budget summary for a project
func (h *BudgetHandler) GetProjectBudgetSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var project models.Project
	if err := h.db.First(&project, "id = ?", projectID).Error; err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	// Get allocations by category
	var categoryBreakdown []struct {
		Category      string  `json:"category"`
		PlannedAmount float64 `json:"planned_amount"`
		ActualAmount  float64 `json:"actual_amount"`
	}
	h.db.Model(&models.BudgetAllocation{}).
		Select("category, COALESCE(SUM(planned_amount), 0) as planned_amount, COALESCE(SUM(actual_amount), 0) as actual_amount").
		Where("project_id = ?", projectID).
		Group("category").
		Scan(&categoryBreakdown)

	// Get task-level budgets
	var taskBudgets []struct {
		TaskID          uuid.UUID `json:"task_id"`
		TaskTitle       string    `json:"task_title"`
		AllocatedBudget float64   `json:"allocated_budget"`
		TotalCost       float64   `json:"total_cost"`
	}
	h.db.Table("tasks").
		Select("tasks.id as task_id, tasks.title as task_title, tasks.allocated_budget, tasks.total_cost").
		Where("tasks.project_id = ?", projectID).
		Scan(&taskBudgets)

	// Calculate totals
	var totalPlanned, totalActual float64
	for _, cat := range categoryBreakdown {
		totalPlanned += cat.PlannedAmount
		totalActual += cat.ActualAmount
	}

	var budgetUtilization float64
	if project.TotalBudget > 0 {
		budgetUtilization = (project.SpentBudget / project.TotalBudget) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"project_id":         projectID,
		"total_budget":       project.TotalBudget,
		"allocated_budget":   project.AllocatedBudget,
		"spent_budget":       project.SpentBudget,
		"remaining_budget":   project.TotalBudget - project.SpentBudget,
		"budget_utilization": budgetUtilization,
		"category_breakdown": categoryBreakdown,
		"task_budgets":       taskBudgets,
		"total_planned":      totalPlanned,
		"total_actual":       totalActual,
		"variance":           totalPlanned - totalActual,
	})
}

// GetTaskBudgetSummary retrieves budget summary for a task
func (h *BudgetHandler) GetTaskBudgetSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	var task models.Tasks
	if err := h.db.First(&task, "id = ?", taskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get allocations by category
	var categoryBreakdown []struct {
		Category      string  `json:"category"`
		PlannedAmount float64 `json:"planned_amount"`
		ActualAmount  float64 `json:"actual_amount"`
	}
	h.db.Model(&models.BudgetAllocation{}).
		Select("category, COALESCE(SUM(planned_amount), 0) as planned_amount, COALESCE(SUM(actual_amount), 0) as actual_amount").
		Where("task_id = ?", taskID).
		Group("category").
		Scan(&categoryBreakdown)

	var budgetUtilization float64
	if task.AllocatedBudget > 0 {
		budgetUtilization = (task.TotalCost / task.AllocatedBudget) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id":            taskID,
		"allocated_budget":   task.AllocatedBudget,
		"labor_cost":         task.LaborCost,
		"material_cost":      task.MaterialCost,
		"equipment_cost":     task.EquipmentCost,
		"other_cost":         task.OtherCost,
		"total_cost":         task.TotalCost,
		"remaining_budget":   task.AllocatedBudget - task.TotalCost,
		"budget_utilization": budgetUtilization,
		"category_breakdown": categoryBreakdown,
	})
}

// DeleteBudgetAllocation soft deletes a budget allocation
func (h *BudgetHandler) DeleteBudgetAllocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	allocationID := vars["id"]

	var allocation models.BudgetAllocation
	if err := h.db.First(&allocation, "id = ?", allocationID).Error; err != nil {
		http.Error(w, "Budget allocation not found", http.StatusNotFound)
		return
	}

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update project or task budget
	if allocation.ProjectID != nil {
		tx.Model(&models.Project{}).
			Where("id = ?", allocation.ProjectID).
			Update("allocated_budget", gorm.Expr("allocated_budget - ?", allocation.PlannedAmount))
		tx.Model(&models.Project{}).
			Where("id = ?", allocation.ProjectID).
			Update("spent_budget", gorm.Expr("spent_budget - ?", allocation.ActualAmount))
	}

	if err := tx.Delete(&allocation).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete budget allocation", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Deleted budget allocation: %s", allocationID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Budget allocation deleted successfully",
	})
}
