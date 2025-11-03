package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DocumentStatus defines the status of a document
type DocumentStatus string

const (
	DocumentStatusDraft    DocumentStatus = "draft"
	DocumentStatusPending  DocumentStatus = "pending"
	DocumentStatusApproved DocumentStatus = "approved"
	DocumentStatusRejected DocumentStatus = "rejected"
	DocumentStatusArchived DocumentStatus = "archived"
	DocumentStatusDeleted  DocumentStatus = "deleted"
)

// DocumentAccessLevel defines access level for document permissions
type DocumentAccessLevel string

const (
	DocumentAccessNone    DocumentAccessLevel = "none"
	DocumentAccessView    DocumentAccessLevel = "view"
	DocumentAccessComment DocumentAccessLevel = "comment"
	DocumentAccessEdit    DocumentAccessLevel = "edit"
	DocumentAccessManage  DocumentAccessLevel = "manage"
)

// DocumentAuditAction defines the type of audit action
type DocumentAuditAction string

const (
	DocumentAuditActionCreate           DocumentAuditAction = "create"
	DocumentAuditActionView             DocumentAuditAction = "view"
	DocumentAuditActionDownload         DocumentAuditAction = "download"
	DocumentAuditActionEdit             DocumentAuditAction = "edit"
	DocumentAuditActionDelete           DocumentAuditAction = "delete"
	DocumentAuditActionShare            DocumentAuditAction = "share"
	DocumentAuditActionUnshare          DocumentAuditAction = "unshare"
	DocumentAuditActionVersionCreate    DocumentAuditAction = "version_create"
	DocumentAuditActionVersionRollback  DocumentAuditAction = "version_rollback"
	DocumentAuditActionPermissionChange DocumentAuditAction = "permission_change"
	DocumentAuditActionStatusChange     DocumentAuditAction = "status_change"
)

// DocumentMetadata stores flexible metadata as JSON
type DocumentMetadata map[string]interface{}

// Scan implements sql.Scanner interface
func (m *DocumentMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = make(DocumentMetadata)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, m)
}

// Value implements driver.Valuer interface
func (m DocumentMetadata) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(make(DocumentMetadata))
	}
	return json.Marshal(m)
}

// DocumentCategory represents document categories for organization
type DocumentCategory struct {
	ID                 uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	Name               string            `gorm:"size:100;not null" json:"name"`
	Description        string            `gorm:"type:text" json:"description"`
	ParentID           *uuid.UUID        `gorm:"type:uuid" json:"parent_id"`
	Parent             *DocumentCategory `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Color              string            `gorm:"size:20" json:"color"` // For UI representation
	Icon               string            `gorm:"size:50" json:"icon"`  // Icon name/class
	BusinessVerticalID *uuid.UUID        `gorm:"type:uuid" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`
	IsActive           bool              `gorm:"default:true" json:"is_active"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	DeletedAt          gorm.DeletedAt    `gorm:"index" json:"deleted_at,omitempty"`
}

func (dc *DocumentCategory) BeforeCreate(tx *gorm.DB) (err error) {
	dc.ID = uuid.New()
	return
}

