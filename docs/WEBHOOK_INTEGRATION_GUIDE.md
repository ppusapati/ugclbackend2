# Webhook System Integration Guide

## Integration Steps

This guide explains how to integrate the webhook system into your UGCL backend application.

## Step 1: Database Migration

Add webhook table creation to your database initialization:

### Option A: Add to config/migrations.go

```go
package config

import (
	"github.com/UGCL/backend/migrations"
	"gorm.io/gorm"
)

func InitDatabase() (*gorm.DB, error) {
	// ... existing database setup code ...

	// Create webhook tables
	if err := migrations.CreateWebhookTables(db); err != nil {
		return nil, fmt.Errorf("failed to create webhook tables: %w", err)
	}

	return db, nil
}
```

### Option B: Using GORM AutoMigrate in main.go

```go
import "github.com/UGCL/backend/models"

// After connecting to database
db.AutoMigrate(
	&models.Webhook{},
	&models.WebhookDelivery{},
	&models.WebhookLog{},
)

// Create indexes
db.Exec(`
	CREATE INDEX IF NOT EXISTS idx_webhooks_business_id_is_active 
	ON webhooks(business_id, is_active);
	
	CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id_status 
	ON webhook_deliveries(webhook_id, status);
`)
```

## Step 2: Register Webhook Routes

Update your `routes/routes.go` or main route registration:

```go
package routes

import (
	"github.com/gin-gonic/gin"
)

func RegisterAllRoutes(router *gin.Engine) {
	// ... existing routes ...
	
	// Register webhook routes
	RegisterWebhookRoutes(router)
	
	// ... other routes ...
}
```

## Step 3: Start Webhook Services (main.go)

Add webhook retry scheduler to your main.go:

```go
package main

import (
	"time"
	"github.com/UGCL/backend/utils"
	"log"
)

func main() {
	// ... existing initialization ...

	// Start webhook retry scheduler (runs every 5 minutes)
	webhookRetryScheduler := utils.NewWebhookRetryScheduler(5 * time.Minute)
	webhookRetryScheduler.Start()

	// Optional: Start webhook cleanup scheduler (runs daily, keeps 90 days of data)
	webhookCleanupScheduler := utils.NewWebhookCleanupScheduler(24 * time.Hour, 90)
	webhookCleanupScheduler.Start()

	// ... rest of initialization ...

	// Graceful shutdown
	defer func() {
		webhookRetryScheduler.Stop()
		webhookCleanupScheduler.Stop()
	}()
}
```

## Step 4: Add Webhook Trigger Middleware to API Routes

Apply the auto-trigger webhook middleware to your protected API routes:

### Option A: Global Middleware (Recommended for all routes)

```go
// In routes/routes.go or main.go
apiRoutes := router.Group("/api/v1")
apiRoutes.Use(middleware.SecurityMiddleware)
apiRoutes.Use(middleware.JWTMiddleware)
apiRoutes.Use(middleware.AutoTriggerWebhookMiddleware)  // Add this

// Register API routes here
registerUserRoutes(apiRoutes)
registerBusinessRoutes(apiRoutes)
registerSiteRoutes(apiRoutes)
// ... etc
```

### Option B: Per-Route Middleware (More control)

For specific routes that should trigger webhooks:

```go
// User routes
apiRoutes.POST("/users", 
	middleware.WebhookEventTriggerMiddleware(models.EventCreate, "User"),
	handlers.CreateUser)

apiRoutes.PUT("/users/:id", 
	middleware.WebhookEventTriggerMiddleware(models.EventUpdate, "User"),
	handlers.UpdateUser)

// Site routes
apiRoutes.POST("/sites", 
	middleware.WebhookEventTriggerMiddleware(models.EventCreate, "Site"),
	handlers.CreateSite)

apiRoutes.PUT("/sites/:id", 
	middleware.WebhookEventTriggerMiddleware(models.EventUpdate, "Site"),
	handlers.UpdateSite)
```

## Step 5: Enable Helper Functions

Ensure the middleware helper function exists in `middleware/helpers.go`:

```go
package middleware

import (
	"github.com/gin-gonic/gin"
)

// GetBusinessIDFromContext extracts business ID from JWT claims
func GetBusinessIDFromContext(c *gin.Context) (uint, bool) {
	businessIDInterface, exists := c.Get("business_id")
	if !exists {
		return 0, false
	}

	businessID, ok := businessIDInterface.(uint)
	if !ok {
		return 0, false
	}

	return businessID, true
}
```

## Step 6: Configure Environment Variables (Optional)

Add to your `.env` file:

```env
# Webhook Configuration
WEBHOOK_RETRY_INTERVAL=5           # Minutes between retry attempts
WEBHOOK_CLEANUP_INTERVAL=24        # Hours between cleanup runs
WEBHOOK_DATA_RETENTION_DAYS=90     # Days to keep webhook data
WEBHOOK_TIMEOUT=10                 # Seconds timeout for webhook requests
WEBHOOK_MAX_RETRIES=5              # Maximum retry attempts
```

