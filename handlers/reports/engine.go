package reports

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

var sqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const reportEngineMetadataCacheTTL = 15 * time.Minute

type cachedResolvedTable struct {
	qualified string
	expiresAt time.Time
}

type cachedFormSchema struct {
	schema    []map[string]interface{}
	expiresAt time.Time
}

var reportTableResolutionLoadGroup singleflight.Group
var reportFormSchemaLoadGroup singleflight.Group

var reportTableResolutionCache = struct {
	mu      sync.RWMutex
	entries map[string]cachedResolvedTable
}{entries: make(map[string]cachedResolvedTable)}

var reportFormSchemaCache = struct {
	mu      sync.RWMutex
	entries map[string]cachedFormSchema
}{entries: make(map[string]cachedFormSchema)}

func getCachedResolvedTable(key string) (string, bool) {
	reportTableResolutionCache.mu.RLock()
	entry, ok := reportTableResolutionCache.entries[key]
	reportTableResolutionCache.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt) {
		reportTableResolutionCache.mu.Lock()
		if current, still := reportTableResolutionCache.entries[key]; still && time.Now().After(current.expiresAt) {
			delete(reportTableResolutionCache.entries, key)
		}
		reportTableResolutionCache.mu.Unlock()
		return "", false
	}
	return entry.qualified, true
}

func setCachedResolvedTable(key, qualified string) {
	reportTableResolutionCache.mu.Lock()
	reportTableResolutionCache.entries[key] = cachedResolvedTable{qualified: qualified, expiresAt: time.Now().Add(reportEngineMetadataCacheTTL)}
	reportTableResolutionCache.mu.Unlock()
}

func getCachedFormSchema(key string) ([]map[string]interface{}, bool) {
	reportFormSchemaCache.mu.RLock()
	entry, ok := reportFormSchemaCache.entries[key]
	reportFormSchemaCache.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		reportFormSchemaCache.mu.Lock()
		if current, still := reportFormSchemaCache.entries[key]; still && time.Now().After(current.expiresAt) {
			delete(reportFormSchemaCache.entries, key)
		}
		reportFormSchemaCache.mu.Unlock()
		return nil, false
	}

	cloned := make([]map[string]interface{}, len(entry.schema))
	for i := range entry.schema {
		row := make(map[string]interface{}, len(entry.schema[i]))
		for k, v := range entry.schema[i] {
			row[k] = v
		}
		cloned[i] = row
	}
	return cloned, true
}

func setCachedFormSchema(key string, schema []map[string]interface{}) {
	cloned := make([]map[string]interface{}, len(schema))
	for i := range schema {
		row := make(map[string]interface{}, len(schema[i]))
		for k, v := range schema[i] {
			row[k] = v
		}
		cloned[i] = row
	}

	reportFormSchemaCache.mu.Lock()
	reportFormSchemaCache.entries[key] = cachedFormSchema{schema: cloned, expiresAt: time.Now().Add(reportEngineMetadataCacheTTL)}
	reportFormSchemaCache.mu.Unlock()
}

// viewColumnsCache caches the set of actual columns present in each resolved table/view.
// Used by resolveColumnRef to rewrite references to columns the view does not project.
var viewColumnsCache = struct {
	mu      sync.Mutex
	entries map[string]struct {
		cols      map[string]bool
		expiresAt time.Time
	}
}{entries: make(map[string]struct {
	cols      map[string]bool
	expiresAt time.Time
})}

