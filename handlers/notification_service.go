package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"text/template"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// NotificationService handles notification creation and delivery
type NotificationService struct {
	db *gorm.DB
}

// NewNotificationService creates a new notification service instance
func NewNotificationService() *NotificationService {
	return &NotificationService{
		db: config.DB,
	}
}

// NotificationContext holds data for template rendering
type NotificationContext struct {
	FormTitle          string
	FormCode           string
	SubmitterName      string
	SubmitterID        string
	ApproverName       string
	ApproverID         string
	CurrentState       string
	PreviousState      string
	Action             string
	Comment            string
	FormData           map[string]interface{}
	BusinessVertical   string
	SiteName           string
	SubmissionID       string
	WorkflowID         string
	TransitionID       string
	BusinessVerticalID string
}

// ProcessTransitionNotifications processes notifications for a workflow transition
func (ns *NotificationService) ProcessTransitionNotifications(
	submission *models.FormSubmission,
	transition *models.WorkflowTransition,
	workflowDef *models.WorkflowDefinition,
	transitionDef *models.WorkflowTransitionDef,
	actorName string,
) error {
	if transitionDef.Notifications == nil || len(transitionDef.Notifications) == 0 {
		return nil
	}

	// Build notification context
	context := ns.buildNotificationContext(submission, transition, actorName)

	// Process each notification configuration
	for _, notifConfig := range transitionDef.Notifications {
		if err := ns.processNotification(submission, transition, workflowDef, notifConfig, context); err != nil {
			log.Printf("❌ Error processing notification: %v", err)
			// Continue processing other notifications even if one fails
			continue
		}
	}

	return nil
}

// processNotification processes a single notification configuration
func (ns *NotificationService) processNotification(
	submission *models.FormSubmission,
	transition *models.WorkflowTransition,
	workflowDef *models.WorkflowDefinition,
	notifConfig models.TransitionNotification,
	context NotificationContext,
) error {
	// Resolve recipients
	recipientIDs, err := ns.resolveRecipients(notifConfig.Recipients, submission, context)
	if err != nil {
		return fmt.Errorf("failed to resolve recipients: %w", err)
	}

	if len(recipientIDs) == 0 {
		log.Printf("⚠️  No recipients resolved for notification")
		return nil
	}

	// Render templates
	title, err := ns.renderTemplate(notifConfig.TitleTemplate, context)
	if err != nil {
		return fmt.Errorf("failed to render title template: %w", err)
	}

	body, err := ns.renderTemplate(notifConfig.BodyTemplate, context)
	if err != nil {
		return fmt.Errorf("failed to render body template: %w", err)
	}

	// Determine priority
	priority := models.NotificationPriorityNormal
	if notifConfig.Priority != "" {
		priority = models.NotificationPriority(notifConfig.Priority)
	}

	// Determine channels
	channels := notifConfig.Channels
	if len(channels) == 0 {
		channels = []string{"in_app"}
	}

	// Create notifications for each recipient
	for _, recipientID := range recipientIDs {
		// Check user preferences
		shouldSend, channel := ns.checkUserPreferences(recipientID, models.NotificationTypeWorkflowTransition, channels)
		if !shouldSend {
			continue
		}

		notification := models.Notification{
			UserID:             recipientID,
			Type:               models.NotificationTypeWorkflowTransition,
			Priority:           priority,
			Title:              title,
			Body:               body,
			SubmissionID:       &submission.ID,
			WorkflowID:         submission.WorkflowID,
			TransitionID:       &transition.ID,
			FormCode:           submission.FormCode,
			BusinessVerticalID: &submission.BusinessVerticalID,
			Status:             models.NotificationStatusPending,
			Channel:            models.NotificationChannel(channel),
		}

		// Create notification in database
		if err := ns.db.Create(&notification).Error; err != nil {
			log.Printf("❌ Failed to create notification for user %s: %v", recipientID, err)
			continue
		}

		log.Printf("✅ Created notification for user %s: %s", recipientID, title)

		// Mark as sent (in production, this would be done by delivery service)
		notification.MarkAsSent()
		ns.db.Save(&notification)
	}

	return nil
}

