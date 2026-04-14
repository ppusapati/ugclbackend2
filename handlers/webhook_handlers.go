package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"
)

// CreateWebhookRequest represents the request body for creating a webhook
type CreateWebhookRequest struct {
	URL           string            `json:"url" binding:"required,url"`
	Events        []string          `json:"events" binding:"required"`
	ResourceTypes []string          `json:"resource_types"`
	Secret        string            `json:"secret"`
	Headers       map[string]string `json:"headers"`
	MaxRetries    int               `json:"max_retries"`
	RetryInterval int               `json:"retry_interval"`
}

// UpdateWebhookRequest represents the request body for updating a webhook
type UpdateWebhookRequest struct {
	URL           string            `json:"url"`
	Events        []string          `json:"events"`
	ResourceTypes []string          `json:"resource_types"`
	Headers       map[string]string `json:"headers"`
	MaxRetries    int               `json:"max_retries"`
	RetryInterval int               `json:"retry_interval"`
	IsActive      bool              `json:"is_active"`
}

// CreateWebhook creates a new webhook subscription
// @Summary Create webhook subscription
// @Description Create a new webhook for real-time event notifications
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param req body CreateWebhookRequest true "Webhook creation request"
// @Success 201 {object} models.Webhook
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /api/v1/webhooks [post]
func CreateWebhook(c *gin.Context) {
	var req CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Extract validated active business ID from context.
	businessID, exists := middleware.GetBusinessIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Business ID not found"})
		return
	}

	// Set default max retries
	maxRetries := req.MaxRetries
	if maxRetries == 0 {
		maxRetries = 5
	}

	// Set default retry interval
	retryInterval := req.RetryInterval
	if retryInterval == 0 {
		retryInterval = 300
	}

	headers := make(datatypes.JSONMap, len(req.Headers))
	for key, value := range req.Headers {
		headers[key] = value
	}

	webhook := &models.Webhook{
		BusinessID:    businessID,
		URL:           req.URL,
		Events:        datatypes.JSONSlice[string](req.Events),
		ResourceTypes: datatypes.JSONSlice[string](req.ResourceTypes),
		Secret:        req.Secret,
		Headers:       headers,
		MaxRetries:    maxRetries,
		RetryInterval: retryInterval,
		IsActive:      true,
		Status:        models.StatusActive,
	}

	webhookService := utils.NewWebhookService(config.DB)
	if err := webhookService.CreateWebhook(webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to create webhook: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, webhook)
}

// ListWebhooks retrieves all webhooks for a business
// @Summary List webhooks
// @Description Get all webhooks for the authenticated business
// @Tags Webhooks
// @Produce json
// @Success 200 {array} models.Webhook
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /api/v1/webhooks [get]
func ListWebhooks(c *gin.Context) {
	businessID, exists := middleware.GetBusinessIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Business ID not found"})
		return
	}

	webhookService := utils.NewWebhookService(config.DB)
	webhooks, err := webhookService.GetWebhooksByBusiness(businessID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to fetch webhooks: %v", err)})
		return
	}

	c.JSON(http.StatusOK, webhooks)
}

// GetWebhook retrieves a specific webhook by ID
// @Summary Get webhook
// @Description Get details of a specific webhook
// @Tags Webhooks
// @Produce json
// @Param id path int true "Webhook ID"
// @Success 200 {object} models.Webhook
// @Failure 404 {object} map[string]string "Not found"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /api/v1/webhooks/{id} [get]
func GetWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	businessID, exists := middleware.GetBusinessIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Business ID not found"})
		return
	}

	webhookService := utils.NewWebhookService(config.DB)
	webhook, err := webhookService.GetWebhook(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	// Verify ownership
	if webhook.BusinessID != businessID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, webhook)
}