// getViewColumns returns the set of column names for the given qualified table/view name.
// Results are cached for reportEngineMetadataCacheTTL.
func (re *ReportEngine) getViewColumns(qualifiedTable string) map[string]bool {
	key := strings.ToLower(qualifiedTable)

	viewColumnsCache.mu.Lock()
	if entry, ok := viewColumnsCache.entries[key]; ok && time.Now().Before(entry.expiresAt) {
		viewColumnsCache.mu.Unlock()
		return entry.cols
	}
	viewColumnsCache.mu.Unlock()

	// Strip quotes for information_schema lookup. Tenant tables also exist in
	// public, so match the schema too when the name is qualified.
	parts := strings.SplitN(qualifiedTable, ".", 2)
	tablePart := strings.Trim(parts[len(parts)-1], `"`)
	columnsQuery := `SELECT column_name FROM information_schema.columns WHERE lower(table_name) = lower(?) AND table_schema NOT IN ('pg_catalog','information_schema')`
	columnsArgs := []interface{}{tablePart}
	if len(parts) == 2 {
		columnsQuery += ` AND table_schema = ?`
		columnsArgs = append(columnsArgs, strings.Trim(parts[0], `"`))
	}

	var colRows []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	re.db.Raw(columnsQuery, columnsArgs...).Scan(&colRows)

	cols := make(map[string]bool, len(colRows))
	for _, r := range colRows {
		cols[strings.ToLower(r.ColumnName)] = true
	}

	viewColumnsCache.mu.Lock()
	viewColumnsCache.entries[key] = struct {
		cols      map[string]bool
		expiresAt time.Time
	}{cols: cols, expiresAt: time.Now().Add(reportEngineMetadataCacheTTL)}
	viewColumnsCache.mu.Unlock()

	return cols
}

// invalidateViewColumns drops the cached column set for a qualified table/view,
// e.g. after a form report view is recreated with a different column list.
func invalidateViewColumns(qualifiedTable string) {
	viewColumnsCache.mu.Lock()
	delete(viewColumnsCache.entries, strings.ToLower(qualifiedTable))
	viewColumnsCache.mu.Unlock()
}

// ReportEngine handles dynamic report generation from form submission tables
type ReportEngine struct {
	db *gorm.DB
}

// NewReportEngine creates a new report engine bound to a tenant-scoped db
func NewReportEngine(db *gorm.DB) *ReportEngine {
	return &ReportEngine{
		db: db,
	}
}

// currentSchema returns the first schema on the connection's search_path,
// i.e. the tenant schema for a tenant-scoped session. Report data sources
// resolve only within it: the per-tenant tables (attendance_sessions,
// attendance_events, ...) also exist, empty, in public, and other tenants'
// schemas are on the same server, so falling back anywhere else either
// silently returns zero rows or reads another tenant's data.
func (re *ReportEngine) currentSchema() string {
	schemaName, err := currentSchemaName(re.db)
	if err != nil {
		return ""
	}
	return schemaName
}

// ReportResult represents the result of a report execution
type ReportResult struct {
	Headers  []ReportHeader           `json:"headers"`
	Data     []map[string]interface{} `json:"data"`
	Summary  map[string]interface{}   `json:"summary,omitempty"`
	MetaData ReportMetaData           `json:"metadata"`
}

// ReportHeader represents column information
type ReportHeader struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	DataType   string `json:"data_type"`
	Format     string `json:"format,omitempty"`
	Sortable   bool   `json:"sortable"`
	Filterable bool   `json:"filterable"`
}

// ReportMetaData contains execution metadata
type ReportMetaData struct {
	TotalRows     int       `json:"total_rows"`
	ExecutionTime int64     `json:"execution_time_ms"`
	GeneratedAt   time.Time `json:"generated_at"`
	CacheKey      string    `json:"cache_key,omitempty"`
}

