package models

import (
	"time"

	"github.com/google/uuid"
)

// WebPushSubscription stores browser push endpoints for a user/device.
type WebPushSubscription struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID         string     `gorm:"size:255;not null;index" json:"user_id"`
	Endpoint       string     `gorm:"size:2000;not null;uniqueIndex" json:"endpoint"`
	P256DH         string     `gorm:"size:1000;not null" json:"p256dh"`
	Auth           string     `gorm:"size:255;not null" json:"auth"`
	ExpirationTime *time.Time `json:"expiration_time,omitempty"`
	UserAgent      *string    `gorm:"size:1000" json:"user_agent,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (WebPushSubscription) TableName() string {
	return "web_push_subscriptions"
}
