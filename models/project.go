package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Project represents a project with KMZ data
type Project struct {
	ID                 uuid.UUID         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code               string            `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Name               string            `gorm:"size:255;not null" json:"name"`
	Description        string            `gorm:"type:text" json:"description,omitempty"`
	BusinessVerticalID uuid.UUID         `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	BusinessVertical   *BusinessVertical `gorm:"foreignKey:BusinessVerticalID" json:"business_vertical,omitempty"`

	// File information
	KMZFileName   string     `gorm:"size:255" json:"kmz_file_name,omitempty"`
	KMZFilePath   string     `gorm:"size:500" json:"kmz_file_path,omitempty"`
	KMZUploadedAt *time.Time `json:"kmz_uploaded_at,omitempty"`

	// GeoJSON data extracted from KMZ
	GeoJSONData json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"geojson_data,omitempty"`

	// Project timeline
	StartDate       *time.Time `json:"start_date,omitempty"`
	EndDate         *time.Time `json:"end_date,omitempty"`
	ActualStartDate *time.Time `json:"actual_start_date,omitempty"`
	ActualEndDate   *time.Time `json:"actual_end_date,omitempty"`

	// Budget
	TotalBudget     float64 `gorm:"type:decimal(15,2);default:0" json:"total_budget"`
	AllocatedBudget float64 `gorm:"type:decimal(15,2);default:0" json:"allocated_budget"`
	SpentBudget     float64 `gorm:"type:decimal(15,2);default:0" json:"spent_budget"`
	Currency        string  `gorm:"size:10;default:'INR'" json:"currency"`

	// Status
	Status   string  `gorm:"size:50;not null;default:'draft';index" json:"status"` // draft, active, on-hold, completed, cancelled
	Progress float64 `gorm:"type:decimal(5,2);default:0" json:"progress"`          // 0-100

	// Workflow integration
	WorkflowID *uuid.UUID          `gorm:"type:uuid" json:"workflow_id,omitempty"`
	Workflow   *WorkflowDefinition `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`

	// Metadata
	CreatedBy string     `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy string     `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Zones             []Zone             `gorm:"foreignKey:ProjectID" json:"zones,omitempty"`
	Tasks             []Tasks            `gorm:"foreignKey:ProjectID" json:"tasks,omitempty"`
	BudgetAllocations []BudgetAllocation `gorm:"foreignKey:ProjectID" json:"budget_allocations,omitempty"`
}

// TableName specifies the table name for Project
func (Project) TableName() string {
	return "projects"
}

// Zone represents a geographical zone within a project
type Zone struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Project   *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`

	Name        string `gorm:"size:255;not null" json:"name"`
	Code        string `gorm:"size:50" json:"code,omitempty"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	Label       string `gorm:"size:255" json:"label,omitempty"`

	// Geometry data (PostGIS)
	Geometry string  `gorm:"type:geometry(Geometry,4326)" json:"geometry,omitempty"`
	Centroid string  `gorm:"type:geometry(Point,4326)" json:"centroid,omitempty"`
	Area     float64 `gorm:"type:decimal(15,2)" json:"area,omitempty"` // in square meters

	// GeoJSON representation
	GeoJSON json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"geojson,omitempty"`

	// Additional properties from KMZ
	Properties json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"properties,omitempty"`

	// Metadata
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Nodes []Node `gorm:"foreignKey:ZoneID" json:"nodes,omitempty"`
}

// TableName specifies the table name for Zone
func (Zone) TableName() string {
	return "zones"
}

// Node represents a point/node within a zone (start or stop node)
type Node struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ZoneID    uuid.UUID `gorm:"type:uuid;not null;index" json:"zone_id"`
	Zone      *Zone     `gorm:"foreignKey:ZoneID" json:"zone,omitempty"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Project   *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`

	Name        string `gorm:"size:255;not null" json:"name"`
	Code        string `gorm:"size:50" json:"code,omitempty"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	Label       string `gorm:"size:255" json:"label,omitempty"`
	NodeType    string `gorm:"size:50;not null;index" json:"node_type"` // start, stop, waypoint

	// Location (PostGIS)
	Location  string  `gorm:"type:geometry(Point,4326);not null" json:"location"`
	Latitude  float64 `gorm:"type:decimal(10,8)" json:"latitude"`
	Longitude float64 `gorm:"type:decimal(11,8)" json:"longitude"`
	Elevation float64 `gorm:"type:decimal(10,2)" json:"elevation,omitempty"`

	// GeoJSON representation
	GeoJSON json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"geojson,omitempty"`

	// Additional properties from KMZ
	Properties json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"properties,omitempty"`

	// Status
	Status string `gorm:"size:50;default:'available';index" json:"status"` // available, allocated, in-progress, completed

	// Metadata
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	StartTasks []Tasks `gorm:"foreignKey:StartNodeID" json:"start_tasks,omitempty"`
	StopTasks  []Tasks `gorm:"foreignKey:StopNodeID" json:"stop_tasks,omitempty"`
}

