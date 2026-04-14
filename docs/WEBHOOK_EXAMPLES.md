# Webhook System Examples

This document provides concrete examples for implementing webhooks in your backend.

## Backend Implementation Examples

### Example 1: Auto-Trigger on User Creation

```go
// handlers/user_handlers.go

func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	businessID, _ := middleware.GetBusinessIDFromContext(c)

	// Create user
	user := &models.User{
		Email:      req.Email,
		Name:       req.Name,
		BusinessID: businessID,
	}

	if err := config.DB.Create(user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store response data for middleware
	c.Set("resource_id", fmt.Sprintf("%d", user.ID))
	c.Set("response_data", map[string]interface{}{
		"id":    user.ID,
		"email": user.Email,
		"name":  user.Name,
	})

	c.JSON(http.StatusCreated, user)
}
```

### Example 2: Manual Webhook Trigger

```go
// handlers/report_handlers.go

func GenerateReport(c *gin.Context) {
	businessID, _ := middleware.GetBusinessIDFromContext(c)

	// Generate report
	report := generateReportData()

	// Save to database
	savedReport := saveReport(report)

	// Manually trigger webhook
	webhookService := utils.NewWebhookService(config.DB)
	_ = webhookService.TriggerWebhook(
		models.EventCreate,
		"Report",
		fmt.Sprintf("%d", savedReport.ID),
		businessID,
		map[string]interface{}{
			"id":         savedReport.ID,
			"name":       report.Name,
			"type":       report.Type,
			"status":     "generated",
			"url":        fmt.Sprintf("/api/v1/reports/%d", savedReport.ID),
		},
	)

	c.JSON(http.StatusOK, savedReport)
}
```

### Example 3: Filtering Webhook Payload

```go
// utils/webhook_utils.go - Extended

// FilterWebhookPayload removes sensitive data from webhook payload
func FilterWebhookPayload(payload map[string]interface{}, sensitiveFields []string) map[string]interface{} {
	filtered := make(map[string]interface{})
	
	for key, value := range payload {
		isSensitive := false
		for _, field := range sensitiveFields {
			if key == field {
				isSensitive = true
				break
			}
		}
		
		if !isSensitive {
			filtered[key] = value
		}
	}
	
	return filtered
}

// Example usage:
sensitiveFields := []string{"password", "secret", "token", "ssn"}
cleanPayload := FilterWebhookPayload(originalPayload, sensitiveFields)
```

### Example 4: Route Registration with Webhooks

```go
// routes/routes.go

func RegisterAllRoutes(router *gin.Engine) {
	// Public routes (no auth required)
	router.POST("/register", handlers.Register)
	router.POST("/login", handlers.Login)
	router.POST("/api/v1/webhooks/incoming", handlers.WebhookIncomingHandler)

	// Protected routes
	api := router.Group("/api/v1", 
		middleware.SecurityMiddleware,
		middleware.JWTMiddleware,
		middleware.AutoTriggerWebhookMiddleware, // Global webhook trigger
	)

	// User management
	api.POST("/users", handlers.CreateUser)
	api.PUT("/users/:id", handlers.UpdateUser)
	api.GET("/users/:id", handlers.GetUser)
	api.DELETE("/users/:id", handlers.DeleteUser)

	// Business management
	api.POST("/business", handlers.CreateBusiness)
	api.PUT("/business/:id", handlers.UpdateBusiness)

	// Site management
	api.POST("/sites", handlers.CreateSite)
	api.PUT("/sites/:id", handlers.UpdateSite)

	// Webhook management (separate routes)
	webhookGroup := router.Group("/api/v1/webhooks",
		middleware.SecurityMiddleware,
		middleware.JWTMiddleware,
	)
	webhookRoutes.RegisterWebhookRoutes(webhookGroup)
}
```

### Example 5: Async Webhook Processing with Goroutines

```go
// utils/webhook_service.go - Extended

// TriggerWebhookAsync triggers a webhook with full async processing
func (ws *WebhookService) TriggerWebhookAsync(
	eventType models.WebhookEventType,
	resourceType string,
	resourceID string,
	businessID uint,
	data map[string]interface{},
	timeout time.Duration,
) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		select {
		case <-time.After(time.Second):
			ws.TriggerWebhook(eventType, resourceType, resourceID, businessID, data)
		case <-ctx.Done():
			log.Printf("Webhook trigger timeout for %s", resourceType)
		}
	}()

	return nil
}
```

## C# Consumer Implementation

### Example 1: ASP.NET Core Webhook Receiver

