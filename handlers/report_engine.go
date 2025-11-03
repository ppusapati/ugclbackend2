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

// ReportEngine handles dynamic report generation from form submission tables
type ReportEngine struct {
	db *gorm.DB
}

// NewReportEngine creates a new report engine
func NewReportEngine() *ReportEngine {
	return &ReportEngine{
		db: config.DB,
	}
}

// ReportResult represents the result of a report execution
type ReportResult struct {
	Headers []ReportHeader         `json:"headers"`
	Data    []map[string]interface{} `json:"data"`
	Summary map[string]interface{}   `json:"summary,omitempty"`
	MetaData ReportMetaData         `json:"metadata"`
}

// ReportHeader represents column information
type ReportHeader struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	DataType  string `json:"data_type"`
	Format    string `json:"format,omitempty"`
	Sortable  bool   `json:"sortable"`
	Filterable bool  `json:"filterable"`
}

// ReportMetaData contains execution metadata
type ReportMetaData struct {
	TotalRows     int       `json:"total_rows"`
	ExecutionTime int64     `json:"execution_time_ms"`
	GeneratedAt   time.Time `json:"generated_at"`
	CacheKey      string    `json:"cache_key,omitempty"`
}

// ExecuteReport generates a report based on the definition
func (re *ReportEngine) ExecuteReport(
	reportDef *models.ReportDefinition,
	runtimeFilters []models.ReportFilter,
	userID string,
) (*ReportResult, error) {
	startTime := time.Now()

	// Log execution
	execution := &models.ReportExecution{
		ID:            uuid.New(),
		ReportID:      reportDef.ID,
		ExecutionType: "manual",
		ExecutedBy:    userID,
		StartedAt:     startTime,
		Status:        "running",
	}

	// Parse report configuration
	var dataSources []models.DataSource
	if err := json.Unmarshal(reportDef.DataSources, &dataSources); err != nil {
		return nil, fmt.Errorf("invalid data sources: %v", err)
	}

	var fields []models.ReportField
	if err := json.Unmarshal(reportDef.Fields, &fields); err != nil {
		return nil, fmt.Errorf("invalid fields: %v", err)
	}

	var filters []models.ReportFilter
	if len(reportDef.Filters) > 0 {
		json.Unmarshal(reportDef.Filters, &filters)
	}

	// Merge runtime filters
	filters = append(filters, runtimeFilters...)

	var groupings []models.ReportGrouping
	if len(reportDef.Groupings) > 0 {
		json.Unmarshal(reportDef.Groupings, &groupings)
	}

	var aggregations []models.ReportAggregation
	if len(reportDef.Aggregations) > 0 {
		json.Unmarshal(reportDef.Aggregations, &aggregations)
	}

	var sortings []models.ReportSorting
	if len(reportDef.Sorting) > 0 {
		json.Unmarshal(reportDef.Sorting, &sortings)
	}

	// Build SQL query
	query, args, err := re.buildQuery(dataSources, fields, filters, groupings, aggregations, sortings)
	if err != nil {
		execution.Status = "failed"
		execution.ErrorMessage = err.Error()
		re.saveExecution(execution)
		return nil, err
	}

	log.Printf("🔍 Executing Report Query:\n%s\nArgs: %v", query, args)

	// Execute query
	rows, err := re.db.Raw(query, args...).Rows()
	if err != nil {
		execution.Status = "failed"
		execution.ErrorMessage = err.Error()
		re.saveExecution(execution)
		return nil, fmt.Errorf("query execution failed: %v", err)
	}
	defer rows.Close()

	// Build result
	result := &ReportResult{
		Headers:  re.buildHeaders(fields, aggregations),
		Data:     []map[string]interface{}{},
		Summary:  make(map[string]interface{}),
		MetaData: ReportMetaData{
			GeneratedAt: time.Now(),
		},
	}

	// Process rows
	columns, _ := rows.Columns()
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = re.formatValue(values[i], re.getFieldDataType(col, fields, aggregations))
		}
		result.Data = append(result.Data, row)
	}

	// Calculate summary if aggregations exist
	if len(aggregations) > 0 {
		result.Summary = re.calculateSummary(result.Data, aggregations)
	}

	// Update metadata
	result.MetaData.TotalRows = len(result.Data)
	result.MetaData.ExecutionTime = time.Since(startTime).Milliseconds()

	// Save successful execution
	execution.Status = "completed"
	execution.CompletedAt = &result.MetaData.GeneratedAt
	execution.Duration = int(result.MetaData.ExecutionTime)
	execution.RowCount = result.MetaData.TotalRows
	re.saveExecution(execution)

	log.Printf("✅ Report executed successfully: %d rows in %dms", result.MetaData.TotalRows, result.MetaData.ExecutionTime)

	return result, nil
}