// TableName specifies the table name for Node
func (Node) TableName() string {
	return "nodes"
}

// Task represents a work task allocated to users
type Tasks struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code        string    `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description,omitempty"`

	// Project context
	ProjectID uuid.UUID  `gorm:"type:uuid;not null;index" json:"project_id"`
	Project   *Project   `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	ZoneID    *uuid.UUID `gorm:"type:uuid;index" json:"zone_id,omitempty"`
	Zone      *Zone      `gorm:"foreignKey:ZoneID" json:"zone,omitempty"`

	// Node references
	StartNodeID uuid.UUID `gorm:"type:uuid;not null;index" json:"start_node_id"`
	StartNode   *Node     `gorm:"foreignKey:StartNodeID" json:"start_node,omitempty"`
	StopNodeID  uuid.UUID `gorm:"type:uuid;not null;index" json:"stop_node_id"`
	StopNode    *Node     `gorm:"foreignKey:StopNodeID" json:"stop_node,omitempty"`

	// Timeline
	PlannedStartDate *time.Time `json:"planned_start_date,omitempty"`
	PlannedEndDate   *time.Time `json:"planned_end_date,omitempty"`
	ActualStartDate  *time.Time `json:"actual_start_date,omitempty"`
	ActualEndDate    *time.Time `json:"actual_end_date,omitempty"`

	// Budget
	AllocatedBudget float64 `gorm:"type:decimal(15,2);default:0" json:"allocated_budget"`
	LaborCost       float64 `gorm:"type:decimal(15,2);default:0" json:"labor_cost"`
	MaterialCost    float64 `gorm:"type:decimal(15,2);default:0" json:"material_cost"`
	EquipmentCost   float64 `gorm:"type:decimal(15,2);default:0" json:"equipment_cost"`
	OtherCost       float64 `gorm:"type:decimal(15,2);default:0" json:"other_cost"`
	TotalCost       float64 `gorm:"type:decimal(15,2);default:0" json:"total_cost"`

	// Status and progress
	Status   string  `gorm:"size:50;not null;default:'pending';index" json:"status"` // pending, assigned, in-progress, on-hold, completed, cancelled
	Progress float64 `gorm:"type:decimal(5,2);default:0" json:"progress"`            // 0-100
	Priority string  `gorm:"size:20;default:'medium';index" json:"priority"`         // low, medium, high, critical

	// Workflow integration
	WorkflowID   *uuid.UUID          `gorm:"type:uuid" json:"workflow_id,omitempty"`
	Workflow     *WorkflowDefinition `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	CurrentState string              `gorm:"size:50;index" json:"current_state,omitempty"`

	// Form submission (if using app_forms)
	FormSubmissionID *uuid.UUID      `gorm:"type:uuid" json:"form_submission_id,omitempty"`
	FormSubmission   *FormSubmission `gorm:"foreignKey:FormSubmissionID" json:"form_submission,omitempty"`

	// Additional data
	Metadata json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	// Metadata
	CreatedBy string     `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy string     `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Assignments []TaskAssignment `gorm:"foreignKey:TaskID" json:"assignments,omitempty"`
	AuditLogs   []TaskAuditLog   `gorm:"foreignKey:TaskID" json:"audit_logs,omitempty"`
	Comments    []TaskComment    `gorm:"foreignKey:TaskID" json:"comments,omitempty"`
	Attachments []TaskAttachment `gorm:"foreignKey:TaskID" json:"attachments,omitempty"`
}

// TableName specifies the table name for Task
func (Tasks) TableName() string {
	return "tasks"
}

