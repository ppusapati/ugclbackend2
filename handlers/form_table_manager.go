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
	db            *gorm.DB
	schemaManager *SchemaManager
}

// NewFormTableManager creates a new form table manager
func NewFormTableManager() *FormTableManager {
	return &FormTableManager{
		db:            config.DB,
		schemaManager: NewSchemaManager(),
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
	BusinessVerticalID uuid.UUID  `gorm:"type:uuid;not null;index" json:"business_vertical_id"`
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
	return ftm.CreateFormTableWithSchema(form, nil)
}

// CreateFormTableInSchema creates a dedicated table for a form within a specific schema
func (ftm *FormTableManager) CreateFormTableInSchema(form *models.AppForm, schemaName string) error {
	return ftm.CreateFormTableInSchemaWithSchema(form, schemaName, nil)
}

// CreateFormTableInSchemaWithSchema creates a dedicated table with optional inferred schema in a specific database schema
func (ftm *FormTableManager) CreateFormTableInSchemaWithSchema(form *models.AppForm, schemaName string, inferredSchema map[string]interface{}) error {
	if form.DBTableName == "" {
		return fmt.Errorf("form %s has no table name defined", form.Code)
	}

	// Ensure schema exists
	if schemaName != "" && schemaName != "public" {
		exists, err := ftm.schemaManager.SchemaExists(schemaName)
		if err != nil {
			return fmt.Errorf("failed to check schema existence: %v", err)
		}
		if !exists {
			return fmt.Errorf("schema %s does not exist", schemaName)
		}
	}

	// Get full table name (schema.table)
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, form.DBTableName)

	log.Printf("📊 Creating dedicated table: %s for form: %s in schema: %s", fullTableName, form.Code, schemaName)

	// Parse form schema to get field definitions
	var formSchema map[string]interface{}

	// Priority: inferred schema > form_schema > steps
	if inferredSchema != nil {
		formSchema = inferredSchema
		log.Printf("🔍 Using inferred schema with %d fields", len(inferredSchema["fields"].([]map[string]interface{})))
	} else if len(form.FormSchema) > 0 && string(form.FormSchema) != "{}" {
		if err := json.Unmarshal(form.FormSchema, &formSchema); err != nil {
			return fmt.Errorf("failed to parse form schema: %v", err)
		}
	} else if len(form.Steps) > 0 && string(form.Steps) != "[]" {
		// Extract fields from steps structure
		fields, err := ftm.ExtractFieldsFromSteps(form.Steps)
		if err != nil {
			return fmt.Errorf("failed to extract fields from steps: %v", err)
		}
		if len(fields) > 0 {
			formSchema = map[string]interface{}{
				"fields": fields,
			}
			log.Printf("📋 Extracted %d fields from steps structure", len(fields))
		}
	}

	// Build CREATE TABLE SQL with schema-qualified table name
	sql := ftm.buildCreateTableSQLInSchema(schemaName, form.DBTableName, formSchema)

	// Execute table creation
	if err := ftm.db.Exec(sql).Error; err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	log.Printf("✅ Successfully created table: %s in schema: %s", form.DBTableName, schemaName)
	return nil
}

