package migrations

import (
	"p9e.in/ugcl/models"
	"gorm.io/gorm"
)

// CreateWebhookTables creates webhook-related tables
func CreateWebhookTables(db *gorm.DB) error {
	// Auto migrate webhook models
	if err := db.AutoMigrate(
		&models.Webhook{},
		&models.WebhookDelivery{},
		&models.WebhookLog{},
	); err != nil {
		return err
	}

	// Create indexes for better query performance
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_webhooks_business_id_is_active 
		ON webhooks(business_id, is_active);
		
		CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id_status 
		ON webhook_deliveries(webhook_id, status);
		
		CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status_next_retry 
		ON webhook_deliveries(status, next_retry_at) 
		WHERE status = 'RETRY_SCHEDULED';
		
		CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at 
		ON webhook_deliveries(created_at);
		
		CREATE INDEX IF NOT EXISTS idx_webhook_logs_webhook_id 
		ON webhook_logs(webhook_id);
		
		CREATE INDEX IF NOT EXISTS idx_webhook_logs_delivery_id 
		ON webhook_logs(delivery_id);
		
		CREATE INDEX IF NOT EXISTS idx_webhook_logs_created_at 
		ON webhook_logs(created_at);
	`).Error; err != nil {
		return err
	}

	return nil
}

// DropWebhookTables drops webhook-related tables
func DropWebhookTables(db *gorm.DB) error {
	return db.Migrator().DropTable(
		&models.WebhookLog{},
		&models.WebhookDelivery{},
		&models.Webhook{},
	)
}