// TaskAssignment represents user assignments to tasks with roles
type TaskAssignment struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TaskID uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
	Task   *Tasks    `gorm:"foreignKey:TaskID" json:"task,omitempty"`

	// User assignment
	UserID   string `gorm:"size:255;not null;index" json:"user_id"`
	UserName string `gorm:"size:255" json:"user_name,omitempty"`
	UserType string `gorm:"size:50;not null;index" json:"user_type"` // employee, contractor, supervisor
	Role     string `gorm:"size:50;not null" json:"role"`            // worker, supervisor, manager, approver

	// Assignment details
	AssignedBy string     `gorm:"size:255;not null" json:"assigned_by"`
	AssignedAt time.Time  `gorm:"not null" json:"assigned_at"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`

	// Status
	Status   string `gorm:"size:50;not null;default:'active';index" json:"status"` // active, inactive, completed
	IsActive bool   `gorm:"default:true" json:"is_active"`

	// Permissions
	CanEdit    bool `gorm:"default:false" json:"can_edit"`
	CanApprove bool `gorm:"default:false" json:"can_approve"`

	// Metadata
	Notes     string     `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name for TaskAssignment
func (TaskAssignment) TableName() string {
	return "task_assignments"
}

// BudgetAllocation represents budget allocation at project or task level
type BudgetAllocation struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`

	// Project or Task reference
	ProjectID *uuid.UUID `gorm:"type:uuid;index" json:"project_id,omitempty"`
	Project   *Project   `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	TaskID    *uuid.UUID `gorm:"type:uuid;index" json:"task_id,omitempty"`
	Task      *Tasks     `gorm:"foreignKey:TaskID" json:"task,omitempty"`

	// Budget details
	Category      string  `gorm:"size:50;not null;index" json:"category"` // labor, material, equipment, overhead, contingency
	Description   string  `gorm:"type:text" json:"description,omitempty"`
	PlannedAmount float64 `gorm:"type:decimal(15,2);not null" json:"planned_amount"`
	ActualAmount  float64 `gorm:"type:decimal(15,2);default:0" json:"actual_amount"`
	Currency      string  `gorm:"size:10;default:'INR'" json:"currency"`

	// Timeline
	AllocationDate time.Time  `gorm:"not null" json:"allocation_date"`
	StartDate      *time.Time `json:"start_date,omitempty"`
	EndDate        *time.Time `json:"end_date,omitempty"`

	// Status
	Status string `gorm:"size:50;not null;default:'allocated';index" json:"status"` // allocated, in-use, spent, cancelled

	// Approval
	ApprovedBy string     `gorm:"size:255" json:"approved_by,omitempty"`
	ApprovedAt *time.Time `json:"approved_at,omitempty"`

	// Metadata
	Notes     string     `gorm:"type:text" json:"notes,omitempty"`
	CreatedBy string     `gorm:"size:255;not null" json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name for BudgetAllocation
func (BudgetAllocation) TableName() string {
	return "budget_allocations"
}

