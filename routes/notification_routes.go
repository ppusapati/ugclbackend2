package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
)

// RegisterNotificationRoutes registers all notification-related routes
func RegisterNotificationRoutes(api *mux.Router, admin *mux.Router) {
	notifHandler := &handlers.NotificationHandler{}
	adminHandler := &handlers.NotificationAdminHandler{}

	// User notification endpoints (any authenticated user)
	// These routes are under /api/v1/notifications

	// Get notifications for current user
	api.HandleFunc("/notifications", notifHandler.GetNotifications).Methods("GET")

	// ⚠️ Specific paths MUST be registered before parameterized {id} routes
	// so gorilla/mux matches them correctly.

	// Get unread count
	api.HandleFunc("/notifications/unread-count", notifHandler.GetUnreadCount).Methods("GET")

	// Mark all notifications as read
	api.HandleFunc("/notifications/read-all", notifHandler.MarkAllNotificationsAsRead).Methods("PATCH")

	// Get user preferences
	api.HandleFunc("/notifications/preferences", notifHandler.GetNotificationPreferences).Methods("GET")

	// Update user preferences
	api.HandleFunc("/notifications/preferences", notifHandler.UpdateNotificationPreferences).Methods("PUT")

	// Web push subscription management
	api.HandleFunc("/notifications/push/public-key", notifHandler.GetWebPushPublicKey).Methods("GET")
	api.HandleFunc("/notifications/push/subscriptions", notifHandler.SaveWebPushSubscription).Methods("POST")
	api.HandleFunc("/notifications/push/subscriptions", notifHandler.DeleteWebPushSubscription).Methods("DELETE")
	api.HandleFunc("/notifications/push/mobile/tokens", notifHandler.GetMobilePushTokens).Methods("GET")
	api.HandleFunc("/notifications/push/mobile/tokens", notifHandler.SaveMobilePushToken).Methods("POST")
	api.HandleFunc("/notifications/push/mobile/tokens", notifHandler.DeleteMobilePushToken).Methods("DELETE")
	api.HandleFunc("/notifications/push/test", notifHandler.SendTestWebPush).Methods("POST")
	api.HandleFunc("/notifications/push/mobile/test", notifHandler.SendTestMobilePush).Methods("POST")

	// Server-Sent Events stream for real-time notifications
	api.HandleFunc("/notifications/stream", notifHandler.StreamNotifications).Methods("GET")

	// Parameterized routes LAST so they don't swallow specific paths above
	// Get single notification
	api.HandleFunc("/notifications/{id}", notifHandler.GetNotification).Methods("GET")

	// Mark notification as read
	api.HandleFunc("/notifications/{id}/read", notifHandler.MarkNotificationAsRead).Methods("PATCH")

	// Delete notification
	api.HandleFunc("/notifications/{id}", notifHandler.DeleteNotification).Methods("DELETE")

	// Admin notification endpoints (require admin permissions)
	// These routes are available under both:
	// - /api/v1/admin/notifications/* (legacy)
	// - /api/v1/admin/notification-rules* (frontend/doc contract)

	// Get all notification rules
	admin.Handle("/notifications/rules", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetAllNotificationRules))).Methods("GET")
	admin.Handle("/notification-rules", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetAllNotificationRules))).Methods("GET")

	// Get single notification rule
	admin.Handle("/notifications/rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetNotificationRule))).Methods("GET")
	admin.Handle("/notification-rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetNotificationRule))).Methods("GET")

	// Create notification rule
	admin.Handle("/notifications/rules", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.CreateNotificationRule))).Methods("POST")
	admin.Handle("/notification-rules", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.CreateNotificationRule))).Methods("POST")

	// Update notification rule
	admin.Handle("/notifications/rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.UpdateNotificationRule))).Methods("PUT")
	admin.Handle("/notification-rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.UpdateNotificationRule))).Methods("PUT")

	// Delete notification rule
	admin.Handle("/notifications/rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.DeleteNotificationRule))).Methods("DELETE")
	admin.Handle("/notification-rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.DeleteNotificationRule))).Methods("DELETE")

	// Toggle notification rule active status
	admin.Handle("/notifications/rules/{id}/toggle", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.ToggleNotificationRule))).Methods("PATCH")
	admin.Handle("/notification-rules/{id}/toggle", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.ToggleNotificationRule))).Methods("PATCH")
	admin.Handle("/notification-rules/{id}/status", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.ToggleNotificationRule))).Methods("PATCH")

	// Get notification statistics
	admin.Handle("/notifications/stats", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetNotificationStats))).Methods("GET")
	admin.Handle("/notification-rules/stats", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetNotificationStats))).Methods("GET")
	admin.Handle("/notifications/push/mobile-tokens", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(notifHandler.GetMobilePushTokensForAdmin))).Methods("GET")
}