// buildQuery constructs the SQL query from report configuration
func (re *ReportEngine) buildQuery(
	dataSources []models.DataSource,
	fields []models.ReportField,
	filters []models.ReportFilter,
	groupings []models.ReportGrouping,
	aggregations []models.ReportAggregation,
	sortings []models.ReportSorting,
) (string, []interface{}, error) {
	var query strings.Builder
	var args []interface{}
	argIndex := 1

	// SELECT clause
	query.WriteString("SELECT ")
	selectClauses := []string{}

	// Add regular fields
	for _, field := range fields {
		if !field.IsVisible {
			continue
		}

		alias := field.Alias
		if alias == "" {
			alias = field.FieldName
		}

		if field.Aggregation != "" {
			selectClauses = append(selectClauses, fmt.Sprintf(
				"%s(%s.%s) AS %s",
				strings.ToUpper(field.Aggregation),
				field.DataSource,
				field.FieldName,
				alias,
			))
		} else {
			selectClauses = append(selectClauses, fmt.Sprintf(
				"%s.%s AS %s",
				field.DataSource,
				field.FieldName,
				alias,
			))
		}
	}

	// Add aggregations
	for _, agg := range aggregations {
		alias := agg.Alias
		if alias == "" {
			alias = fmt.Sprintf("%s_%s", strings.ToLower(agg.Function), agg.FieldName)
		}

		if agg.Function == "COUNT_DISTINCT" {
			selectClauses = append(selectClauses, fmt.Sprintf(
				"COUNT(DISTINCT %s.%s) AS %s",
				agg.DataSource,
				agg.FieldName,
				alias,
			))
		} else {
			selectClauses = append(selectClauses, fmt.Sprintf(
				"%s(%s.%s) AS %s",
				agg.Function,
				agg.DataSource,
				agg.FieldName,
				alias,
			))
		}
	}

	if len(selectClauses) == 0 {
		return "", nil, fmt.Errorf("no fields selected")
	}

	query.WriteString(strings.Join(selectClauses, ", "))

	// FROM clause
	if len(dataSources) == 0 {
		return "", nil, fmt.Errorf("no data sources specified")
	}

	query.WriteString(fmt.Sprintf("\nFROM %s AS %s", dataSources[0].TableName, dataSources[0].Alias))

	// JOIN clauses (for multi-table reports)
	for i := 1; i < len(dataSources); i++ {
		ds := dataSources[i]
		joinType := "LEFT JOIN"
		if ds.JoinType != "" {
			joinType = strings.ToUpper(ds.JoinType) + " JOIN"
		}

		query.WriteString(fmt.Sprintf(
			"\n%s %s AS %s ON %s",
			joinType,
			ds.TableName,
			ds.Alias,
			ds.JoinOn,
		))
	}

	// WHERE clause
	whereClauses := []string{}

	// Always exclude soft-deleted records
	for _, ds := range dataSources {
		whereClauses = append(whereClauses, fmt.Sprintf("%s.deleted_at IS NULL", ds.Alias))
	}

	// Add filters
	for _, filter := range filters {
		clause, filterArgs := re.buildFilterClause(filter, &argIndex)
		if clause != "" {
			whereClauses = append(whereClauses, clause)
			args = append(args, filterArgs...)
		}
	}

	if len(whereClauses) > 0 {
		query.WriteString("\nWHERE " + strings.Join(whereClauses, " AND "))
	}

	// GROUP BY clause
	if len(groupings) > 0 {
		groupClauses := []string{}
		for _, group := range groupings {
			groupClauses = append(groupClauses, fmt.Sprintf("%s.%s", group.DataSource, group.FieldName))
		}
		query.WriteString("\nGROUP BY " + strings.Join(groupClauses, ", "))
	}

	// ORDER BY clause
	if len(sortings) > 0 {
		orderClauses := []string{}
		for _, sort := range sortings {
			direction := "ASC"
			if strings.ToUpper(sort.Direction) == "DESC" {
				direction = "DESC"
			}

			if sort.DataSource != "" {
				orderClauses = append(orderClauses, fmt.Sprintf("%s.%s %s", sort.DataSource, sort.FieldName, direction))
			} else {
				orderClauses = append(orderClauses, fmt.Sprintf("%s %s", sort.FieldName, direction))
			}
		}
		query.WriteString("\nORDER BY " + strings.Join(orderClauses, ", "))
	}

	return query.String(), args, nil
}

