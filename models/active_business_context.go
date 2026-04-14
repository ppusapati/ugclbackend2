package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserActiveBusinessContext stores the currently selected business per user/client pair.
type UserActiveBusinessContext struct {
	ID         uuid.UUID         `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID         `gorm:"type:uuid;not null;index:idx_user_active_business_client,unique" json:"user_id"`
	User       *User             `gorm:"foreignKey:UserID" json:"user,omitempty"`
	BusinessID uuid.UUID         `gorm:"type:uuid;not null;index;index:idx_user_active_business_client,unique" json:"business_id"`
	Business   *BusinessVertical `gorm:"foreignKey:BusinessID;references:ID" json:"business,omitempty"`
	ClientKey  string            `gorm:"size:100;not null;default:default;index:idx_user_active_business_client,unique" json:"client_key"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func (uabc *UserActiveBusinessContext) BeforeCreate(tx *gorm.DB) (err error) {
	uabc.ID = uuid.New()
	return nil
}
