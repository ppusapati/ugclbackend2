package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

type savePushSubscriptionRequest struct {
	Endpoint       string `json:"endpoint"`
	ExpirationTime *int64 `json:"expirationTime"`
	Keys           struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type deletePushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
}

type saveMobilePushTokenRequest struct {
	Token      string  `json:"token"`
	Platform   string  `json:"platform"`
	DeviceID   *string `json:"device_id,omitempty"`
	DeviceName *string `json:"device_name,omitempty"`
	AppVersion *string `json:"app_version,omitempty"`
}

type deleteMobilePushTokenRequest struct {
	Token    *string `json:"token,omitempty"`
	DeviceID *string `json:"device_id,omitempty"`
}

type testPushNotificationRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

type mobilePushTokenDTO struct {
	ID           string  `json:"id"`
	Platform     string  `json:"platform"`
	DeviceID     *string `json:"device_id,omitempty"`
	DeviceName   *string `json:"device_name,omitempty"`
	AppVersion   *string `json:"app_version,omitempty"`
	IsActive     bool    `json:"is_active"`
	LastSeenAt   string  `json:"last_seen_at"`
	CreatedAt    string  `json:"created_at"`
	TokenPreview string  `json:"token_preview"`
	UserID       string  `json:"user_id,omitempty"`
}

func parseBoolQuery(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}

func maskToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 12 {
		return token
	}
	return token[:8] + "..." + token[len(token)-4:]
}

func toMobilePushTokenDTOs(tokens []models.MobilePushToken, includeUserID bool) []mobilePushTokenDTO {
	result := make([]mobilePushTokenDTO, 0, len(tokens))
	for _, token := range tokens {
		dto := mobilePushTokenDTO{
			ID:           token.ID.String(),
			Platform:     token.Platform,
			DeviceID:     token.DeviceID,
			DeviceName:   token.DeviceName,
			AppVersion:   token.AppVersion,
			IsActive:     token.IsActive,
			LastSeenAt:   token.LastSeenAt.Format(time.RFC3339),
			CreatedAt:    token.CreatedAt.Format(time.RFC3339),
			TokenPreview: maskToken(token.Token),
		}
		if includeUserID {
			dto.UserID = token.UserID
		}
		result = append(result, dto)
	}

	return result
}

// GetWebPushPublicKey returns VAPID public key for browser PushManager subscription.
// GET /api/v1/notifications/push/public-key
func (h *NotificationHandler) GetWebPushPublicKey(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	publicKey := getNotificationService().GetWebPushPublicKey()
	if publicKey == "" {
		http.Error(w, "web push not configured", http.StatusNotImplemented)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"public_key": publicKey})
}

// SaveWebPushSubscription saves or updates a browser push subscription for current user.
// POST /api/v1/notifications/push/subscriptions
func (h *NotificationHandler) SaveWebPushSubscription(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req savePushSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var expiration *time.Time
	if req.ExpirationTime != nil {
		t := time.UnixMilli(*req.ExpirationTime)
		expiration = &t
	}

	ua := strings.TrimSpace(r.Header.Get("User-Agent"))
	var userAgent *string
	if ua != "" {
		userAgent = &ua
	}

	if err := getNotificationService().UpsertWebPushSubscription(
		claims.UserID,
		strings.TrimSpace(req.Endpoint),
		strings.TrimSpace(req.Keys.P256DH),
		strings.TrimSpace(req.Keys.Auth),
		expiration,
		userAgent,
	); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "push subscription saved"})
}

// DeleteWebPushSubscription removes a saved browser push subscription for current user.
// DELETE /api/v1/notifications/push/subscriptions
func (h *NotificationHandler) DeleteWebPushSubscription(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))
	if endpoint == "" {
		var req deletePushSubscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			endpoint = strings.TrimSpace(req.Endpoint)
		}
	}

	if err := getNotificationService().DeleteWebPushSubscription(claims.UserID, endpoint); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "push subscription removed"})
}

