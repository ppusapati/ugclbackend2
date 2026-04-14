# Webhook System Documentation

## Overview

The UGCL backend webhook system enables real-time integration with third-party applications by sending event notifications when resources are created or updated. This documentation covers how to set up, configure, and consume webhooks.

## Features

- **Event-Driven Architecture**: Automatic notifications on Create and Update operations
- **HMAC-SHA256 Signatures**: Secure webhook delivery with cryptographic verification
- **Automatic Retry Logic**: Exponential backoff retry strategy (5 configurable attempts)
- **Delivery Status Tracking**: Complete delivery history and logging
- **Resource Filtering**: Subscribe to specific resource types
- **Custom Headers**: Support for custom HTTP headers in webhook requests
- **Webhook Testing**: Built-in test endpoint for validating configurations

## Supported Events

### Event Types
- `CREATE`: Triggered when a resource is created
- `UPDATE`: Triggered when a resource is updated

### Resource Types
- User
- Business
- Site
- DPRSite
- Wrapping
- EWay
- Water
- Stock
- DairySite
- Material
- Diesel
- Payment
- NMRVehicle
- Report
- Project
- Notification
- Document
- Workflow

## Authentication

All webhook management endpoints require JWT authentication. Include your JWT token in the `Authorization` header:

```
Authorization: Bearer <your_jwt_token>
```

## API Endpoints

### 1. Create Webhook Subscription

**Endpoint**: `POST /api/v1/webhooks`

**Request Body**:
```json
{
  "url": "https://your-api.example.com/webhooks/events",
  "events": ["CREATE", "UPDATE"],
  "resource_types": ["User", "Site"],
  "secret": "your-webhook-secret",
  "headers": {
    "Authorization": "Bearer your-token",
    "X-Custom-Header": "value"
  },
  "max_retries": 5,
  "retry_interval": 300
}
```

**Parameters**:
- `url` (required): HTTPS endpoint URL where webhook events will be sent
- `events` (required): Array of event types to subscribe to
- `resource_types` (optional): Array of resource types to monitor (empty = all types)
- `secret` (optional): Shared secret for HMAC signature verification (auto-generated if not provided)
- `headers` (optional): Custom HTTP headers to include in webhook requests
- `max_retries` (optional): Number of retry attempts (default: 5)
- `retry_interval` (optional): Initial retry interval in seconds (default: 300)

**Response** (201 Created):
```json
{
  "id": 1,
  "business_id": 123,
  "url": "https://your-api.example.com/webhooks/events",
  "events": ["CREATE", "UPDATE"],
  "resource_types": ["User", "Site"],
  "status": "ACTIVE",
  "is_active": true,
  "created_at": "2024-04-14T10:30:00Z"
}
```

### 2. List Webhooks

**Endpoint**: `GET /api/v1/webhooks`

**Response** (200 OK):
```json
[
  {
    "id": 1,
    "business_id": 123,
    "url": "https://your-api.example.com/webhooks/events",
    "status": "ACTIVE",
    "is_active": true,
    "created_at": "2024-04-14T10:30:00Z"
  }
]
```

### 3. Get Webhook Details

**Endpoint**: `GET /api/v1/webhooks/{id}`

**Response** (200 OK):
```json
{
  "id": 1,
  "business_id": 123,
  "url": "https://your-api.example.com/webhooks/events",
  "events": ["CREATE", "UPDATE"],
  "resource_types": ["User", "Site"],
  "secret": "your-webhook-secret",
  "status": "ACTIVE",
  "max_retries": 5,
  "is_active": true,
  "created_at": "2024-04-14T10:30:00Z"
}
```

### 4. Update Webhook

**Endpoint**: `PUT /api/v1/webhooks/{id}`

**Request Body** (any field can be updated):
```json
{
  "url": "https://new-endpoint.example.com/webhooks",
  "events": ["CREATE"],
  "is_active": true
}
```

**Response** (200 OK): Updated webhook object

### 5. Delete Webhook

**Endpoint**: `DELETE /api/v1/webhooks/{id}`

**Response** (204 No Content)

### 6. Test Webhook

**Endpoint**: `POST /api/v1/webhooks/{id}/test`

Sends a test webhook event to verify your webhook endpoint is configured correctly.

**Response** (200 OK):
```json
{
  "message": "Test webhook sent successfully"
}
```

### 7. Get Delivery History

**Endpoint**: `GET /api/v1/webhooks/{id}/deliveries?limit=50`

Retrieves the delivery history for a webhook subscription.