// buildFilterClause constructs a WHERE clause for a filter
func (re *ReportEngine) buildFilterClause(filter models.ReportFilter, argIndex *int) (string, []interface{}) {
	var args []interface{}
	fieldRef := fmt.Sprintf("%s.%s", filter.DataSource, filter.FieldName)

	switch strings.ToLower(filter.Operator) {
	case "eq", "=":
		clause := fmt.Sprintf("%s = $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "ne", "!=", "<>":
		clause := fmt.Sprintf("%s != $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "gt", ">":
		clause := fmt.Sprintf("%s > $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "gte", ">=":
		clause := fmt.Sprintf("%s >= $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "lt", "<":
		clause := fmt.Sprintf("%s < $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "lte", "<=":
		clause := fmt.Sprintf("%s <= $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "like", "contains":
		clause := fmt.Sprintf("%s ILIKE $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{fmt.Sprintf("%%%v%%", filter.Value)}

	case "starts_with":
		clause := fmt.Sprintf("%s ILIKE $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{fmt.Sprintf("%v%%", filter.Value)}

	case "ends_with":
		clause := fmt.Sprintf("%s ILIKE $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{fmt.Sprintf("%%%v", filter.Value)}

	case "in":
		if values, ok := filter.Value.([]interface{}); ok && len(values) > 0 {
			placeholders := []string{}
			for _, val := range values {
				placeholders = append(placeholders, fmt.Sprintf("$%d", *argIndex))
				args = append(args, val)
				*argIndex++
			}
			clause := fmt.Sprintf("%s IN (%s)", fieldRef, strings.Join(placeholders, ", "))
			return clause, args
		}

	case "between":
		if values, ok := filter.Value.([]interface{}); ok && len(values) == 2 {
			clause := fmt.Sprintf("%s BETWEEN $%d AND $%d", fieldRef, *argIndex, *argIndex+1)
			*argIndex += 2
			return clause, []interface{}{values[0], values[1]}
		}

	case "is_null":
		return fmt.Sprintf("%s IS NULL", fieldRef), nil

	case "is_not_null":
		return fmt.Sprintf("%s IS NOT NULL", fieldRef), nil

	case "date_equals":
		clause := fmt.Sprintf("DATE(%s) = $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "date_before":
		clause := fmt.Sprintf("DATE(%s) < $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "date_after":
		clause := fmt.Sprintf("DATE(%s) > $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}

	case "this_week":
		return fmt.Sprintf("%s >= date_trunc('week', CURRENT_DATE) AND %s < date_trunc('week', CURRENT_DATE) + interval '1 week'", fieldRef, fieldRef), nil

	case "this_month":
		return fmt.Sprintf("%s >= date_trunc('month', CURRENT_DATE) AND %s < date_trunc('month', CURRENT_DATE) + interval '1 month'", fieldRef, fieldRef), nil

	case "this_year":
		return fmt.Sprintf("%s >= date_trunc('year', CURRENT_DATE) AND %s < date_trunc('year', CURRENT_DATE) + interval '1 year'", fieldRef, fieldRef), nil

	case "last_n_days":
		if days, ok := filter.Value.(float64); ok {
			return fmt.Sprintf("%s >= CURRENT_DATE - interval '%d days'", fieldRef, int(days)), nil
		}
	}

	return "", nil
}

// buildHeaders creates column headers for the result
func (re *ReportEngine) buildHeaders(fields []models.ReportField, aggregations []models.ReportAggregation) []ReportHeader {
	headers := []ReportHeader{}

	for _, field := range fields {
		if !field.IsVisible {
			continue
		}

		label := field.Alias
		if label == "" {
			label = field.FieldName
		}

		headers = append(headers, ReportHeader{
			Key:        label,
			Label:      label,
			DataType:   field.DataType,
			Format:     field.Format,
			Sortable:   true,
			Filterable: true,
		})
	}

	for _, agg := range aggregations {
		label := agg.Alias
		if label == "" {
			label = fmt.Sprintf("%s_%s", strings.ToLower(agg.Function), agg.FieldName)
		}

		headers = append(headers, ReportHeader{
			Key:        label,
			Label:      label,
			DataType:   "number",
			Format:     agg.Format,
			Sortable:   true,
			Filterable: false,
		})
	}

	return headers
}

// getFieldDataType gets the data type for a field
func (re *ReportEngine) getFieldDataType(colName string, fields []models.ReportField, aggregations []models.ReportAggregation) string {
	for _, f := range fields {
		if f.Alias == colName || f.FieldName == colName {
			return f.DataType
		}
	}

	for _, agg := range aggregations {
		if agg.Alias == colName {
			return "number"
		}
	}

	return "text"
}

// formatValue formats a value based on its data type
func (re *ReportEngine) formatValue(value interface{}, dataType string) interface{} {
	if value == nil {
		return nil
	}

	// Handle byte arrays (from database)
	if b, ok := value.([]byte); ok {
		return string(b)
	}

	return value
}

// calculateSummary calculates summary statistics
func (re *ReportEngine) calculateSummary(data []map[string]interface{}, aggregations []models.ReportAggregation) map[string]interface{} {
	summary := make(map[string]interface{})
	summary["total_records"] = len(data)

	// Add aggregation results (already calculated in SQL)
	if len(data) > 0 {
		for _, agg := range aggregations {
			key := agg.Alias
			if key == "" {
				key = fmt.Sprintf("%s_%s", strings.ToLower(agg.Function), agg.FieldName)
			}
			if val, exists := data[0][key]; exists {
				summary[key] = val
			}
		}
	}

	return summary
}

// saveExecution saves report execution history
func (re *ReportEngine) saveExecution(execution *models.ReportExecution) {
	if err := re.db.Create(execution).Error; err != nil {
		log.Printf("⚠️  Failed to save report execution: %v", err)
	}
}

// GetFormTableSchema retrieves the schema of a form table for report builder UI
func (re *ReportEngine) GetFormTableSchema(tableName string) ([]map[string]interface{}, error) {
	query := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := re.db.Raw(query, tableName).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schema []map[string]interface{}
	for rows.Next() {
		var colName, dataType, isNullable string
		var colDefault *string
		var maxLength *int

		rows.Scan(&colName, &dataType, &isNullable, &colDefault, &maxLength)

		schema = append(schema, map[string]interface{}{
			"name":       colName,
			"type":       re.mapSQLTypeToReportType(dataType),
			"nullable":   isNullable == "YES",
			"max_length": maxLength,
		})
	}

	return schema, nil
}

// mapSQLTypeToReportType maps SQL types to report field types
func (re *ReportEngine) mapSQLTypeToReportType(sqlType string) string {
	sqlType = strings.ToLower(sqlType)

	switch {
	case strings.Contains(sqlType, "char"), strings.Contains(sqlType, "text"):
		return "text"
	case strings.Contains(sqlType, "int"), strings.Contains(sqlType, "serial"):
		return "number"
	case strings.Contains(sqlType, "numeric"), strings.Contains(sqlType, "decimal"), strings.Contains(sqlType, "float"), strings.Contains(sqlType, "double"):
		return "decimal"
	case strings.Contains(sqlType, "bool"):
		return "boolean"
	case strings.Contains(sqlType, "date") && !strings.Contains(sqlType, "time"):
		return "date"
	case strings.Contains(sqlType, "timestamp"), strings.Contains(sqlType, "datetime"):
		return "datetime"
	case strings.Contains(sqlType, "time"):
		return "time"
	case strings.Contains(sqlType, "json"):
		return "json"
	case strings.Contains(sqlType, "uuid"):
		return "uuid"
	default:
		return "text"
	}
}
