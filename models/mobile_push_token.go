package models

import (
	"time"

	"github.com/google/uuid"
)

// MobilePushToken stores mobile device push tokens for Android/iOS devices.
type MobilePushToken struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID     string    `gorm:"size:255;not null;index" json:"user_id"`
	Token      string    `gorm:"size:512;not null;uniqueIndex" json:"token"`
	Platform   string    `gorm:"size:20;not null;index" json:"platform"`
	DeviceID   *string   `gorm:"size:255;index" json:"device_id,omitempty"`
	DeviceName *string   `gorm:"size:255" json:"device_name,omitempty"`
	AppVersion *string   `gorm:"size:50" json:"app_version,omitempty"`
	IsActive   bool      `gorm:"default:true;index" json:"is_active"`
	LastSeenAt time.Time `gorm:"index" json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (MobilePushToken) TableName() string {
	return "mobile_push_tokens"
}
