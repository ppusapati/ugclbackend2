package utils

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// WebhookDeliveryConfig holds configuration for webhook deliveries
type WebhookDeliveryConfig struct {
	DefaultTimeout    time.Duration
	DefaultMaxRetries int
	RetryDelays       map[int]time.Duration // Attempt number -> delay
}

// DefaultWebhookConfig returns default webhook configuration
func DefaultWebhookConfig() *WebhookDeliveryConfig {
	return &WebhookDeliveryConfig{
		DefaultTimeout:    10 * time.Second,
		DefaultMaxRetries: 5,
		RetryDelays: map[int]time.Duration{
			1: 30 * time.Second, // 30 seconds
			2: 2 * time.Minute,  // 2 minutes
			3: 5 * time.Minute,  // 5 minutes
			4: 15 * time.Minute, // 15 minutes
			5: 1 * time.Hour,    // 1 hour
		},
	}
}

// GenerateHMACSignature generates HMAC-SHA256 signature for webhook payload
func GenerateHMACSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateUUID returns a random UUID string.
func GenerateUUID() string {
	return uuid.NewString()
}

// GenerateRandomString returns a secure random hexadecimal string.
func GenerateRandomString(length int) string {
	if length <= 0 {
		return ""
	}

	byteLen := (length + 1) / 2
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return uuid.NewString()
	}

	s := hex.EncodeToString(b)
	if len(s) > length {
		return s[:length]
	}
	return s
}

// VerifyHMACSignature verifies webhook signature
func VerifyHMACSignature(payload []byte, signature string, secret string) bool {
	expected := GenerateHMACSignature(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// WebhookDeliveryRequest represents an HTTP request to deliver a webhook
type WebhookDeliveryRequest struct {
	URL        string
	Payload    interface{}
	Secret     string
	Headers    map[string]string
	Timeout    time.Duration
	Attempt    int
	MaxRetries int
}

// SendWebhook sends an HTTP POST request to deliver webhook
func SendWebhook(req *WebhookDeliveryRequest) (*http.Response, error) {
	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Generate signature
	signature := GenerateHMACSignature(payloadBytes, req.Secret)
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Create HTTP request
	httpReq, err := http.NewRequest(http.MethodPost, req.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Webhook-Signature", signature)
	httpReq.Header.Set("X-Webhook-Timestamp", timestamp)
	httpReq.Header.Set("X-Webhook-Delivery-ID", GenerateUUID())
	httpReq.Header.Set("X-Webhook-Attempt", fmt.Sprintf("%d", req.Attempt))
	httpReq.Header.Set("X-Webhook-Max-Retries", fmt.Sprintf("%d", req.MaxRetries))
	httpReq.Header.Set("User-Agent", "UGCL-Webhook-Engine/1.0")

	// Add custom headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: req.Timeout,
	}

	// Send request
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// ParseWebhookResponse extracts response body from HTTP response
func ParseWebhookResponse(resp *http.Response) (string, error) {
	if resp == nil {
		return "", fmt.Errorf("response is nil")
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// CalculateNextRetry calculates the next retry time with exponential backoff
func CalculateNextRetry(attempt int, config *WebhookDeliveryConfig) *time.Time {
	var delay time.Duration

	if customDelay, exists := config.RetryDelays[attempt]; exists {
		delay = customDelay
	} else {
		// Exponential backoff: 2^attempt minutes (capped at 1 hour)
		exponential := time.Duration(1<<(attempt-1)) * time.Minute
		if exponential > time.Hour {
			exponential = time.Hour
		}
		delay = exponential
	}

	nextRetry := time.Now().Add(delay)
	return &nextRetry
}

// IsRetryableStatusCode determines if an HTTP status code is retryable
func IsRetryableStatusCode(statusCode int) bool {
	// Retry on server errors and timeout-like scenarios
	retryableCodes := map[int]bool{
		408: true, // Request Timeout
		429: true, // Too Many Requests
		500: true, // Internal Server Error
		502: true, // Bad Gateway
		503: true, // Service Unavailable
		504: true, // Gateway Timeout
	}
	return retryableCodes[statusCode]
}

// IsSuccessStatusCode checks if HTTP status code indicates success
func IsSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
