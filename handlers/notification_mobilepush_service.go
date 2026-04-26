package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

var (
	firebaseMessagingOnce    sync.Once
	firebaseMessagingClient  *messaging.Client
	firebaseMessagingErr     error
	mobilePushUnavailableLog sync.Once
)

func (ns *NotificationService) GetMobilePushConfigurationStatus() (bool, string) {
	_, err := ns.getFirebaseMessagingClient()
	if err != nil {
		return false, err.Error()
	}
	return true, "configured"
}

func (ns *NotificationService) getFirebaseMessagingClient() (*messaging.Client, error) {
	firebaseMessagingOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		jsonCreds := strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"))
		fileCreds := strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_FILE"))

		var (
			app *firebase.App
			err error
		)

		switch {
		case jsonCreds != "":
			app, err = firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(jsonCreds)))
		case fileCreds != "":
			app, err = firebase.NewApp(ctx, nil, option.WithCredentialsFile(fileCreds))
		default:
			firebaseMessagingErr = fmt.Errorf("mobile push disabled: FIREBASE_SERVICE_ACCOUNT_JSON or FIREBASE_SERVICE_ACCOUNT_FILE is not configured")
			return
		}

		if err != nil {
			firebaseMessagingErr = fmt.Errorf("failed to initialize firebase app: %w", err)
			return
		}

		firebaseMessagingClient, firebaseMessagingErr = app.Messaging(ctx)
		if firebaseMessagingErr != nil {
			firebaseMessagingErr = fmt.Errorf("failed to create firebase messaging client: %w", firebaseMessagingErr)
		}
	})

	if firebaseMessagingErr != nil {
		return nil, firebaseMessagingErr
	}
	return firebaseMessagingClient, nil
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePlatform(platform string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	switch normalized {
	case "android", "ios":
		return normalized, nil
	default:
		return "", fmt.Errorf("platform must be either 'android' or 'ios'")
	}
}

func (ns *NotificationService) UpsertMobilePushToken(
	userID string,
	token string,
	platform string,
	deviceID *string,
	deviceName *string,
	appVersion *string,
) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}

	normalizedPlatform, err := normalizePlatform(platform)
	if err != nil {
		return err
	}

	now := time.Now()
	deviceID = normalizeOptionalString(deviceID)
	deviceName = normalizeOptionalString(deviceName)
	appVersion = normalizeOptionalString(appVersion)

	var existing models.MobilePushToken
	err = ns.db.Where("token = ?", token).First(&existing).Error
	if err == nil {
		existing.UserID = userID
		existing.Platform = normalizedPlatform
		existing.DeviceID = deviceID
		existing.DeviceName = deviceName
		existing.AppVersion = appVersion
		existing.IsActive = true
		existing.LastSeenAt = now
		return ns.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	record := models.MobilePushToken{
		UserID:     userID,
		Token:      token,
		Platform:   normalizedPlatform,
		DeviceID:   deviceID,
		DeviceName: deviceName,
		AppVersion: appVersion,
		IsActive:   true,
		LastSeenAt: now,
	}
	return ns.db.Create(&record).Error
}

func (ns *NotificationService) DeleteMobilePushToken(userID string, token *string, deviceID *string) error {
	token = normalizeOptionalString(token)
	deviceID = normalizeOptionalString(deviceID)

	if token == nil && deviceID == nil {
		return fmt.Errorf("either token or device_id is required")
	}

	query := ns.db.Where("user_id = ?", userID)
	if token != nil {
		query = query.Where("token = ?", *token)
	}
	if deviceID != nil {
		query = query.Where("device_id = ?", *deviceID)
	}

	return query.Delete(&models.MobilePushToken{}).Error
}

func (ns *NotificationService) ListMobilePushTokens(userID string, includeInactive bool) ([]models.MobilePushToken, error) {
	query := ns.db.Where("user_id = ?", userID)
	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}

	var tokens []models.MobilePushToken
	if err := query.Order("last_seen_at DESC, created_at DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}

	return tokens, nil
}

func (ns *NotificationService) CountActiveMobilePushTokens(userID string) (int64, error) {
	var count int64
	if err := ns.db.Model(&models.MobilePushToken{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (ns *NotificationService) isMobilePushEnabled(userID string, notifType models.NotificationType) bool {
	var prefs models.NotificationPreference
	if err := ns.db.Where("user_id = ?", userID).First(&prefs).Error; err != nil {
		return true
	}

	if !prefs.EnableMobilePush {
		return false
	}

	for _, disabledType := range prefs.DisabledTypes {
		if disabledType == string(notifType) {
			return false
		}
	}

	return true
}

func (ns *NotificationService) markTokenInactive(token string) {
	_ = ns.db.Model(&models.MobilePushToken{}).
		Where("token = ?", token).
		Updates(map[string]interface{}{
			"is_active":    false,
			"updated_at":   time.Now(),
			"last_seen_at": time.Now(),
		}).Error
}

func (ns *NotificationService) SendMobilePushToUser(
	userID string,
	notifType models.NotificationType,
	title string,
	body string,
	data map[string]string,
) {
	if !ns.isMobilePushEnabled(userID, notifType) {
		return
	}

	client, err := ns.getFirebaseMessagingClient()
	if err != nil {
		// Keep this as informational so mobile push can be opt-in via env without breaking app flow.
		mobilePushUnavailableLog.Do(func() {
			log.Printf("ℹ️ mobile push unavailable: %v", err)
		})
		return
	}

	var tokens []string
	if err := ns.db.Model(&models.MobilePushToken{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Pluck("token", &tokens).Error; err != nil {
		log.Printf("⚠️ mobile push: failed to load device tokens for user %s: %v", userID, err)
		return
	}

	if len(tokens) == 0 {
		return
	}

	if data == nil {
		data = map[string]string{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound:            "default",
					ContentAvailable: true,
				},
			},
		},
	}

	resp, err := client.SendEachForMulticast(ctx, msg)
	if err != nil {
		log.Printf("⚠️ mobile push: send failed for user %s: %v", userID, err)
		return
	}

	if resp.FailureCount == 0 {
		return
	}

	for i, r := range resp.Responses {
		if r.Success || i >= len(tokens) {
			continue
		}

		errText := strings.ToLower(r.Error.Error())
		if strings.Contains(errText, "registration-token-not-registered") ||
			strings.Contains(errText, "invalid registration token") ||
			strings.Contains(errText, "invalid-argument") {
			ns.markTokenInactive(tokens[i])
		}
	}
}

func (ns *NotificationService) SendTestMobilePushToUser(userID, title, body, actionURL string) (int64, error) {
	if _, err := ns.getFirebaseMessagingClient(); err != nil {
		return 0, err
	}

	count, err := ns.CountActiveMobilePushTokens(userID)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}

	ns.SendMobilePushToUser(
		userID,
		models.NotificationTypeSystemAlert,
		title,
		body,
		map[string]string{
			"type":       string(models.NotificationTypeSystemAlert),
			"action_url": actionURL,
			"source":     "mobile_push_test",
		},
	)

	return count, nil
}