// resolveRecipients resolves recipient user IDs from recipient definitions
func (ns *NotificationService) resolveRecipients(
	recipients []models.NotificationRecipientDef,
	submission *models.FormSubmission,
	context NotificationContext,
) ([]string, error) {
	userIDs := make(map[string]bool) // Using map to deduplicate

	for _, recipient := range recipients {
		switch recipient.Type {
		case "user":
			if recipient.Value != "" {
				userIDs[recipient.Value] = true
			}

		case "submitter":
			userIDs[context.SubmitterID] = true

		case "approver":
			if context.ApproverID != "" {
				userIDs[context.ApproverID] = true
			}

		case "role":
			users, err := ns.getUsersByRole(recipient.RoleID)
			if err != nil {
				log.Printf("⚠️  Failed to get users by role %s: %v", recipient.RoleID, err)
				continue
			}
			for _, userID := range users {
				userIDs[userID] = true
			}

		case "business_role":
			users, err := ns.getUsersByBusinessRole(recipient.BusinessRoleID, submission.BusinessVerticalID)
			if err != nil {
				log.Printf("⚠️  Failed to get users by business role %s: %v", recipient.BusinessRoleID, err)
				continue
			}
			for _, userID := range users {
				userIDs[userID] = true
			}

		case "permission":
			users, err := ns.getUsersByPermission(recipient.PermissionCode)
			if err != nil {
				log.Printf("⚠️  Failed to get users by permission %s: %v", recipient.PermissionCode, err)
				continue
			}
			for _, userID := range users {
				userIDs[userID] = true
			}

		case "field_value":
			// Get user ID from form data field
			if formData, ok := context.FormData[recipient.Value].(string); ok {
				userIDs[formData] = true
			}

		case "attribute":
			// ABAC - query users by attributes
			users, err := ns.getUsersByAttributes(recipient.AttributeQuery)
			if err != nil {
				log.Printf("⚠️  Failed to get users by attributes: %v", err)
				continue
			}
			for _, userID := range users {
				userIDs[userID] = true
			}

		case "policy":
			// PBAC - evaluate policy to get users
			users, err := ns.getUsersByPolicy(recipient.PolicyID, context)
			if err != nil {
				log.Printf("⚠️  Failed to get users by policy %s: %v", recipient.PolicyID, err)
				continue
			}
			for _, userID := range users {
				userIDs[userID] = true
			}
		}
	}

	// Convert map keys to slice
	result := make([]string, 0, len(userIDs))
	for userID := range userIDs {
		result = append(result, userID)
	}

	return result, nil
}

// getUsersByRole gets all user IDs with a specific role
func (ns *NotificationService) getUsersByRole(roleID string) ([]string, error) {
	if roleID == "" {
		return nil, nil
	}

	roleUUID, err := uuid.Parse(roleID)
	if err != nil {
		return nil, err
	}

	var users []models.User
	if err := ns.db.Where("role_id = ?", roleUUID).Find(&users).Error; err != nil {
		return nil, err
	}

	userIDs := make([]string, len(users))
	for i, user := range users {
		userIDs[i] = user.ID.String()
	}

	return userIDs, nil
}

// getUsersByBusinessRole gets all user IDs with a specific business role
func (ns *NotificationService) getUsersByBusinessRole(businessRoleID string, businessVerticalID uuid.UUID) ([]string, error) {
	if businessRoleID == "" {
		return nil, nil
	}

	roleUUID, err := uuid.Parse(businessRoleID)
	if err != nil {
		return nil, err
	}

	var userBusinessRoles []models.UserBusinessRole
	if err := ns.db.Where("business_role_id = ?", roleUUID).Find(&userBusinessRoles).Error; err != nil {
		return nil, err
	}

	// Filter by business vertical
	var filteredRoles []models.UserBusinessRole
	for _, ubr := range userBusinessRoles {
		var businessRole models.BusinessRole
		if err := ns.db.First(&businessRole, ubr.BusinessRoleID).Error; err == nil {
			if businessRole.BusinessVerticalID == businessVerticalID {
				filteredRoles = append(filteredRoles, ubr)
			}
		}
	}

	userIDs := make([]string, len(filteredRoles))
	for i, ubr := range filteredRoles {
		userIDs[i] = ubr.UserID.String()
	}

	return userIDs, nil
}

// getUsersByPermission gets all user IDs with a specific permission
func (ns *NotificationService) getUsersByPermission(permissionCode string) ([]string, error) {
	if permissionCode == "" {
		return nil, nil
	}

	// Get permission
	var permission models.Permission
	if err := ns.db.Where("code = ?", permissionCode).First(&permission).Error; err != nil {
		return nil, err
	}

	// Get all roles with this permission
	var rolePermissions []models.RolePermission
	if err := ns.db.Where("permission_id = ?", permission.ID).Find(&rolePermissions).Error; err != nil {
		return nil, err
	}

	// Get all users with these roles
	roleIDs := make([]uuid.UUID, len(rolePermissions))
	for i, rp := range rolePermissions {
		roleIDs[i] = rp.RoleID
	}

	var users []models.User
	if err := ns.db.Where("role_id IN ?", roleIDs).Find(&users).Error; err != nil {
		return nil, err
	}

	userIDs := make([]string, len(users))
	for i, user := range users {
		userIDs[i] = user.ID.String()
	}

	return userIDs, nil
}