// buildCreateTableSQLInSchema generates the SQL for creating a form table within a specific schema
func (ftm *FormTableManager) buildCreateTableSQLInSchema(schemaName, tableName string, formSchema map[string]interface{}) string {
	var columns []string

	// Get full table name
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, tableName)

	// Base fields (always present)
	columns = append(columns,
		"id UUID PRIMARY KEY DEFAULT gen_random_uuid()",
		"created_by VARCHAR(255) NOT NULL",
		"created_at TIMESTAMP NOT NULL DEFAULT NOW()",
		"updated_by VARCHAR(255)",
		"updated_at TIMESTAMP DEFAULT NOW()",
		"deleted_by VARCHAR(255)",
		"deleted_at TIMESTAMP",
		"business_vertical_id UUID NOT NULL REFERENCES public.business_verticals(id)",
		"site_id UUID REFERENCES public.sites(id)",
		"workflow_id UUID REFERENCES public.workflow_definitions(id)",
		"current_state VARCHAR(50) NOT NULL DEFAULT 'draft'",
		"form_id UUID NOT NULL REFERENCES public.app_forms(id)",
		"form_code VARCHAR(50) NOT NULL",
	)

	// Parse form fields from schema
	if fields, ok := formSchema["fields"].([]interface{}); ok {
		log.Printf("📋 Processing %d fields from formSchema", len(fields))
		for idx, field := range fields {
			if fieldMap, ok := field.(map[string]interface{}); ok {
				log.Printf("🔍 Field %d: %+v", idx, fieldMap)
				columnDef := ftm.getColumnDefinition(fieldMap)
				if columnDef != "" {
					log.Printf("✅ Adding column: %s", columnDef)
					columns = append(columns, columnDef)
				} else {
					log.Printf("⚠️  Empty column definition for field %d", idx)
				}
			}
		}
	} else {
		log.Printf("⚠️  formSchema['fields'] is not []interface{}, checking for []map[string]interface{}")
		if fieldsRaw, exists := formSchema["fields"]; exists {
			log.Printf("🔍 fields type: %T, value: %+v", fieldsRaw, fieldsRaw)
			if fieldSlice, ok := fieldsRaw.([]map[string]interface{}); ok {
				log.Printf("📋 Processing %d fields as []map[string]interface{}", len(fieldSlice))
				for idx, fieldMap := range fieldSlice {
					log.Printf("🔍 Field %d: %+v", idx, fieldMap)
					columnDef := ftm.getColumnDefinition(fieldMap)
					if columnDef != "" {
						log.Printf("✅ Adding column: %s", columnDef)
						columns = append(columns, columnDef)
					} else {
						log.Printf("⚠️  Empty column definition for field %d", idx)
					}
				}
			}
		} else {
			log.Printf("⚠️  No 'fields' key in formSchema at all")
		}
	}

	log.Printf("📊 Total columns: %d (base: 13, custom: %d)", len(columns), len(columns)-13)

	// Create table with schema-qualified name
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n);", fullTableName, strings.Join(columns, ",\n  "))

	// Add indexes with schema-qualified names
	indexPrefix := strings.ReplaceAll(fullTableName, ".", "_")
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_business_vertical ON %s(business_vertical_id);", indexPrefix, fullTableName)
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_site ON %s(site_id);", indexPrefix, fullTableName)
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_state ON %s(current_state);", indexPrefix, fullTableName)
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_form ON %s(form_id);", indexPrefix, fullTableName)
	sql += fmt.Sprintf("\nCREATE INDEX IF NOT EXISTS idx_%s_deleted ON %s(deleted_at);", indexPrefix, fullTableName)

	return sql
}

// ExtractFieldsFromSteps extracts field definitions from the steps structure
func (ftm *FormTableManager) ExtractFieldsFromSteps(steps json.RawMessage) ([]map[string]interface{}, error) {
	log.Printf("🔍 ExtractFieldsFromSteps - Raw steps JSON: %s", string(steps))

	var stepsData []map[string]interface{}
	if err := json.Unmarshal(steps, &stepsData); err != nil {
		log.Printf("❌ Failed to unmarshal steps: %v", err)
		return nil, err
	}

	log.Printf("📊 Parsed %d steps", len(stepsData))

	allFields := make([]map[string]interface{}, 0)
	for stepIdx, step := range stepsData {
		log.Printf("🔍 Step %d: %v", stepIdx, step)
		if fields, ok := step["fields"].([]interface{}); ok {
			log.Printf("📋 Found %d fields in step %d", len(fields), stepIdx)
			for fieldIdx, field := range fields {
				if fieldMap, ok := field.(map[string]interface{}); ok {
					log.Printf("🔍 Field %d in step %d: %v", fieldIdx, stepIdx, fieldMap)
					// Normalize field structure
					normalizedField := map[string]interface{}{
						"name": fieldMap["id"], // Use id as name
						"type": fieldMap["type"],
					}
					if label, ok := fieldMap["label"]; ok {
						normalizedField["label"] = label
					}
					if required, ok := fieldMap["required"]; ok {
						normalizedField["required"] = required
					}
					log.Printf("✅ Normalized field: %v", normalizedField)
					allFields = append(allFields, normalizedField)
				}
			}
		} else {
			log.Printf("⚠️  No fields found in step %d", stepIdx)
		}
	}

	log.Printf("✅ Total extracted fields: %d", len(allFields))
	return allFields, nil
}

