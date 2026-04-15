package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AttendanceSessionStatusActive    = "active"
	AttendanceSessionStatusCompleted = "completed"
	AttendanceSessionStatusFlagged   = "flagged"

	AttendanceEventTypeCheckIn   = "check_in"
	AttendanceEventTypeHeartbeat = "heartbeat"
	AttendanceEventTypeCheckOut  = "check_out"
	AttendanceEventTypeAnomaly   = "anomaly"

	AttendanceValidationAccepted = "accepted"
	AttendanceValidationFlagged  = "flagged"
	AttendanceValidationRejected = "rejected"
)

// AttendanceSession stores the current attendance state for an employee at a site.
type AttendanceSession struct {
	ID                 uuid.UUID        `gorm:"type:uuid;primaryKey" json:"id"`
	UserID             uuid.UUID        `gorm:"type:uuid;not null;index:idx_attendance_sessions_user_status,priority:1" json:"userId"`
	User               User             `gorm:"foreignKey:UserID" json:"user,omitempty"`
	SiteID             uuid.UUID        `gorm:"type:uuid;not null;index:idx_attendance_sessions_site_status,priority:1" json:"siteId"`
	Site               Site             `gorm:"foreignKey:SiteID" json:"site,omitempty"`
	BusinessVerticalID uuid.UUID        `gorm:"type:uuid;not null;index:idx_attendance_sessions_business_status,priority:1" json:"businessVerticalId"`
	BusinessVertical   BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	Status             string           `gorm:"size:20;not null;default:'active';index:idx_attendance_sessions_user_status,priority:2;index:idx_attendance_sessions_site_status,priority:2;index:idx_attendance_sessions_business_status,priority:2" json:"status"`
	CheckInAt          time.Time        `gorm:"not null;index" json:"checkInAt"`
	CheckOutAt         *time.Time       `gorm:"index" json:"checkOutAt,omitempty"`
	LastSeenAt         time.Time        `gorm:"not null;index" json:"lastSeenAt"`
	CheckInLatitude    float64          `gorm:"not null" json:"checkInLatitude"`
	CheckInLongitude   float64          `gorm:"not null" json:"checkInLongitude"`
	CheckInAccuracy    float64          `gorm:"not null" json:"checkInAccuracy"`
	CheckOutLatitude   *float64         `json:"checkOutLatitude,omitempty"`
	CheckOutLongitude  *float64         `json:"checkOutLongitude,omitempty"`
	CheckOutAccuracy   *float64         `json:"checkOutAccuracy,omitempty"`
	LastLatitude       float64          `gorm:"not null" json:"lastLatitude"`
	LastLongitude      float64          `gorm:"not null" json:"lastLongitude"`
	LastAccuracy       float64          `gorm:"not null" json:"lastAccuracy"`
	DeviceID           string           `gorm:"size:128;not null;index" json:"deviceId"`
	ValidationMethod   string           `gorm:"size:50;not null" json:"validationMethod"`
	ValidationStatus   string           `gorm:"size:20;not null;default:'accepted'" json:"validationStatus"`
	ValidationReason   *string          `gorm:"size:255" json:"validationReason,omitempty"`
	AnomalyFlags       *string          `gorm:"type:jsonb" json:"anomalyFlags,omitempty"`
	Metadata           *string          `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt          time.Time        `json:"createdAt"`
	UpdatedAt          time.Time        `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt   `gorm:"index" json:"-"`

	Events []AttendanceEvent `gorm:"foreignKey:SessionID" json:"events,omitempty"`
	Pings  []TrackingPing    `gorm:"foreignKey:SessionID" json:"pings,omitempty"`
}

