package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserLoginEvent stores authenticated login events for account-security auditing.
type UserLoginEvent struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index:idx_user_login_events_user_time,priority:1" json:"user_id"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	LoginAt   time.Time `gorm:"not null;index:idx_user_login_events_user_time,priority:2" json:"login_at"`
	IPAddress string    `gorm:"size:45" json:"ip_address"`
	UserAgent string    `gorm:"size:512" json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

func (ule *UserLoginEvent) BeforeCreate(tx *gorm.DB) (err error) {
	ule.ID = uuid.New()
	if ule.LoginAt.IsZero() {
		ule.LoginAt = time.Now().UTC()
	}
	return
}
