package reports

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"sync"

	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

var reportViewIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var (
	reportViewSyncCacheMu sync.RWMutex
	reportViewSyncCache   = map[string]uint32{}
)

// dropdownResolution describes how to resolve a dropdown UUID to its display label in SQL.
type dropdownResolution struct {
	table     string // sanitized table name to JOIN/subquery
	labelCol  string // column to SELECT for the display value
	valueCol  string // column to match against the stored UUID
	extraCond string // extra WHERE safety condition (e.g. "deleted_at IS NULL")
}

// inferTableNameFromEndpointForView extracts a safe SQL identifier from the last meaningful
// path segment of a URL (e.g. "/api/v1/sites?foo=bar" → "sites").
func inferTableNameFromEndpointForView(endpoint string) string {
	if idx := strings.Index(endpoint, "?"); idx >= 0 {
		endpoint = endpoint[:idx]
	}
	parts := strings.Split(strings.TrimRight(endpoint, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		seg := strings.TrimSpace(parts[i])
		if reportViewIdentifierPattern.MatchString(seg) {
			return seg
		}
	}
	return ""
}

// resolveDropdownEndpoint maps an apiEndpoint string to a SQL lookup descriptor.
// Uses the same heuristics as resolveReferenceValue in workflow_engine_dedicated.go.
func resolveDropdownEndpoint(apiEndpoint string) *dropdownResolution {
	norm := strings.ToLower(strings.TrimSpace(apiEndpoint))
	if norm == "" {
		return nil
	}
	// Dynamic form lookup endpoints return synthesized payloads (id + form_data fields),
	// not a physical SQL table with display columns. Skip SQL JOIN inference here.
	if strings.Contains(norm, "/forms/") && strings.Contains(norm, "/lookup") {
		return nil
	}
	if strings.Contains(norm, "/sites") {
		return &dropdownResolution{table: "sites", labelCol: "name", valueCol: "id", extraCond: "deleted_at IS NULL"}
	}
	if strings.Contains(norm, "/users") {
		return &dropdownResolution{table: "users", labelCol: "name", valueCol: "id", extraCond: "is_active = TRUE"}
	}
	if strings.Contains(norm, "business_vertical") || (strings.Contains(norm, "business") && strings.Contains(norm, "vertical")) {
		return &dropdownResolution{table: "business_verticals", labelCol: "name", valueCol: "id", extraCond: "is_active = TRUE"}
	}
	if strings.Contains(norm, "business-roles") || strings.Contains(norm, "business_roles") {
		return &dropdownResolution{table: "business_roles", labelCol: "name", valueCol: "id", extraCond: "is_active = TRUE"}
	}
	if strings.Contains(norm, "/roles") {
		return &dropdownResolution{table: "roles", labelCol: "name", valueCol: "id", extraCond: "is_active = TRUE"}
	}
	if strings.Contains(norm, "/modules") {
		return &dropdownResolution{table: "modules", labelCol: "name", valueCol: "id", extraCond: "is_active = TRUE"}
	}
	// Generic: infer table from endpoint tail segment.
	if inferred := inferTableNameFromEndpointForView(norm); inferred != "" {
		return &dropdownResolution{table: inferred, labelCol: "name", valueCol: "id", extraCond: ""}
	}
	return nil
}

// buildDropdownResolutionMap parses the full form schema and returns a map of
// normalizedFieldName → dropdownResolution for every dropdown field that has an apiEndpoint.
func buildDropdownResolutionMap(form models.AppForm) map[string]dropdownResolution {
	result := map[string]dropdownResolution{}

	normKey := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.ReplaceAll(s, " ", "_")
		s = strings.ReplaceAll(s, "-", "_")
		return s
	}

	var parseFields func(raw interface{})
	parseFields = func(raw interface{}) {
		switch v := raw.(type) {
		case map[string]interface{}:
			fieldType, _ := v["type"].(string)
			apiEndpoint, _ := v["apiEndpoint"].(string)

			if strings.EqualFold(strings.TrimSpace(fieldType), "dropdown") && strings.TrimSpace(apiEndpoint) != "" {
				res := resolveDropdownEndpoint(apiEndpoint)
				if res != nil {
					// Allow overriding the display column with a custom displayField.
					if df, ok := v["displayField"].(string); ok && reportViewIdentifierPattern.MatchString(strings.TrimSpace(df)) {
						cp := *res
						cp.labelCol = strings.TrimSpace(df)
						res = &cp
					}
					// Register under both the field "id" (auto-generated) and "name" (semantic).
					for _, key := range []string{"id", "name"} {
						val, _ := v[key].(string)
						nk := normKey(val)
						if nk != "" && reportViewIdentifierPattern.MatchString(nk) {
							result[nk] = *res
						}
					}
				}
			}

			// Recurse into nested structures.
			for _, key := range []string{"fields", "children", "sections", "components"} {
				parseFields(v[key])
			}
			if cols, ok := v["columns"].([]interface{}); ok {
				for _, col := range cols {
					if colMap, ok := col.(map[string]interface{}); ok {
						parseFields(colMap["fields"])
					}
				}
			}

		case []interface{}:
			for _, item := range v {
				parseFields(item)
			}
		}
	}

	for _, raw := range []json.RawMessage{form.FormSchema, form.Steps, form.CoreFields} {
		s := strings.TrimSpace(string(raw))
		if s == "" || s == "null" || s == "{}" || s == "[]" {
			continue
		}
		var parsed interface{}
		if err := json.Unmarshal(raw, &parsed); err == nil {
			parseFields(parsed)
		}
	}
	return result
}