// formFieldIDPattern matches auto-generated form field IDs like "field_1775802520595"
var formFieldIDPattern = regexp.MustCompile(`^field_\d+$`)

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
		Headers: re.buildHeaders(fields, aggregations),
		Data:    []map[string]interface{}{},
		Summary: make(map[string]interface{}),
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

	resolvedDataSources, err := re.resolveDataSources(dataSources)
	if err != nil {
		return "", nil, err
	}
	dataSources = resolvedDataSources

	// Resolved table per data source alias, so field references can be checked
	// against the columns that table/view actually has.
	sourceTables := make(map[string]string, len(dataSources))
	for _, ds := range dataSources {
		sourceTables[strings.ToLower(ds.Alias)] = ds.TableName
	}

	if err := validateDataSourceRefs(dataSources, fields, filters, groupings, aggregations, sortings); err != nil {
		return "", nil, err
	}

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
			fieldRef, refErr := re.resolveColumnRef(sourceTables, field.DataSource, field.FieldName)
			if refErr != nil {
				return "", nil, refErr
			}
			aggExpr, aggErr := aggregateExpr(field.Aggregation, fieldRef)
			if aggErr != nil {
				return "", nil, aggErr
			}
			selectClauses = append(selectClauses, fmt.Sprintf(
				"%s AS %s",
				aggExpr,
				re.quoteIdentifier(alias),
			))
		} else {
			fieldExpr, refErr := re.resolveFieldExpression(sourceTables, field)
			if refErr != nil {
				return "", nil, refErr
			}
			selectClauses = append(selectClauses, fmt.Sprintf(
				"%s AS %s",
				fieldExpr,
				re.quoteIdentifier(alias),
			))
		}
	}

	// Add aggregations
	for _, agg := range aggregations {
		alias := agg.Alias
		if alias == "" {
			alias = fmt.Sprintf("%s_%s", strings.ToLower(agg.Function), agg.FieldName)
		}

		fieldRef, refErr := re.resolveColumnRef(sourceTables, agg.DataSource, agg.FieldName)
		if refErr != nil {
			return "", nil, refErr
		}

		aggExpr, aggErr := aggregateExpr(agg.Function, fieldRef)
		if aggErr != nil {
			return "", nil, aggErr
		}
		selectClauses = append(selectClauses, fmt.Sprintf(
			"%s AS %s",
			aggExpr,
			re.quoteIdentifier(alias),
		))
	}

	if len(selectClauses) == 0 {
		return "", nil, fmt.Errorf("no fields selected")
	}

	query.WriteString(strings.Join(selectClauses, ", "))

	// FROM clause
	if len(dataSources) == 0 {
		return "", nil, fmt.Errorf("no data sources specified")
	}

	baseAlias, aliasErr := re.safeIdentifier(dataSources[0].Alias)
	if aliasErr != nil {
		return "", nil, aliasErr
	}
	query.WriteString(fmt.Sprintf("\nFROM %s AS %s", dataSources[0].TableName, baseAlias))
	requiredJoins := re.collectRequiredJoins(fields, filters, groupings, sortings)
	for _, joinClause := range requiredJoins {
		query.WriteString("\n" + joinClause)
	}

	// JOIN clauses (for multi-table reports). join_type and join_on come from
	// the saved definition, so both are rebuilt from validated parts.
	joinScope := map[string]bool{strings.ToLower(baseAlias): true}
	for i := 1; i < len(dataSources); i++ {
		ds := dataSources[i]
		dsAlias, dsAliasErr := re.safeIdentifier(ds.Alias)
		if dsAliasErr != nil {
			return "", nil, dsAliasErr
		}
		joinType, joinTypeErr := joinKeyword(ds.JoinType)
		if joinTypeErr != nil {
			return "", nil, fmt.Errorf("data source %s: %w", dsAlias, joinTypeErr)
		}
		joinScope[strings.ToLower(dsAlias)] = true
		joinOn, joinOnErr := renderJoinCondition(ds.JoinOn, joinScope)
		if joinOnErr != nil {
			return "", nil, fmt.Errorf("data source %s: %w", dsAlias, joinOnErr)
		}

		query.WriteString(fmt.Sprintf(
			"\n%s %s AS %s ON %s",
			joinType,
			ds.TableName,
			dsAlias,
			joinOn,
		))
	}

	// WHERE clause
	whereClauses := []string{}

	// Automatic form_code filter: scope query to the selected form's submissions only.
	// Parameterized to prevent SQL injection.
	for _, ds := range dataSources {
		if strings.TrimSpace(ds.FormCode) == "" {
			continue
		}
		dsAlias, aliasErr := re.safeIdentifier(ds.Alias)
		if aliasErr != nil {
			return "", nil, aliasErr
		}
		whereClauses = append(whereClauses,
			fmt.Sprintf("%s.form_code = $%d", re.quoteIdentifier(dsAlias), argIndex))
		args = append(args, strings.TrimSpace(ds.FormCode))
		argIndex++
	}

	// Always exclude soft-deleted records
	for _, ds := range dataSources {
		alias, aliasErr := re.safeIdentifier(ds.Alias)
		if aliasErr != nil {
			return "", nil, aliasErr
		}
		whereClauses = append(whereClauses, fmt.Sprintf("%s.deleted_at IS NULL", alias))
	}
	if _, ok := requiredJoins["sites"]; ok {
		whereClauses = append(whereClauses, "sites.deleted_at IS NULL")
	}

	// Add filters
	for _, filter := range filters {
		clause, filterArgs, filterErr := re.buildFilterClause(sourceTables, filter, &argIndex)
		if filterErr != nil {
			return "", nil, filterErr
		}
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
			groupRef, refErr := re.resolveColumnRef(sourceTables, group.DataSource, group.FieldName)
			if refErr != nil {
				return "", nil, refErr
			}
			groupClauses = append(groupClauses, groupRef)
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
				sortRef, refErr := re.resolveColumnRef(sourceTables, sort.DataSource, sort.FieldName)
				if refErr != nil {
					return "", nil, refErr
				}
				orderClauses = append(orderClauses, fmt.Sprintf("%s %s", sortRef, direction))
			} else {
				sortField, sortErr := re.safeIdentifier(sort.FieldName)
				if sortErr != nil {
					return "", nil, sortErr
				}
				orderClauses = append(orderClauses, fmt.Sprintf("%s %s", sortField, direction))
			}
		}
		query.WriteString("\nORDER BY " + strings.Join(orderClauses, ", "))
	}

	return query.String(), args, nil
}

