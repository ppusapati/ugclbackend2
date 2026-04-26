package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"
)

type documentWorkflowTransitionRequest struct {
	Action   string                 `json:"action"`
	Comment  string                 `json:"comment"`
	Metadata map[string]interface{} `json:"metadata"`
}

type documentWorkflowHistoryItem struct {
	ID             uuid.UUID              `json:"id"`
	FromState      string                 `json:"from_state"`
	ToState        string                 `json:"to_state"`
	Action         string                 `json:"action"`
	Comment        string                 `json:"comment,omitempty"`
	ActorID        *uuid.UUID             `json:"actor_id,omitempty"`
	ActorName      string                 `json:"actor_name,omitempty"`
	TransitionedAt time.Time              `json:"transitioned_at"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type documentWorkflowResponse struct {
	DocumentID       uuid.UUID                     `json:"document_id"`
	WorkflowID       *uuid.UUID                    `json:"workflow_id,omitempty"`
	CurrentState     string                        `json:"current_state"`
	Status           models.DocumentStatus         `json:"status"`
	AvailableActions []models.WorkflowAction       `json:"available_actions"`
	History          []documentWorkflowHistoryItem `json:"history"`
}

type documentWorkflowOption struct {
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	InitialState string    `json:"initial_state"`
}

const (
	documentWorkflowPermissionReview  = "document:review"
	documentWorkflowPermissionApprove = "document:approve"
	documentWorkflowPermissionReject  = "document:reject"
	documentWorkflowPermissionRevise  = "document:revise"
	legacyDocumentWorkflowPermission  = "document:update"
)

func defaultDocumentWorkflowDefinition() *models.WorkflowDefinition {
	transitions := []models.WorkflowTransitionDef{
		{
			From:       "draft",
			To:         "submitted_for_review",
			Action:     "submit",
			Label:      "Submit for Review",
			Permission: documentWorkflowPermissionReview,
			Notifications: []models.TransitionNotification{
				{
					Recipients:    []models.NotificationRecipientDef{{Type: "permission", PermissionCode: documentWorkflowPermissionReview}},
					TitleTemplate: "Document {{.FormTitle}} submitted for review",
					BodyTemplate:  "{{.ApproverName}} submitted the document for review. Current state: {{.CurrentState}}.",
					Priority:      "normal",
					Channels:      []string{"in_app"},
				},
			},
		},
		{From: "submitted_for_review", To: "under_review", Action: "start_review", Label: "Start Review", Permission: documentWorkflowPermissionReview},
		{
			From:            "under_review",
			To:              "needs_revision",
			Action:          "request_revision",
			Label:           "Request Revision",
			Permission:      documentWorkflowPermissionRevise,
			RequiresComment: true,
			Notifications: []models.TransitionNotification{
				{
					Recipients:    []models.NotificationRecipientDef{{Type: "submitter"}},
					TitleTemplate: "Revision requested for {{.FormTitle}}",
					BodyTemplate:  "{{.ApproverName}} requested revisions. Comment: {{.Comment}}",
					Priority:      "high",
					Channels:      []string{"in_app"},
				},
			},
		},
		{
			From:       "under_review",
			To:         "approved",
			Action:     "approve",
			Label:      "Approve",
			Permission: documentWorkflowPermissionApprove,
			Notifications: []models.TransitionNotification{
				{
					Recipients:    []models.NotificationRecipientDef{{Type: "submitter"}},
					TitleTemplate: "Document {{.FormTitle}} approved",
					BodyTemplate:  "{{.ApproverName}} approved the document.",
					Priority:      "normal",
					Channels:      []string{"in_app"},
				},
			},
		},
		{
			From:            "under_review",
			To:              "rejected",
			Action:          "reject",
			Label:           "Reject",
			Permission:      documentWorkflowPermissionReject,
			RequiresComment: true,
			Notifications: []models.TransitionNotification{
				{
					Recipients:    []models.NotificationRecipientDef{{Type: "submitter"}},
					TitleTemplate: "Document {{.FormTitle}} rejected",
					BodyTemplate:  "{{.ApproverName}} rejected the document. Comment: {{.Comment}}",
					Priority:      "high",
					Channels:      []string{"in_app"},
				},
			},
		},
		{
			From:       "needs_revision",
			To:         "resubmitted",
			Action:     "resubmit",
			Label:      "Resubmit",
			Permission: documentWorkflowPermissionRevise,
			Notifications: []models.TransitionNotification{
				{
					Recipients:    []models.NotificationRecipientDef{{Type: "permission", PermissionCode: documentWorkflowPermissionReview}},
					TitleTemplate: "Document {{.FormTitle}} resubmitted",
					BodyTemplate:  "{{.ApproverName}} resubmitted document for review.",
					Priority:      "normal",
					Channels:      []string{"in_app"},
				},
			},
		},
		{From: "resubmitted", To: "under_review", Action: "start_review", Label: "Start Review", Permission: documentWorkflowPermissionReview},
		{
			From:       "resubmitted",
			To:         "approved",
			Action:     "approve",
			Label:      "Approve",
			Permission: documentWorkflowPermissionApprove,
			Notifications: []models.TransitionNotification{
				{
					Recipients:    []models.NotificationRecipientDef{{Type: "submitter"}},
					TitleTemplate: "Document {{.FormTitle}} approved",
					BodyTemplate:  "{{.ApproverName}} approved the document.",
					Priority:      "normal",
					Channels:      []string{"in_app"},
				},
			},
		},
		{
			From:            "resubmitted",
			To:              "rejected",
			Action:          "reject",
			Label:           "Reject",
			Permission:      documentWorkflowPermissionReject,
			RequiresComment: true,
			Notifications: []models.TransitionNotification{
				{
					Recipients:    []models.NotificationRecipientDef{{Type: "submitter"}},
					TitleTemplate: "Document {{.FormTitle}} rejected",
					BodyTemplate:  "{{.ApproverName}} rejected the document. Comment: {{.Comment}}",
					Priority:      "high",
					Channels:      []string{"in_app"},
				},
			},
		},
		{
			From:            "resubmitted",
			To:              "needs_revision",
			Action:          "request_revision",
			Label:           "Request Revision",
			Permission:      documentWorkflowPermissionRevise,
			RequiresComment: true,
			Notifications: []models.TransitionNotification{
				{
					Recipients:    []models.NotificationRecipientDef{{Type: "submitter"}},
					TitleTemplate: "Revision requested for {{.FormTitle}}",
					BodyTemplate:  "{{.ApproverName}} requested revisions. Comment: {{.Comment}}",
					Priority:      "high",
					Channels:      []string{"in_app"},
				},
			},
		},
	}

	transitionsJSON, _ := json.Marshal(transitions)

	return &models.WorkflowDefinition{
		Code:         "document-review-default",
		Name:         "Document Review Default",
		InitialState: "draft",
		Transitions:  transitionsJSON,
		IsActive:     true,
	}
}

func mapDocumentStateToStatus(state string) models.DocumentStatus {
	switch state {
	case "approved":
		return models.DocumentStatusApproved
	case "rejected":
		return models.DocumentStatusRejected
	case "submitted_for_review", "under_review", "resubmitted":
		return models.DocumentStatusPending
	case "needs_revision", "draft":
		return models.DocumentStatusDraft
	default:
		return models.DocumentStatusPending
	}
}

func resolveDocumentWorkflowDefinition(document *models.Document) *models.WorkflowDefinition {
	if document.Workflow != nil && document.Workflow.IsActive {
		return document.Workflow
	}
	return defaultDocumentWorkflowDefinition()
}

func resolveInitialDocumentState(workflow *models.WorkflowDefinition) string {
	if workflow != nil && strings.TrimSpace(workflow.InitialState) != "" {
		return strings.TrimSpace(workflow.InitialState)
	}
	return "draft"
}

func userHasWorkflowPermission(userPermissions []string, requiredPermission string) bool {
	if strings.TrimSpace(requiredPermission) == "" {
		return true
	}

	fallbackPermission := ""
	switch requiredPermission {
	case documentWorkflowPermissionReview, documentWorkflowPermissionApprove, documentWorkflowPermissionReject, documentWorkflowPermissionRevise:
		fallbackPermission = legacyDocumentWorkflowPermission
	}

	for _, permission := range userPermissions {
		if permission == "admin_all" || permission == "*:*:*" || utils.MatchesPermission(permission, requiredPermission) {
			return true
		}

		if fallbackPermission != "" && utils.MatchesPermission(permission, fallbackPermission) {
			return true
		}
	}

	return false
}

func getAvailableDocumentWorkflowActions(document *models.Document, userPermissions []string) ([]models.WorkflowAction, []models.WorkflowTransitionDef, error) {
	workflowDef := resolveDocumentWorkflowDefinition(document)

	var transitions []models.WorkflowTransitionDef
	if err := json.Unmarshal(workflowDef.Transitions, &transitions); err != nil {
		return nil, nil, err
	}

	actions := make([]models.WorkflowAction, 0)
	for _, transition := range transitions {
		if transition.From != document.CurrentState {
			continue
		}

		if !userHasWorkflowPermission(userPermissions, transition.Permission) {
			continue
		}

		label := transition.Label
		if strings.TrimSpace(label) == "" {
			label = transition.Action
		}

		actions = append(actions, models.WorkflowAction{
			Action:          transition.Action,
			Label:           label,
			ToState:         transition.To,
			RequiresComment: transition.RequiresComment,
			Permission:      transition.Permission,
		})
	}

	return actions, transitions, nil
}

func getDocumentWorkflowHistory(documentID uuid.UUID) ([]documentWorkflowHistoryItem, error) {
	var logs []models.DocumentAuditLog
	if err := config.DB.
		Preload("User").
		Where("document_id = ? AND action = ?", documentID, models.DocumentAuditActionStatusChange).
		Order("created_at DESC").
		Find(&logs).Error; err != nil {
		return nil, err
	}

	history := make([]documentWorkflowHistoryItem, 0, len(logs))
	for _, log := range logs {
		item := documentWorkflowHistoryItem{
			ID:             log.ID,
			TransitionedAt: log.CreatedAt,
			Metadata:       map[string]interface{}{},
		}

		if log.UserID != nil {
			item.ActorID = log.UserID
		}
		if log.User != nil {
			item.ActorName = strings.TrimSpace(log.User.Name)
		}

		if log.Details != nil {
			for key, value := range log.Details {
				item.Metadata[key] = value
			}

			if fromState, ok := log.Details["from_state"].(string); ok {
				item.FromState = fromState
			}
			if toState, ok := log.Details["to_state"].(string); ok {
				item.ToState = toState
			}
			if action, ok := log.Details["workflow_action"].(string); ok {
				item.Action = action
			}
			if comment, ok := log.Details["comment"].(string); ok {
				item.Comment = comment
			}
		}

		history = append(history, item)
	}

	return history, nil
}

func ListDocumentWorkflowsHandler(w http.ResponseWriter, r *http.Request) {
	var workflows []models.WorkflowDefinition
	if err := config.DB.
		Where("is_active = ?", true).
		Order("name ASC").
		Find(&workflows).Error; err != nil {
		http.Error(w, "failed to fetch workflows: "+err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]documentWorkflowOption, 0, len(workflows))
	for _, workflow := range workflows {
		items = append(items, documentWorkflowOption{
			ID:           workflow.ID,
			Code:         workflow.Code,
			Name:         workflow.Name,
			Description:  workflow.Description,
			InitialState: workflow.InitialState,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workflows": items,
		"total":     len(items),
	})
}

func GetDocumentWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}

	var document models.Document
	if err := config.DB.Preload("Workflow").First(&document, "id = ?", documentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "document not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch document workflow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.TrimSpace(document.CurrentState) == "" {
		document.CurrentState = resolveInitialDocumentState(resolveDocumentWorkflowDefinition(&document))
	}

	userPermissions := middleware.GetEffectivePermissions(r)
	actions, _, err := getAvailableDocumentWorkflowActions(&document, userPermissions)
	if err != nil {
		http.Error(w, "invalid workflow configuration: "+err.Error(), http.StatusInternalServerError)
		return
	}

	history, err := getDocumentWorkflowHistory(document.ID)
	if err != nil {
		http.Error(w, "failed to fetch workflow history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(documentWorkflowResponse{
		DocumentID:       document.ID,
		WorkflowID:       document.WorkflowID,
		CurrentState:     document.CurrentState,
		Status:           document.Status,
		AvailableActions: actions,
		History:          history,
	})
}

func TransitionDocumentWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user := middleware.GetUser(r)
	if user.ID == uuid.Nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	documentID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}

	var req documentWorkflowTransitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	req.Action = strings.TrimSpace(req.Action)
	req.Comment = strings.TrimSpace(req.Comment)
	if req.Action == "" {
		http.Error(w, "action is required", http.StatusBadRequest)
		return
	}

	var document models.Document
	if err := config.DB.Preload("Workflow").First(&document, "id = ?", documentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "document not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.TrimSpace(document.CurrentState) == "" {
		document.CurrentState = resolveInitialDocumentState(resolveDocumentWorkflowDefinition(&document))
	}

	userPermissions := middleware.GetEffectivePermissions(r)
	_, transitions, err := getAvailableDocumentWorkflowActions(&document, userPermissions)
	if err != nil {
		http.Error(w, "invalid workflow configuration: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var targetTransition *models.WorkflowTransitionDef
	for _, transition := range transitions {
		if transition.From == document.CurrentState && transition.Action == req.Action {
			t := transition
			targetTransition = &t
			break
		}
	}

	if targetTransition == nil {
		http.Error(w, "workflow action is not available for the current state", http.StatusBadRequest)
		return
	}

	if !userHasWorkflowPermission(userPermissions, targetTransition.Permission) {
		http.Error(w, "insufficient permission for this workflow action", http.StatusForbidden)
		return
	}

	if targetTransition.RequiresComment && req.Comment == "" {
		http.Error(w, "comment is required for this action", http.StatusBadRequest)
		return
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}

	fromState := document.CurrentState
	fromStatus := document.Status
	toState := targetTransition.To
	toStatus := mapDocumentStateToStatus(toState)

	tx := config.DB.Begin()
	defer func() {
		if rec := recover(); rec != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&models.Document{}).
		Where("id = ?", document.ID).
		Updates(map[string]interface{}{
			"current_state": toState,
			"status":        toStatus,
			"updated_at":    time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update workflow state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	details := models.DocumentMetadata{
		"from_state":      fromState,
		"to_state":        toState,
		"workflow_action": req.Action,
		"comment":         req.Comment,
		"previous_status": fromStatus,
		"new_status":      toStatus,
		"metadata":        metadata,
	}

	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &user.ID,
		Action:     models.DocumentAuditActionStatusChange,
		Details:    details,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to save workflow audit record: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit workflow transition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	workflowDef := resolveDocumentWorkflowDefinition(&document)
	if targetTransition != nil && len(targetTransition.Notifications) > 0 {
		workflowID := document.WorkflowID
		if workflowID == nil {
			generatedWorkflowID := uuid.New()
			workflowID = &generatedWorkflowID
		}

		businessVerticalID := uuid.Nil
		if document.BusinessVerticalID != nil {
			businessVerticalID = *document.BusinessVerticalID
		}

		formDataJSON, _ := json.Marshal(map[string]interface{}{
			"document_id":     document.ID.String(),
			"title":           document.Title,
			"file_name":       document.FileName,
			"current_state":   toState,
			"workflow_action": req.Action,
		})

		transitionMetadataJSON, _ := json.Marshal(metadata)
		transitionEvent := models.WorkflowTransition{
			ID:             uuid.New(),
			SubmissionID:   document.ID,
			FromState:      fromState,
			ToState:        toState,
			Action:         req.Action,
			ActorID:        user.ID.String(),
			ActorName:      user.Name,
			ActorRole:      claims.Role,
			Comment:        req.Comment,
			Metadata:       transitionMetadataJSON,
			TransitionedAt: time.Now(),
		}

		notificationSubmission := models.FormSubmission{
			ID:                 document.ID,
			FormCode:           "document",
			WorkflowID:         workflowID,
			CurrentState:       toState,
			FormData:           formDataJSON,
			SubmittedBy:        document.UploadedByID.String(),
			BusinessVerticalID: businessVerticalID,
			Form: &models.AppForm{
				Title: document.Title,
			},
			Workflow: workflowDef,
		}

		notificationService := NewNotificationService()
		if err := notificationService.ProcessTransitionNotifications(
			&notificationSubmission,
			&transitionEvent,
			workflowDef,
			targetTransition,
			user.Name,
		); err != nil {
			log.Printf("⚠️ failed to process document workflow notifications: %v", err)
		}
	}

	if err := config.DB.Preload("Category").Preload("Tags").Preload("UploadedBy").Preload("Workflow").First(&document, "id = ?", document.ID).Error; err != nil {
		http.Error(w, "failed to fetch updated document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	actions, _, err := getAvailableDocumentWorkflowActions(&document, userPermissions)
	if err != nil {
		http.Error(w, "invalid workflow configuration after transition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	history, err := getDocumentWorkflowHistory(document.ID)
	if err != nil {
		http.Error(w, "failed to fetch workflow history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Document workflow updated successfully",
		"document": document,
		"workflow": documentWorkflowResponse{
			DocumentID:       document.ID,
			WorkflowID:       document.WorkflowID,
			CurrentState:     document.CurrentState,
			Status:           document.Status,
			AvailableActions: actions,
			History:          history,
		},
	})
}
