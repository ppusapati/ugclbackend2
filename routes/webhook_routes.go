package routes

import (
	"github.com/gin-gonic/gin"
	"p9e.in/ugcl/handlers"
)

// RegisterWebhookRoutes registers all webhook-related routes
func RegisterWebhookRoutes(router *gin.Engine) {
	// Public webhook endpoint (for receiving from third-party)
	router.POST("/api/v1/webhooks/incoming", WebhookIncomingHandler)

	// Protected webhook management routes
	webhookGroup := router.Group("/api/v1/webhooks")

	// CRUD operations
	webhookGroup.POST("", handlers.CreateWebhook)
	webhookGroup.GET("", handlers.ListWebhooks)
	webhookGroup.GET("/:id", handlers.GetWebhook)
	webhookGroup.PUT("/:id", handlers.UpdateWebhook)
	webhookGroup.DELETE("/:id", handlers.DeleteWebhook)

	// Webhook test and history
	webhookGroup.POST("/:id/test", handlers.TestWebhook)
	webhookGroup.GET("/:id/deliveries", handlers.GetWebhookDeliveryHistory)
	webhookGroup.GET("/deliveries/:deliveryId/logs", handlers.GetDeliveryLogs)
}

// WebhookIncomingHandler handles incoming webhook requests from third-party
// This is a public endpoint for receiving confirmations or responses
func WebhookIncomingHandler(c *gin.Context) {
	// This endpoint can be used by third-party systems to:
	// 1. Acknowledge webhook delivery
	// 2. Send status updates back to the system
	// In a real scenario, you might want to validate the request here

	c.JSON(200, gin.H{
		"status": "received",
		"message": "Webhook received successfully",
	})
}