// ensureReportViewsForDataSources validates data sources and ensures per-form report views exist.
func ensureReportViewsForDataSources(db *gorm.DB, raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var dataSources []models.DataSource
	if err := json.Unmarshal(raw, &dataSources); err != nil {
		return fmt.Errorf("invalid data_sources payload: %w", err)
	}

	seen := make(map[string]struct{})
	for _, ds := range dataSources {
		formCode := strings.TrimSpace(ds.FormCode)
		if formCode == "" {
			continue
		}
		key := strings.ToUpper(formCode)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		if _, err := EnsureReportFormViewByCode(db, formCode); err != nil {
			return err
		}
	}

	return nil
}

// EnsureReportFormViewByCode creates or refreshes a deterministic per-form reporting view.
func EnsureReportFormViewByCode(db *gorm.DB, formCode string) (string, error) {
	trimmedCode := strings.TrimSpace(formCode)
	if trimmedCode == "" {
		return "", fmt.Errorf("form_code is required")
	}

	var form models.AppForm
	err := db.Where("LOWER(code) = LOWER(?)", trimmedCode).
		Select("id", "code", "is_active", "form_schema", "steps", "core_fields").
		First(&form).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("form not found for form_code: %s", trimmedCode)
		}
		return "", fmt.Errorf("failed to load form by code %s: %w", trimmedCode, err)
	}

	return EnsureReportFormViewForForm(db, form)
}

