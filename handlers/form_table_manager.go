package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// FormTableManager handles dynamic table creation and data management for forms
type FormTableManager struct {
	db *gorm.DB
}

// NewFormTableManager creates a new form table manager
func NewFormTableManager() *FormTableManager {
	return &FormTableManager{
		db: config.DB,
	}
}

// BaseFormFields represents the standard fields all form tables must have
type BaseFormFields struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedBy string     `gorm:"size:255;not null" json:"created_by"`
	CreatedAt time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedBy string     `gorm:"size:255" json:"updated_by,omitempty"`
	UpdatedAt time.Time  `gorm:"default:now()" json:"updated_at"`
	DeletedBy string     `gorm:"size:255" json:"deleted_by,omitempty"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Business context
	BusinessVerticalID uuid.UUID `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
	SiteID             *uuid.UUID `gorm:"type:uuid;index" json:"site_id,omitempty"`

	// Workflow integration
	WorkflowID   *uuid.UUID `gorm:"type:uuid" json:"workflow_id,omitempty"`
	CurrentState string     `gorm:"size:50;not null;default:'draft';index" json:"current_state"`

	// Reference to form
	FormID   uuid.UUID `gorm:"type:uuid;not null;index" json:"form_id"`
	FormCode string    `gorm:"size:50;not null;index" json:"form_code"`
}

// CreateFormTable creates a dedicated table for a form based on its schema
func (ftm *FormTableManager) CreateFormTable(form *models.AppForm) error {
	if form.DBTableName == "" {
		return fmt.Errorf("form %s has no table name defined", form.Code)
	}

	log.Printf("📊 Creating dedicated table: %s for form: %s", form.DBTableName, form.Code)

	// Parse form schema to get field definitions
	var formSchema map[string]interface{}
	if len(form.FormSchema) > 0 && string(form.FormSchema) != "{}" {
		if err := json.Unmarshal(form.FormSchema, &formSchema); err != nil {
			return fmt.Errorf("failed to parse form schema: %v", err)
		}
	}

	// Build CREATE TABLE SQL
	sql := ftm.buildCreateTableSQL(form.DBTableName, formSchema)

	// Execute table creation
	if err := ftm.db.Exec(sql).Error; err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	log.Printf("✅ Successfully created table: %s", form.DBTableName)
	return nil
}

// buildCreateTableSQL generates the SQL for creating a form table
func (ftm *FormTableManager) buildCreateTableSQL(tableName string, formSchema map[string]interface{}) string {
	var columns []string

	// Base fields (always present)
	columns = append(columns,
		"id UUID PRIMARY KEY DEFAULT gen_random_uuid()",
		"created_by VARCHAR(255) NOT NULL",
		"created_at TIMESTAMP NOT NULL DEFAULT NOW()",
		"updated_by VARCHAR(255)",
		"updated_at TIMESTAMP DEFAULT NOW()",
		"deleted_by VARCHAR(255)",
		"deleted_at TIMESTAMP",
		"business_vertical_id UUID NOT NULL REFERENCES business_verticals(id)",
		"site_id UUID REFERENCES sites(id)",
		"workflow_id UUID REFERENCES workflow_definitions(id)",
		"current_state VARCHAR(50) NOT NULL DEFAULT 'draft'",
		"form_id UUID NOT NULL REFERENCES app_forms(id)",
		"form_code VARCHAR(50) NOT NULL",
	)

	// Parse form fields from schema
	if fields, ok := formSchema["fields"].([]interface{}); ok {
		for _, field := range fields {
			if fieldMap, ok := field.(map[string]interface{}); ok {
				columnDef := ftm.getColumnDefinition(fieldMap)
				if columnDef != "" {
					columns = append(columns, columnDef)
				}
			}
		}
	}

	// Create indexes
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n);", tableName, strings.Join(columns, ",\n  "))

	// Add indexes
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_business_vertical ON %s(business_vertical_id);", tableName, tableName)
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_site ON %s(site_id);", tableName, tableName)
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_state ON %s(current_state);", tableName, tableName)
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_form ON %s(form_id);", tableName, tableName)
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_deleted ON %s(deleted_at);", tableName, tableName)

	return sql
}

// getColumnDefinition converts form field definition to SQL column definition
func (ftm *FormTableManager) getColumnDefinition(field map[string]interface{}) string {
	name, ok := field["name"].(string)
	if !ok || name == "" {
		return ""
	}

	// Sanitize column name
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")

	fieldType, _ := field["type"].(string)
	required, _ := field["required"].(bool)

	var sqlType string
	switch fieldType {
	case "text", "textarea", "email", "url", "phone":
		if maxLength, ok := field["max_length"].(float64); ok && maxLength > 0 {
			sqlType = fmt.Sprintf("VARCHAR(%d)", int(maxLength))
		} else {
			sqlType = "TEXT"
		}
	case "number", "integer":
		sqlType = "INTEGER"
	case "decimal", "currency":
		sqlType = "DECIMAL(15,2)"
	case "date":
		sqlType = "DATE"
	case "datetime", "timestamp":
		sqlType = "TIMESTAMP"
	case "time":
		sqlType = "TIME"
	case "boolean", "checkbox":
		sqlType = "BOOLEAN"
	case "select", "radio":
		sqlType = "VARCHAR(255)"
	case "multiselect", "checkbox_group":
		sqlType = "JSONB"
	case "file", "image":
		sqlType = "VARCHAR(500)" // Store file path
	case "json", "object":
		sqlType = "JSONB"
	default:
		sqlType = "TEXT"
	}

	column := fmt.Sprintf("%s %s", name, sqlType)

	if required {
		column += " NOT NULL"
	}

	return column
}

