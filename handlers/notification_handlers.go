package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// NotificationHandler handles notification operations
type NotificationHandler struct{}

var notificationService = NewNotificationService()

// GetNotifications retrieves notifications for the current user
// GET /api/v1/notifications
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	filters := make(map[string]interface{})

	if notifType := r.URL.Query().Get("type"); notifType != "" {
		filters["type"] = notifType
	}
	if priority := r.URL.Query().Get("priority"); priority != "" {
		filters["priority"] = priority
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}
	if read := r.URL.Query().Get("read"); read != "" {
		filters["read"] = read == "true"
	}
	if formCode := r.URL.Query().Get("form_code"); formCode != "" {
		filters["form_code"] = formCode
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filters["limit"] = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filters["offset"] = o
		}
	}

	// Get notifications
	notifications, err := notificationService.GetNotificationsForUser(claims.UserID, filters)
	if err != nil {
		log.Printf("❌ Error fetching notifications: %v", err)
		http.Error(w, "failed to fetch notifications", http.StatusInternalServerError)
		return
	}

	// Get unread count
	unreadCount, _ := notificationService.GetUnreadCount(claims.UserID)

	// Convert to DTOs
	dtos := make([]models.NotificationDTO, len(notifications))
	for i, notif := range notifications {
		dtos[i] = notif.ToDTO()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notifications": dtos,
		"count":         len(dtos),
		"unread_count":  unreadCount,
		"has_more":      len(dtos) == filters["limit"].(int) || len(dtos) == 50,
	})
}

// GetNotification retrieves a specific notification
// GET /api/v1/notifications/:id
func (h *NotificationHandler) GetNotification(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	notificationID := vars["id"]

	id, err := uuid.Parse(notificationID)
	if err != nil {
		http.Error(w, "invalid notification ID", http.StatusBadRequest)
		return
	}

	var notification models.Notification
	if err := notificationService.db.First(&notification, id).Error; err != nil {
		http.Error(w, "notification not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if notification.UserID != claims.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notification": notification.ToDTO(),
	})
}

// MarkNotificationAsRead marks a notification as read
// PATCH /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	notificationID := vars["id"]

	id, err := uuid.Parse(notificationID)
	if err != nil {
		http.Error(w, "invalid notification ID", http.StatusBadRequest)
		return
	}

	var notification models.Notification
	if err := notificationService.db.First(&notification, id).Error; err != nil {
		http.Error(w, "notification not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if notification.UserID != claims.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Mark as read
	notification.MarkAsRead()
	if err := notificationService.db.Save(&notification).Error; err != nil {
		log.Printf("❌ Error marking notification as read: %v", err)
		http.Error(w, "failed to mark as read", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "notification marked as read",
		"notification": notification.ToDTO(),
	})
}

// MarkAllNotificationsAsRead marks all notifications as read for the current user
// PATCH /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	result := notificationService.db.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", claims.UserID).
		Update("read_at", "NOW()")

	if result.Error != nil {
		log.Printf("❌ Error marking all notifications as read: %v", result.Error)
		http.Error(w, "failed to mark all as read", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "all notifications marked as read",
		"count":   result.RowsAffected,
	})
}