## Step 7: Update go.mod Dependencies

Ensure you have required dependencies:

```
go get -u github.com/gin-gonic/gin
go get -u gorm.io/gorm
go get -u gorm.io/datatypes
```

## Usage Examples

### Creating a Webhook from Your Backend Code

```go
import (
	"github.com/UGCL/backend/config"
	"github.com/UGCL/backend/models"
	"github.com/UGCL/backend/utils"
)

// Example: Manually trigger a webhook
func NotifyExternalSystem(resourceID string, businessID uint, data map[string]interface{}) error {
	webhookService := utils.NewWebhookService(config.DB)
	
	return webhookService.TriggerWebhook(
		models.EventCreate,
		"User",
		resourceID,
		businessID,
		data,
	)
}
```

### Accessing Webhook Status from Code

```go
func CheckWebhookStatus(webhookID uint) error {
	webhookService := utils.NewWebhookService(config.DB)
	
	webhook, err := webhookService.GetWebhook(webhookID)
	if err != nil {
		return err
	}
	
	log.Printf("Webhook status: %v", webhook.Status)
	
	// Get delivery history
	deliveries, err := webhookService.GetDeliveryHistory(webhookID, 10)
	if err != nil {
		return err
	}
	
	for _, delivery := range deliveries {
		log.Printf("Attempt %d: %v", delivery.Attempt, delivery.Status)
	}
	
	return nil
}
```

## Testing the Integration

### 1. Test Webhook Creation

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://webhook.site/your-unique-id",
    "events": ["CREATE", "UPDATE"],
    "resource_types": ["User", "Site"],
    "secret": "my-secret-key"
  }'
```

### 2. Test Webhook Delivery

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/1/test \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 3. Monitor Deliveries

```bash
curl -X GET http://localhost:8080/api/v1/webhooks/1/deliveries?limit=10 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Production Deployment Checklist

- [ ] Database migrations completed
- [ ] Webhook routes registered
- [ ] Retry scheduler started in main.go
- [ ] Cleanup scheduler started (optional but recommended)
- [ ] Webhook middleware added to all relevant routes
- [ ] Environment variables configured
- [ ] HTTPS enabled on webhook URLs
- [ ] Firewall allows outbound HTTPS connections
- [ ] Monitoring/alerting set up for webhook failures
- [ ] Tested with test webhook endpoint
- [ ] Documented webhook secret rotation procedure
- [ ] Backup strategy for webhook tables included

## Monitoring and Maintenance

### Check Webhook Health
```sql
-- Active webhooks
SELECT id, business_id, url, status, is_active, created_at 
FROM webhooks 
WHERE is_active = true;

-- Failed deliveries
SELECT id, webhook_id, resource_type, status, attempt, error, created_at 
FROM webhook_deliveries 
WHERE status = 'FAILED' 
ORDER BY created_at DESC 
LIMIT 10;

-- Pending retries
SELECT id, webhook_id, resource_type, next_retry_at 
FROM webhook_deliveries 
WHERE status = 'RETRY_SCHEDULED' AND next_retry_at <= NOW();
```

### Archive Old Data
```sql
-- Archive webhooks older than 90 days (optional)
DELETE FROM webhook_logs 
WHERE created_at < NOW() - INTERVAL '90 days';

DELETE FROM webhook_deliveries 
WHERE created_at < NOW() - INTERVAL '90 days' 
AND status IN ('SUCCESS', 'FAILED');
```

## Troubleshooting

### Webhooks Not Triggering
1. Verify webhook is active: `is_active = true`
2. Check middleware is applied to your routes
3. Verify business_id is correctly extracted
4. Check logs for middleware errors

### High Failure Rate
1. Check if target endpoint is responding
2. Verify HMAC signature calculation is correct
3. Check network connectivity
4. Review error messages in webhook_deliveries table

### Performance Issues
1. Ensure indexes are created on webhook tables
2. Archive old webhook data regularly
3. Monitor retry scheduler frequency
4. Consider async processing for large payloads

## Security Considerations

1. **Secret Management**: Store webhook secrets securely, never in logs
2. **HTTPS Only**: Always use HTTPS for webhook URLs
3. **Rate Limiting**: Implement rate limiting on your webhook endpoint
4. **Payload Whitelist**: Only include necessary data in webhooks
5. **Signature Verification**: Always verify HMAC signatures on consumer side
6. **IP Whitelisting**: Consider restricting webhook IPs if possible

## Support and Questions

Refer to [WEBHOOK_DOCUMENTATION.md](WEBHOOK_DOCUMENTATION.md) for API reference and C# integration examples.
