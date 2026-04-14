package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// IntegrationHealth returns service health for third-party integrations.
func IntegrationHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"service":   "integration-api",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// WebhookContract returns webhook delivery contract details for provider implementations.
func WebhookContract(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"delivery_headers": []string{
			"X-Webhook-Signature",
			"X-Webhook-Delivery-ID",
			"X-Webhook-Attempt",
			"X-Webhook-Max-Retries",
			"X-Webhook-Timestamp",
		},
		"signature_algorithm": "HMAC-SHA256",
		"timestamp_format":    "RFC3339",
		"notes": []string{
			"Validate X-Webhook-Signature using shared secret.",
			"Reject stale or replayed events using X-Webhook-Timestamp and delivery ID.",
			"Return 2xx only when payload is successfully processed.",
		},
	})
}