// CreateFormTableWithSchema creates a dedicated table for a form with optional inferred schema
func (ftm *FormTableManager) CreateFormTableWithSchema(form *models.AppForm, inferredSchema map[string]interface{}) error {
	if form.DBTableName == "" {
		return fmt.Errorf("form %s has no table name defined", form.Code)
	}

	log.Printf("📊 Creating dedicated table: %s for form: %s", form.DBTableName, form.Code)

	// Parse form schema to get field definitions
	var formSchema map[string]interface{}

	// Priority: inferred schema > form_schema > steps
	if inferredSchema != nil {
		formSchema = inferredSchema
		log.Printf("🔍 Using inferred schema with %d fields", len(inferredSchema["fields"].([]map[string]interface{})))
	} else if len(form.FormSchema) > 0 && string(form.FormSchema) != "{}" {
		if err := json.Unmarshal(form.FormSchema, &formSchema); err != nil {
			return fmt.Errorf("failed to parse form schema: %v", err)
		}
	} else if len(form.Steps) > 0 && string(form.Steps) != "[]" {
		// Extract fields from steps structure
		fields, err := ftm.ExtractFieldsFromSteps(form.Steps)
		if err != nil {
			return fmt.Errorf("failed to extract fields from steps: %v", err)
		}
		if len(fields) > 0 {
			formSchema = map[string]interface{}{
				"fields": fields,
			}
			log.Printf("📋 Extracted %d fields from steps structure", len(fields))
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
		log.Printf("📋 Processing %d fields from formSchema", len(fields))
		for idx, field := range fields {
			if fieldMap, ok := field.(map[string]interface{}); ok {
				log.Printf("🔍 Field %d: %+v", idx, fieldMap)
				columnDef := ftm.getColumnDefinition(fieldMap)
				if columnDef != "" {
					log.Printf("✅ Adding column: %s", columnDef)
					columns = append(columns, columnDef)
				} else {
					log.Printf("⚠️  Empty column definition for field %d", idx)
				}
			}
		}
	} else {
		log.Printf("⚠️  formSchema['fields'] is not []interface{}, checking for []map[string]interface{}")
		// Try to handle []map[string]interface{} which might come from ExtractFieldsFromSteps
		if fieldsRaw, exists := formSchema["fields"]; exists {
			log.Printf("🔍 fields type: %T, value: %+v", fieldsRaw, fieldsRaw)
			if fieldSlice, ok := fieldsRaw.([]map[string]interface{}); ok {
				log.Printf("📋 Processing %d fields as []map[string]interface{}", len(fieldSlice))
				for idx, fieldMap := range fieldSlice {
					log.Printf("🔍 Field %d: %+v", idx, fieldMap)
					columnDef := ftm.getColumnDefinition(fieldMap)
					if columnDef != "" {
						log.Printf("✅ Adding column: %s", columnDef)
						columns = append(columns, columnDef)
					} else {
						log.Printf("⚠️  Empty column definition for field %d", idx)
					}
				}
			}
		} else {
			log.Printf("⚠️  No 'fields' key in formSchema at all")
		}
	}

	log.Printf("📊 Total columns: %d (base: 13, custom: %d)", len(columns), len(columns)-13)

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
	return ftm.InsertFormDataInSchema("", tableName, formID, formCode, businessVerticalID, siteID, workflowID, initialState, formData, userID)
}

// InsertFormDataInSchema inserts form submission data into the dedicated table within a specific schema
func (ftm *FormTableManager) InsertFormDataInSchema(
	schemaName string,
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
	// Get full table name
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, tableName)

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
		fullTableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	var returnedID uuid.UUID
	err := ftm.db.Raw(sql, values...).Row().Scan(&returnedID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert form data: %v", err)
	}

	log.Printf("✅ Inserted record %s into table %s", returnedID, fullTableName)
	return returnedID, nil
}

