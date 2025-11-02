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

	// Get single notification
	api.HandleFunc("/notifications/{id}", notifHandler.GetNotification).Methods("GET")

	// Mark notification as read
	api.HandleFunc("/notifications/{id}/read", notifHandler.MarkNotificationAsRead).Methods("PATCH")

	// Mark all notifications as read
	api.HandleFunc("/notifications/read-all", notifHandler.MarkAllNotificationsAsRead).Methods("PATCH")

	// Delete notification
	api.HandleFunc("/notifications/{id}", notifHandler.DeleteNotification).Methods("DELETE")

	// Get unread count
	api.HandleFunc("/notifications/unread-count", notifHandler.GetUnreadCount).Methods("GET")

	// Get user preferences
	api.HandleFunc("/notifications/preferences", notifHandler.GetNotificationPreferences).Methods("GET")

	// Update user preferences
	api.HandleFunc("/notifications/preferences", notifHandler.UpdateNotificationPreferences).Methods("PUT")

	// Server-Sent Events stream for real-time notifications
	api.HandleFunc("/notifications/stream", notifHandler.StreamNotifications).Methods("GET")

	// Admin notification endpoints (require admin permissions)
	// These routes are under /api/v1/admin/notifications

	// Get all notification rules
	admin.Handle("/notifications/rules", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetAllNotificationRules))).Methods("GET")

	// Get single notification rule
	admin.Handle("/notifications/rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetNotificationRule))).Methods("GET")

	// Create notification rule
	admin.Handle("/notifications/rules", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.CreateNotificationRule))).Methods("POST")

	// Update notification rule
	admin.Handle("/notifications/rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.UpdateNotificationRule))).Methods("PUT")

	// Delete notification rule
	admin.Handle("/notifications/rules/{id}", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.DeleteNotificationRule))).Methods("DELETE")

	// Toggle notification rule active status
	admin.Handle("/notifications/rules/{id}/toggle", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.ToggleNotificationRule))).Methods("PATCH")

	// Get notification statistics
	admin.Handle("/notifications/stats", middleware.RequirePermission("manage_notifications")(
		http.HandlerFunc(adminHandler.GetNotificationStats))).Methods("GET")
}
