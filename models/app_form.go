package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Module represents a module/section in the application (e.g., Projects, HR, Finance)
type Module struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code         string    `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	Description  string    `gorm:"type:text" json:"description,omitempty"`
	Icon         string    `gorm:"size:50" json:"icon,omitempty"`
	Route        string    `gorm:"size:200" json:"route,omitempty"`
	DisplayOrder int       `gorm:"default:0" json:"display_order"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Database schema for this module (stores form tables)
	SchemaName string `gorm:"size:63" json:"schema_name,omitempty"`

	// Access control - which business verticals can access this module
	AccessibleVerticals StringArray `gorm:"type:jsonb;default:'[]'" json:"accessible_verticals"`

	// Required permission to view this module
	RequiredPermission string `gorm:"size:100" json:"required_permission,omitempty"`

	// Relationships
	Forms []AppForm `gorm:"foreignKey:ModuleID" json:"forms,omitempty"`
}

// IsAccessibleInVertical checks if the module is accessible in a given vertical
func (m *Module) IsAccessibleInVertical(verticalCode string) bool {
	// If no verticals specified, module is accessible everywhere
	if len(m.AccessibleVerticals) == 0 {
		return true
	}
	for _, v := range m.AccessibleVerticals {
		if v == verticalCode {
			return true
		}
	}
	return false
}

// TableName specifies the table name for Module
func (Module) TableName() string {
	return "modules"
}

// StringArray is a custom type for JSONB string arrays
type StringArray []string

// Scan implements the sql.Scanner interface for StringArray
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		*s = []string{}
		return nil
	}

	return json.Unmarshal(bytes, s)
}

// Value implements the driver.Valuer interface for StringArray
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(s)
}

// GormDataType defines the data type for GORM
func (StringArray) GormDataType() string {
	return "jsonb"
}

// AppForm represents a form/feature in the mobile application
type AppForm struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code        string    `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	Version     string    `gorm:"size:50;not null;default:'1.0.0'" json:"version"`

	// Module association
	ModuleID uuid.UUID `gorm:"type:uuid;not null" json:"module_id"`
	Module   *Module   `gorm:"foreignKey:ModuleID" json:"module,omitempty"`

	// Navigation
	Route        string `gorm:"size:200;not null" json:"route"`
	Icon         string `gorm:"size:50" json:"icon,omitempty"`
	DisplayOrder int    `gorm:"default:0" json:"display_order"`

	// Access control
	RequiredPermission  string      `gorm:"size:100" json:"required_permission,omitempty"`
	AllowedRoles        StringArray `gorm:"type:jsonb;default:'[]'" json:"allowed_roles,omitempty"`
	AccessibleVerticals StringArray `gorm:"type:jsonb;default:'[]'" json:"accessible_verticals"`

	// Form definition (JSON-based)
	FormSchema   json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"form_schema,omitempty"`
	Steps        json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"steps,omitempty"`
	CoreFields   json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"core_fields,omitempty"`
	Validations  json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"validations,omitempty"`
	Dependencies json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"dependencies,omitempty"`

	// Workflow integration
	WorkflowID   *uuid.UUID `gorm:"type:uuid" json:"workflow_id,omitempty"`
	InitialState string     `gorm:"size:100;default:'draft'" json:"initial_state,omitempty"`

	// Database table mapping
	DBTableName   string `gorm:"size:255" json:"table_name,omitempty"`
	SchemaVersion int    `gorm:"default:1" json:"schema_version"`

	// Metadata
	IsActive  bool       `gorm:"default:true" json:"is_active"`
	Audit     bool       `gorm:"default:false" json:"audit"`
	CreatedBy string     `gorm:"size:255" json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// TableName specifies the table name for AppForm
func (AppForm) TableName() string {
	return "app_forms"
}

// AppFormDTO represents the simplified data transfer object for forms
type AppFormDTO struct {
	Code                string   `json:"code"`
	Title               string   `json:"title"`
	Description         string   `json:"description,omitempty"`
	Module              string   `json:"module"`
	Route               string   `json:"route"`
	Icon                string   `json:"icon,omitempty"`
	RequiredPermission  string   `json:"required_permission,omitempty"`
	AccessibleVerticals []string `json:"accessible_verticals,omitempty"`
	DisplayOrder        int      `json:"display_order"`
}

// ToDTO converts an AppForm to AppFormDTO
func (f *AppForm) ToDTO() AppFormDTO {
	moduleCode := ""
	if f.Module != nil {
		moduleCode = f.Module.Code
	}

	return AppFormDTO{
		Code:                f.Code,
		Title:               f.Title,
		Description:         f.Description,
		Module:              moduleCode,
		Route:               f.Route,
		Icon:                f.Icon,
		RequiredPermission:  f.RequiredPermission,
		AccessibleVerticals: f.AccessibleVerticals,
		DisplayOrder:        f.DisplayOrder,
	}
}

// ToDTOWithSchema converts an AppForm to full DTO including schema
func (f *AppForm) ToDTOWithSchema() map[string]interface{} {
	dto := f.ToDTO()

	result := map[string]interface{}{
		"code":                 dto.Code,
		"title":                dto.Title,
		"description":          dto.Description,
		"module":               dto.Module,
		"route":                dto.Route,
		"icon":                 dto.Icon,
		"required_permission":  dto.RequiredPermission,
		"accessible_verticals": dto.AccessibleVerticals,
		"display_order":        dto.DisplayOrder,
	}

	// Include form schema if present
	if len(f.FormSchema) > 0 && string(f.FormSchema) != "{}" {
		var schema interface{}
		if err := json.Unmarshal(f.FormSchema, &schema); err == nil {
			result["form_schema"] = schema
		}
	}

	// Include steps if present
	if len(f.Steps) > 0 && string(f.Steps) != "[]" {
		var steps interface{}
		if err := json.Unmarshal(f.Steps, &steps); err == nil {
			result["steps"] = steps
		}
	}

	return result
}

// IsAccessibleInVertical checks if the form is accessible in a given vertical
func (f *AppForm) IsAccessibleInVertical(verticalCode string) bool {
	for _, v := range f.AccessibleVerticals {
		if v == verticalCode {
			return true
		}
	}
	return false
}
