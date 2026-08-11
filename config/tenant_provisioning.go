package config

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

var validSchemaNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// reservedSchemaNames blocks tenant schema names that would collide with
// Postgres system schemas or the control schema itself.
var reservedSchemaNames = map[string]bool{
	"public":             true,
	"control":            true,
	"pg_catalog":         true,
	"information_schema": true,
	"pg_toast":           true,
	"pg_temp":            true,
}

// ValidateTenantSchemaName enforces the same identifier rules Postgres
// itself requires, plus the reserved-name list, before any schema DDL runs.
// Schema names are interpolated into raw SQL (Postgres does not support
// parameterized identifiers), so this check is the injection boundary —
// callers must reject on error, never attempt to sanitize and continue.
func ValidateTenantSchemaName(name string) error {
	if !validSchemaNamePattern.MatchString(name) {
		return fmt.Errorf("invalid schema name %q: must start with a letter or underscore and contain only lowercase letters, digits, and underscores", name)
	}
	if reservedSchemaNames[name] {
		return fmt.Errorf("schema name %q is reserved", name)
	}
	return nil
}

// ProvisionTenantSchema creates a new tenant's schema and runs the full
// migration list against it, so a newly created tenant ends up with the
// exact same table set as every other tenant. Idempotent: safe to retry if
// a prior attempt failed partway (CREATE SCHEMA IF NOT EXISTS + gormigrate's
// own applied-migration bookkeeping, scoped per schema, both no-op cleanly).
func ProvisionTenantSchema(schemaName string) error {
	if DB == nil {
		return errDBNotConnected
	}
	if err := ValidateTenantSchemaName(schemaName); err != nil {
		return err
	}

	if err := DB.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)).Error; err != nil {
		return fmt.Errorf("create tenant schema %s: %w", schemaName, err)
	}

	tenantDB, closeSession, err := TenantScopedSession(schemaName)
	if err != nil {
		return err
	}
	defer closeSession()

	if err := runMigrationsWithRecovery(tenantDB); err != nil {
		return fmt.Errorf("migrate tenant schema %s: %w", schemaName, err)
	}

	// The unified RBAC tables (rbac_roles, rbac_role_permissions,
	// user_role_assignments) are deliberately NOT part of the main
	// Migrations() list — that list predates the RBAC cutover and the RBAC
	// tables were originally created only by the one-time
	// PrepareRBACMigration cutover tool. Every tenant schema still needs
	// them, so create them here directly via AutoMigrate. Do not call
	// PrepareRBACMigration itself: its verification step assumes existing
	// legacy data to copy and hard-fails on an empty schema (a real bug
	// this caught during testing) — a fresh tenant has no legacy data to
	// migrate from in the first place.
	if err := tenantDB.AutoMigrate(
		&models.RBACRole{},
		&models.RBACRolePermission{},
		&models.UserRoleAssignment{},
	); err != nil {
		return fmt.Errorf("migrate tenant schema %s RBAC tables: %w", schemaName, err)
	}

	return nil
}

// runMigrationsWithRecovery runs Migrations() with panic recovery and one
// automatic retry. gorm.io/driver/postgres@v1.5.11 has a confirmed
// nil-pointer bug in AlterColumn (missing nil-check on a failed type
// assertion) that intermittently panics on a genuinely fresh, empty schema
// — specifically observed on the Zone model's PostGIS geometry column,
// which every existing production schema already had, so this migrator
// code path was never previously exercised against a brand-new table.
// Confirmed non-deterministic: retrying the same migration against the same
// schema succeeds. This is a narrow, local workaround for the tenant
// provisioning path only — it does not touch the shared gorm dependency or
// the existing single-schema production migration path.
func runMigrationsWithRecovery(db *gorm.DB) (err error) {
	for attempt := 1; attempt <= 2; attempt++ {
		err = func() (result error) {
			defer func() {
				if r := recover(); r != nil {
					result = fmt.Errorf("migration panicked (attempt %d/2): %v", attempt, r)
				}
			}()
			return Migrations(db)
		}()

		if err == nil {
			return nil
		}
	}
	return err
}