func (re *ReportEngine) resolveFieldExpression(sourceTables map[string]string, field models.ReportField) (string, error) {
	// Form field IDs (e.g. "field_1775802520595") are stored as JSONB keys in
	// form_submissions.form_data. Extract them with the ->> operator.
	if formFieldIDPattern.MatchString(field.FieldName) {
		safeAlias, err := re.safeIdentifier(field.DataSource)
		if err != nil {
			return "", err
		}
		aliasRef := re.quoteIdentifier(safeAlias)
		// Safe: field.FieldName is validated to match ^field_\d+$ (no SQL injection risk).
		// 1) If JSON value is an object, prefer its "name" display value.
		// 2) Else, for UUID-like legacy values, try resolving site name.
		// 3) Fallback to the raw text value.
		return fmt.Sprintf(`CASE
	WHEN jsonb_typeof(%s.form_data->'%s') = 'object' THEN COALESCE(%s.form_data->'%s'->>'name', %s.form_data->>'%s')
	ELSE COALESCE((SELECT s.name FROM sites AS s WHERE s.id::text = %s.form_data->>'%s' AND s.deleted_at IS NULL LIMIT 1), %s.form_data->>'%s')
END`, aliasRef, field.FieldName, aliasRef, field.FieldName, aliasRef, field.FieldName, aliasRef, field.FieldName, aliasRef, field.FieldName), nil
	}
	return re.resolveColumnRef(sourceTables, field.DataSource, field.FieldName)
}

