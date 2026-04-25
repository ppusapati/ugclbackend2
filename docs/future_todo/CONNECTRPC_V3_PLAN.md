# connectRPC_V3 — Migration Plan
**Status**: Planned / Frozen — execute when mobile and web clients are feature-frozen  
**Target release tag**: `connectRPC_V3`  
**Documented**: 2026-04-22

---

## Summary

Migrate the UGCL backend from Gorilla Mux + net/http REST to ConnectRPC (connectrpc.com/connect).  
Migration strategy: **Strangler Fig** — ConnectRPC mounts alongside existing Gorilla Mux router on the same port. Both coexist throughout migration. Domains migrated one at a time.

ConnectRPC's **connect protocol** keeps JSON-over-HTTP/1.1 compatibility for browser/mobile clients while unlocking gRPC + HTTP/2 when both sides support it. No envoy/gateway proxy required.

---

## Performance Gains

| Area | Improvement |
|---|---|
| Payload size | Protobuf ~3–5× smaller than JSON (critical for report exports, document listings) |
| Serialization speed | Protobuf marshal/unmarshal ~5–10× faster than encoding/json |
| Connection efficiency | HTTP/2 multiplexing eliminates per-request connection overhead |
| Streaming | SSE notifications → first-class server-streaming RPCs |
| Type safety | Eliminates `map[string]interface{}` allocations throughout handlers |

---

## Baseline (as of 2026-04-22)

- ~280–300 HTTP endpoints across 10 route files
- Primary framework: Gorilla Mux + net/http (95% migrated from Gin)
- Residual Gin: `webhook_routes.go` only (bridged into Mux)
- Auth stack: JWT → RBAC (global) → RBAC (business) → ABAC (optional)
- SSE streaming: `GET /api/v1/notifications/stream`
- File uploads: multipart/form-data → Google Cloud Storage
- Spatial data: PostGIS geometry fields via `paulmach/orb`
- JSONB fields: `gorm.io/datatypes`
- No existing `.proto` files (`docs/proto/` is empty)
- gRPC already indirect dep via GCS SDK (`google.golang.org/grpc v1.72.1`)
- Test coverage sparse (~11 active tests)

---

## Permanent Exclusions (never migrate to gRPC)

- **File upload endpoints**: `handlers/file_handler.go`, `handlers/document_bulk_handler.go`, `handlers/file_local.go` — multipart/form-data stays raw HTTP
- **Webhook endpoints**: `routes/webhook_routes.go` — HMAC signature validation over raw body; stays Gin/net/http
- **GORM + PostgreSQL + PostGIS**: database layer entirely unchanged, ConnectRPC is transport only

---

## Proto Directory Structure

```
proto/
  buf.yaml
  buf.gen.yaml
  common/v1/
    common.proto          # UUIDValue, PageRequest/Response, GeoPoint (WKT), Timestamp, JsonStruct
  auth/v1/
    auth.proto            # AuthService: Login, RefreshToken, Logout
  users/v1/
    users.proto           # UserService: CRUD users, assign roles, role levels
  business/v1/
    business.proto        # BusinessService: business management, membership
  projects/v1/
    projects.proto        # ProjectService: projects, tasks, budget, workflow
  documents/v1/
    documents.proto       # DocumentService: CRUD, versions, shares, categories, tags (NO upload RPCs)
  reports/v1/
    reports.proto         # ReportService: definitions, execution, export, dashboards
  abac/v1/
    abac.proto            # ABACService: policies, attributes, approvals
  chat/v1/
    chat.proto            # ChatService: conversations, messages, participants
  notifications/v1/
    notifications.proto   # NotificationService: unary list/mark-read + server-streaming StreamNotifications
gen/                      # buf generate output — DO NOT EDIT
```

---

## Phase 1: Toolchain & Proto Foundation
*All steps parallel. No existing code changes.*

1. Install `buf` CLI; add `buf.yaml` + `buf.gen.yaml` to repo root with `protoc-gen-go` + `protoc-gen-connect-go` plugins
2. Add to go.mod: `connectrpc.com/connect`, `google.golang.org/protobuf`
3. Create `proto/` directory structure as above
4. Define shared types in `proto/common/v1/common.proto`: `UUIDValue`, `PageRequest`/`PageResponse`, `GeoPoint` (WKT string wrapper), `google.protobuf.Timestamp`, `google.protobuf.Struct` for JSONB
5. Define all domain `.proto` service files (shape derived from `models/` + `routes/`)
6. Run `buf generate` → produces `gen/` directory

**Reference files for proto field shapes:**
- `routes/routes_v2.go` — all endpoint names/verbs
- `models/` — all model structs for field shapes
- `handlers/` — request/response structures

---

## Phase 2: Interceptor Layer
*Replaces the JWT → RBAC → ABAC middleware chain. Depends on Phase 1.*