```csharp
using System;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.Logging;
using YourNamespace.Models;
using YourNamespace.Services;

namespace YourNamespace.Controllers
{
    [ApiController]
    [Route("api/[controller]")]
    public class WebhookController : ControllerBase
    {
        private readonly ILogger<WebhookController> _logger;
        private readonly IWebhookProcessingService _processingService;

        public WebhookController(
            ILogger<WebhookController> logger,
            IWebhookProcessingService processingService)
        {
            _logger = logger;
            _processingService = processingService;
        }

        [HttpPost("ugcl-events")]
        public async Task<IActionResult> ReceiveUGCLWebhook([FromHeader(Name = "X-Webhook-Signature")] string signature)
        {
            try
            {
                // Read the request body
                Request.EnableBuffering();
                using (var reader = new StreamReader(Request.Body))
                {
                    var body = await reader.ReadToEndAsync();
                    Request.Body.Position = 0;

                    // Verify signature
                    var webhookSecret = Environment.GetEnvironmentVariable("UGCL_WEBHOOK_SECRET");
                    if (!VerifySignature(body, signature, webhookSecret))
                    {
                        _logger.LogWarning("Invalid webhook signature");
                        return Unauthorized("Invalid signature");
                    }

                    // Deserialize webhook
                    var options = new JsonSerializerOptions { PropertyNameCaseInsensitive = true };
                    var webhook = JsonSerializer.Deserialize<WebhookPayload>(body, options);

                    // Process webhook asynchronously
                    _ = _processingService.ProcessWebhookAsync(webhook);

                    // Return success immediately
                    return Accepted(new { message = "Webhook received and queued for processing" });
                }
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Error processing webhook");
                return StatusCode(500, new { error = "Internal server error" });
            }
        }

        private bool VerifySignature(string requestBody, string signature, string secret)
        {
            using (var hmac = new HMACSHA256(Encoding.UTF8.GetBytes(secret)))
            {
                var hash = hmac.ComputeHash(Encoding.UTF8.GetBytes(requestBody));
                var expectedSignature = BitConverter.ToString(hash).Replace("-", "").ToLower();
                return expectedSignature == signature.ToLower();
            }
        }
    }

    // Models
    public class WebhookPayload
    {
        public string Id { get; set; }
        public string Event { get; set; }
        public string ResourceType { get; set; }
        public string ResourceId { get; set; }
        public JsonElement Data { get; set; }
        public DateTime Timestamp { get; set; }
        public uint BusinessId { get; set; }
        public string Version { get; set; }
    }
}
```

### Example 2: SQL Server Data Sync Service

```csharp
using System;
using System.Data;
using System.Threading.Tasks;
using Microsoft.Data.SqlClient;
using Microsoft.Extensions.Logging;

namespace YourNamespace.Services
{
    public interface IWebhookProcessingService
    {
        Task ProcessWebhookAsync(WebhookPayload webhook);
    }

    public class WebhookProcessingService : IWebhookProcessingService
    {
        private readonly string _connectionString;
        private readonly ILogger<WebhookProcessingService> _logger;

        public WebhookProcessingService(string connectionString, ILogger<WebhookProcessingService> logger)
        {
            _connectionString = connectionString;
            _logger = logger;
        }

        public async Task ProcessWebhookAsync(WebhookPayload webhook)
        {
            try
            {
                using (var connection = new SqlConnection(_connectionString))
                {
                    await connection.OpenAsync();

                    switch (webhook.Event)
                    {
                        case "CREATE":
                            await HandleCreateEvent(connection, webhook);
                            break;
                        case "UPDATE":
                            await HandleUpdateEvent(connection, webhook);
                            break;
                        default:
                            _logger.LogWarning($"Unknown event type: {webhook.Event}");
                            break;
                    }
                }
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, $"Error processing webhook: {webhook.Id}");
                throw;
            }
        }

        private async Task HandleCreateEvent(SqlConnection connection, WebhookPayload webhook)
        {
            var commandText = @"
                INSERT INTO IntegratedEvents (
                    WebhookId, EventType, ResourceType, ResourceId, Payload, BusinessId, ProcessedAt
                ) VALUES (
                    @webhookId, @eventType, @resourceType, @resourceId, @payload, @businessId, @processedAt
                )";

            using (var command = new SqlCommand(commandText, connection))
            {
                command.CommandTimeout = 30;
                command.Parameters.AddWithValue("@webhookId", webhook.Id);
                command.Parameters.AddWithValue("@eventType", webhook.Event);
                command.Parameters.AddWithValue("@resourceType", webhook.ResourceType);
                command.Parameters.AddWithValue("@resourceId", webhook.ResourceId);
                command.Parameters.AddWithValue("@payload", webhook.Data.GetRawText());
                command.Parameters.AddWithValue("@businessId", webhook.BusinessId);
                command.Parameters.AddWithValue("@processedAt", DateTime.UtcNow);

                await command.ExecuteNonQueryAsync();
                _logger.LogInformation($"Webhook {webhook.Id} processed successfully");
            }
        }

        private async Task HandleUpdateEvent(SqlConnection connection, WebhookPayload webhook)
        {
            var commandText = @"
                UPDATE IntegratedEvents 
                SET Payload = @payload, UpdatedAt = @updatedAt
                WHERE ResourceId = @resourceId AND EventType = 'UPDATE'";

            using (var command = new SqlCommand(commandText, connection))
            {
                command.CommandTimeout = 30;
                command.Parameters.AddWithValue("@payload", webhook.Data.GetRawText());
                command.Parameters.AddWithValue("@updatedAt", DateTime.UtcNow);
                command.Parameters.AddWithValue("@resourceId", webhook.ResourceId);

                var recordsAffected = await command.ExecuteNonQueryAsync();
                _logger.LogInformation($"Updated {recordsAffected} records for webhook {webhook.Id}");
            }
        }
    }
}
```

