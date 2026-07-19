package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TrustedDevice stores per-user trusted mobile device bindings for offline login policy.
type TrustedDevice struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID      `gorm:"type:uuid;not null;index:idx_trusted_device_user_client,priority:1" json:"userId"`
	User             User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ClientID         string         `gorm:"size:128;not null;index:idx_trusted_device_user_client,priority:2" json:"clientId"`
	InstallID        *string        `gorm:"size:128;index" json:"installId,omitempty"`
	Platform         *string        `gorm:"size:32" json:"platform,omitempty"`
	DeviceName       *string        `gorm:"size:255" json:"deviceName,omitempty"`
	AppVersion       *string        `gorm:"size:64" json:"appVersion,omitempty"`
	OfflineAllowed   bool           `gorm:"default:true;index" json:"offlineAllowed"`
	LastSeenAt       time.Time      `gorm:"index" json:"lastSeenAt"`
	RevokedAt        *time.Time     `json:"revokedAt,omitempty"`
	RevocationReason *string        `gorm:"size:255" json:"revocationReason,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (TrustedDevice) TableName() string {
	return "trusted_devices"
}

func (td *TrustedDevice) BeforeCreate(tx *gorm.DB) (err error) {
	if td.ID == uuid.Nil {
		td.ID = uuid.New()
	}
	if td.LastSeenAt.IsZero() {
		td.LastSeenAt = time.Now().UTC()
	}
	return nil
}
