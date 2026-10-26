package reports

import (
	"strings"
	"testing"
	"time"
)

// seedViewColumns primes viewColumnsCache so resolveColumnRef never hits the database.
func seedViewColumns(t *testing.T, qualifiedTable string, cols ...string) {
	t.Helper()
	set := make(map[string]bool, len(cols))
	for _, c := range cols {
		set[c] = true
	}
	viewColumnsCache.mu.Lock()
	viewColumnsCache.entries[strings.ToLower(qualifiedTable)] = struct {
		cols      map[string]bool
		expiresAt time.Time
	}{cols: set, expiresAt: time.Now().Add(time.Hour)}
	viewColumnsCache.mu.Unlock()
	t.Cleanup(func() { invalidateViewColumns(qualifiedTable) })
}

func TestResolveColumnRef(t *testing.T) {
	re := &ReportEngine{}
	formView := `"sreeugcl"."report_form_v_dairy_report"`
	plainTable := `"tenant_a"."attendance_sessions"`
	seedViewColumns(t, formView, "id", "form_code", "current_state", "submitted_at", "site_id", "form_data", "field_1", "milk_qty")
	seedViewColumns(t, plainTable, "id", "user_id")
	sourceTables := map[string]string{"dairy_report": formView, "sessions": plainTable}

	tests := []struct {
		name       string
		dataSource string
		fieldName  string
		expected   string
	}{
		{"projected view column", "dairy_report", "milk_qty", `"dairy_report"."milk_qty"`},
		{"inferred key missing from view reads form_data", "dairy_report", "approver", `COALESCE("dairy_report".form_data->'approver'->>'name', "dairy_report".form_data->>'approver')`},
		{"missing column without form_data is NULL", "sessions", "approver", `NULL::text /* stale field: sessions.approver */`},
		{"unknown data source keeps raw ref", "other", "approver", `"other"."approver"`},
		{"submission_id maps to id", "dairy_report", "submission_id", `"dairy_report"."id"`},
		{"submitted_by_name uses creator join", "dairy_report", "submitted_by_name", `"creator"."name"`},
		{"site_name uses sites join", "dairy_report", "site_name", `"sites"."name"`},
		{"workflow field uses wf_latest join", "dairy_report", "wf_last_action_by", `"wf_latest"."actor_name"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := re.resolveColumnRef(sourceTables, tt.dataSource, tt.fieldName)
			if err != nil {
				t.Fatalf("resolveColumnRef(%q, %q) error: %v", tt.dataSource, tt.fieldName, err)
			}
			if got != tt.expected {
				t.Errorf("resolveColumnRef(%q, %q) = %s, want %s", tt.dataSource, tt.fieldName, got, tt.expected)
			}
		})
	}
}

func TestResolveColumnRefRejectsUnsafeIdentifiers(t *testing.T) {
	re := &ReportEngine{}
	for _, fieldName := range []string{"approver'--", "a b", ""} {
		if _, err := re.resolveColumnRef(map[string]string{}, "dairy_report", fieldName); err == nil {
			t.Errorf("resolveColumnRef accepted unsafe field name %q", fieldName)
		}
	}
}
