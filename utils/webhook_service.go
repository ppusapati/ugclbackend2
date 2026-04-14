package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/UGCL/backend/models"
	"gorm.io/gorm"
)

// WebhookService manages webhook operations
type WebhookService struct {
	db     *gorm.DB
	config *WebhookDeliveryConfig
}

// NewWebhookService creates a new webhook service
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{
		db:     db,
		config: DefaultWebhookConfig(),
	}
}

// CreateWebhook creates a new webhook subscription
func (ws *WebhookService) CreateWebhook(webhook *models.Webhook) error {
	if webhook.Secret == "" {
		webhook.Secret = GenerateRandomString(32)
	}
	webhook.Status = models.StatusActive
	return ws.db.Create(webhook).Error
}

// UpdateWebhook updates an existing webhook
func (ws *WebhookService) UpdateWebhook(webhook *models.Webhook) error {
	return ws.db.Model(webhook).Updates(webhook).Error
}

// GetWebhook retrieves a webhook by ID
func (ws *WebhookService) GetWebhook(id uint) (*models.Webhook, error) {
	var webhook models.Webhook
	err := ws.db.First(&webhook, id).Error
	return &webhook, err
}

// GetWebhooksByBusiness retrieves all webhooks for a business
func (ws *WebhookService) GetWebhooksByBusiness(businessID uint) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := ws.db.Where("business_id = ? AND is_active = true", businessID).Find(&webhooks).Error
	return webhooks, err
}

// DeleteWebhook soft-deletes a webhook
func (ws *WebhookService) DeleteWebhook(id uint) error {
	return ws.db.Model(&models.Webhook{}).Where("id = ?", id).Update("is_active", false).Error
}

// TriggerWebhook triggers webhook deliveries for an event
func (ws *WebhookService) TriggerWebhook(eventType models.WebhookEventType, resourceType string, resourceID string, businessID uint, data map[string]interface{}) error {
	// Get active webhooks for this business
	webhooks, err := ws.GetWebhooksByBusiness(businessID)
	if err != nil {
		return fmt.Errorf("failed to fetch webhooks: %w", err)
	}

	// Create payload
	payload := models.NewWebhookPayload(eventType, resourceType, resourceID, businessID, data)

	// Check each webhook and create delivery if it matches event and resource type
	for _, webhook := range webhooks {
		if ws.shouldTriggerWebhook(&webhook, eventType, resourceType) {
			// Create delivery record
			delivery := &models.WebhookDelivery{
				WebhookID:   webhook.ID,
				EventType:   eventType,
				ResourceType: resourceType,
				ResourceID:  resourceID,
				Status:      "PENDING",
				Attempt:     1,
				MaxAttempts: webhook.MaxRetries,
			}

			// Marshal payload to JSON
			payloadJSON, _ := json.Marshal(payload)
			delivery.Payload = payloadJSON

			if err := ws.db.Create(delivery).Error; err != nil {
				log.Printf("Failed to create webhook delivery: %v", err)
				continue
			}

			// Send webhook asynchronously
			go ws.sendWebhookDelivery(webhook, delivery, payload)
		}
	}

	return nil
}

// shouldTriggerWebhook checks if webhook should be triggered based on configuration
func (ws *WebhookService) shouldTriggerWebhook(webhook *models.Webhook, eventType models.WebhookEventType, resourceType string) bool {
	// Check event type
	var events []string
	if err := json.Unmarshal(webhook.Events, &events); err != nil {
		log.Printf("Failed to unmarshal events: %v", err)
		return false
	}

	eventMatches := false
	for _, event := range events {
		if event == string(eventType) {
			eventMatches = true
			break
		}
	}

	if !eventMatches {
		return false
	}

	// Check resource type (if specified)
	var resourceTypes []string
	if err := json.Unmarshal(webhook.ResourceTypes, &resourceTypes); err != nil {
		log.Printf("Failed to unmarshal resource types: %v", err)
		return false
	}

	// If no resource types specified, trigger for all
	if len(resourceTypes) == 0 {
		return true
	}

	for _, resType := range resourceTypes {
		if resType == resourceType {
			return true
		}
	}

	return false
}