// InsertFormData inserts form submission data into the dedicated table
func (ftm *FormTableManager) InsertFormData(
	tableName string,
	formID uuid.UUID,
	formCode string,
	businessVerticalID uuid.UUID,
	siteID *uuid.UUID,
	workflowID *uuid.UUID,
	initialState string,
	formData map[string]interface{},
	userID string,
) (uuid.UUID, error) {
	// Add base fields to form data
	recordID := uuid.New()
	formData["id"] = recordID
	formData["form_id"] = formID
	formData["form_code"] = formCode
	formData["business_vertical_id"] = businessVerticalID
	formData["site_id"] = siteID
	formData["workflow_id"] = workflowID
	formData["current_state"] = initialState
	formData["created_by"] = userID
	formData["created_at"] = time.Now()
	formData["updated_at"] = time.Now()

	// Build INSERT SQL dynamically
	var columns []string
	var placeholders []string
	var values []interface{}
	i := 1

	for col, val := range formData {
		columns = append(columns, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	sql := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING id",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	var returnedID uuid.UUID
	err := ftm.db.Raw(sql, values...).Scan(&returnedID).Error
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert form data: %v", err)
	}

	log.Printf("✅ Inserted record %s into table %s", returnedID, tableName)
	return returnedID, nil
}

// UpdateFormData updates form submission data in the dedicated table
func (ftm *FormTableManager) UpdateFormData(
	tableName string,
	recordID uuid.UUID,
	formData map[string]interface{},
	userID string,
) error {
	// Add update metadata
	formData["updated_by"] = userID
	formData["updated_at"] = time.Now()

	// Build UPDATE SQL dynamically
	var setClauses []string
	var values []interface{}
	i := 1

	for col, val := range formData {
		// Skip read-only fields
		if col == "id" || col == "created_by" || col == "created_at" {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		values = append(values, val)
		i++
	}

	// Add WHERE clause
	values = append(values, recordID)
	whereClause := fmt.Sprintf("id = $%d", i)

	sql := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s",
		tableName,
		strings.Join(setClauses, ", "),
		whereClause,
	)

	err := ftm.db.Exec(sql, values...).Error
	if err != nil {
		return fmt.Errorf("failed to update form data: %v", err)
	}

	log.Printf("✅ Updated record %s in table %s", recordID, tableName)
	return nil
}

// GetFormData retrieves form submission data from the dedicated table
func (ftm *FormTableManager) GetFormData(tableName string, recordID uuid.UUID) (map[string]interface{}, error) {
	sql := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 AND deleted_at IS NULL", tableName)

	var result map[string]interface{}
	rows, err := ftm.db.Raw(sql, recordID).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to query form data: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("record not found")
	}

	columns, _ := rows.Columns()
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	result = make(map[string]interface{})
	for i, col := range columns {
		result[col] = values[i]
	}

	return result, nil
}

// GetFormDataList retrieves multiple form submissions from the dedicated table
func (ftm *FormTableManager) GetFormDataList(
	tableName string,
	businessVerticalID uuid.UUID,
	filters map[string]interface{},
) ([]map[string]interface{}, error) {
	// Build WHERE clause
	var whereClauses []string
	var values []interface{}
	i := 1

	whereClauses = append(whereClauses, fmt.Sprintf("business_vertical_id = $%d", i))
	values = append(values, businessVerticalID)
	i++

	whereClauses = append(whereClauses, "deleted_at IS NULL")

	for key, val := range filters {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", key, i))
		values = append(values, val)
		i++
	}

	sql := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s ORDER BY created_at DESC",
		tableName,
		strings.Join(whereClauses, " AND "),
	)

	rows, err := ftm.db.Raw(sql, values...).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to query form data: %v", err)
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		result := make(map[string]interface{})
		for i, col := range columns {
			result[col] = values[i]
		}
		results = append(results, result)
	}

	return results, nil
}

// SoftDeleteFormData soft deletes a record in the dedicated table
func (ftm *FormTableManager) SoftDeleteFormData(tableName string, recordID uuid.UUID, userID string) error {
	sql := fmt.Sprintf(
		"UPDATE %s SET deleted_at = $1, deleted_by = $2 WHERE id = $3 AND deleted_at IS NULL",
		tableName,
	)

	err := ftm.db.Exec(sql, time.Now(), userID, recordID).Error
	if err != nil {
		return fmt.Errorf("failed to delete form data: %v", err)
	}

	log.Printf("✅ Soft deleted record %s from table %s", recordID, tableName)
	return nil
}

// UpdateWorkflowState updates only the workflow state of a record
func (ftm *FormTableManager) UpdateWorkflowState(tableName string, recordID uuid.UUID, newState string, userID string) error {
	sql := fmt.Sprintf(
		"UPDATE %s SET current_state = $1, updated_by = $2, updated_at = $3 WHERE id = $4",
		tableName,
	)

	err := ftm.db.Exec(sql, newState, userID, time.Now(), recordID).Error
	if err != nil {
		return fmt.Errorf("failed to update workflow state: %v", err)
	}

	log.Printf("✅ Updated workflow state to %s for record %s in table %s", newState, recordID, tableName)
	return nil
}

// DropFormTable drops a form's dedicated table (use with caution!)
func (ftm *FormTableManager) DropFormTable(tableName string) error {
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", tableName)

	err := ftm.db.Exec(sql).Error
	if err != nil {
		return fmt.Errorf("failed to drop table: %v", err)
	}

	log.Printf("⚠️  Dropped table: %s", tableName)
	return nil
}

// TableExists checks if a table exists in the database
func (ftm *FormTableManager) TableExists(tableName string) (bool, error) {
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = $1
		)
	`

	err := ftm.db.Raw(sql, tableName).Scan(&exists).Error
	return exists, err
}