// UpdateWebhook updates an existing webhook
// @Summary Update webhook
// @Description Update webhook configuration
// @Tags Webhooks
// @Accept json
// @Produce json
// @Param id path int true "Webhook ID"
// @Param req body UpdateWebhookRequest true "Update request"
// @Success 200 {object} models.Webhook
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /api/v1/webhooks/{id} [put]
func UpdateWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	businessID, exists := middleware.GetBusinessIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Business ID not found"})
		return
	}

	var req UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	webhookService := utils.NewWebhookService(config.DB)
	webhook, err := webhookService.GetWebhook(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	// Verify ownership
	if webhook.BusinessID != businessID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Update fields
	if req.URL != "" {
		webhook.URL = req.URL
	}
	if len(req.Events) > 0 {
		webhook.Events = datatypes.JSONSlice[string](req.Events)
	}
	if len(req.ResourceTypes) > 0 {
		webhook.ResourceTypes = datatypes.JSONSlice[string](req.ResourceTypes)
	}
	if len(req.Headers) > 0 {
		headers := make(datatypes.JSONMap, len(req.Headers))
		for key, value := range req.Headers {
			headers[key] = value
		}
		webhook.Headers = headers
	}
	if req.MaxRetries > 0 {
		webhook.MaxRetries = req.MaxRetries
	}
	if req.RetryInterval > 0 {
		webhook.RetryInterval = req.RetryInterval
	}
	webhook.IsActive = req.IsActive

	if err := webhookService.UpdateWebhook(webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to update webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, webhook)
}

// DeleteWebhook deletes a webhook
// @Summary Delete webhook
// @Description Delete a webhook subscription
// @Tags Webhooks
// @Param id path int true "Webhook ID"
// @Success 204
// @Failure 404 {object} map[string]string "Not found"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /api/v1/webhooks/{id} [delete]
func DeleteWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	businessID, exists := middleware.GetBusinessIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Business ID not found"})
		return
	}

	webhookService := utils.NewWebhookService(config.DB)
	webhook, err := webhookService.GetWebhook(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	// Verify ownership
	if webhook.BusinessID != businessID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := webhookService.DeleteWebhook(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to delete webhook: %v", err)})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// TestWebhook sends a test webhook delivery
// @Summary Test webhook
// @Description Send a test webhook delivery to verify configuration
// @Tags Webhooks
// @Param id path int true "Webhook ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Not found"
// @Router /api/v1/webhooks/{id}/test [post]
func TestWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	businessID, exists := middleware.GetBusinessIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Business ID not found"})
		return
	}

	webhookService := utils.NewWebhookService(config.DB)
	webhook, err := webhookService.GetWebhook(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	// Verify ownership
	if webhook.BusinessID != businessID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := webhookService.TestWebhookDelivery(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to send test webhook: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test webhook sent successfully"})
}

// GetWebhookDeliveryHistory retrieves delivery history for a webhook
// @Summary Get delivery history
// @Description Get delivery history for a webhook
// @Tags Webhooks
// @Produce json
// @Param id path int true "Webhook ID"
// @Param limit query int false "Limit results (default: 50)"
// @Success 200 {array} models.WebhookDelivery
// @Failure 404 {object} map[string]string "Not found"
// @Router /api/v1/webhooks/{id}/deliveries [get]
func GetWebhookDeliveryHistory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsedLimit, err := strconv.Atoi(l); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	businessID, exists := middleware.GetBusinessIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Business ID not found"})
		return
	}

	webhookService := utils.NewWebhookService(config.DB)
	webhook, err := webhookService.GetWebhook(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	if webhook.BusinessID != businessID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	deliveries, err := webhookService.GetDeliveryHistory(uint(id), limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery history not found"})
		return
	}

	c.JSON(http.StatusOK, deliveries)
}

// GetDeliveryLogs retrieves logs for a specific delivery
// @Summary Get delivery logs
// @Description Get detailed logs for a webhook delivery
// @Tags Webhooks
// @Produce json
// @Param deliveryId path int true "Delivery ID"
// @Success 200 {array} models.WebhookLog
// @Failure 404 {object} map[string]string "Not found"
// @Router /api/v1/webhooks/deliveries/{deliveryId}/logs [get]
func GetDeliveryLogs(c *gin.Context) {
	deliveryID, err := strconv.ParseUint(c.Param("deliveryId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delivery ID"})
		return
	}

	businessID, exists := middleware.GetBusinessIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Business ID not found"})
		return
	}

	webhookService := utils.NewWebhookService(config.DB)
	delivery, webhook, err := webhookService.GetWebhookDelivery(uint(deliveryID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook delivery not found"})
		return
	}

	if webhook.BusinessID != businessID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	logs, err := webhookService.GetDeliveryLogs(uint(deliveryID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery logs not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"delivery_id": delivery.ID, "logs": logs})
}