// sendWebhookDelivery sends a webhook delivery with retry logic
func (ws *WebhookService) sendWebhookDelivery(webhook *models.Webhook, delivery *models.WebhookDelivery, payload *models.WebhookPayload) {
	var headers map[string]string
	if err := json.Unmarshal(webhook.Headers, &headers); err != nil {
		headers = make(map[string]string)
	}

	req := &WebhookDeliveryRequest{
		URL:        webhook.URL,
		Payload:    payload,
		Secret:     webhook.Secret,
		Headers:    headers,
		Timeout:    10 * time.Second,
		Attempt:    delivery.Attempt,
		MaxRetries: webhook.MaxRetries,
	}

	resp, err := SendWebhook(req)

	now := time.Now()
	if err != nil {
		// Log error
		ws.logWebhookError(delivery, "FAILED", err.Error())

		// Update delivery status
		delivery.Status = "FAILED"
		delivery.Error = err.Error()
		delivery.UpdatedAt = now

		// Schedule retry if attempts remaining
		if delivery.Attempt < delivery.MaxAttempts {
			nextRetry := CalculateNextRetry(delivery.Attempt, ws.config)
			delivery.NextRetryAt = nextRetry
			delivery.Status = "RETRY_SCHEDULED"
		}

		ws.db.Model(delivery).Updates(delivery)
		return
	}

	// Parse response
	respBody, _ := ParseWebhookResponse(resp)
	delivery.SentAt = &now
	delivery.HTTPStatus = resp.StatusCode
	delivery.Response = respBody

	if IsSuccessStatusCode(resp.StatusCode) {
		delivery.Status = "SUCCESS"
		ws.logWebhookEvent(delivery, "SENT")
		ws.updateWebhookStatus(webhook, models.StatusActive)
	} else if IsRetryableStatusCode(resp.StatusCode) {
		if delivery.Attempt < delivery.MaxAttempts {
			nextRetry := CalculateNextRetry(delivery.Attempt, ws.config)
			delivery.NextRetryAt = nextRetry
			delivery.Status = "RETRY_SCHEDULED"
			ws.logWebhookEvent(delivery, "RETRY")
		} else {
			delivery.Status = "FAILED"
			delivery.Error = fmt.Sprintf("Max retries exceeded. Last HTTP status: %d", resp.StatusCode)
			ws.logWebhookError(delivery, "FAILED", delivery.Error)
			ws.updateWebhookStatus(webhook, models.StatusFailed)
		}
	} else {
		// Non-retryable error
		delivery.Status = "FAILED"
		delivery.Error = fmt.Sprintf("Non-retryable HTTP error: %d", resp.StatusCode)
		ws.logWebhookError(delivery, "FAILED", delivery.Error)
		ws.updateWebhookStatus(webhook, models.StatusFailed)
	}

	delivery.UpdatedAt = now
	ws.db.Model(delivery).Updates(delivery)
}

// RetryFailedDeliveries retries pending webhook deliveries
func (ws *WebhookService) RetryFailedDeliveries() error {
	var deliveries []models.WebhookDelivery

	// Find deliveries ready for retry
	now := time.Now()
	err := ws.db.Where("status = 'RETRY_SCHEDULED' AND next_retry_at <= ?", now).Find(&deliveries).Error
	if err != nil {
		return fmt.Errorf("failed to fetch retry deliveries: %w", err)
	}

	for _, delivery := range deliveries {
		var webhook models.Webhook
		if err := ws.db.First(&webhook, delivery.WebhookID).Error; err != nil {
			continue
		}

		var payload models.WebhookPayload
		if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
			continue
		}

		delivery.Attempt++
		ws.db.Model(&delivery).Update("attempt", delivery.Attempt)

		go ws.sendWebhookDelivery(&webhook, &delivery, &payload)
	}

	return nil
}

// updateWebhookStatus updates webhook status
func (ws *WebhookService) updateWebhookStatus(webhook *models.Webhook, status models.WebhookStatus) error {
	return ws.db.Model(webhook).Update("status", status).Error
}

// logWebhookEvent logs webhook event
func (ws *WebhookService) logWebhookEvent(delivery *models.WebhookDelivery, action string) error {
	log := &models.WebhookLog{
		WebhookID:    delivery.WebhookID,
		DeliveryID:   delivery.ID,
		EventType:    delivery.EventType,
		ResourceType: delivery.ResourceType,
		Action:       action,
		Payload:      delivery.Payload,
		Response:     delivery.Response,
	}
	return ws.db.Create(log).Error
}

// logWebhookError logs webhook error
func (ws *WebhookService) logWebhookError(delivery *models.WebhookDelivery, action string, errMsg string) error {
	log := &models.WebhookLog{
		WebhookID:    delivery.WebhookID,
		DeliveryID:   delivery.ID,
		EventType:    delivery.EventType,
		ResourceType: delivery.ResourceType,
		Action:       action,
		Payload:      delivery.Payload,
		Error:        errMsg,
	}
	return ws.db.Create(log).Error
}

// GetDeliveryHistory retrieves delivery history for a webhook
func (ws *WebhookService) GetDeliveryHistory(webhookID uint, limit int) ([]models.WebhookDelivery, error) {
	var deliveries []models.WebhookDelivery
	err := ws.db.Where("webhook_id = ?", webhookID).
		Order("created_at DESC").
		Limit(limit).
		Find(&deliveries).Error
	return deliveries, err
}

// GetDeliveryLogs retrieves logs for a delivery
func (ws *WebhookService) GetDeliveryLogs(deliveryID uint) ([]models.WebhookLog, error) {
	var logs []models.WebhookLog
	err := ws.db.Where("delivery_id = ?", deliveryID).
		Order("created_at DESC").
		Find(&logs).Error
	return logs, err
}

// TestWebhookDelivery sends a test webhook delivery
func (ws *WebhookService) TestWebhookDelivery(webhookID uint) error {
	webhook, err := ws.GetWebhook(webhookID)
	if err != nil {
		return err
	}

	// Create test payload
	testData := map[string]interface{}{
		"test": true,
		"message": "This is a test webhook delivery",
	}

	payload := models.NewWebhookPayload(models.EventCreate, "Test", "test-123", webhook.BusinessID, testData)

	// Create test delivery
	delivery := &models.WebhookDelivery{
		WebhookID:   webhook.ID,
		EventType:   models.EventCreate,
		ResourceType: "Test",
		ResourceID:  "test-123",
		Status:      "PENDING",
		Attempt:     1,
		MaxAttempts: 1,
	}

	payloadJSON, _ := json.Marshal(payload)
	delivery.Payload = payloadJSON

	if err := ws.db.Create(delivery).Error; err != nil {
		return err
	}

	// Send test webhook
	go ws.sendWebhookDelivery(webhook, delivery, payload)

	return nil
}
