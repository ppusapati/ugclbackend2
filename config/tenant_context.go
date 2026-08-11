package config

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var errDBNotConnected = errors.New("database is not connected")

// tenantCtxKey is an unexported type to prevent collisions with other
// packages' context keys, following the same pattern as
// middleware.userClaimsKey etc.
type tenantCtxKey int

const tenantSchemaKey tenantCtxKey = iota

// WithTenantSchema returns a context carrying the resolved tenant schema
// name, for DBFromContext to pick up. Called once per request by the
// tenant-resolution middleware (Phase 3), after JWTMiddleware has decoded
// the tenant claim.
func WithTenantSchema(ctx context.Context, schemaName string) context.Context {
	return context.WithValue(ctx, tenantSchemaKey, schemaName)
}

// TenantSchemaFromContext returns the tenant schema name stashed by
// WithTenantSchema, or "" if none is present (e.g. an unauthenticated route,
// a route that predates tenancy, or a background job with no request
// context at all).
func TenantSchemaFromContext(ctx context.Context) string {
	schema, _ := ctx.Value(tenantSchemaKey).(string)
	return schema
}

// noopCleanup is returned by DBFromContext when no tenant-scoped connection
// was opened, so callers can always unconditionally defer the cleanup func
// without a nil check.
func noopCleanup() error { return nil }

// DBFromContext returns the correct *gorm.DB for the given request context:
// a tenant-scoped session if a tenant schema is present in ctx, or the
// legacy global DB otherwise. This is the single point every handler should
// go through instead of referencing the config.DB package variable
// directly — see the architecture plan's Phase 2 for why (a mechanical,
// codebase-wide replacement of ~589 direct config.DB call sites).
//
// Falling back to the global DB when no tenant is in context is deliberate:
// it means this function can be introduced and every call site migrated to
// use it BEFORE any middleware actually sets a tenant in context (Phase 3),
// with zero behavior change in the interim — the whole point of splitting
// this into independently-shippable phases.
//
// The returned cleanup func MUST be deferred by the caller. It is a no-op
// when the fallback global DB is returned (no dedicated connection was
// opened), so it is always safe to defer unconditionally:
//
//	db, cleanup, err := config.DBFromContext(r.Context())
//	if err != nil { ... }
//	defer cleanup()
//
// Every tenant-scoped session returned here goes through the same
// connection-pinning and RESET-on-release discipline as
// TenantScopedSession — see that function's doc comment for why both
// matter (confirmed by testing: a connection released without resetting
// search_path silently carries it into the next, unrelated request that
// happens to reuse the same pooled connection).
func DBFromContext(ctx context.Context) (*gorm.DB, func() error, error) {
	schemaName := TenantSchemaFromContext(ctx)
	if schemaName == "" {
		return DB, noopCleanup, nil
	}

	return tenantScopedSessionWithContextImpl(ctx, schemaName)
}