// DeleteNotification deletes a notification
// DELETE /api/v1/notifications/:id
func (h *NotificationHandler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	notificationID := vars["id"]

	id, err := uuid.Parse(notificationID)
	if err != nil {
		http.Error(w, "invalid notification ID", http.StatusBadRequest)
		return
	}

	var notification models.Notification
	if err := notificationService.db.First(&notification, id).Error; err != nil {
		http.Error(w, "notification not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if notification.UserID != claims.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := notificationService.db.Delete(&notification).Error; err != nil {
		log.Printf("❌ Error deleting notification: %v", err)
		http.Error(w, "failed to delete notification", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetUnreadCount returns the count of unread notifications
// GET /api/v1/notifications/unread-count
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	count, err := notificationService.GetUnreadCount(claims.UserID)
	if err != nil {
		log.Printf("❌ Error getting unread count: %v", err)
		http.Error(w, "failed to get unread count", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": count,
	})
}

// GetNotificationPreferences returns user's notification preferences
// GET /api/v1/notifications/preferences
func (h *NotificationHandler) GetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var prefs models.NotificationPreference
	if err := notificationService.db.Where("user_id = ?", claims.UserID).First(&prefs).Error; err != nil {
		// Create default preferences if not found
		prefs = models.NotificationPreference{
			UserID:        claims.UserID,
			EnableInApp:   true,
			EnableEmail:   true,
			EnableSMS:     false,
			EnableWebPush: true,
			DisabledTypes: []string{},
		}
		notificationService.db.Create(&prefs)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"preferences": prefs,
	})
}

// UpdateNotificationPreferences updates user's notification preferences
// PUT /api/v1/notifications/preferences
func (h *NotificationHandler) UpdateNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		EnableInApp        *bool    `json:"enable_in_app"`
		EnableEmail        *bool    `json:"enable_email"`
		EnableSMS          *bool    `json:"enable_sms"`
		EnableWebPush      *bool    `json:"enable_web_push"`
		DisabledTypes      []string `json:"disabled_types"`
		QuietHoursEnabled  *bool    `json:"quiet_hours_enabled"`
		QuietHoursStart    *string  `json:"quiet_hours_start"`
		QuietHoursEnd      *string  `json:"quiet_hours_end"`
		DigestEnabled      *bool    `json:"digest_enabled"`
		DigestFrequency    *string  `json:"digest_frequency"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get or create preferences
	var prefs models.NotificationPreference
	if err := notificationService.db.Where("user_id = ?", claims.UserID).First(&prefs).Error; err != nil {
		prefs = models.NotificationPreference{UserID: claims.UserID}
	}

	// Update fields
	if req.EnableInApp != nil {
		prefs.EnableInApp = *req.EnableInApp
	}
	if req.EnableEmail != nil {
		prefs.EnableEmail = *req.EnableEmail
	}
	if req.EnableSMS != nil {
		prefs.EnableSMS = *req.EnableSMS
	}
	if req.EnableWebPush != nil {
		prefs.EnableWebPush = *req.EnableWebPush
	}
	if req.DisabledTypes != nil {
		prefs.DisabledTypes = req.DisabledTypes
	}
	if req.QuietHoursEnabled != nil {
		prefs.QuietHoursEnabled = *req.QuietHoursEnabled
	}
	if req.QuietHoursStart != nil {
		prefs.QuietHoursStart = *req.QuietHoursStart
	}
	if req.QuietHoursEnd != nil {
		prefs.QuietHoursEnd = *req.QuietHoursEnd
	}
	if req.DigestEnabled != nil {
		prefs.DigestEnabled = *req.DigestEnabled
	}
	if req.DigestFrequency != nil {
		prefs.DigestFrequency = *req.DigestFrequency
	}

	// Save
	if err := notificationService.db.Save(&prefs).Error; err != nil {
		log.Printf("❌ Error saving preferences: %v", err)
		http.Error(w, "failed to save preferences", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     "preferences updated successfully",
		"preferences": prefs,
	})
}

// StreamNotifications streams notifications via Server-Sent Events
// GET /api/v1/notifications/stream
func (h *NotificationHandler) StreamNotifications(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a channel for new notifications
	// In production, this would use a pub/sub system like Redis
	// For now, we'll just keep the connection open
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial message
	w.Write([]byte("data: {\"type\":\"connected\"}\n\n"))
	flusher.Flush()

	// Keep connection alive
	// In production, implement proper SSE with channels/pub-sub
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send heartbeat
			w.Write([]byte("data: {\"type\":\"heartbeat\"}\n\n"))
			flusher.Flush()
		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}