// resolveColumnRef maps a field name offered by the report builder to a SQL
// expression on its data source. Filters, groupings, sortings and aggregations
// go through it too, so they accept the same fields the SELECT does.
func (re *ReportEngine) resolveColumnRef(sourceTables map[string]string, dataSource, fieldName string) (string, error) {
	switch fieldName {
	case "submitted_by_name", "created_by_name":
		return re.safeFieldRef("creator", "name")
	case "site_name":
		return re.safeFieldRef("sites", "name")
	case "business_vertical_name":
		return re.safeFieldRef("business_verticals", "name")
	case "submission_id":
		// submission_id is an alias for the primary key 'id' of the form submissions view/table.
		return re.safeFieldRef(dataSource, "id")
	case "submitted_at", "current_state", "form_code":
		return re.safeFieldRef(dataSource, fieldName)
	case "wf_last_action":
		return re.safeFieldRef("wf_latest", "action")
	case "wf_last_action_by":
		return re.safeFieldRef("wf_latest", "actor_name")
	case "wf_last_action_at":
		return re.safeFieldRef("wf_latest", "transitioned_at")
	case "wf_last_comment":
		return re.safeFieldRef("wf_latest", "comment")
	}

	fieldRef, err := re.safeFieldRef(dataSource, fieldName)
	if err != nil {
		return "", err
	}
	dataSource, fieldName = strings.TrimSpace(dataSource), strings.TrimSpace(fieldName)

	// Validate the column actually exists in the resolved table/view, otherwise
	// the query fails with SQLSTATE 42703. Per-form views only project fields the
	// form schema declares, but the report builder also offers keys inferred from
	// submission form_data (fields since removed from the schema, or written
	// outside it), so those have no view column.
	qualifiedTable, ok := sourceTables[strings.ToLower(dataSource)]
	if !ok {
		return fieldRef, nil
	}
	viewCols := re.getViewColumns(qualifiedTable)
	if len(viewCols) == 0 || viewCols[strings.ToLower(fieldName)] {
		return fieldRef, nil
	}
	aliasRef := re.quoteIdentifier(dataSource)
	if viewCols["form_data"] {
		// Read the key straight from form_data, preferring the display "name" of
		// object values (dropdown selections). Safe: fieldName passed safeIdentifier.
		return fmt.Sprintf(`COALESCE(%s.form_data->'%s'->>'name', %s.form_data->>'%s')`,
			aliasRef, fieldName, aliasRef, fieldName), nil
	}
	// No form_data to fall back on: emit NULL so stale reports degrade gracefully.
	log.Printf("[REPORT ENGINE] column %q not found in %s — substituting NULL", fieldName, qualifiedTable)
	return fmt.Sprintf("NULL::text /* stale field: %s.%s */", dataSource, fieldName), nil
}

func (re *ReportEngine) collectRequiredJoins(
	fields []models.ReportField,
	filters []models.ReportFilter,
	groupings []models.ReportGrouping,
	sortings []models.ReportSorting,
) map[string]string {
	joins := map[string]string{}
	// dataSource is written into the join SQL; buildQuery has already checked it
	// is a declared data source alias (validateDataSourceRefs).
	register := func(fieldName string, dataSource string) {
		switch fieldName {
		case "submitted_by_name", "created_by_name":
			// form_submissions uses submitted_by; legacy reports may use created_by_name alias.
			joins["creator"] = fmt.Sprintf("LEFT JOIN users AS creator ON creator.id::text = %s.submitted_by", dataSource)
		case "site_name":
			joins["sites"] = fmt.Sprintf("LEFT JOIN sites AS sites ON sites.id = %s.site_id", dataSource)
		case "business_vertical_name":
			joins["business_verticals"] = fmt.Sprintf("LEFT JOIN business_verticals AS business_verticals ON business_verticals.id = %s.business_vertical_id", dataSource)
		case "wf_last_action", "wf_last_action_by", "wf_last_action_at", "wf_last_comment":
			// LATERAL subquery: most-recent workflow_transition for each submission row.
			if _, exists := joins["wf_latest"]; !exists {
				joins["wf_latest"] = fmt.Sprintf(
					"LEFT JOIN LATERAL (SELECT action, actor_name, transitioned_at, comment FROM workflow_transitions WHERE submission_id = %s.id ORDER BY transitioned_at DESC LIMIT 1) AS wf_latest ON true",
					dataSource,
				)
			}
		}
	}
	for _, field := range fields {
		register(field.FieldName, field.DataSource)
	}
	for _, filter := range filters {
		register(filter.FieldName, filter.DataSource)
	}
	for _, group := range groupings {
		register(group.FieldName, group.DataSource)
	}
	for _, sort := range sortings {
		register(sort.FieldName, sort.DataSource)
	}
	return joins
}