**Response** (200 OK):
```json
[
  {
    "id": 1,
    "webhook_id": 1,
    "event_type": "CREATE",
    "resource_type": "User",
    "resource_id": "user-123",
    "status": "SUCCESS",
    "http_status": 200,
    "attempt": 1,
    "sent_at": "2024-04-14T10:35:00Z",
    "created_at": "2024-04-14T10:35:00Z"
  }
]
```

### 8. Get Delivery Logs

**Endpoint**: `GET /api/v1/webhooks/deliveries/{deliveryId}/logs`

Retrieves detailed logs for a specific delivery attempt.

**Response** (200 OK):
```json
[
  {
    "id": 1,
    "delivery_id": 1,
    "event_type": "CREATE",
    "action": "SENT",
    "response": "OK",
    "created_at": "2024-04-14T10:35:00Z"
  }
]
```

## Webhook Payload Structure

When an event occurs, an HTTP POST request is sent to your webhook URL with the following JSON payload:

```json
{
  "id": "d4c6f4d6-0c7a-4c0c-8d7e-6c0b0c6b0c8d",
  "event": "CREATE",
  "resource_type": "User",
  "resource_id": "user-123",
  "data": {
    "id": "user-123",
    "email": "user@example.com",
    "name": "John Doe",
    "created_at": "2024-04-14T10:35:00Z"
  },
  "timestamp": "2024-04-14T10:35:00Z",
  "business_id": 123,
  "version": "1.0"
}
```

## Security

### HMAC Signature Verification

Every webhook request includes an `X-Webhook-Signature` header containing an HMAC-SHA256 signature. 

**Steps to verify**:
1. Retrieve the webhook secret from your webhook configuration
2. Calculate HMAC-SHA256 of the entire request body using the secret
3. Compare with the `X-Webhook-Signature` header value

### Request Headers

Each webhook request includes the following headers:

```
Content-Type: application/json
X-Webhook-Signature: <HMAC-SHA256 signature>
X-Webhook-Delivery-ID: <unique delivery ID>
X-Webhook-Attempt: <attempt number>
X-Webhook-Max-Retries: <max retries count>
User-Agent: UGCL-Webhook-Engine/1.0
```

### C# Example - Verifying Webhook Signature

```csharp
using System;
using System.IO;
using System.Security.Cryptography;
using System.Text;

public class WebhookVerifier
{
    public static bool VerifyWebhookSignature(string requestBody, string signature, string secret)
    {
        using (var hmac = new HMACSHA256(Encoding.UTF8.GetBytes(secret)))
        {
            byte[] hash = hmac.ComputeHash(Encoding.UTF8.GetBytes(requestBody));
            string expectedSignature = BitConverter.ToString(hash).Replace("-", "").ToLower();
            return expectedSignature == signature.ToLower();
        }
    }
}
```

## Retry Strategy

Failed webhook deliveries are retried automatically using exponential backoff:

| Attempt | Delay |
|---------|-------|
| 1 | 30 seconds |
| 2 | 2 minutes |
| 3 | 5 minutes |
| 4 | 15 minutes |
| 5 | 1 hour |

**Retry Conditions**:
- HTTP 408 (Request Timeout)
- HTTP 429 (Too Many Requests)
- HTTP 5xx (Server Errors)
- Network timeouts

**Non-Retryable Conditions**:
- HTTP 4xx (Client Errors - except 408 and 429)
- Invalid URL
- Request timeout after retries exhausted

## C# Integration Examples

### 1. Null Check and Deserialization

```csharp
using System;
using System.IO;
using System.Text.Json;
using Microsoft.AspNetCore.Mvc;

[ApiController]
[Route("api/webhooks")]
public class WebhookController : ControllerBase
{
    private readonly string _webhookSecret = "your-webhook-secret";

    [HttpPost("events")]
    public async Task<IActionResult> ReceiveWebhook()
    {
        using (var reader = new StreamReader(Request.Body))
        {
            string requestBody = await reader.ReadToEndAsync();
            
            if (string.IsNullOrEmpty(requestBody))
                return BadRequest("Request body is empty");

            // Verify signature
            if (!Request.Headers.TryGetValue("X-Webhook-Signature", out var signature))
                return Unauthorized("Missing signature header");

            if (!WebhookVerifier.VerifyWebhookSignature(requestBody, signature.ToString(), _webhookSecret))
                return Unauthorized("Invalid signature");

            // Deserialize payload
            var options = new JsonSerializerOptions { PropertyNameCaseInsensitive = true };
            var webhook = JsonSerializer.Deserialize<WebhookPayload>(requestBody, options);

            // Process webhook
            await ProcessWebhook(webhook);

            return Accepted();
        }
    }

    private async Task ProcessWebhook(WebhookPayload webhook)
    {
        switch (webhook.Event)
        {
            case "CREATE":
                await HandleCreate(webhook);
                break;
            case "UPDATE":
                await HandleUpdate(webhook);
                break;
        }
    }

    private async Task HandleCreate(WebhookPayload webhook)
    {
        // Handle CREATE event
        Console.WriteLine($"Resource {webhook.ResourceType} created: {webhook.ResourceId}");
        // Sync with SQL Server
    }

    private async Task HandleUpdate(WebhookPayload webhook)
    {
        // Handle UPDATE event
        Console.WriteLine($"Resource {webhook.ResourceType} updated: {webhook.ResourceId}");
        // Sync with SQL Server
    }
}

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
```