// TenantScopedSession opens a dedicated, isolated *gorm.DB bound to the
// given tenant schema. The caller owns its lifecycle and MUST call the
// returned cleanup func when done, never reuse the session beyond that.
// Intended for one-off admin operations (provisioning, seeding) where no
// request context is available. Per-request handler code should use
// config.DBFromContext(r.Context()) instead, which shares this exact
// implementation (tenantScopedSessionWithContextImpl below) but threads the
// request's own context through for cancellation/timeout propagation.
//
// A schema-scoped session MUST run on a connection dedicated to it for its
// entire lifetime, not one borrowed from and returned to the shared pool —
// gorm.Session{NewDB: true} does NOT provide this; it still runs on
// DB.Config.ConnPool, the same pool every other request uses, so a SET
// search_path there would leak into whichever request the pool hands that
// connection to next. sql.DB.Conn(ctx) pins one physical connection for
// exclusive use until Close() returns it.
//
// IMPORTANT, confirmed by a failing test: sql.Conn.Close() returns the
// connection to the pool WITHOUT resetting session-level state — search_path
// survives across reuse. An earlier version of this comment claimed
// otherwise; it was wrong. The cleanup func below explicitly runs
// RESET search_path before releasing the connection specifically because
// Close() does not do this. This is defense in depth, not the only
// safeguard: every caller (this function's own callers, and Phase 2's
// per-request accessor) must still explicitly SET search_path at the start
// of its own session rather than assume a clean connection — the reset here
// just means a caller that forgets gets the safe "public" default instead
// of silently inheriting a previous tenant's schema.
//
// search_path is "schemaName, public" — public must stay included because
// PostGIS is not relocatable (Postgres rejects ALTER EXTENSION postgis SET
// SCHEMA outright) and its geometry/geography types live in public
// permanently. Listing the tenant schema first means Postgres resolves any
// unqualified name there before falling through to public, so this is safe
// as long as public itself holds no tenant application tables — see the
// Phase 6 migration, which moves (not copies) every existing table out of
// public rather than leaving duplicates behind.
func TenantScopedSession(schemaName string) (*gorm.DB, func() error, error) {
	return tenantScopedSessionWithContextImpl(context.Background(), schemaName)
}

// tenantScopedSessionWithContextImpl is the single real implementation
// behind both TenantScopedSession (admin/provisioning code, no request
// context available) and DBFromContext (per-request code, propagates the
// request's context for cancellation/timeout). Keeping one implementation
// avoids the two call paths silently drifting apart on something like the
// RESET-on-release behavior below, which testing proved is easy to get
// wrong and hard to notice until it causes cross-tenant leakage.
func tenantScopedSessionWithContextImpl(ctx context.Context, schemaName string) (*gorm.DB, func() error, error) {
	if DB == nil {
		return nil, noopCleanup, errDBNotConnected
	}
	if err := ValidateTenantSchemaName(schemaName); err != nil {
		return nil, noopCleanup, err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("access underlying sql.DB: %w", err)
	}

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("acquire dedicated connection: %w", err)
	}

	tenantDB, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{
		PrepareStmt: false,
		Logger:      DB.Config.Logger,
	})
	if err != nil {
		conn.Close()
		return nil, noopCleanup, fmt.Errorf("open tenant-scoped session: %w", err)
	}

	// schemaName is already validated by ValidateTenantSchemaName's strict
	// ^[a-z_][a-z0-9_]{0,62}$ pattern above, so direct interpolation here
	// cannot introduce SQL injection — Postgres does not support
	// parameterized identifiers.
	if err := tenantDB.WithContext(ctx).Exec(fmt.Sprintf("SET search_path TO %s, public", schemaName)).Error; err != nil {
		conn.Close()
		return nil, noopCleanup, fmt.Errorf("set search_path to %s: %w", schemaName, err)
	}

	cleanup := func() error {
		// Best-effort: if this fails, the connection may still carry
		// search_path when it returns to the pool. Close it regardless —
		// database/sql pools are finite and leaking the connection object
		// itself would be worse than a possibly-unreset one.
		//
		// IMPORTANT, confirmed by a failing test: sql.Conn.Close() returns
		// the connection to the pool WITHOUT resetting session-level state
		// — search_path survives across reuse. This RESET is not optional
		// cleanliness, it is the fix for a real cross-tenant leakage path.
		_ = tenantDB.Exec("RESET search_path").Error
		return conn.Close()
	}

	return tenantDB, cleanup, nil
}

// MigrateAllActiveTenants re-runs the migration list against every active
// tenant's schema. Intended to be run as a required deploy step whenever
// config/migrations.go changes, so tenant schemas never drift out of sync
// with each other — see the risk-mitigation plan for schema drift.
func MigrateAllActiveTenants() error {
	if DB == nil {
		return errDBNotConnected
	}

	var schemaNames []string
	if err := DB.Table("control.tenants").
		Where("is_active = ?", true).
		Pluck("schema_name", &schemaNames).Error; err != nil {
		return fmt.Errorf("load active tenant schemas: %w", err)
	}

	var errs []error
	for _, schemaName := range schemaNames {
		if err := ProvisionTenantSchema(schemaName); err != nil {
			errs = append(errs, fmt.Errorf("tenant schema %s: %w", schemaName, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
