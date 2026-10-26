package models

import (
	"time"

	"github.com/google/uuid"
)

// IntegrationKey maps a third-party integration's API key to the tenant
// schema that owns it. It lives in the control schema because API-key
// requests carry no JWT, so the tenant must be resolvable from the key alone,
// before any tenant schema is known. The integration itself (scopes, IPs,
// URLs) stays in the tenant's own third_party_integrations table; this row
// only mirrors what is needed to find it.
type IntegrationKey struct {
	TenantSchema  string    `gorm:"size:63;primaryKey"`
	IntegrationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	APIKeyPrefix  string    `gorm:"size:12;not null;index"` // first 12 chars of the key, for indexed lookup
	APIKeyHash    string    `gorm:"size:128;not null"`      // bcrypt, same value as the tenant row
	Status        string    `gorm:"size:20;not null;index"`
	UpdatedAt     time.Time
}

func (IntegrationKey) TableName() string {
	return "control.integration_keys"
}
