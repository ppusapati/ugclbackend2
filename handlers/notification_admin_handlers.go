package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// NotificationAdminHandler handles admin operations for notifications
type NotificationAdminHandler struct{}

// GetAllNotificationRules retrieves all notification rules
func (h *NotificationAdminHandler) GetAllNotificationRules(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	query := config.DB.Model(&models.NotificationRule{})

	// Filter by workflow ID
	if workflowID := r.URL.Query().Get("workflow_id"); workflowID != "" {
		if wfUUID, err := uuid.Parse(workflowID); err == nil {
			query = query.Where("workflow_id = ?", wfUUID)
		}
	}

	// Filter by active status
	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		query = query.Where("is_active = ?", isActive == "true")
	}

	var rules []models.NotificationRule
	if err := query.Preload("Recipients").Order("created_at DESC").Find(&rules).Error; err != nil {
		http.Error(w, "Failed to fetch notification rules", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
		"count": len(rules),
	})
}

// GetNotificationRule retrieves a single notification rule by ID
func (h *NotificationAdminHandler) GetNotificationRule(w http.ResponseWriter, r *http.Request) {
	// Get rule ID from URL
	vars := mux.Vars(r)
	ruleID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	var rule models.NotificationRule
	if err := config.DB.
		Preload("Recipients").
		First(&rule, "id = ?", ruleID).Error; err != nil {
		http.Error(w, "Notification rule not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// CreateNotificationRule creates a new notification rule
func (h *NotificationAdminHandler) CreateNotificationRule(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req struct {
		Code             string                         `json:"code"`
		Name             string                         `json:"name"`
		Description      string                         `json:"description"`
		WorkflowID       *uuid.UUID                     `json:"workflow_id"`
		TriggerOnStates  []string                       `json:"trigger_on_states"`
		TriggerOnActions []string                       `json:"trigger_on_actions"`
		TitleTemplate    string                         `json:"title_template"`
		BodyTemplate     string                         `json:"body_template"`
		Priority         models.NotificationPriority    `json:"priority"`
		Channels         []models.NotificationChannel   `json:"channels"`
		Recipients       []models.NotificationRecipient `json:"recipients"`
		Conditions       map[string]interface{}         `json:"conditions"`
		IsActive         bool                           `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Code == "" || req.Name == "" {
		http.Error(w, "Code and name are required", http.StatusBadRequest)
		return
	}
	if req.TitleTemplate == "" {
		http.Error(w, "Title template is required", http.StatusBadRequest)
		return
	}

	// Convert conditions to JSONMap
	var conditionsJSON models.JSONMap
	if req.Conditions != nil {
		conditionsJSON = req.Conditions
	}

	// Convert arrays to StringArray
	var triggerStates models.StringArray = req.TriggerOnStates
	var triggerActions models.StringArray = req.TriggerOnActions

	// Convert channels to ChannelArray
	var channels models.ChannelArray = req.Channels
	if len(channels) == 0 {
		channels = []models.NotificationChannel{models.NotificationChannelInApp}
	}

	// Create notification rule
	rule := models.NotificationRule{
		Code:             req.Code,
		Name:             req.Name,
		Description:      req.Description,
		WorkflowID:       req.WorkflowID,
		TriggerOnStates:  triggerStates,
		TriggerOnActions: triggerActions,
		TitleTemplate:    req.TitleTemplate,
		BodyTemplate:     req.BodyTemplate,
		Priority:         req.Priority,
		Channels:         channels,
		Conditions:       conditionsJSON,
		IsActive:         req.IsActive,
		CreatedBy:        userID,
	}

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create rule
	if err := tx.Create(&rule).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to create notification rule: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create recipients
	for _, recipient := range req.Recipients {
		recipient.NotificationRuleID = rule.ID
		if err := tx.Create(&recipient).Error; err != nil {
			tx.Rollback()
			http.Error(w, "Failed to create recipients", http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// Load rule with recipients
	config.DB.Preload("Recipients").First(&rule, rule.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// UpdateNotificationRule updates an existing notification rule
func (h *NotificationAdminHandler) UpdateNotificationRule(w http.ResponseWriter, r *http.Request) {
	// Get rule ID from URL
	vars := mux.Vars(r)
	ruleID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	// Get existing rule
	var rule models.NotificationRule
	if err := config.DB.First(&rule, "id = ?", ruleID).Error; err != nil {
		http.Error(w, "Notification rule not found", http.StatusNotFound)
		return
	}

	// Parse request body
	var req struct {
		Code             string                         `json:"code"`
		Name             string                         `json:"name"`
		Description      string                         `json:"description"`
		WorkflowID       *uuid.UUID                     `json:"workflow_id"`
		TriggerOnStates  []string                       `json:"trigger_on_states"`
		TriggerOnActions []string                       `json:"trigger_on_actions"`
		TitleTemplate    string                         `json:"title_template"`
		BodyTemplate     string                         `json:"body_template"`
		Priority         models.NotificationPriority    `json:"priority"`
		Channels         []models.NotificationChannel   `json:"channels"`
		Recipients       []models.NotificationRecipient `json:"recipients"`
		Conditions       map[string]interface{}         `json:"conditions"`
		IsActive         bool                           `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields
	rule.Code = req.Code
	rule.Name = req.Name
	rule.Description = req.Description
	rule.WorkflowID = req.WorkflowID
	rule.TriggerOnStates = req.TriggerOnStates
	rule.TriggerOnActions = req.TriggerOnActions
	rule.TitleTemplate = req.TitleTemplate
	rule.BodyTemplate = req.BodyTemplate
	rule.Priority = req.Priority
	rule.Channels = req.Channels
	rule.IsActive = req.IsActive
	rule.UpdatedAt = time.Now()

	// Convert conditions to JSONMap
	if req.Conditions != nil {
		rule.Conditions = req.Conditions
	}

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update rule
	if err := tx.Save(&rule).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to update notification rule", http.StatusInternalServerError)
		return
	}

	// Delete existing recipients
	if err := tx.Where("notification_rule_id = ?", rule.ID).Delete(&models.NotificationRecipient{}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to update recipients", http.StatusInternalServerError)
		return
	}

	// Create new recipients
	for _, recipient := range req.Recipients {
		recipient.NotificationRuleID = rule.ID
		recipient.ID = uuid.New()
		if err := tx.Create(&recipient).Error; err != nil {
			tx.Rollback()
			http.Error(w, "Failed to create recipients", http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// Load rule with recipients
	config.DB.Preload("Recipients").First(&rule, rule.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// DeleteNotificationRule deletes a notification rule
func (h *NotificationAdminHandler) DeleteNotificationRule(w http.ResponseWriter, r *http.Request) {
	// Get rule ID from URL
	vars := mux.Vars(r)
	ruleID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	// Check if rule exists
	var rule models.NotificationRule
	if err := config.DB.First(&rule, "id = ?", ruleID).Error; err != nil {
		http.Error(w, "Notification rule not found", http.StatusNotFound)
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete recipients first (foreign key constraint)
	if err := tx.Where("notification_rule_id = ?", ruleID).Delete(&models.NotificationRecipient{}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete recipients", http.StatusInternalServerError)
		return
	}

	// Delete rule
	if err := tx.Delete(&rule).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to delete notification rule", http.StatusInternalServerError)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Notification rule deleted successfully",
	})
}

// ToggleNotificationRule toggles a notification rule's active status
func (h *NotificationAdminHandler) ToggleNotificationRule(w http.ResponseWriter, r *http.Request) {
	// Get rule ID from URL
	vars := mux.Vars(r)
	ruleID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}

	// Get existing rule
	var rule models.NotificationRule
	if err := config.DB.First(&rule, "id = ?", ruleID).Error; err != nil {
		http.Error(w, "Notification rule not found", http.StatusNotFound)
		return
	}

	// Toggle active status
	rule.IsActive = !rule.IsActive
	rule.UpdatedAt = time.Now()

	if err := config.DB.Save(&rule).Error; err != nil {
		http.Error(w, "Failed to update notification rule", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// GetNotificationStats returns statistics about notifications
func (h *NotificationAdminHandler) GetNotificationStats(w http.ResponseWriter, r *http.Request) {
	type Stats struct {
		TotalRules         int64 `json:"total_rules"`
		ActiveRules        int64 `json:"active_rules"`
		TotalNotifications int64 `json:"total_notifications"`
		PendingCount       int64 `json:"pending_count"`
		SentCount          int64 `json:"sent_count"`
		ReadCount          int64 `json:"read_count"`
		FailedCount        int64 `json:"failed_count"`
	}

	var stats Stats

	// Count rules
	config.DB.Model(&models.NotificationRule{}).Count(&stats.TotalRules)
	config.DB.Model(&models.NotificationRule{}).Where("is_active = ?", true).Count(&stats.ActiveRules)

	// Count notifications
	config.DB.Model(&models.Notification{}).Count(&stats.TotalNotifications)
	config.DB.Model(&models.Notification{}).Where("status = ?", models.NotificationStatusPending).Count(&stats.PendingCount)
	config.DB.Model(&models.Notification{}).Where("status = ?", models.NotificationStatusSent).Count(&stats.SentCount)
	config.DB.Model(&models.Notification{}).Where("read_at IS NOT NULL").Count(&stats.ReadCount)
	config.DB.Model(&models.Notification{}).Where("status = ?", models.NotificationStatusFailed).Count(&stats.FailedCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
