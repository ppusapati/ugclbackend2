package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ReportDefinition represents a saved report configuration
type ReportDefinition struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code        string    `gorm:"size:100;uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	Category    string    `gorm:"size:100" json:"category,omitempty"` // Analytics, Operations, Finance, etc.

	// Report configuration
	ReportType   string          `gorm:"size:50;not null" json:"report_type"` // table, chart, pivot, dashboard, kpi
	DataSources  json.RawMessage `gorm:"type:jsonb;not null" json:"data_sources"`
	Fields       json.RawMessage `gorm:"type:jsonb;not null" json:"fields"`
	Filters      json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"filters,omitempty"`
	Groupings    json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"groupings,omitempty"`
	Aggregations json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"aggregations,omitempty"`
	Sorting      json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"sorting,omitempty"`
	Calculations json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"calculations,omitempty"`

	// Visualization settings
	ChartType   string          `gorm:"size:50" json:"chart_type,omitempty"` // bar, line, pie, area, scatter
	ChartConfig json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"chart_config,omitempty"`
	Layout      json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"layout,omitempty"`

	// Access control
	BusinessVerticalID uuid.UUID   `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	IsPublic           bool        `gorm:"default:false" json:"is_public"`
	AllowedRoles       StringArray `gorm:"type:jsonb;default:'[]'" json:"allowed_roles,omitempty"`
	AllowedUsers       StringArray `gorm:"type:jsonb;default:'[]'" json:"allowed_users,omitempty"`

	// Scheduling
	IsScheduled     bool            `gorm:"default:false" json:"is_scheduled"`
	ScheduleConfig  json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"schedule_config,omitempty"`
	Recipients      StringArray     `gorm:"type:jsonb;default:'[]'" json:"recipients,omitempty"`
	LastExecutedAt  *time.Time      `json:"last_executed_at,omitempty"`
	NextExecutionAt *time.Time      `json:"next_execution_at,omitempty"`

	// Export settings
	ExportFormats StringArray     `gorm:"type:jsonb;default:'[\"pdf\",\"excel\",\"csv\"]'" json:"export_formats"`
	ExportOptions json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"export_options,omitempty"`

	// Metadata
	IsActive   bool        `gorm:"default:true" json:"is_active"`
	IsFavorite bool        `gorm:"default:false" json:"is_favorite"`
	Tags       StringArray `gorm:"type:jsonb;default:'[]'" json:"tags,omitempty"`
	CreatedBy  string      `gorm:"size:255;not null" json:"created_by"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedBy  string      `gorm:"size:255" json:"updated_by,omitempty"`
	UpdatedAt  time.Time   `json:"updated_at"`
	DeletedAt  *time.Time  `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name for ReportDefinition
func (ReportDefinition) TableName() string {
	return "report_definitions"
}

// DataSource represents a form table to query from
type DataSource struct {
	Alias     string `json:"alias"`      // e.g., "inspections", "sites"
	TableName string `json:"table_name"` // e.g., "form_site_inspection"
	FormCode  string `json:"form_code"`  // Reference to original form
	FormID    string `json:"form_id"`
	JoinType  string `json:"join_type,omitempty"` // left, right, inner (for multi-table reports)
	JoinOn    string `json:"join_on,omitempty"`   // join condition
}

// ReportField represents a field to include in the report
type ReportField struct {
	FieldName   string `json:"field_name"`            // Column name in the table
	Alias       string `json:"alias,omitempty"`       // Display name
	DataSource  string `json:"data_source"`           // Which table/alias
	DataType    string `json:"data_type"`             // text, number, date, boolean
	IsVisible   bool   `json:"is_visible"`            // Show in output
	Width       int    `json:"width,omitempty"`       // Column width (for table reports)
	Format      string `json:"format,omitempty"`      // Date format, number format, etc.
	Aggregation string `json:"aggregation,omitempty"` // sum, avg, count, min, max
	Order       int    `json:"order"`                 // Display order
}

// ReportFilter represents a filter condition
type ReportFilter struct {
	FieldName  string      `json:"field_name"`
	DataSource string      `json:"data_source"`
	Operator   string      `json:"operator"` // eq, ne, gt, lt, gte, lte, in, between, like, is_null, is_not_null
	Value      interface{} `json:"value"`
	LogicalOp  string      `json:"logical_op,omitempty"` // AND, OR
	GroupID    string      `json:"group_id,omitempty"`   // For complex filter grouping
}

// ReportGrouping represents a GROUP BY clause
type ReportGrouping struct {
	FieldName  string `json:"field_name"`
	DataSource string `json:"data_source"`
	Order      int    `json:"order"`
}

// ReportAggregation represents aggregation functions
type ReportAggregation struct {
	Function   string `json:"function"` // SUM, AVG, COUNT, MIN, MAX, COUNT_DISTINCT
	FieldName  string `json:"field_name"`
	DataSource string `json:"data_source"`
	Alias      string `json:"alias"`
	Format     string `json:"format,omitempty"`
}

// ReportCalculation represents a calculated/computed field
type ReportCalculation struct {
	Name       string `json:"name"`
	Expression string `json:"expression"` // SQL expression or formula
	DataType   string `json:"data_type"`
	Format     string `json:"format,omitempty"`
}

// ReportSorting represents ORDER BY clauses
type ReportSorting struct {
	FieldName  string `json:"field_name"`
	DataSource string `json:"data_source,omitempty"`
	Direction  string `json:"direction"` // ASC, DESC
	Order      int    `json:"order"`
}

// ScheduleConfig represents scheduling configuration
type ScheduleConfig struct {
	Frequency  string `json:"frequency"`              // daily, weekly, monthly
	Time       string `json:"time"`                   // HH:MM
	DayOfWeek  int    `json:"day_of_week,omitempty"`  // 0-6 (Sunday-Saturday)
	DayOfMonth int    `json:"day_of_month,omitempty"` // 1-31
	Timezone   string `json:"timezone"`
	Enabled    bool   `json:"enabled"`
}

// ReportExecution represents a report execution history
type ReportExecution struct {
	ID            uuid.UUID         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ReportID      uuid.UUID         `gorm:"type:uuid;not null;index" json:"report_id"`
	Report        *ReportDefinition `gorm:"foreignKey:ReportID" json:"report,omitempty"`
	ExecutionType string            `gorm:"size:50;not null" json:"execution_type"` // manual, scheduled, api
	ExecutedBy    string            `gorm:"size:255" json:"executed_by,omitempty"`

	// Execution details
	StartedAt   time.Time  `gorm:"not null" json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Duration    int        `json:"duration"`                       // milliseconds
	Status      string     `gorm:"size:50;not null" json:"status"` // running, completed, failed

	// Results
	RowCount     int    `json:"row_count"`
	ResultSize   int64  `json:"result_size"` // bytes
	FilePath     string `gorm:"size:500" json:"file_path,omitempty"`
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`

	// Filters applied (for historical tracking)
	AppliedFilters json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"applied_filters,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for ReportExecution
func (ReportExecution) TableName() string {
	return "report_executions"
}

// ReportWidget represents a widget in a dashboard
type ReportWidget struct {
	ID          uuid.UUID         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	DashboardID uuid.UUID         `gorm:"type:uuid;not null;index" json:"dashboard_id"`
	ReportID    uuid.UUID         `gorm:"type:uuid;not null" json:"report_id"`
	Report      *ReportDefinition `gorm:"foreignKey:ReportID" json:"report,omitempty"`

	Title       string          `gorm:"size:255" json:"title"`
	Position    json.RawMessage `gorm:"type:jsonb;not null" json:"position"` // {x, y, w, h}
	RefreshRate int             `json:"refresh_rate"`                        // seconds, 0 = manual

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for ReportWidget
func (ReportWidget) TableName() string {
	return "report_widgets"
}

// Dashboard represents a collection of report widgets
type Dashboard struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code        string    `gorm:"size:100;uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`

	BusinessVerticalID uuid.UUID       `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	Layout             json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"layout"`

	// Access control
	IsPublic     bool        `gorm:"default:false" json:"is_public"`
	AllowedRoles StringArray `gorm:"type:jsonb;default:'[]'" json:"allowed_roles,omitempty"`

	IsActive  bool `gorm:"default:true" json:"is_active"`
	IsDefault bool `gorm:"default:false" json:"is_default"` // Default dashboard for role/vertical

	CreatedBy string     `gorm:"size:255;not null" json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Widgets []ReportWidget `gorm:"foreignKey:DashboardID" json:"widgets,omitempty"`
}

// TableName specifies the table name for Dashboard
func (Dashboard) TableName() string {
	return "dashboards"
}

// ReportTemplate represents pre-built report templates
type ReportTemplate struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code        string    `gorm:"size:100;uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	Category    string    `gorm:"size:100" json:"category"`
	Icon        string    `gorm:"size:50" json:"icon,omitempty"`

	// Template configuration (same structure as ReportDefinition)
	Template json.RawMessage `gorm:"type:jsonb;not null" json:"template"`

	IsActive   bool `gorm:"default:true" json:"is_active"`
	UsageCount int  `gorm:"default:0" json:"usage_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for ReportTemplate
func (ReportTemplate) TableName() string {
	return "report_templates"
}

// ReportShare represents shared report links
type ReportShare struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ReportID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"report_id"`
	ShareToken string     `gorm:"size:100;uniqueIndex;not null" json:"share_token"`
	ShareType  string     `gorm:"size:50;not null" json:"share_type"` // public, password, users
	Password   string     `gorm:"size:255" json:"password,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	MaxViews   int        `json:"max_views,omitempty"`
	ViewCount  int        `gorm:"default:0" json:"view_count"`
	IsActive   bool       `gorm:"default:true" json:"is_active"`
	SharedBy   string     `gorm:"size:255;not null" json:"shared_by"`
	CreatedAt  time.Time  `json:"created_at"`
}

// TableName specifies the table name for ReportShare
func (ReportShare) TableName() string {
	return "report_shares"
}