7. `interceptors/auth.go` — unary+streaming interceptor; extracts JWT from `Authorization: Bearer` header via `connect.Request.Header()`; stashes claims into context. Port logic from `middleware/jwt.go`
8. `interceptors/rbac.go` — reads permission requirement from procedure name → permissions registry map; calls existing RBAC service logic. Port from `middleware/authorization_refactored.go`
9. `interceptors/business_context.go` — extracts `X-Business-ID` / `X-Business-Code` from headers; stashes into context. Port from `middleware/business_auth.go`
10. `interceptors/abac.go` — optional ABAC policy evaluation. Port from `middleware/abac_middleware.go`
11. `interceptors/chain.go` — compositor wiring order: auth → rbac → business → abac. Mirror `Authorize()` pattern in `middleware/authorization_refactored.go`
12. `interceptors/permission_registry.go` — map of `FullProcedureName → []string{required_permissions}`. Replaces per-route `RequirePermission("x")` decorators

---

## Phase 3: Domain Service Migration
*Each domain independent — parallel across team. Depends on Phase 2.*  
*Order: low-risk first.*

For each domain: **define service interface → implement handler → mount in router → deprecate old HTTP handler.**

| Step | Domain | Source |
|---|---|---|
| 13 | Auth | `handlers/auth.go` |
| 14 | Users & Roles | user/role handlers in `handlers/` |
| 15 | Business Vertical | `handlers/business_management.go` |
| 16 | Projects | `handlers/` project handlers, `routes/project_routes.go` |
| 17 | Documents | `routes/document_routes.go` *(file upload endpoints stay raw HTTP)* |
| 18 | Reports | `routes/report_routes.go` |
| 19 | ABAC / Attributes | `routes/abac_routes.go` |
| 20 | Chat | `routes/chat_routes.go` |
| 21 | Notifications | `routes/notification_routes.go`; SSE → server-streaming `StreamNotifications` RPC |

---

## Phase 4: Router Mount & Coexistence
*Depends on Phase 2. Start as soon as first Phase 3 domain is ready.*

22. In `main.go`: create connect handler per service; mount via `router.PathPrefix("/connect/").Handler(connectHandler)`. Existing `/api/v1/` routes continue working
23. CORS middleware: pass `Content-Type: application/proto` and `Connect-Protocol-Version` headers
24. Keep Gin webhook routes untouched — separate mount

---

## Phase 5: File Upload & Webhook Carve-outs
*Parallel with Phase 3.*

25. Document in `proto/README` which endpoints are excluded from gRPC migration (file uploads + webhooks)
26. If chunked document streaming ever needed: implement as separate `proto/storage/v1/UploadDocument` using client-streaming RPC with `bytes` fields — only if explicitly required

---

## Phase 6: Cleanup & Observability
*Depends on Phase 3 completion per domain.*

27. Remove deprecated Gorilla Mux route registrations per domain after ConnectRPC equivalent is live + tested
28. Replace `gin_adapters.go` with direct `net/http` webhook handlers → eliminates Gin dependency entirely
29. Wire OpenTelemetry (already indirect dep via GCS) into ConnectRPC interceptors for distributed tracing per RPC
30. Update Swagger/OpenAPI via `protoc-gen-openapiv2` buf plugin

---

## Verification Steps

1. **Phase 1 done**: `buf lint` and `buf generate` succeed with no errors
2. **Phase 2 done**: unit tests for each interceptor using `connectrpc.com/connect/connecttest`
3. **Per domain (Phase 3)**: existing Postman collections + `buf curl` / `grpcurl` against new endpoint; verify parity with old REST endpoint
4. **Phase 4 done**: hit both `/api/v1/auth/login` (old) and `/connect/auth.v1.AuthService/Login` (new); confirm identical behavior
5. **Phase 6 done**: `go mod tidy` confirms Gin removed from direct deps; `go vet ./...` clean

---

## Future Considerations

1. **Frontend/mobile codegen**: `buf` generates TypeScript + Dart/Swift clients from the same `.proto` files — end-to-end type safety once web and mobile are ready. This is the primary reason to freeze clients before starting this migration.
2. **Scope**: ~280 endpoints = 3–4 months solo. Prioritize highest-traffic domains (auth, projects, reports) first if partial rollout is preferred.
3. **Proto schema governance**: establish `buf breaking` CI check before first proto commit to catch breaking changes early.
4. **Azure ERP integration**: ConnectRPC/gRPC is the natural bridge protocol for Solar ERP microservices on Azure — this migration directly enables that integration path.

---

## Key Files Reference

| File | Role in migration |
|---|---|
| `routes/routes_v2.go` | Source of truth for all endpoint → handler mappings |
| `middleware/authorization_refactored.go` | Interceptor design blueprint |
| `middleware/jwt.go` | JWT logic to port into auth interceptor |
| `middleware/business_auth.go` | Business context logic for interceptor |
| `middleware/abac_middleware.go` | ABAC interceptor source |
| `handlers/chat_handlers.go` | Canonical handler pattern to rewrite in ConnectRPC style |
| `main.go` | Router bootstrap — Phase 4 mount point |
| `config/migrations.go` | Unchanged — GORM stays |
| `docs/proto/` | Empty dir — proto files go here or in new `proto/` root dir |