// getUsersByAttributes gets user IDs matching attribute query (ABAC)
func (ns *NotificationService) getUsersByAttributes(attributeQuery map[string]interface{}) ([]string, error) {
	if len(attributeQuery) == 0 {
		return nil, nil
	}

	// Query user_attributes table
	var userAttributes []models.UserAttribute
	query := ns.db.Model(&models.UserAttribute{})

	// Build WHERE conditions for each attribute
	for key, value := range attributeQuery {
		query = query.Where("attribute_name = ? AND attribute_value = ?", key, fmt.Sprint(value))
	}

	if err := query.Find(&userAttributes).Error; err != nil {
		return nil, err
	}

	// Deduplicate user IDs
	userIDMap := make(map[string]bool)
	for _, ua := range userAttributes {
		userIDMap[ua.UserID.String()] = true
	}

	userIDs := make([]string, 0, len(userIDMap))
	for userID := range userIDMap {
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

// getUsersByPolicy gets user IDs by evaluating a policy (PBAC)
func (ns *NotificationService) getUsersByPolicy(policyID string, context NotificationContext) ([]string, error) {
	// For now, return empty - full PBAC evaluation would go here
	// This would involve evaluating policy conditions against user contexts
	log.Printf("⚠️  Policy-based recipient resolution not yet implemented for policy %s", policyID)
	return nil, nil
}

// checkUserPreferences checks if user wants to receive notifications
func (ns *NotificationService) checkUserPreferences(userID string, notifType models.NotificationType, requestedChannels []string) (bool, string) {
	var prefs models.NotificationPreference
	if err := ns.db.Where("user_id = ?", userID).First(&prefs).Error; err != nil {
		// No preferences set, use defaults
		return true, "in_app"
	}

	// Check if type is disabled
	for _, disabledType := range prefs.DisabledTypes {
		if disabledType == string(notifType) {
			return false, ""
		}
	}

	// Determine channel based on preferences
	for _, channel := range requestedChannels {
		switch channel {
		case "in_app":
			if prefs.EnableInApp {
				return true, "in_app"
			}
		case "email":
			if prefs.EnableEmail {
				return true, "email"
			}
		case "sms":
			if prefs.EnableSMS {
				return true, "sms"
			}
		case "web_push":
			if prefs.EnableWebPush {
				return true, "web_push"
			}
		}
	}

	// Default to in-app if enabled
	if prefs.EnableInApp {
		return true, "in_app"
	}

	return false, ""
}

// renderTemplate renders a template string with the given context
func (ns *NotificationService) renderTemplate(templateStr string, context NotificationContext) (string, error) {
	// Create template
	tmpl, err := template.New("notification").Parse(templateStr)
	if err != nil {
		return "", err
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// buildNotificationContext builds the context for template rendering
func (ns *NotificationService) buildNotificationContext(
	submission *models.FormSubmission,
	transition *models.WorkflowTransition,
	actorName string,
) NotificationContext {
	// Parse form data
	var formData map[string]interface{}
	if err := json.Unmarshal(submission.FormData, &formData); err != nil {
		log.Printf("⚠️  Failed to unmarshal form data: %v", err)
		formData = make(map[string]interface{})
	}

	// Get form title
	formTitle := submission.FormCode
	if submission.Form != nil {
		formTitle = submission.Form.Title
	}

	// Get business vertical name
	businessVertical := ""
	if submission.BusinessVertical != nil {
		businessVertical = submission.BusinessVertical.Name
	}

	// Get site name
	siteName := ""
	if submission.SiteID != nil {
		var site models.Site
		if err := ns.db.First(&site, submission.SiteID).Error; err == nil {
			siteName = site.Name
		}
	}

	return NotificationContext{
		FormTitle:          formTitle,
		FormCode:           submission.FormCode,
		SubmitterName:      submission.SubmittedBy, // You might want to fetch actual name
		SubmitterID:        submission.SubmittedBy,
		ApproverName:       actorName,
		ApproverID:         transition.ActorID,
		CurrentState:       submission.CurrentState,
		PreviousState:      transition.FromState,
		Action:             transition.Action,
		Comment:            transition.Comment,
		FormData:           formData,
		BusinessVertical:   businessVertical,
		SiteName:           siteName,
		SubmissionID:       submission.ID.String(),
		WorkflowID:         submission.WorkflowID.String(),
		TransitionID:       transition.ID.String(),
		BusinessVerticalID: submission.BusinessVerticalID.String(),
	}
}

// GetNotificationsForUser retrieves notifications for a specific user
func (ns *NotificationService) GetNotificationsForUser(
	userID string,
	filters map[string]interface{},
) ([]models.Notification, error) {
	query := ns.db.Where("user_id = ?", userID)

	// Apply filters
	if notifType, ok := filters["type"].(string); ok && notifType != "" {
		query = query.Where("type = ?", notifType)
	}
	if priority, ok := filters["priority"].(string); ok && priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if read, ok := filters["read"].(bool); ok {
		if read {
			query = query.Where("read_at IS NOT NULL")
		} else {
			query = query.Where("read_at IS NULL")
		}
	}
	if formCode, ok := filters["form_code"].(string); ok && formCode != "" {
		query = query.Where("form_code = ?", formCode)
	}

	// Pagination
	if limit, ok := filters["limit"].(int); ok && limit > 0 {
		query = query.Limit(limit)
	} else {
		query = query.Limit(50)
	}

	if offset, ok := filters["offset"].(int); ok && offset > 0 {
		query = query.Offset(offset)
	}

	var notifications []models.Notification
	if err := query.Order("created_at DESC").Find(&notifications).Error; err != nil {
		return nil, err
	}

	return notifications, nil
}

// GetUnreadCount gets the count of unread notifications for a user
func (ns *NotificationService) GetUnreadCount(userID string) (int64, error) {
	var count int64
	if err := ns.db.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
