package handlers

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

var reportViewIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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

	sql, err := buildReportFormViewSQL(form, viewName)
	if err != nil {
		return "", err
	}

	if err := db.Exec(sql).Error; err != nil {
		return "", fmt.Errorf("failed to create/update report view %s: %w", viewName, err)
	}

	return viewName, nil
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

	seen := make(map[string]struct{})
	for _, field := range buildFormFieldList(form) {
		fieldID, _ := field["id"].(string)
		fieldID = strings.TrimSpace(fieldID)
		if !formFieldIDPattern.MatchString(fieldID) {
			continue
		}
		if _, exists := seen[fieldID]; exists {
			continue
		}
		seen[fieldID] = struct{}{}

		selectedColumns = append(selectedColumns,
			fmt.Sprintf(`fs.form_data->>'%s' AS "%s"`, fieldID, fieldID),
		)
	}

	return fmt.Sprintf(`CREATE OR REPLACE VIEW public.%s AS
SELECT
  %s
FROM public.form_submissions AS fs
WHERE fs.form_code = '%s'`,
		quoteSQLIdentifier(viewName),
		strings.Join(selectedColumns, ",\n  "),
		escapeSQLLiteral(strings.TrimSpace(form.Code)),
	), nil
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