### Example 3: Dependency Injection Setup (Startup.cs)

```csharp
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;

namespace YourNamespace
{
    public class Startup
    {
        public IConfiguration Configuration { get; }

        public Startup(IConfiguration configuration)
        {
            Configuration = configuration;
        }

        public void ConfigureServices(IServiceCollection services)
        {
            // Register webhook processing service
            var connectionString = Configuration.GetConnectionString("DefaultConnection");
            services.AddScoped<IWebhookProcessingService>(
                provider => new WebhookProcessingService(
                    connectionString,
                    provider.GetRequiredService<ILogger<WebhookProcessingService>>()
                )
            );

            services.AddControllers();
        }

        public void Configure(IApplicationBuilder app, IHostingEnvironment env)
        {
            // ... other configuration ...
            
            app.UseEndpoints(endpoints =>
            {
                endpoints.MapControllers();
            });
        }
    }
}
```

### Example 4: SQL Server Schema for Webhook Integration

```sql
-- Create table for webhook events
CREATE TABLE [dbo].[IntegratedEvents] (
    [Id] INT IDENTITY(1,1) PRIMARY KEY,
    [WebhookId] NVARCHAR(MAX) NOT NULL,
    [EventType] NVARCHAR(50) NOT NULL,
    [ResourceType] NVARCHAR(100) NOT NULL,
    [ResourceId] NVARCHAR(MAX) NOT NULL,
    [Payload] NVARCHAR(MAX) NOT NULL,
    [BusinessId] INT NOT NULL,
    [ProcessedAt] DATETIME NOT NULL,
    [UpdatedAt] DATETIME NULL,
    [CreatedAt] DATETIME DEFAULT GETUTCDATE(),
    CONSTRAINT [FK_BusinessId] FOREIGN KEY ([BusinessId]) REFERENCES [dbo].[Businesses]([Id])
);

-- Create indexes
CREATE INDEX [IX_IntegratedEvents_ResourceId] ON [dbo].[IntegratedEvents]([ResourceId]);
CREATE INDEX [IX_IntegratedEvents_EventType] ON [dbo].[IntegratedEvents]([EventType]);
CREATE INDEX [IX_IntegratedEvents_CreatedAt] ON [dbo].[IntegratedEvents]([CreatedAt]);
CREATE INDEX [IX_IntegratedEvents_BusinessId] ON [dbo].[IntegratedEvents]([BusinessId]);
```

## Environment Configuration

### Go Backend (.env)

```env
# Database
DB_DSN=postgres://user:password@localhost:5432/ugcl_db
DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=10

# Webhooks
WEBHOOK_RETRY_INTERVAL=5
WEBHOOK_CLEANUP_INTERVAL=24
WEBHOOK_DATA_RETENTION_DAYS=90
WEBHOOK_TIMEOUT=10
WEBHOOK_MAX_RETRIES=5
```

### C# (.NET Core) (appsettings.json)

```json
{
  "ConnectionStrings": {
    "DefaultConnection": "Server=YOUR_SERVER;Database=YOUR_DB;User Id=YOUR_USER;Password=YOUR_PASS;"
  },
  "Webhooks": {
    "Secret": "your-webhook-secret-key",
    "MaxRetries": 5,
    "TimeoutSeconds": 30
  },
  "Logging": {
    "LogLevel": {
      "Default": "Information"
    }
  }
}
```

## Testing

### Postman Collection Example

```json
{
  "info": {
    "name": "UGCL Webhook API",
    "description": "Test collection for webhook endpoints"
  },
  "item": [
    {
      "name": "Create Webhook",
      "request": {
        "method": "POST",
        "header": [
          {
            "key": "Authorization",
            "value": "Bearer {{token}}"
          },
          {
            "key": "Content-Type",
            "value": "application/json"
          }
        ],
        "url": {
          "raw": "http://localhost:8080/api/v1/webhooks",
          "host": ["localhost"],
          "port": "8080",
          "path": ["api", "v1", "webhooks"]
        },
        "body": {
          "mode": "raw",
          "raw": "{\n  \"url\": \"https://webhook.site/your-id\",\n  \"events\": [\"CREATE\", \"UPDATE\"],\n  \"resource_types\": [\"User\"],\n  \"secret\": \"test-secret\"\n}"
        }
      }
    }
  ]
}
```
