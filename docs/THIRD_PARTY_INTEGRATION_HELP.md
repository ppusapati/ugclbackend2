# Third-Party Integration Help Guide

## Purpose

This guide explains how to safely expose only required APIs to third-party providers while using webhooks for near real-time sync.

Design goals:
- Keep internal APIs isolated from third-party access.
- Allow only provider-specific integration endpoints.
- Enforce API key + method + path + source IP checks.
- Use webhook signatures and timestamps for secure delivery handling.

## Current Implementation Summary

Security and route controls are implemented in:
- middleware/jwt.go
- routes/integration_routes.go
- routes/webhook_routes.go
- handlers/integration_handlers.go
- utils/webhook_utils.go

### API Key Scopes

The middleware now supports these third-party keys:
- THIRD_PARTY_SYNC_KEY
- THIRD_PARTY_PROVIDER_A_KEY
- THIRD_PARTY_PROVIDER_B_KEY

Each key is path-scoped and method-scoped.

### Path Access Rules

THIRD_PARTY_SYNC_KEY can access:
- /api/v1/integrations/*
- /api/v1/webhooks/incoming
- /api/v1/webhooks/incoming/

THIRD_PARTY_PROVIDER_A_KEY can access:
- /api/v1/integrations/provider-a/*
- /api/v1/webhooks/incoming/provider-a

THIRD_PARTY_PROVIDER_B_KEY can access:
- /api/v1/integrations/provider-b/*
- /api/v1/webhooks/incoming/provider-b

### Method Rules

Third-party keys are limited to:
- GET
- POST

## Environment Configuration

Set these in .env or your production secret manager:

- THIRD_PARTY_SYNC_KEY
- THIRD_PARTY_PROVIDER_A_KEY
- THIRD_PARTY_PROVIDER_B_KEY

- THIRD_PARTY_SYNC_ALLOWED_IPS
- THIRD_PARTY_PROVIDER_A_ALLOWED_IPS
- THIRD_PARTY_PROVIDER_B_ALLOWED_IPS

Optional:
- PARTNER_PORTAL_ALLOWED_IPS

### Allowed IP Format

Use comma-separated public source IPs from provider infrastructure.

Example:
THIRD_PARTY_PROVIDER_A_ALLOWED_IPS=203.0.113.10,198.51.100.25

Notes:
- If an allowlist variable is empty, the middleware falls back to the default internal whitelist.
- SkipIPCheck is false for third-party keys, so allowlists are enforced.

## Integration Endpoints

General integration endpoints:
- GET /api/v1/integrations/health
- GET /api/v1/integrations/webhook-contract

Provider-specific endpoints:
- GET /api/v1/integrations/provider-a/health
- GET /api/v1/integrations/provider-a/webhook-contract
- GET /api/v1/integrations/provider-b/health
- GET /api/v1/integrations/provider-b/webhook-contract

Inbound webhook callback endpoints:
- POST /api/v1/webhooks/incoming
- POST /api/v1/webhooks/incoming/{provider}

## Webhook Security Contract

Outbound webhook headers include:
- X-Webhook-Signature
- X-Webhook-Timestamp
- X-Webhook-Delivery-ID
- X-Webhook-Attempt
- X-Webhook-Max-Retries

Receiver requirements:
1. Verify X-Webhook-Signature using shared secret and payload.
2. Validate X-Webhook-Timestamp freshness.
3. Reject replayed X-Webhook-Delivery-ID values.
4. Return 2xx only after successful processing.

## Provider Onboarding Checklist

1. Generate a dedicated API key for the provider.
2. Assign only provider-specific key scope (A or B), not broad sync key.
3. Collect provider outbound source IP addresses.
4. Fill provider allowed IP env variable.
5. Share webhook signing secret securely.
6. Confirm provider validates signature and timestamp.
7. Test with non-production endpoint first.
8. Enable production webhook subscription.

## Validation Commands

Use these from trusted source IPs.

Example health check:
curl -X GET "http://localhost:8080/api/v1/integrations/provider-a/health" \
  -H "x-api-key: <THIRD_PARTY_PROVIDER_A_KEY>"

Example webhook callback simulation:
curl -X POST "http://localhost:8080/api/v1/webhooks/incoming/provider-a" \
  -H "x-api-key: <THIRD_PARTY_PROVIDER_A_KEY>" \
  -H "Content-Type: application/json" \
  -d "{\"event\":\"test\"}"

Expected failures:
- Wrong path for key: 403
- Wrong method for key: 405
- IP not in allowlist: 403
- Invalid or missing key: 401

## Operational Guidance

- Rotate provider keys periodically.
- Keep separate keys per provider and environment.
- Log and monitor denied requests by AppName, IP, path, method.
- Disable only affected provider key during incident response.

## Troubleshooting

Problem: Provider gets 403 path not allowed.
- Verify key matches provider path scope in middleware.
- Verify request path exactly matches allowed prefix.

Problem: Provider gets 403 IP not allowed.
- Confirm outbound NAT IP from provider.
- Update corresponding *_ALLOWED_IPS variable.
- Restart service if environment values are loaded only at startup.

Problem: Provider receives webhook but rejects signature.
- Confirm shared secret matches configured webhook secret.
- Confirm body bytes used for HMAC are exact raw payload.

## Recommended Next Improvements

1. Add provider-specific rate limits.
2. Add provider-specific webhook secrets and rotation metadata.
3. Add replay cache storage for delivery IDs on receiver side examples.
4. Add integration dashboard for key status, last delivery, and error rates.
