package handlers

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"p9e.in/ugcl/config"
)

// SchemaManager handles PostgreSQL schema operations
type SchemaManager struct {
	db *gorm.DB
}

// NewSchemaManager creates a new schema manager
func NewSchemaManager() *SchemaManager {
	return &SchemaManager{
		db: config.DB,
	}
}

// GenerateSchemaName generates a valid PostgreSQL schema name from module code
// PostgreSQL schema names must start with a letter or underscore and contain only
// letters, digits, and underscores
func (sm *SchemaManager) GenerateSchemaName(moduleCode string) string {
	// Convert to lowercase
	name := strings.ToLower(moduleCode)

	// Replace spaces and hyphens with underscores
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")

	// Remove any characters that are not letters, digits, or underscores
	reg := regexp.MustCompile(`[^a-z0-9_]`)
	name = reg.ReplaceAllString(name, "")

	// Ensure it starts with a letter or underscore (prefix with underscore if starts with digit)
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}

	// Limit length (PostgreSQL identifier limit is 63 bytes)
	if len(name) > 63 {
		name = name[:63]
	}

	return name
}

// CreateSchema creates a new PostgreSQL schema
func (sm *SchemaManager) CreateSchema(schemaName string) error {
	// Validate schema name
	if !sm.isValidSchemaName(schemaName) {
		return fmt.Errorf("invalid schema name: %s", schemaName)
	}

	log.Printf("📁 Creating schema: %s", schemaName)

	// Create schema if it doesn't exist
	sql := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)
	if err := sm.db.Exec(sql).Error; err != nil {
		return fmt.Errorf("failed to create schema %s: %v", schemaName, err)
	}

	log.Printf("✅ Successfully created schema: %s", schemaName)
	return nil
}

// SchemaExists checks if a schema exists in the database
func (sm *SchemaManager) SchemaExists(schemaName string) (bool, error) {
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.schemata
			WHERE schema_name = $1
		)
	`
	err := sm.db.Raw(sql, schemaName).Scan(&exists).Error
	return exists, err
}

// DropSchema drops a schema and all its contents (use with caution!)
func (sm *SchemaManager) DropSchema(schemaName string, cascade bool) error {
	log.Printf("⚠️  Dropping schema: %s (cascade=%v)", schemaName, cascade)

	var sql string
	if cascade {
		sql = fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName)
	} else {
		sql = fmt.Sprintf("DROP SCHEMA IF EXISTS %s", schemaName)
	}

	if err := sm.db.Exec(sql).Error; err != nil {
		return fmt.Errorf("failed to drop schema %s: %v", schemaName, err)
	}

	log.Printf("✅ Successfully dropped schema: %s", schemaName)
	return nil
}

// ListTablesInSchema lists all tables in a given schema
func (sm *SchemaManager) ListTablesInSchema(schemaName string) ([]string, error) {
	var tables []string
	sql := `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = $1
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`
	rows, err := sm.db.Raw(sql, schemaName).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to list tables in schema %s: %v", schemaName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// GetFullTableName returns the fully qualified table name (schema.table)
func (sm *SchemaManager) GetFullTableName(schemaName, tableName string) string {
	if schemaName == "" || schemaName == "public" {
		return tableName
	}
	return fmt.Sprintf("%s.%s", schemaName, tableName)
}

// TableExistsInSchema checks if a table exists in a specific schema
func (sm *SchemaManager) TableExistsInSchema(schemaName, tableName string) (bool, error) {
	var exists bool
	sql := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = $1
			AND table_name = $2
		)
	`
	err := sm.db.Raw(sql, schemaName, tableName).Scan(&exists).Error
	return exists, err
}

// SetSearchPath sets the search path for the current session
// This allows queries to find tables in the specified schemas
func (sm *SchemaManager) SetSearchPath(schemas ...string) error {
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}

	// Always include public schema
	hasPublic := false
	for _, s := range schemas {
		if s == "public" {
			hasPublic = true
			break
		}
	}
	if !hasPublic {
		schemas = append(schemas, "public")
	}

	sql := fmt.Sprintf("SET search_path TO %s", strings.Join(schemas, ", "))
	return sm.db.Exec(sql).Error
}

// GrantSchemaUsage grants USAGE privilege on a schema to a role
func (sm *SchemaManager) GrantSchemaUsage(schemaName, roleName string) error {
	sql := fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", schemaName, roleName)
	return sm.db.Exec(sql).Error
}

// GrantAllOnSchema grants ALL privileges on all tables in a schema to a role
func (sm *SchemaManager) GrantAllOnSchema(schemaName, roleName string) error {
	sql := fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA %s TO %s", schemaName, roleName)
	return sm.db.Exec(sql).Error
}

// isValidSchemaName validates that a schema name is safe to use
func (sm *SchemaManager) isValidSchemaName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	// Must start with a letter or underscore
	if !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z') || name[0] == '_') {
		return false
	}

	// Must only contain letters, digits, and underscores
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	// Check for reserved names
	reserved := map[string]bool{
		"public":             true,
		"pg_catalog":         true,
		"information_schema": true,
		"pg_toast":           true,
		"pg_temp":            true,
	}
	if reserved[strings.ToLower(name)] {
		return false
	}

	return true
}

// GetSchemaStats returns statistics about a schema
func (sm *SchemaManager) GetSchemaStats(schemaName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get table count
	var tableCount int64
	sql := `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'`
	if err := sm.db.Raw(sql, schemaName).Scan(&tableCount).Error; err != nil {
		return nil, err
	}
	stats["table_count"] = tableCount

	// Get total row count across all tables
	tables, err := sm.ListTablesInSchema(schemaName)
	if err != nil {
		return nil, err
	}

	var totalRows int64
	for _, table := range tables {
		var count int64
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schemaName, table)
		if err := sm.db.Raw(countSQL).Scan(&count).Error; err != nil {
			continue
		}
		totalRows += count
	}
	stats["total_rows"] = totalRows
	stats["tables"] = tables

	return stats, nil
}
