package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Site represents a physical site/location within a business vertical
// For example: Water Works has 4 sites, Solar Works has 12 sites
type Site struct {
	ID                 uuid.UUID        `gorm:"type:uuid;primaryKey" json:"id"`
	Name               string           `gorm:"size:100;not null" json:"name"`               // e.g., "Water Site A", "Solar Panel Site 1"
	Code               string           `gorm:"size:50;uniqueIndex;not null" json:"code"`   // e.g., "WATER_SITE_A", "SOLAR_01"
	Description        string           `gorm:"size:255" json:"description"`
	BusinessVerticalID uuid.UUID        `gorm:"type:uuid;not null;index" json:"businessVerticalId"`
	BusinessVertical   BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	Location           *string          `gorm:"type:jsonb" json:"location,omitempty"` // JSON with lat, lng, address
	Geofence           *string          `gorm:"type:jsonb" json:"geofence,omitempty"` // JSON array of coordinates: [{lat, lng}, ...]
	IsActive           bool             `gorm:"default:true" json:"isActive"`
	CreatedAt          time.Time        `json:"createdAt"`
	UpdatedAt          time.Time        `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt   `gorm:"index" json:"-"`

	// Relationships
	UserSiteAccess []UserSiteAccess `gorm:"foreignKey:SiteID" json:"-"`
}

// UserSiteAccess represents which sites a user has access to within a business vertical
// This allows fine-grained control: a user might have access to only 2 out of 4 water sites
type UserSiteAccess struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	SiteID    uuid.UUID `gorm:"type:uuid;not null;index" json:"siteId"`
	Site      Site      `gorm:"foreignKey:SiteID" json:"site,omitempty"`

	// Access levels for this specific site
	CanRead   bool `gorm:"default:true" json:"canRead"`
	CanCreate bool `gorm:"default:false" json:"canCreate"`
	CanUpdate bool `gorm:"default:false" json:"canUpdate"`
	CanDelete bool `gorm:"default:false" json:"canDelete"`

	AssignedAt time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"assignedAt"`
	AssignedBy *uuid.UUID `gorm:"type:uuid" json:"assignedBy,omitempty"` // Who granted this access
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// BeforeCreate hook for Site
func (s *Site) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return
}

// BeforeCreate hook for UserSiteAccess
func (usa *UserSiteAccess) BeforeCreate(tx *gorm.DB) (err error) {
	if usa.ID == uuid.Nil {
		usa.ID = uuid.New()
	}
	return
}

// Helper function to check if a user has access to a specific site
func (u *User) HasSiteAccess(db *gorm.DB, siteID uuid.UUID) bool {
	var access UserSiteAccess
	err := db.Where("user_id = ? AND site_id = ? AND can_read = ?", u.ID, siteID, true).
		First(&access).Error
	return err == nil
}

// Helper function to get all sites a user has access to within a business vertical
func (u *User) GetAccessibleSites(db *gorm.DB, businessVerticalID uuid.UUID) ([]Site, error) {
	var sites []Site
	err := db.
		Joins("JOIN user_site_accesses ON user_site_accesses.site_id = sites.id").
		Where("user_site_accesses.user_id = ? AND sites.business_vertical_id = ?", u.ID, businessVerticalID).
		Find(&sites).Error
	return sites, err
}

// Helper function to get site IDs user can access
func (u *User) GetAccessibleSiteIDs(db *gorm.DB, businessVerticalID uuid.UUID) ([]uuid.UUID, error) {
	var siteIDs []uuid.UUID
	err := db.Table("user_site_accesses").
		Select("user_site_accesses.site_id").
		Joins("JOIN sites ON sites.id = user_site_accesses.site_id").
		Where("user_site_accesses.user_id = ? AND sites.business_vertical_id = ? AND user_site_accesses.can_read = ?",
			u.ID, businessVerticalID, true).
		Pluck("site_id", &siteIDs).Error
	return siteIDs, err
}