// UpdateFormData updates form submission data in the dedicated table
func (ftm *FormTableManager) UpdateFormData(
	tableName string,
	recordID uuid.UUID,
	formData map[string]interface{},
	userID string,
) error {
	return ftm.UpdateFormDataInSchema("", tableName, recordID, formData, userID)
}

// UpdateFormDataInSchema updates form submission data in the dedicated table within a specific schema
func (ftm *FormTableManager) UpdateFormDataInSchema(
	schemaName string,
	tableName string,
	recordID uuid.UUID,
	formData map[string]interface{},
	userID string,
) error {
	// Get full table name
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, tableName)

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
		fullTableName,
		strings.Join(setClauses, ", "),
		whereClause,
	)

	err := ftm.db.Exec(sql, values...).Error
	if err != nil {
		return fmt.Errorf("failed to update form data: %v", err)
	}

	log.Printf("✅ Updated record %s in table %s", recordID, fullTableName)
	return nil
}

// GetFormData retrieves form submission data from the dedicated table
func (ftm *FormTableManager) GetFormData(tableName string, recordID uuid.UUID) (map[string]interface{}, error) {
	return ftm.GetFormDataInSchema("", tableName, recordID)
}

// GetFormDataInSchema retrieves form submission data from the dedicated table within a specific schema
func (ftm *FormTableManager) GetFormDataInSchema(schemaName string, tableName string, recordID uuid.UUID) (map[string]interface{}, error) {
	// Get full table name
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, tableName)

	sql := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 AND deleted_at IS NULL", fullTableName)

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
	return ftm.GetFormDataListInSchema("", tableName, businessVerticalID, filters)
}

// GetFormDataListInSchema retrieves multiple form submissions from the dedicated table within a specific schema
func (ftm *FormTableManager) GetFormDataListInSchema(
	schemaName string,
	tableName string,
	businessVerticalID uuid.UUID,
	filters map[string]interface{},
) ([]map[string]interface{}, error) {
	// Get full table name
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, tableName)

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
		fullTableName,
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
	return ftm.SoftDeleteFormDataInSchema("", tableName, recordID, userID)
}

// SoftDeleteFormDataInSchema soft deletes a record in the dedicated table within a specific schema
func (ftm *FormTableManager) SoftDeleteFormDataInSchema(schemaName string, tableName string, recordID uuid.UUID, userID string) error {
	// Get full table name
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, tableName)

	sql := fmt.Sprintf(
		"UPDATE %s SET deleted_at = $1, deleted_by = $2 WHERE id = $3 AND deleted_at IS NULL",
		fullTableName,
	)

	err := ftm.db.Exec(sql, time.Now(), userID, recordID).Error
	if err != nil {
		return fmt.Errorf("failed to delete form data: %v", err)
	}

	log.Printf("✅ Soft deleted record %s from table %s", recordID, fullTableName)
	return nil
}

// UpdateWorkflowState updates only the workflow state of a record
func (ftm *FormTableManager) UpdateWorkflowState(tableName string, recordID uuid.UUID, newState string, userID string) error {
	return ftm.UpdateWorkflowStateInSchema("", tableName, recordID, newState, userID)
}

