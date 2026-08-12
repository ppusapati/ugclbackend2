package config

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// ControlSchemaName is the fixed schema holding the tenant registry and
// platform-operator accounts. It is never treated as a tenant schema.
const ControlSchemaName = "control"

// EnsureControlSchema creates the control schema (if missing) and migrates
// the tenant registry table into it. Safe to call on every startup —
// CREATE SCHEMA IF NOT EXISTS and AutoMigrate are both idempotent.
//
// This is intentionally separate from the main Migrations() list: the
// control schema is not a tenant schema and must never be provisioned by
// the per-tenant migration runner (see ProvisionTenantSchema).
func EnsureControlSchema(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is required")
	}

	if err := db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", ControlSchemaName)).Error; err != nil {
		return fmt.Errorf("create control schema: %w", err)
	}

	if err := db.AutoMigrate(&models.Tenant{}, &models.PlatformAdmin{}); err != nil {
		return fmt.Errorf("migrate control schema tables: %w", err)
	}

	return nil
}
