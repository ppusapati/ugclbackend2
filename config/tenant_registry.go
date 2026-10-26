package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"p9e.in/ugcl/models"
)

// ControlSchemaName is the fixed schema holding the tenant registry and
// platform-operator accounts. It is never treated as a tenant schema.
const ControlSchemaName = "control"

// integrationKeyPrefixLen matches the APIKeyPrefix length stored on
// third_party_integrations, so registry lookups can use that column as-is.
const integrationKeyPrefixLen = 12

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

	if err := db.AutoMigrate(&models.Tenant{}, &models.PlatformAdmin{}, &models.IntegrationKey{}); err != nil {
		return fmt.Errorf("migrate control schema tables: %w", err)
	}

	return nil
}

// ActiveTenantSchemas returns the schema name of every active tenant.
// Process-level jobs (boot migrations, cache prewarm, report view sync) have
// no request to take a tenant from, so they must visit each tenant
// explicitly instead of falling back to the global DB's public schema.
func ActiveTenantSchemas() ([]string, error) {
	if DB == nil {
		return nil, errDBNotConnected
	}

	var schemaNames []string
	if err := DB.Table("control.tenants").
		Where("is_active = ?", true).
		Pluck("schema_name", &schemaNames).Error; err != nil {
		return nil, fmt.Errorf("load active tenant schemas: %w", err)
	}
	return schemaNames, nil
}

// tenantIntegrationsTable returns the explicitly schema-qualified
// third_party_integrations table for a tenant, so a tenant missing the table
// errors instead of silently falling through to public via search_path.
func tenantIntegrationsTable(tenantSchema string) (string, error) {
	if err := ValidateTenantSchemaName(tenantSchema); err != nil {
		return "", err
	}
	return tenantSchema + ".third_party_integrations", nil
}

// RegisterIntegrationKey upserts the control-schema row that maps an
// integration's API key to its tenant. Pass the tenant-scoped db (or its
// transaction) so the registry write commits or rolls back with the tenant
// row — control.integration_keys is schema-qualified, so the tenant's
// connection can write it directly.
func RegisterIntegrationKey(db *gorm.DB, tenantSchema string, integration models.ThirdPartyIntegration) error {
	if err := ValidateTenantSchemaName(tenantSchema); err != nil {
		return err
	}

	row := models.IntegrationKey{
		TenantSchema:  tenantSchema,
		IntegrationID: integration.ID,
		APIKeyPrefix:  integration.APIKeyPrefix,
		APIKeyHash:    integration.APIKeyHash,
		Status:        string(integration.Status),
		UpdatedAt:     time.Now(),
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_schema"}, {Name: "integration_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"api_key_prefix", "api_key_hash", "status", "updated_at"}),
	}).Create(&row).Error
}

// UnregisterIntegrationKey removes an integration's key from the control registry.
func UnregisterIntegrationKey(db *gorm.DB, tenantSchema string, integrationID uuid.UUID) error {
	return db.Where("tenant_schema = ? AND integration_id = ?", tenantSchema, integrationID).
		Delete(&models.IntegrationKey{}).Error
}

// ResolveIntegrationKey finds the tenant and active integration an API key
// belongs to. Candidates are narrowed by key prefix in the control registry
// so only matching rows pay for a bcrypt comparison; the integration is then
// loaded from that tenant's own schema, which stays the source of truth.
// ok is false when no active integration matches.
func ResolveIntegrationKey(apiKey string) (tenantSchema string, integration models.ThirdPartyIntegration, ok bool, err error) {
	if DB == nil {
		return "", integration, false, errDBNotConnected
	}

	prefix := apiKey
	if len(prefix) > integrationKeyPrefixLen {
		prefix = prefix[:integrationKeyPrefixLen]
	}

	var candidates []models.IntegrationKey
	if err := DB.Where("api_key_prefix = ? AND status = ?", prefix, models.IntegrationStatusActive).
		Find(&candidates).Error; err != nil {
		return "", integration, false, fmt.Errorf("load integration key candidates: %w", err)
	}

	for _, candidate := range candidates {
		if bcrypt.CompareHashAndPassword([]byte(candidate.APIKeyHash), []byte(apiKey)) != nil {
			continue
		}
		table, tableErr := tenantIntegrationsTable(candidate.TenantSchema)
		if tableErr != nil {
			return "", integration, false, tableErr
		}
		var found models.ThirdPartyIntegration
		loadErr := DB.Table(table).
			Where("id = ? AND status = ?", candidate.IntegrationID, models.IntegrationStatusActive).
			First(&found).Error
		if errors.Is(loadErr, gorm.ErrRecordNotFound) {
			continue // registry row is stale: integration deleted or deactivated in the tenant
		}
		if loadErr != nil {
			return "", integration, false, fmt.Errorf("load integration %s from %s: %w", candidate.IntegrationID, candidate.TenantSchema, loadErr)
		}
		// The key was verified against the registry hash; only accept it if the
		// tenant row still carries that same hash (i.e. it was not regenerated).
		if found.APIKeyHash != candidate.APIKeyHash {
			continue
		}
		return candidate.TenantSchema, found, true, nil
	}

	return "", integration, false, nil
}

// RecordIntegrationAccess bumps an integration's access counters in its tenant schema.
func RecordIntegrationAccess(tenantSchema string, integrationID string, count int64, at time.Time) error {
	if DB == nil {
		return errDBNotConnected
	}
	table, err := tenantIntegrationsTable(tenantSchema)
	if err != nil {
		return err
	}
	return DB.Table(table).
		Where("id = ?", integrationID).
		Updates(map[string]interface{}{
			"last_access_at": at,
			"access_count":   gorm.Expr("access_count + ?", count),
		}).Error
}

// SyncIntegrationKeyRegistry reconciles control.integration_keys with every
// active tenant's third_party_integrations: each live integration is upserted
// and rows for integrations that no longer exist are removed. Keys created
// before the registry existed are backfilled this way, and any drift heals on
// the next boot.
func SyncIntegrationKeyRegistry() error {
	schemaNames, err := ActiveTenantSchemas()
	if err != nil {
		return err
	}

	var errs []error
	for _, schemaName := range schemaNames {
		if err := syncTenantIntegrationKeys(schemaName); err != nil {
			errs = append(errs, fmt.Errorf("tenant schema %s: %w", schemaName, err))
		}
	}
	return errors.Join(errs...)
}

func syncTenantIntegrationKeys(schemaName string) error {
	table, err := tenantIntegrationsTable(schemaName)
	if err != nil {
		return err
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var integrations []models.ThirdPartyIntegration
		if err := tx.Table(table).Find(&integrations).Error; err != nil {
			return fmt.Errorf("load integrations: %w", err)
		}

		liveIDs := make([]uuid.UUID, 0, len(integrations))
		for _, integration := range integrations {
			if err := RegisterIntegrationKey(tx, schemaName, integration); err != nil {
				return fmt.Errorf("register integration %s: %w", integration.ID, err)
			}
			liveIDs = append(liveIDs, integration.ID)
		}

		stale := tx.Where("tenant_schema = ?", schemaName)
		if len(liveIDs) > 0 {
			stale = stale.Where("integration_id NOT IN ?", liveIDs)
		}
		return stale.Delete(&models.IntegrationKey{}).Error
	})
}
