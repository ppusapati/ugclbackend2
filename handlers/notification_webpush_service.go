package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"p9e.in/ugcl/models"
)

func (ns *NotificationService) getWebPushConfig() (publicKey, privateKey, subject string, ok bool) {
	publicKey = strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY"))
	privateKey = strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY"))
	subject = strings.TrimSpace(os.Getenv("VAPID_SUBJECT"))
	if subject == "" {
		subject = "mailto:no-reply@ugcl.local"
	}
	ok = publicKey != "" && privateKey != ""
	return
}

func (ns *NotificationService) GetWebPushPublicKey() string {
	publicKey, _, _, ok := ns.getWebPushConfig()
	if !ok {
		return ""
	}
	return publicKey
}

func (ns *NotificationService) UpsertWebPushSubscription(
	userID string,
	endpoint string,
	p256dh string,
	auth string,
	expirationTime *time.Time,
	userAgent *string,
) error {
	if endpoint == "" || p256dh == "" || auth == "" {
		return fmt.Errorf("endpoint, keys.p256dh and keys.auth are required")
	}

	var existing models.WebPushSubscription
	err := ns.db.Where("endpoint = ?", endpoint).First(&existing).Error
	if err == nil {
		existing.UserID = userID
		existing.P256DH = p256dh
		existing.Auth = auth
		existing.ExpirationTime = expirationTime
		existing.UserAgent = userAgent
		return ns.db.Save(&existing).Error
	}

	sub := models.WebPushSubscription{
		UserID:         userID,
		Endpoint:       endpoint,
		P256DH:         p256dh,
		Auth:           auth,
		ExpirationTime: expirationTime,
		UserAgent:      userAgent,
	}
	return ns.db.Create(&sub).Error
}

func (ns *NotificationService) DeleteWebPushSubscription(userID, endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	return ns.db.
		Where("user_id = ? AND endpoint = ?", userID, endpoint).
		Delete(&models.WebPushSubscription{}).Error
}

func (ns *NotificationService) SendWebPushToUser(userID, title, body, actionURL, tag string) {
	publicKey, privateKey, subject, ok := ns.getWebPushConfig()
	if !ok {
		return
	}

	var prefs models.NotificationPreference
	if err := ns.db.Where("user_id = ?", userID).First(&prefs).Error; err == nil {
		if !prefs.EnableWebPush {
			return
		}
	}

	var subscriptions []models.WebPushSubscription
	if err := ns.db.Where("user_id = ?", userID).Find(&subscriptions).Error; err != nil {
		log.Printf("⚠️ web-push: failed to load subscriptions for %s: %v", userID, err)
		return
	}
	if len(subscriptions) == 0 {
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"url":   actionURL,
		"tag":   tag,
	})

	for _, sub := range subscriptions {
		resp, err := webpush.SendNotification(payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256DH,
				Auth:   sub.Auth,
			},
		}, &webpush.Options{
			Subscriber:      subject,
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			TTL:             60,
		})

		if err != nil {
			log.Printf("⚠️ web-push: send failed for endpoint %s: %v", sub.Endpoint, err)
			continue
		}

		if resp != nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
				_ = ns.db.Where("endpoint = ?", sub.Endpoint).Delete(&models.WebPushSubscription{}).Error
				continue
			}
			if resp.StatusCode >= 400 {
				log.Printf("⚠️ web-push: endpoint %s returned status %d", sub.Endpoint, resp.StatusCode)
			}
		}
	}
}