// EnsureReportFormViewForForm creates or refreshes a report view using an in-memory form definition.
func EnsureReportFormViewForForm(db *gorm.DB, form models.AppForm) (string, error) {
	viewName, err := reportFormViewName(form.Code)
	if err != nil {
		return "", err
	}

	signature := reportFormSyncSignature(form)
	if isReportViewSyncCached(viewName, signature) {
		exists, err := reportViewExists(db, viewName)
		if err == nil && exists {
			return viewName, nil
		}
	}

	sql, err := buildReportFormViewSQL(form, viewName)
	if err != nil {
		return "", err
	}

	// Use a transaction-scoped advisory lock per view name to prevent concurrent
	// dashboard/report requests from racing on DROP/CREATE and triggering 42P07.
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext(?), 0)`, viewName).Error; err != nil {
			return fmt.Errorf("failed to lock report view %s for sync: %w", viewName, err)
		}

		// sql is two semicolon-separated statements: DROP VIEW IF EXISTS + CREATE VIEW.
		// Execute them individually so GORM does not choke on the multi-statement string.
		parts := strings.SplitN(sql, ";", 2)
		for _, stmt := range parts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("failed to create/update report view %s: %w", viewName, err)
			}
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	setReportViewSyncCached(viewName, signature)

	return viewName, nil
}

func reportFormSyncSignature(form models.AppForm) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(strings.ToLower(form.Code))))
	_, _ = h.Write(form.FormSchema)
	_, _ = h.Write(form.Steps)
	_, _ = h.Write(form.CoreFields)
	return h.Sum32()
}

func isReportViewSyncCached(viewName string, signature uint32) bool {
	reportViewSyncCacheMu.RLock()
	defer reportViewSyncCacheMu.RUnlock()
	v, ok := reportViewSyncCache[viewName]
	return ok && v == signature
}

func setReportViewSyncCached(viewName string, signature uint32) {
	reportViewSyncCacheMu.Lock()
	defer reportViewSyncCacheMu.Unlock()
	reportViewSyncCache[viewName] = signature
}

func reportViewExists(db *gorm.DB, viewName string) (bool, error) {
	qualified := fmt.Sprintf("public.%s", quoteSQLIdentifier(viewName))
	var exists bool
	if err := db.Raw(`SELECT to_regclass(?) IS NOT NULL`, qualified).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// EnsureAllActiveFormReportViews creates or refreshes report views for all active forms.
// Returns the number of successfully synced form views.
func EnsureAllActiveFormReportViews(db *gorm.DB) (int, error) {
	var forms []models.AppForm
	if err := db.Where("is_active = ?", true).
		Select("id", "code", "form_schema", "steps", "core_fields").
		Find(&forms).Error; err != nil {
		return 0, fmt.Errorf("failed to load active forms for report view sync: %w", err)
	}

	count := 0
	for _, form := range forms {
		if strings.TrimSpace(form.Code) == "" {
			continue
		}
		if _, err := EnsureReportFormViewForForm(db, form); err != nil {
			return count, fmt.Errorf("failed syncing report view for form %s: %w", form.Code, err)
		}
		count++
	}

	return count, nil
}

func buildReportFormViewSQL(form models.AppForm, viewName string) (string, error) {
	if strings.TrimSpace(form.Code) == "" {
		return "", fmt.Errorf("cannot build report view: form code is empty")
	}

	if !reportViewIdentifierPattern.MatchString(viewName) {
		return "", fmt.Errorf("invalid report view identifier: %s", viewName)
	}

	selectedColumns := []string{
		`fs.id AS id`,
		`fs.form_id`,
		`fs.form_code`,
		`fs.business_vertical_id`,
		`fs.site_id`,
		`fs.workflow_id`,
		`fs.current_state`,
		`fs.submitted_by`,
		`fs.submitted_at`,
		`fs.deleted_at`,
		`fs.form_data`,
	}

	normalizeColumnName := func(value string) string {
		name := strings.TrimSpace(value)
		if name == "" {
			return ""
		}
		name = strings.ToLower(name)
		name = strings.ReplaceAll(name, " ", "_")
		name = strings.ReplaceAll(name, "-", "_")
		return name
	}

	seen := make(map[string]struct{})

	// Build dropdown resolution map: for dropdown fields with an apiEndpoint, the view
	// emits a COALESCE subquery that resolves the stored UUID to its display label.
	dropdownMap := buildDropdownResolutionMap(form)

	appendProjectedColumn := func(alias string, candidateKeys ...string) {
		alias = normalizeColumnName(alias)
		if alias == "" || !sqlIdentifierPattern.MatchString(alias) {
			return
		}
		if _, exists := seen[alias]; exists {
			return
		}

		// Collect de-duplicated raw JSONB extraction expressions.
		rawExprs := make([]string, 0, len(candidateKeys))
		seenKeys := make(map[string]struct{})
		for _, key := range candidateKeys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if _, exists := seenKeys[key]; exists {
				continue
			}
			seenKeys[key] = struct{}{}
			rawExprs = append(rawExprs, fmt.Sprintf(`fs.form_data->>'%s'`, escapeSQLLiteral(key)))
		}
		if len(rawExprs) == 0 {
			return
		}
		seen[alias] = struct{}{}

		// For dropdown fields with a resolvable reference, emit a COALESCE subquery
		// that looks up the display label from the referenced table.
		if res, ok := dropdownMap[alias]; ok &&
			reportViewIdentifierPattern.MatchString(res.table) &&
			reportViewIdentifierPattern.MatchString(res.labelCol) &&
			reportViewIdentifierPattern.MatchString(res.valueCol) {

			rawExpr := rawExprs[0]
			if len(rawExprs) > 1 {
				rawExpr = fmt.Sprintf("COALESCE(%s)", strings.Join(rawExprs, ", "))
			}
			cond := fmt.Sprintf(`%s.%s::text = %s`,
				quoteSQLIdentifier(res.table),
				quoteSQLIdentifier(res.valueCol),
				rawExpr,
			)
			if res.extraCond != "" {
				cond += " AND " + res.extraCond
			}
			selectedColumns = append(selectedColumns, fmt.Sprintf(
				`COALESCE((SELECT %s.%s::text FROM %s WHERE %s LIMIT 1), %s) AS %s`,
				quoteSQLIdentifier(res.table),
				quoteSQLIdentifier(res.labelCol),
				quoteSQLIdentifier(res.table),
				cond,
				rawExpr,
				quoteSQLIdentifier(alias),
			))
			return
		}

		// Default: plain JSONB text extraction.
		if len(rawExprs) == 1 {
			selectedColumns = append(selectedColumns,
				fmt.Sprintf(`%s AS %s`, rawExprs[0], quoteSQLIdentifier(alias)),
			)
			return
		}
		selectedColumns = append(selectedColumns,
			fmt.Sprintf(`COALESCE(%s) AS %s`, strings.Join(rawExprs, ", "), quoteSQLIdentifier(alias)),
		)
	}

	for _, field := range buildFormFieldList(form) {
		fieldID, _ := field["id"].(string)
		fieldID = strings.TrimSpace(fieldID)
		fieldName, _ := field["name"].(string)
		fieldName = normalizeColumnName(fieldName)

		// Support both naming schemes:
		// 1) legacy auto-generated field ids like field_1775802520595
		// 2) semantic schema names like end_time used by dedicated form tables and existing reports
		appendProjectedColumn(fieldID, fieldID, fieldName)
		appendProjectedColumn(fieldName, fieldName, fieldID)
	}

	// Return DROP + CREATE so that column type changes (e.g. text → varchar from a new
	// COALESCE subquery) are always accepted. DROP IF EXISTS prevents errors on first run.
	createDDL := fmt.Sprintf(`CREATE VIEW public.%s AS