// DocumentTag represents tags for document classification
type DocumentTag struct {
	ID                 uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	Name               string            `gorm:"size:50;not null;uniqueIndex:idx_tag_business" json:"name"`
	Color              string            `gorm:"size:20" json:"color"`
	BusinessVerticalID *uuid.UUID        `gorm:"type:uuid;uniqueIndex:idx_tag_business" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

func (dt *DocumentTag) BeforeCreate(tx *gorm.DB) (err error) {
	dt.ID = uuid.New()
	return
}

// Document represents a document in the system
type Document struct {
	ID            uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	Title         string            `gorm:"size:255;not null" json:"title"`
	Description   string            `gorm:"type:text" json:"description"`
	FileName      string            `gorm:"size:255;not null" json:"file_name"`
	FileSize      int64             `gorm:"not null" json:"file_size"`          // Size in bytes
	FileType      string            `gorm:"size:100;not null" json:"file_type"` // MIME type
	FileExtension string            `gorm:"size:20;not null" json:"file_extension"`
	FilePath      string            `gorm:"size:500;not null" json:"file_path"` // Storage path
	FileHash      string            `gorm:"size:64" json:"file_hash"`           // SHA256 hash for deduplication
	ThumbnailPath string            `gorm:"size:500" json:"thumbnail_path"`
	PreviewPath   string            `gorm:"size:500" json:"preview_path"`
	Status        DocumentStatus    `gorm:"type:varchar(20);default:'draft'" json:"status"`
	Version       int               `gorm:"default:1" json:"version"`
	CategoryID    *uuid.UUID        `gorm:"type:uuid" json:"category_id"`
	Category      *DocumentCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	// Use a distinct join table name to avoid collision with the DocumentTag base table (document_tags)
	// The default many2many table name "document_tags" conflicts with the DocumentTag model table name.
	// Renaming to "document_tag_links" ensures correct FK references: documents(id) and document_tags(id)
	Tags               []DocumentTag       `gorm:"many2many:document_tag_links;" json:"tags,omitempty"`
	Metadata           DocumentMetadata    `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	BusinessVerticalID *uuid.UUID          `gorm:"type:uuid;not null" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical   `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`
	UploadedByID       uuid.UUID           `gorm:"type:uuid;not null" json:"uploaded_by_id"`
	UploadedBy         *User               `gorm:"foreignKey:UploadedByID" json:"uploaded_by,omitempty"`
	WorkflowID         *uuid.UUID          `gorm:"type:uuid" json:"workflow_id"`
	Workflow           *WorkflowDefinition `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	CurrentState       string              `gorm:"size:50" json:"current_state"`
	ExpiresAt          *time.Time          `json:"expires_at,omitempty"`
	IsPublic           bool                `gorm:"default:false" json:"is_public"`
	DownloadCount      int                 `gorm:"default:0" json:"download_count"`
	ViewCount          int                 `gorm:"default:0" json:"view_count"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
	DeletedAt          gorm.DeletedAt      `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Versions    []DocumentVersion    `gorm:"foreignKey:DocumentID" json:"versions,omitempty"`
	Permissions []DocumentPermission `gorm:"foreignKey:DocumentID" json:"permissions,omitempty"`
	AuditLogs   []DocumentAuditLog   `gorm:"foreignKey:DocumentID" json:"audit_logs,omitempty"`
	Shares      []DocumentShare      `gorm:"foreignKey:DocumentID" json:"shares,omitempty"`
}

func (d *Document) BeforeCreate(tx *gorm.DB) (err error) {
	d.ID = uuid.New()
	return
}

// DocumentVersion represents a version of a document
type DocumentVersion struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	DocumentID       uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	Document         *Document `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
	VersionNumber    int       `gorm:"not null" json:"version_number"`
	FileName         string    `gorm:"size:255;not null" json:"file_name"`
	FileSize         int64     `gorm:"not null" json:"file_size"`
	FileType         string    `gorm:"size:100;not null" json:"file_type"`
	FilePath         string    `gorm:"size:500;not null" json:"file_path"`
	FileHash         string    `gorm:"size:64" json:"file_hash"`
	ChangeLog        string    `gorm:"type:text" json:"change_log"`
	CreatedByID      uuid.UUID `gorm:"type:uuid;not null" json:"created_by_id"`
	CreatedBy        *User     `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	IsCurrentVersion bool      `gorm:"default:false" json:"is_current_version"`
}

func (dv *DocumentVersion) BeforeCreate(tx *gorm.DB) (err error) {
	dv.ID = uuid.New()
	return
}

