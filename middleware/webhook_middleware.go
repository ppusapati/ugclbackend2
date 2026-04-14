package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"
)

// WebhookEventTriggerMiddleware intercepts API calls and triggers webhooks
// Apply this middleware to routes that should trigger webhooks
func WebhookEventTriggerMiddleware(eventType models.WebhookEventType, resourceType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Call the main handler first
		c.Next()

		// Only trigger webhooks on successful requests
		if c.Writer.Status() >= 400 {
			return
		}

		// Check if request method matches the event type
		if eventType == models.EventCreate && c.Request.Method != http.MethodPost {
			return
		}
		if eventType == models.EventUpdate && c.Request.Method != http.MethodPut && c.Request.Method != http.MethodPatch {
			return
		}

		// Don't trigger for webhook management endpoints
		if strings.Contains(c.Request.URL.Path, "webhooks") {
			return
		}

		// Extract business ID
		businessID, exists := GetBusinessIDFromContext(c)
		if !exists {
			return
		}

		// Extract resource ID from URL path or response
		resourceID := extractResourceIDFromContext(c)
		if resourceID == "" {
			return
		}

		// Extract response data for payload
		responseData := make(map[string]interface{})
		if v, exists := c.Get("response_data"); exists {
			if data, ok := v.(map[string]interface{}); ok {
				responseData = data
			}
		}

		// Trigger webhooks asynchronously
		go func() {
			webhookService := utils.NewWebhookService(config.DB)
			webhookService.TriggerWebhook(eventType, resourceType, resourceID, businessID, responseData)
		}()
	}
}

// StoreResponseDataMiddleware stores response data in context for webhook delivery
// Use this before sending JSON response
func StoreResponseDataMiddleware(c *gin.Context) {
	// Capture the response writer
	originalWriter := c.Writer
	responseWriter := &ResponseWriter{ResponseWriter: originalWriter}
	c.Writer = responseWriter

	c.Next()

	// Store the response in context if it's JSON
	if strings.Contains(originalWriter.Header().Get("Content-Type"), "application/json") {
		c.Set("response_data", responseWriter.body)
	}
}

// ResponseWriter wraps gin.ResponseWriter to capture response body
type ResponseWriter struct {
	gin.ResponseWriter
	body map[string]interface{}
}

// Write implements gin.ResponseWriter
func (w *ResponseWriter) Write(b []byte) (int, error) {
	// Try to unmarshal JSON response
	if len(b) > 0 {
		// For now, just return the bytes
		// In production, parse JSON here
	}
	return w.ResponseWriter.Write(b)
}

// extractResourceIDFromContext extracts resource ID from URL or context
func extractResourceIDFromContext(c *gin.Context) string {
	// Try to get from URL parameters first
	idParam := c.Param("id")
	if idParam != "" {
		return idParam
	}

	siteIDParam := c.Param("siteId")
	if siteIDParam != "" {
		return siteIDParam
	}

	businessCodeParam := c.Param("businessCode")
	if businessCodeParam != "" {
		return businessCodeParam
	}

	// Try from context if manually set
	if id, exists := c.Get("resource_id"); exists {
		if idStr, ok := id.(string); ok {
			return idStr
		}
	}

	// Extract from URL path as fallback
	parts := strings.Split(c.Request.URL.Path, "/")
	for i, part := range parts {
		if part == "id" || part == "siteId" || part == "businessCode" {
			if i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}

	return ""
}

// AutoTriggerWebhookMiddleware automatically detects and triggers webhooks
// This is a smarter middleware that works on all routes
func AutoTriggerWebhookMiddleware(c *gin.Context) {
	// Store original method
	originalMethod := c.Request.Method

	// Call next handler
	c.Next()

	// Check status code
	if c.Writer.Status() < 200 || c.Writer.Status() >= 400 {
		return
	}

	// Determine event type
	var eventType models.WebhookEventType
	switch originalMethod {
	case http.MethodPost:
		eventType = models.EventCreate
	case http.MethodPut, http.MethodPatch:
		eventType = models.EventUpdate
	default:
		return
	}

	// Extract business ID
	businessID, exists := GetBusinessIDFromContext(c)
	if !exists {
		return
	}

	// Extract resource type and ID
	resourceType := extractResourceTypeFromPath(c.Request.URL.Path)
	resourceID := extractResourceIDFromContext(c)

	if resourceType == "" || resourceID == "" {
		return
	}

	// Trigger webhooks asynchronously
	go func() {
		responseData := make(map[string]interface{})
		if v, exists := c.Get("response_data"); exists {
			if data, ok := v.(map[string]interface{}); ok {
				responseData = data
			}
		}

		webhookService := utils.NewWebhookService(config.DB)
		webhookService.TriggerWebhook(eventType, resourceType, resourceID, businessID, responseData)
	}()
}

// extractResourceTypeFromPath extracts resource type from URL path
func extractResourceTypeFromPath(path string) string {
	// Examples:
	// /api/v1/users/123 -> "User"
	// /api/v1/dprsite/123 -> "DPRSite"
	// /business/{code}/sites/123 -> "Site"

	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	if len(parts) == 0 {
		return ""
	}

	resourcePath := parts[0]

	// Map URL paths to resource types
	typeMap := map[string]string{
		"users":           "User",
		"business":        "Business",
		"sites":           "Site",
		"dprsite":         "DPRSite",
		"wrapping":        "Wrapping",
		"eway":            "EWay",
		"water":           "Water",
		"stock":           "Stock",
		"dairysite":       "DairySite",
		"material":        "Material",
		"diesel":          "Diesel",
		"payment":         "Payment",
		"nmr-vehicle":     "NMRVehicle",
		"reports":         "Report",
		"projects":        "Project",
		"notifications":   "Notification",
		"documents":       "Document",
		"workflows":       "Workflow",
	}

	if resType, exists := typeMap[strings.ToLower(resourcePath)]; exists {
		return resType
	}

	// Return capitalized version if not in map
	return strings.ToUpper(resourcePath[:1]) + resourcePath[1:]
}