// SaveMobilePushToken registers or updates a device token for Android/iOS mobile push notifications.
// POST /api/v1/notifications/push/mobile/tokens
func (h *NotificationHandler) SaveMobilePushToken(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req saveMobilePushTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := getNotificationService().UpsertMobilePushToken(
		claims.UserID,
		strings.TrimSpace(req.Token),
		strings.TrimSpace(req.Platform),
		req.DeviceID,
		req.DeviceName,
		req.AppVersion,
	); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "mobile push token saved"})
}

// DeleteMobilePushToken removes a registered mobile push token for the current user.
// DELETE /api/v1/notifications/push/mobile/tokens
func (h *NotificationHandler) DeleteMobilePushToken(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))

	if token == "" && deviceID == "" {
		var req deleteMobilePushTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.Token != nil {
				token = strings.TrimSpace(*req.Token)
			}
			if req.DeviceID != nil {
				deviceID = strings.TrimSpace(*req.DeviceID)
			}
		}
	}

	var tokenPtr *string
	if token != "" {
		tokenPtr = &token
	}
	var deviceIDPtr *string
	if deviceID != "" {
		deviceIDPtr = &deviceID
	}

	if err := getNotificationService().DeleteMobilePushToken(claims.UserID, tokenPtr, deviceIDPtr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "mobile push token removed"})
}

// GetMobilePushTokens returns registered mobile push tokens for the current user.
// GET /api/v1/notifications/push/mobile/tokens
func (h *NotificationHandler) GetMobilePushTokens(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	includeInactive := parseBoolQuery(r.URL.Query().Get("include_inactive"))
	tokens, err := getNotificationService().ListMobilePushTokens(claims.UserID, includeInactive)
	if err != nil {
		http.Error(w, "failed to load mobile push tokens", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tokens":           toMobilePushTokenDTOs(tokens, false),
		"count":            len(tokens),
		"include_inactive": includeInactive,
	})
}

// GetMobilePushTokensForAdmin returns mobile push tokens for a specified user for QA/debugging.
// GET /api/v1/admin/notifications/push/mobile-tokens?user_id={id}
func (h *NotificationHandler) GetMobilePushTokensForAdmin(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	includeInactive := parseBoolQuery(r.URL.Query().Get("include_inactive"))
	tokens, err := getNotificationService().ListMobilePushTokens(userID, includeInactive)
	if err != nil {
		http.Error(w, "failed to load mobile push tokens", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":          userID,
		"tokens":           toMobilePushTokenDTOs(tokens, true),
		"count":            len(tokens),
		"include_inactive": includeInactive,
	})
}

// SendTestWebPush sends a test web push notification to the current user.
// POST /api/v1/notifications/push/test
func (h *NotificationHandler) SendTestWebPush(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req testPushNotificationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "UGCL Push Test"
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		body = "Push notifications are active for this browser."
	}

	url := strings.TrimSpace(req.URL)
	if url == "" {
		url = "/chat"
	}

	getNotificationService().SendWebPushToUser(
		claims.UserID,
		title,
		body,
		url,
		"push-test-"+claims.UserID,
	)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "test push dispatched"})
}

// SendTestMobilePush sends a test mobile push notification to the current user's registered devices.
// POST /api/v1/notifications/push/mobile/test
func (h *NotificationHandler) SendTestMobilePush(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req testPushNotificationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "UGCL Mobile Push Test"
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		body = "Mobile push notifications are active for this device."
	}

	url := strings.TrimSpace(req.URL)
	if url == "" {
		url = "/chat"
	}

	dispatched, err := getNotificationService().SendTestMobilePushToUser(claims.UserID, title, body, url)
	if err != nil {
		http.Error(w, "failed to dispatch mobile push test", http.StatusInternalServerError)
		return
	}
	if dispatched == 0 {
		http.Error(w, "no active mobile push tokens registered for current user", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":           "test mobile push dispatched",
		"dispatched_tokens": dispatched,
	})
}