// buildFilterClause constructs a WHERE clause for a filter
func (re *ReportEngine) buildFilterClause(sourceTables map[string]string, filter models.ReportFilter, argIndex *int) (string, []interface{}, error) {
	var args []interface{}
	fieldRef, refErr := re.resolveColumnRef(sourceTables, filter.DataSource, filter.FieldName)
	if refErr != nil {
		return "", nil, refErr
	}

	switch strings.ToLower(filter.Operator) {
	case "eq", "=":
		clause := fmt.Sprintf("%s = $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "ne", "!=", "<>":
		clause := fmt.Sprintf("%s != $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "gt", ">":
		clause := fmt.Sprintf("%s > $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "gte", ">=":
		clause := fmt.Sprintf("%s >= $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "lt", "<":
		clause := fmt.Sprintf("%s < $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "lte", "<=":
		clause := fmt.Sprintf("%s <= $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "like", "contains":
		clause := fmt.Sprintf("%s ILIKE $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{fmt.Sprintf("%%%v%%", filter.Value)}, nil

	case "starts_with":
		clause := fmt.Sprintf("%s ILIKE $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{fmt.Sprintf("%v%%", filter.Value)}, nil

	case "ends_with":
		clause := fmt.Sprintf("%s ILIKE $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{fmt.Sprintf("%%%v", filter.Value)}, nil

	case "in":
		if values, ok := filter.Value.([]interface{}); ok && len(values) > 0 {
			placeholders := []string{}
			for _, val := range values {
				placeholders = append(placeholders, fmt.Sprintf("$%d", *argIndex))
				args = append(args, val)
				*argIndex++
			}
			clause := fmt.Sprintf("%s IN (%s)", fieldRef, strings.Join(placeholders, ", "))
			return clause, args, nil
		}

	case "between":
		if values, ok := filter.Value.([]interface{}); ok && len(values) == 2 {
			clause := fmt.Sprintf("%s BETWEEN $%d AND $%d", fieldRef, *argIndex, *argIndex+1)
			*argIndex += 2
			return clause, []interface{}{values[0], values[1]}, nil
		}

	case "is_null":
		return fmt.Sprintf("%s IS NULL", fieldRef), nil, nil

	case "is_not_null":
		return fmt.Sprintf("%s IS NOT NULL", fieldRef), nil, nil

	case "date_equals":
		clause := fmt.Sprintf("DATE(%s) = $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "date_before":
		clause := fmt.Sprintf("DATE(%s) < $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "date_after":
		clause := fmt.Sprintf("DATE(%s) > $%d", fieldRef, *argIndex)
		*argIndex++
		return clause, []interface{}{filter.Value}, nil

	case "this_week":
		return fmt.Sprintf("%s >= date_trunc('week', CURRENT_DATE) AND %s < date_trunc('week', CURRENT_DATE) + interval '1 week'", fieldRef, fieldRef), nil, nil

	case "this_month":
		return fmt.Sprintf("%s >= date_trunc('month', CURRENT_DATE) AND %s < date_trunc('month', CURRENT_DATE) + interval '1 month'", fieldRef, fieldRef), nil, nil

	case "this_year":
		return fmt.Sprintf("%s >= date_trunc('year', CURRENT_DATE) AND %s < date_trunc('year', CURRENT_DATE) + interval '1 year'", fieldRef, fieldRef), nil, nil

	case "last_n_days":
		if days, ok := filter.Value.(float64); ok {
			return fmt.Sprintf("%s >= CURRENT_DATE - interval '%d days'", fieldRef, int(days)), nil, nil
		}
	}

	return "", nil, nil
}

func (re *ReportEngine) safeIdentifier(identifier string) (string, error) {
	id := strings.TrimSpace(identifier)
	if id == "" {
		return "", fmt.Errorf("empty SQL identifier")
	}
	if !sqlIdentifierPattern.MatchString(id) {
		return "", fmt.Errorf("invalid SQL identifier: %s", id)
	}
	return id, nil
}

func (re *ReportEngine) quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func (re *ReportEngine) safeFieldRef(alias, fieldName string) (string, error) {
	safeAlias, err := re.safeIdentifier(alias)
	if err != nil {
		return "", err
	}
	safeField, err := re.safeIdentifier(fieldName)
	if err != nil {
		return "", err
	}
	return re.quoteIdentifier(safeAlias) + "." + re.quoteIdentifier(safeField), nil
}

func (re *ReportEngine) resolveDataSources(dataSources []models.DataSource) ([]models.DataSource, error) {
	resolved := make([]models.DataSource, len(dataSources))
	for i, ds := range dataSources {
		if strings.TrimSpace(ds.Alias) == "" {
			ds.Alias = fmt.Sprintf("t%d", i+1)
		}
		if _, err := re.safeIdentifier(ds.Alias); err != nil {
			return nil, fmt.Errorf("invalid data source alias '%s': %w", ds.Alias, err)
		}

		tableRef, err := re.resolveDataSourceTable(ds)
		if err != nil {
			return nil, err
		}
		ds.TableName = tableRef
		resolved[i] = ds
	}
	return resolved, nil
}

func (re *ReportEngine) resolveDataSourceTable(ds models.DataSource) (string, error) {
	// Form-based reports use a deterministic per-form view with extracted field_* columns.
	// This keeps filters/grouping/sorting stable and avoids direct JSONB expressions everywhere.
	if strings.TrimSpace(ds.FormCode) != "" {
		qualifiedView, err := EnsureReportFormViewByCode(re.db, ds.FormCode)
		if err != nil {
			return "", fmt.Errorf("failed to resolve report view for form_code %s: %w", ds.FormCode, err)
		}
		return qualifiedView, nil
	}

	// No form_code: use the literal table_name (for custom datasources or legacy reports).
	tableCandidate := strings.TrimSpace(ds.TableName)
	if tableCandidate == "" {
		return "", fmt.Errorf("data source requires either form_code or table_name")
	}

	// Legacy auto-heal: if table_name matches an app form code or db_table_name,
	// route through the deterministic per-form reporting view.
	tableLookup := tableCandidate
	if dot := strings.Index(tableLookup, "."); dot >= 0 {
		tableLookup = tableLookup[dot+1:]
	}
	tableLookup = strings.Trim(tableLookup, `"`)
	if tableLookup != "" {
		var form models.AppForm
		if err := re.db.Where("LOWER(code) = LOWER(?) OR LOWER(db_table_name) = LOWER(?)", tableLookup, tableLookup).
			Select("code").
			First(&form).Error; err == nil && strings.TrimSpace(form.Code) != "" {
			qualifiedView, viewErr := EnsureReportFormViewByCode(re.db, form.Code)
			if viewErr != nil {
				return "", fmt.Errorf("failed to resolve report view for legacy datasource %s: %w", tableCandidate, viewErr)
			}
			return qualifiedView, nil
		}
	}

	tenantSchema := re.currentSchema()
	if tenantSchema == "" {
		return "", fmt.Errorf("could not resolve the tenant schema for report data source %s", tableCandidate)
	}

	// Data sources resolve inside the tenant schema only. A "public." prefix
	// is accepted for reports saved before tenants had their own schemas; any
	// other schema would read another tenant's (or the control) data.
	parts := strings.SplitN(tableCandidate, ".", 2)
	cleanTableName := strings.Trim(parts[0], `"`)
	if len(parts) == 2 {
		schema := strings.Trim(parts[0], `"`)
		if schema != tenantSchema && schema != "public" {
			return "", fmt.Errorf("report data source %s is outside the tenant schema", tableCandidate)
		}
		cleanTableName = strings.Trim(parts[1], `"`)
	}
	if _, err := re.safeIdentifier(cleanTableName); err != nil {
		return "", fmt.Errorf("invalid table name '%s': %w", cleanTableName, err)
	}

	// Cache key is scoped by tenant schema: the same table name resolves to a
	// different schema per tenant, so a bare name would leak one tenant's
	// resolution to another via this process-wide cache.
	cacheKey := tenantSchema + "|" + strings.ToLower(cleanTableName)
	if cachedQualified, ok := getCachedResolvedTable(cacheKey); ok {
		return cachedQualified, nil
	}

	loaded, err, _ := reportTableResolutionLoadGroup.Do(cacheKey, func() (interface{}, error) {
		if cachedQualified, ok := getCachedResolvedTable(cacheKey); ok {
			return cachedQualified, nil
		}

		var resolved struct {
			TableSchema string `gorm:"column:table_schema"`
			TableName   string `gorm:"column:table_name"`
		}
		if dbErr := re.db.Raw(`
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE lower(table_name) = lower(?)
		  AND table_type = 'BASE TABLE'
		  AND table_schema = ?
		LIMIT 1
		`, cleanTableName, tenantSchema).Scan(&resolved).Error; dbErr != nil {
			return "", dbErr
		}
		if resolved.TableName == "" {
			return "", fmt.Errorf("report data source table not found in schema %s: %s", tenantSchema, cleanTableName)
		}

		qualified := re.quoteIdentifier(resolved.TableSchema) + "." + re.quoteIdentifier(resolved.TableName)
		setCachedResolvedTable(cacheKey, qualified)
		return qualified, nil
	})
	if err != nil {
		return "", err
	}

	return loaded.(string), nil
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

	normalizedType := strings.ToLower(strings.TrimSpace(dataType))
	shouldFormatDateTime := strings.Contains(normalizedType, "date") || strings.Contains(normalizedType, "time")
	if shouldFormatDateTime {
		switch v := value.(type) {
		case time.Time:
			return v.Format("02-01-2006 15:04")
		case *time.Time:
			if v != nil {
				return v.Format("02-01-2006 15:04")
			}
		case string:
			if parsed, ok := parseReportDateTime(v); ok {
				return parsed.Format("02-01-2006 15:04")
			}
		case []byte:
			s := string(v)
			if parsed, ok := parseReportDateTime(s); ok {
				return parsed.Format("02-01-2006 15:04")
			}
			return s
		}
	}

	// Handle byte arrays (from database)
	if b, ok := value.([]byte); ok {
		return string(b)
	}

	return value
}

func parseReportDateTime(raw string) (time.Time, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, v); err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
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
	cleanTableName := strings.TrimSpace(tableName)
	if cleanTableName == "" {
		return nil, fmt.Errorf("table name is required")
	}

	tenantSchema := re.currentSchema()
	// Tenant-scoped cache key — see resolveDataSourceTable for why.
	cacheKey := tenantSchema + "|" + strings.ToLower(cleanTableName)
	if cachedSchema, ok := getCachedFormSchema(cacheKey); ok {
		return cachedSchema, nil
	}

	loaded, loadErr, _ := reportFormSchemaLoadGroup.Do(cacheKey, func() (interface{}, error) {
		if cachedSchema, ok := getCachedFormSchema(cacheKey); ok {
			return cachedSchema, nil
		}

		var resolved struct {
			TableSchema string `gorm:"column:table_schema"`
			TableName   string `gorm:"column:table_name"`
		}

		// Tenant schema only — see currentSchema.
		resolveQuery := `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE lower(table_name) = lower(?)
		  AND table_type = 'BASE TABLE'
		  AND table_schema = ?
		LIMIT 1
	`

		if err := re.db.Raw(resolveQuery, cleanTableName, tenantSchema).Scan(&resolved).Error; err != nil {
			return nil, err
		}

		if resolved.TableName == "" {
			return nil, fmt.Errorf("table not found: %s", cleanTableName)
		}

		query := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = ?
		AND table_name = ?
		ORDER BY ordinal_position
	`

		rows, err := re.db.Raw(query, resolved.TableSchema, resolved.TableName).Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var schema []map[string]interface{}
		for rows.Next() {
			var colName, dataType, isNullable string
			var colDefault *string
			var maxLength *int

			if err := rows.Scan(&colName, &dataType, &isNullable, &colDefault, &maxLength); err != nil {
				return nil, err
			}

			schema = append(schema, map[string]interface{}{
				"name":       colName,
				"type":       re.mapSQLTypeToReportType(dataType),
				"nullable":   isNullable == "YES",
				"max_length": maxLength,
			})
		}

		if len(schema) == 0 {
			return nil, fmt.Errorf("no readable columns found for table: %s", resolved.TableName)
		}

		setCachedFormSchema(cacheKey, schema)
		return schema, nil
	})
	if loadErr != nil {
		return nil, loadErr
	}

	return loaded.([]map[string]interface{}), nil
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
