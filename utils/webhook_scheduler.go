package utils

import (
	"log"
	"time"

	"github.com/UGCL/backend/config"
)

// WebhookRetryScheduler manages periodic webhook retry attempts
type WebhookRetryScheduler struct {
	webhookService *WebhookService
	interval       time.Duration
	stopChan       chan struct{}
}

// NewWebhookRetryScheduler creates a new webhook retry scheduler
func NewWebhookRetryScheduler(interval time.Duration) *WebhookRetryScheduler {
	return &WebhookRetryScheduler{
		webhookService: NewWebhookService(config.DB),
		interval:       interval,
		stopChan:       make(chan struct{}),
	}
}

// Start starts the webhook retry scheduler
func (scheduler *WebhookRetryScheduler) Start() {
	go func() {
		ticker := time.NewTicker(scheduler.interval)
		defer ticker.Stop()

		for {
			select {
			case <-scheduler.stopChan:
				log.Println("Webhook retry scheduler stopped")
				return
			case <-ticker.C:
				if err := scheduler.webhookService.RetryFailedDeliveries(); err != nil {
					log.Printf("Error retrying webhook deliveries: %v", err)
				}
			}
		}
	}()

	log.Printf("Webhook retry scheduler started with interval: %v", scheduler.interval)
}

// Stop stops the webhook retry scheduler
func (scheduler *WebhookRetryScheduler) Stop() {
	close(scheduler.stopChan)
}

// WebhookCleanupScheduler manages periodic cleanup of old webhook records
type WebhookCleanupScheduler struct {
	db           interface{} // *gorm.DB
	interval     time.Duration
	retentionDays int
	stopChan     chan struct{}
}

// NewWebhookCleanupScheduler creates a new webhook cleanup scheduler
func NewWebhookCleanupScheduler(interval time.Duration, retentionDays int) *WebhookCleanupScheduler {
	return &WebhookCleanupScheduler{
		db:            config.DB,
		interval:      interval,
		retentionDays: retentionDays,
		stopChan:      make(chan struct{}),
	}
}

// Start starts the webhook cleanup scheduler
func (scheduler *WebhookCleanupScheduler) Start() {
	go func() {
		ticker := time.NewTicker(scheduler.interval)
		defer ticker.Stop()

		for {
			select {
			case <-scheduler.stopChan:
				log.Println("Webhook cleanup scheduler stopped")
				return
			case <-ticker.C:
				// Clean up old webhook logs and failed deliveries
				cutoffDate := time.Now().AddDate(0, 0, -scheduler.retentionDays)

				// Note: You'll need to import gorm.DB to execute this
				// This is a placeholder - implement actual cleanup logic
				log.Printf("Running webhook cleanup for records older than %v", cutoffDate)
			}
		}
	}()

	log.Printf("Webhook cleanup scheduler started with retention: %d days", scheduler.retentionDays)
}

// Stop stops the webhook cleanup scheduler
func (scheduler *WebhookCleanupScheduler) Stop() {
	close(scheduler.stopChan)
}