SELECT
  %s
FROM public.form_submissions AS fs
WHERE fs.form_code = '%s'`,
		quoteSQLIdentifier(viewName),
		strings.Join(selectedColumns, ",\n  "),
		escapeSQLLiteral(strings.TrimSpace(form.Code)),
	)
	dropDDL := fmt.Sprintf(`DROP VIEW IF EXISTS public.%s`, quoteSQLIdentifier(viewName))
	return dropDDL + "; " + createDDL, nil
}

func reportFormViewName(formCode string) (string, error) {
	raw := strings.ToLower(strings.TrimSpace(formCode))
	raw = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(raw, "_")
	raw = strings.Trim(raw, "_")
	if raw == "" {
		return "", fmt.Errorf("cannot generate report view name: invalid form code")
	}
	if raw[0] >= '0' && raw[0] <= '9' {
		raw = "f_" + raw
	}

	prefix := "report_form_v_"
	maxLen := 63
	if len(prefix)+len(raw) <= maxLen {
		return prefix + raw, nil
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(formCode))
	suffix := fmt.Sprintf("_%08x", hasher.Sum32())
	allowedRawLen := maxLen - len(prefix) - len(suffix)
	if allowedRawLen < 1 {
		return "", fmt.Errorf("cannot generate report view name for code: %s", formCode)
	}

	return prefix + raw[:allowedRawLen] + suffix, nil
}

func quoteSQLIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func escapeSQLLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