// UpdateWorkflowStateInSchema updates only the workflow state of a record within a specific schema
func (ftm *FormTableManager) UpdateWorkflowStateInSchema(schemaName string, tableName string, recordID uuid.UUID, newState string, userID string) error {
	// Get full table name
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, tableName)

	sql := fmt.Sprintf(
		"UPDATE %s SET current_state = $1, updated_by = $2, updated_at = $3 WHERE id = $4",
		fullTableName,
	)

	err := ftm.db.Exec(sql, newState, userID, time.Now(), recordID).Error
	if err != nil {
		return fmt.Errorf("failed to update workflow state: %v", err)
	}

	log.Printf("✅ Updated workflow state to %s for record %s in table %s", newState, recordID, fullTableName)
	return nil
}

// DropFormTable drops a form's dedicated table (use with caution!)
func (ftm *FormTableManager) DropFormTable(tableName string) error {
	return ftm.DropFormTableInSchema("", tableName)
}

// DropFormTableInSchema drops a form's dedicated table within a specific schema (use with caution!)
func (ftm *FormTableManager) DropFormTableInSchema(schemaName string, tableName string) error {
	// Get full table name
	fullTableName := ftm.schemaManager.GetFullTableName(schemaName, tableName)

	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", fullTableName)

	err := ftm.db.Exec(sql).Error
	if err != nil {
		return fmt.Errorf("failed to drop table: %v", err)
	}

	log.Printf("⚠️  Dropped table: %s", fullTableName)
	return nil
}

// TableExists checks if a table exists in the database
func (ftm *FormTableManager) TableExists(tableName string) (bool, error) {
	return ftm.TableExistsInSchema("public", tableName)
}

// TableExistsInSchema checks if a table exists in a specific schema
func (ftm *FormTableManager) TableExistsInSchema(schemaName string, tableName string) (bool, error) {
	if schemaName == "" {
		schemaName = "public"
	}
	return ftm.schemaManager.TableExistsInSchema(schemaName, tableName)
}

// InferSchemaFromData infers form schema from the submitted data
// This allows dynamic table creation based on actual data structure
func (ftm *FormTableManager) InferSchemaFromData(formData map[string]interface{}) map[string]interface{} {
	fields := make([]map[string]interface{}, 0)

	for fieldName, value := range formData {
		// Skip base fields that are always present
		baseFields := map[string]bool{
			"id": true, "created_by": true, "created_at": true,
			"updated_by": true, "updated_at": true, "deleted_by": true, "deleted_at": true,
			"business_vertical_id": true, "site_id": true,
			"workflow_id": true, "current_state": true,
			"form_id": true, "form_code": true,
		}
		if baseFields[fieldName] {
			continue
		}

		field := map[string]interface{}{
			"name":     fieldName,
			"label":    fieldName, // Default label same as name
			"required": false,     // Default to optional
		}

		// Infer type from value
		switch v := value.(type) {
		case bool:
			field["type"] = "checkbox"
		case int, int8, int16, int32, int64:
			field["type"] = "integer"
		case float32, float64:
			field["type"] = "number"
		case string:
			// Try to detect specific string types
			if len(v) > 0 {
				// Check if it looks like a date (YYYY-MM-DD or ISO8601)
				if len(v) == 10 && v[4] == '-' && v[7] == '-' {
					field["type"] = "date"
				} else if strings.Contains(v, "T") && strings.Contains(v, ":") {
					field["type"] = "datetime"
				} else if len(v) > 500 {
					// Long text -> textarea
					field["type"] = "textarea"
				} else if strings.Contains(v, "@") && strings.Contains(v, ".") {
					// Might be email
					field["type"] = "email"
				} else {
					// Default to text
					field["type"] = "text"
				}
			} else {
				field["type"] = "text"
			}
		case map[string]interface{}, []interface{}:
			// Nested objects/arrays -> store as JSON
			field["type"] = "json"
		default:
			// Unknown type -> text
			field["type"] = "text"
		}

		fields = append(fields, field)
	}

	log.Printf("🔍 Inferred schema with %d fields from form data", len(fields))
	return map[string]interface{}{
		"fields": fields,
	}
}