// DocumentPermission represents fine-grained permissions for documents
type DocumentPermission struct {
	ID             uuid.UUID           `gorm:"type:uuid;primaryKey" json:"id"`
	DocumentID     uuid.UUID           `gorm:"type:uuid;not null;index" json:"document_id"`
	Document       *Document           `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
	UserID         *uuid.UUID          `gorm:"type:uuid" json:"user_id"`
	User           *User               `gorm:"foreignKey:UserID" json:"user,omitempty"`
	RoleID         *uuid.UUID          `gorm:"type:uuid" json:"role_id"`
	Role           *Role               `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	BusinessRoleID *uuid.UUID          `gorm:"type:uuid" json:"business_role_id"`
	BusinessRole   *BusinessRole       `gorm:"foreignKey:BusinessRoleID" json:"business_role,omitempty"`
	AccessLevel    DocumentAccessLevel `gorm:"type:varchar(20);not null" json:"access_level"`
	CanDownload    bool                `gorm:"default:true" json:"can_download"`
	CanShare       bool                `gorm:"default:false" json:"can_share"`
	CanDelete      bool                `gorm:"default:false" json:"can_delete"`
	ExpiresAt      *time.Time          `json:"expires_at,omitempty"`
	GrantedByID    uuid.UUID           `gorm:"type:uuid;not null" json:"granted_by_id"`
	GrantedBy      *User               `gorm:"foreignKey:GrantedByID" json:"granted_by,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

func (dp *DocumentPermission) BeforeCreate(tx *gorm.DB) (err error) {
	dp.ID = uuid.New()
	return
}

// DocumentShare represents a shareable link for a document
type DocumentShare struct {
	ID          uuid.UUID           `gorm:"type:uuid;primaryKey" json:"id"`
	DocumentID  uuid.UUID           `gorm:"type:uuid;not null;index" json:"document_id"`
	Document    *Document           `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
	ShareToken  string              `gorm:"size:64;uniqueIndex;not null" json:"share_token"`
	AccessLevel DocumentAccessLevel `gorm:"type:varchar(20);not null" json:"access_level"`
	CanDownload bool                `gorm:"default:true" json:"can_download"`
	Password    string              `gorm:"size:255" json:"-"`           // Hashed password for protected shares
	MaxAccess   int                 `gorm:"default:0" json:"max_access"` // 0 = unlimited
	AccessCount int                 `gorm:"default:0" json:"access_count"`
	ExpiresAt   *time.Time          `json:"expires_at,omitempty"`
	CreatedByID uuid.UUID           `gorm:"type:uuid;not null" json:"created_by_id"`
	CreatedBy   *User               `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
	IsActive    bool                `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

func (ds *DocumentShare) BeforeCreate(tx *gorm.DB) (err error) {
	ds.ID = uuid.New()
	return
}

// DocumentAuditLog tracks all actions performed on documents
type DocumentAuditLog struct {
	ID         uuid.UUID           `gorm:"type:uuid;primaryKey" json:"id"`
	DocumentID uuid.UUID           `gorm:"type:uuid;not null;index" json:"document_id"`
	Document   *Document           `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
	UserID     *uuid.UUID          `gorm:"type:uuid" json:"user_id"`
	User       *User               `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Action     DocumentAuditAction `gorm:"type:varchar(50);not null" json:"action"`
	Details    DocumentMetadata    `gorm:"type:jsonb;default:'{}'" json:"details"`
	IPAddress  string              `gorm:"size:45" json:"ip_address"`
	UserAgent  string              `gorm:"size:255" json:"user_agent"`
	CreatedAt  time.Time           `json:"created_at"`
}

func (dal *DocumentAuditLog) BeforeCreate(tx *gorm.DB) (err error) {
	dal.ID = uuid.New()
	return
}

// DocumentRetentionPolicy defines retention rules for documents
type DocumentRetentionPolicy struct {
	ID                 uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	Name               string            `gorm:"size:100;not null" json:"name"`
	Description        string            `gorm:"type:text" json:"description"`
	CategoryID         *uuid.UUID        `gorm:"type:uuid" json:"category_id"`
	Category           *DocumentCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	BusinessVerticalID *uuid.UUID        `gorm:"type:uuid" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`
	RetentionDays      int               `gorm:"not null" json:"retention_days"` // Days to keep document
	AutoArchive        bool              `gorm:"default:true" json:"auto_archive"`
	AutoDelete         bool              `gorm:"default:false" json:"auto_delete"`
	IsActive           bool              `gorm:"default:true" json:"is_active"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

func (drp *DocumentRetentionPolicy) BeforeCreate(tx *gorm.DB) (err error) {
	drp.ID = uuid.New()
	return
}