// TaskAuditLog represents audit trail for task changes
type TaskAuditLog struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TaskID uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
	Task   *Tasks    `gorm:"foreignKey:TaskID" json:"task,omitempty"`

	// Change details
	Action   string `gorm:"size:50;not null;index" json:"action"` // created, updated, status_changed, assigned, approved, rejected
	Field    string `gorm:"size:100" json:"field,omitempty"`
	OldValue string `gorm:"type:text" json:"old_value,omitempty"`
	NewValue string `gorm:"type:text" json:"new_value,omitempty"`

	// Actor information
	PerformedBy     string `gorm:"size:255;not null" json:"performed_by"`
	PerformedByName string `gorm:"size:255" json:"performed_by_name,omitempty"`
	Role            string `gorm:"size:100" json:"role,omitempty"`

	// Additional context
	Comment   string          `gorm:"type:text" json:"comment,omitempty"`
	Metadata  json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	IPAddress string          `gorm:"size:50" json:"ip_address,omitempty"`
	UserAgent string          `gorm:"size:500" json:"user_agent,omitempty"`

	// Timestamp
	PerformedAt time.Time `gorm:"not null;index" json:"performed_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName specifies the table name for TaskAuditLog
func (TaskAuditLog) TableName() string {
	return "task_audit_logs"
}

// TaskComment represents comments on tasks
type TaskComment struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TaskID uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
	Task   *Tasks    `gorm:"foreignKey:TaskID" json:"task,omitempty"`

	// Comment details
	Comment     string `gorm:"type:text;not null" json:"comment"`
	CommentType string `gorm:"size:50;default:'general';index" json:"comment_type"` // general, update, issue, resolution

	// Author
	AuthorID   string `gorm:"size:255;not null;index" json:"author_id"`
	AuthorName string `gorm:"size:255" json:"author_name,omitempty"`

	// Parent comment (for replies)
	ParentID *uuid.UUID   `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	Parent   *TaskComment `gorm:"foreignKey:ParentID" json:"parent,omitempty"`

	// Metadata
	IsEdited  bool       `gorm:"default:false" json:"is_edited"`
	EditedAt  *time.Time `json:"edited_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name for TaskComment
func (TaskComment) TableName() string {
	return "task_comments"
}

// TaskAttachment represents file attachments for tasks
type TaskAttachment struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TaskID uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
	Task   *Tasks    `gorm:"foreignKey:TaskID" json:"task,omitempty"`

	// File details
	FileName string `gorm:"size:255;not null" json:"file_name"`
	FilePath string `gorm:"size:500;not null" json:"file_path"`
	FileSize int64  `json:"file_size"`
	FileType string `gorm:"size:100" json:"file_type,omitempty"`
	MimeType string `gorm:"size:100" json:"mime_type,omitempty"`

	// Attachment metadata
	AttachmentType string `gorm:"size:50;default:'document';index" json:"attachment_type"` // document, image, video, other
	Description    string `gorm:"type:text" json:"description,omitempty"`

	// Uploader
	UploadedBy     string `gorm:"size:255;not null" json:"uploaded_by"`
	UploadedByName string `gorm:"size:255" json:"uploaded_by_name,omitempty"`

	// Metadata
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name for TaskAttachment
func (TaskAttachment) TableName() string {
	return "task_attachments"
}

// ProjectRole represents project-specific roles
type ProjectRole struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code        string    `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`

	// Permissions
	Permissions StringArray `gorm:"type:jsonb;default:'[]'" json:"permissions"`

	// Role hierarchy
	Level        int          `gorm:"default:0" json:"level"` // 0=lowest, higher=more authority
	ParentRoleID *uuid.UUID   `gorm:"type:uuid" json:"parent_role_id,omitempty"`
	ParentRole   *ProjectRole `gorm:"foreignKey:ParentRoleID" json:"parent_role,omitempty"`

	// Metadata
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	IsSystemRole bool      `gorm:"default:false" json:"is_system_role"` // Cannot be deleted
	CreatedBy    string    `gorm:"size:255" json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName specifies the table name for ProjectRole
func (ProjectRole) TableName() string {
	return "project_roles"
}

// UserProjectRole represents user-to-project role assignments
type UserProjectRole struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`

	UserID    string       `gorm:"size:255;not null;index" json:"user_id"`
	ProjectID uuid.UUID    `gorm:"type:uuid;not null;index" json:"project_id"`
	Project   *Project     `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	RoleID    uuid.UUID    `gorm:"type:uuid;not null;index" json:"role_id"`
	Role      *ProjectRole `gorm:"foreignKey:RoleID" json:"role,omitempty"`

	// Assignment details
	AssignedBy string     `gorm:"size:255;not null" json:"assigned_by"`
	AssignedAt time.Time  `gorm:"not null" json:"assigned_at"`
	ValidFrom  *time.Time `json:"valid_from,omitempty"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`

	// Status
	IsActive bool `gorm:"default:true;index" json:"is_active"`

	// Metadata
	Notes     string    `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Composite unique index on user + project (only one active role per user per project)
	// UniqueIndex: user_id, project_id, is_active (where is_active = true)
}

// TableName specifies the table name for UserProjectRole
func (UserProjectRole) TableName() string {
	return "user_project_roles"
}

// ProjectMetadata stores additional key-value metadata for projects
type ProjectMetadata map[string]interface{}

// Scan implements the sql.Scanner interface
func (m *ProjectMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = make(ProjectMetadata)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		*m = make(ProjectMetadata)
		return nil
	}

	return json.Unmarshal(bytes, m)
}

// Value implements the driver.Valuer interface
func (m ProjectMetadata) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(make(map[string]interface{}))
	}
	return json.Marshal(m)
}

// GormDataType defines the data type for GORM
func (ProjectMetadata) GormDataType() string {
	return "jsonb"
}
