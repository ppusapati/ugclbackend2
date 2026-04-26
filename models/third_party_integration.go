package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	IntegrationScopePartnerDPRSiteRead    = "partner.dprsite.read"
	IntegrationScopePartnerWrappingRead   = "partner.wrapping.read"
	IntegrationScopePartnerEWayRead       = "partner.eway.read"
	IntegrationScopePartnerWaterRead      = "partner.water.read"
	IntegrationScopePartnerStockRead      = "partner.stock.read"
	IntegrationScopePartnerDairySiteRead  = "partner.dairysite.read"
	IntegrationScopePartnerPaymentRead    = "partner.payment.read"
	IntegrationScopePartnerMaterialRead   = "partner.material.read"
	IntegrationScopePartnerMNRRead        = "partner.mnr.read"
	IntegrationScopePartnerNMRVehicleRead = "partner.nmr_vehicle.read"
	IntegrationScopePartnerContractorRead = "partner.contractor.read"
	IntegrationScopePartnerPaintingRead   = "partner.painting.read"
	IntegrationScopePartnerDieselRead     = "partner.diesel.read"
	IntegrationScopePartnerTasksRead      = "partner.tasks.read"
	IntegrationScopePartnerVehicleLogRead = "partner.vehiclelog.read"
	IntegrationScopeDropdownProxyUse      = "integration.dropdown.proxy"
	IntegrationScopeDocumentAIUse         = "integration.document.ai.use"
)

// IntegrationStatus represents the operational state of a third-party integration.
type IntegrationStatus string

const (
	IntegrationStatusActive    IntegrationStatus = "active"
	IntegrationStatusInactive  IntegrationStatus = "inactive"
	IntegrationStatusSuspended IntegrationStatus = "suspended"
)

// ThirdPartyIntegration holds admin-configured third-party integrations.
// Each record controls which callback URLs are allowlisted, which source IPs
// are permitted to query our APIs, and what data scopes they may access.
type ThirdPartyIntegration struct {
	ID           uuid.UUID                   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name         string                      `gorm:"type:varchar(200);not null"                     json:"name"`
	Description  string                      `gorm:"type:text"                                      json:"description,omitempty"`
	Status       IntegrationStatus           `gorm:"type:varchar(20);not null;default:'active'"     json:"status"`
	Provider     string                      `gorm:"type:varchar(50)"                               json:"provider,omitempty"`
	EndpointURL  string                      `gorm:"type:text"                                      json:"endpoint_url,omitempty"`
	Model        string                      `gorm:"type:varchar(120)"                              json:"model,omitempty"`
	AuthHeader   string                      `gorm:"type:varchar(100)"                              json:"auth_header,omitempty"`
	AuthScheme   string                      `gorm:"type:varchar(50)"                               json:"auth_scheme,omitempty"`
	SecretCipher string                      `gorm:"type:text"                                      json:"-"`
	APIKeyHash   string                      `gorm:"type:varchar(128);not null"                     json:"-"`              // bcrypt-hashed, never returned
	APIKeyPrefix string                      `gorm:"type:varchar(12);not null"                      json:"api_key_prefix"` // first 8 chars for display
	AllowedURLs  datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]'"               json:"allowed_urls"`
	AllowedIPs   datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]'"               json:"allowed_ips"`
	DataScopes   datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]'"               json:"data_scopes"`
	ContactEmail string                      `gorm:"type:varchar(320)"                              json:"contact_email,omitempty"`
	CreatedBy    uuid.UUID                   `gorm:"type:uuid"                                      json:"created_by,omitempty"`
	LastAccessAt *time.Time                  `json:"last_accessed_at,omitempty"`
	AccessCount  int64                       `gorm:"default:0"                                      json:"access_count"`
	CreatedAt    time.Time                   `json:"created_at"`
	UpdatedAt    time.Time                   `json:"updated_at"`
	DeletedAt    gorm.DeletedAt              `gorm:"index"                                          json:"deleted_at,omitempty"`
}