### 2. Sync with SQL Server

```csharp
using System.Data.SqlClient;
using System.Text.Json;

public class WebhookProcessor
{
    private readonly string _connectionString = "your-connection-string";

    public async Task SyncToSqlServer(WebhookPayload webhook)
    {
        using (var connection = new SqlConnection(_connectionString))
        {
            await connection.OpenAsync();

            if (webhook.Event == "CREATE")
            {
                await InsertRecord(connection, webhook);
            }
            else if (webhook.Event == "UPDATE")
            {
                await UpdateRecord(connection, webhook);
            }

            await connection.CloseAsync();
        }
    }

    private async Task InsertRecord(SqlConnection connection, WebhookPayload webhook)
    {
        string query = @"
            INSERT INTO WebhookEvents (EventId, EventType, ResourceType, ResourceId, Payload, BusinessId, CreatedAt)
            VALUES (@eventId, @eventType, @resourceType, @resourceId, @payload, @businessId, @createdAt)";

        using (var command = new SqlCommand(query, connection))
        {
            command.Parameters.AddWithValue("@eventId", webhook.Id);
            command.Parameters.AddWithValue("@eventType", webhook.Event);
            command.Parameters.AddWithValue("@resourceType", webhook.ResourceType);
            command.Parameters.AddWithValue("@resourceId", webhook.ResourceId);
            command.Parameters.AddWithValue("@payload", JsonSerializer.Serialize(webhook.Data));
            command.Parameters.AddWithValue("@businessId", webhook.BusinessId);
            command.Parameters.AddWithValue("@createdAt", webhook.Timestamp);

            await command.ExecuteNonQueryAsync();
        }
    }

    private async Task UpdateRecord(SqlConnection connection, WebhookPayload webhook)
    {
        string query = @"
            UPDATE WebhookEvents 
            SET Payload = @payload, UpdatedAt = @updatedAt
            WHERE ResourceId = @resourceId AND EventType = 'UPDATE'";

        using (var command = new SqlCommand(query, connection))
        {
            command.Parameters.AddWithValue("@payload", JsonSerializer.Serialize(webhook.Data));
            command.Parameters.AddWithValue("@updatedAt", webhook.Timestamp);
            command.Parameters.AddWithValue("@resourceId", webhook.ResourceId);

            await command.ExecuteNonQueryAsync();
        }
    }
}
```

## Best Practices

1. **Always Verify Signatures**: Never trust webhook payloads without verifying the HMAC signature
2. **Respond Quickly**: Return a 2xx status code immediately, then process the webhook asynchronously
3. **Handle Duplicates**: Webhooks may be delivered multiple times; implement idempotency
4. **Log Everything**: Keep detailed logs of webhook deliveries for debugging
5. **Set Reasonable Timeouts**: Your endpoint should respond within 10 seconds
6. **Use HTTPS**: Always use HTTPS URLs for webhooks
7. **Monitor Deliveries**: Regularly check webhook delivery status and retry history
8. **Test Thoroughly**: Use the test webhook endpoint before going live

## Troubleshooting

### Webhook Not Triggering
1. Verify webhook is active (`is_active = true`)
2. Check that event types and resource types match your configuration
3. Ensure the business context is correctly set
4. Review webhook delivery history and logs

### Delivery Failures
1. Check HTTP status code in delivery history
2. Verify your endpoint URL is accessible from the backend
3. Ensure HMAC signature verification is working
4. Review response logs for error messages

### Signature Verification Failing
1. Confirm the webhook secret is correct
2. Ensure you're calculating the signature with the complete request body
3. Check character encoding (UTF-8)
4. Compare raw bytes, not strings

## Rate Limiting

Webhook deliveries respect the same rate limiting as regular API requests. Ensure your endpoint can handle the volume of events.

## Support

For issues or questions regarding webhooks, review the delivery logs and status in the webhook management dashboard or contact your system administrator.