// AttendanceEvent stores immutable lifecycle and anomaly events for auditability.
type AttendanceEvent struct {
	ID                 uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	SessionID          uuid.UUID         `gorm:"type:uuid;not null;index:idx_attendance_events_session_time,priority:1" json:"sessionId"`
	Session            AttendanceSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	UserID             uuid.UUID         `gorm:"type:uuid;not null;index:idx_attendance_events_user_time,priority:1" json:"userId"`
	User               User              `gorm:"foreignKey:UserID" json:"user,omitempty"`
	SiteID             uuid.UUID         `gorm:"type:uuid;not null;index:idx_attendance_events_site_time,priority:1" json:"siteId"`
	Site               Site              `gorm:"foreignKey:SiteID" json:"site,omitempty"`
	BusinessVerticalID uuid.UUID         `gorm:"type:uuid;not null;index:idx_attendance_events_business_time,priority:1" json:"businessVerticalId"`
	BusinessVertical   BusinessVertical  `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	EventType          string            `gorm:"size:20;not null;index" json:"eventType"`
	EventTime          time.Time         `gorm:"not null;index:idx_attendance_events_session_time,priority:2;index:idx_attendance_events_user_time,priority:2;index:idx_attendance_events_site_time,priority:2;index:idx_attendance_events_business_time,priority:2" json:"eventTime"`
	Latitude           float64           `gorm:"not null" json:"latitude"`
	Longitude          float64           `gorm:"not null" json:"longitude"`
	Accuracy           float64           `gorm:"not null" json:"accuracy"`
	DeviceID           string            `gorm:"size:128;not null;index" json:"deviceId"`
	ValidationMethod   string            `gorm:"size:50;not null" json:"validationMethod"`
	ValidationStatus   string            `gorm:"size:20;not null" json:"validationStatus"`
	ValidationReason   *string           `gorm:"size:255" json:"validationReason,omitempty"`
	AnomalyFlags       *string           `gorm:"type:jsonb" json:"anomalyFlags,omitempty"`
	IsMockLocation     bool              `gorm:"default:false" json:"isMockLocation"`
	IsGpsEnabled       bool              `gorm:"default:true" json:"isGpsEnabled"`
	AppState           *string           `gorm:"size:30" json:"appState,omitempty"`
	NetworkStatus      *string           `gorm:"size:30" json:"networkStatus,omitempty"`
	BatteryLevel       *float64          `json:"batteryLevel,omitempty"`
	Payload            *string           `gorm:"type:jsonb" json:"payload,omitempty"`
	ServerReceivedAt   time.Time         `gorm:"not null;index" json:"serverReceivedAt"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt    `gorm:"index" json:"-"`
}

// TrackingPing stores periodic location samples during an active attendance session.
type TrackingPing struct {
	ID                 uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	SessionID          uuid.UUID         `gorm:"type:uuid;not null;index:idx_tracking_pings_session_time,priority:1" json:"sessionId"`
	Session            AttendanceSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	UserID             uuid.UUID         `gorm:"type:uuid;not null;index:idx_tracking_pings_user_time,priority:1" json:"userId"`
	User               User              `gorm:"foreignKey:UserID" json:"user,omitempty"`
	SiteID             uuid.UUID         `gorm:"type:uuid;not null;index:idx_tracking_pings_site_time,priority:1" json:"siteId"`
	Site               Site              `gorm:"foreignKey:SiteID" json:"site,omitempty"`
	BusinessVerticalID uuid.UUID         `gorm:"type:uuid;not null;index:idx_tracking_pings_business_time,priority:1" json:"businessVerticalId"`
	BusinessVertical   BusinessVertical  `gorm:"foreignKey:BusinessVerticalID" json:"businessVertical,omitempty"`
	PingTime           time.Time         `gorm:"not null;index:idx_tracking_pings_session_time,priority:2;index:idx_tracking_pings_user_time,priority:2;index:idx_tracking_pings_site_time,priority:2;index:idx_tracking_pings_business_time,priority:2" json:"pingTime"`
	Latitude           float64           `gorm:"not null" json:"latitude"`
	Longitude          float64           `gorm:"not null" json:"longitude"`
	Accuracy           float64           `gorm:"not null" json:"accuracy"`
	DeviceID           string            `gorm:"size:128;not null;index" json:"deviceId"`
	InsideGeofence     bool              `gorm:"default:false;index" json:"insideGeofence"`
	DistanceFromSiteM  *float64          `json:"distanceFromSiteM,omitempty"`
	IsMockLocation     bool              `gorm:"default:false" json:"isMockLocation"`
	IsGpsEnabled       bool              `gorm:"default:true" json:"isGpsEnabled"`
	ClockSkewSeconds   *int              `json:"clockSkewSeconds,omitempty"`
	SyncStatus         string            `gorm:"size:20;not null;default:'received'" json:"syncStatus"`
	AnomalyFlags       *string           `gorm:"type:jsonb" json:"anomalyFlags,omitempty"`
	Payload            *string           `gorm:"type:jsonb" json:"payload,omitempty"`
	ServerReceivedAt   time.Time         `gorm:"not null;index" json:"serverReceivedAt"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt    `gorm:"index" json:"-"`
}

func (s *AttendanceSession) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (e *AttendanceEvent) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

func (p *TrackingPing) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
