package reports

import (
	"encoding/json"
	"strings"
	"testing"

	"p9e.in/ugcl/models"
)

func TestBuildReportFormViewSQLStaysInTenantSchema(t *testing.T) {
	form := models.AppForm{
		Code: "dairy_report",
		FormSchema: json.RawMessage(`{"fields":[
			{"id":"field_1","name":"site","type":"dropdown","apiEndpoint":"/api/v1/sites"},
			{"id":"field_2","name":"vendor","type":"dropdown","apiEndpoint":"/api/v1/vendors"},
			{"id":"field_3","name":"milk_qty","type":"number"}
		]}`),
	}
	// vendors is missing from the tenant schema, so its lookup must be skipped.
	schemaColumns := map[string]map[string]bool{
		"sites": {"id": true, "name": true, "deleted_at": true},
	}

	sql, err := buildReportFormViewSQL(form, "sreeugcl", "report_form_v_dairy_report", schemaColumns)
	if err != nil {
		t.Fatalf("buildReportFormViewSQL error: %v", err)
	}

	for _, want := range []string{
		`DROP VIEW IF EXISTS "sreeugcl"."report_form_v_dairy_report"`,
		`CREATE VIEW "sreeugcl"."report_form_v_dairy_report" AS`,
		`FROM "sreeugcl".form_submissions AS fs`,
		`FROM "sreeugcl"."sites" AS "sites"`,
		`fs.form_data->>'vendor'`,
		`fs.form_data->>'milk_qty'`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("view SQL missing %q\n%s", want, sql)
		}
	}
	for _, unwanted := range []string{`public.`, `"public"`, `"vendors"`} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("view SQL must not contain %q\n%s", unwanted, sql)
		}
	}
}

func TestBuildReportFormViewSQLRejectsBadSchema(t *testing.T) {
	form := models.AppForm{Code: "dairy_report"}
	if _, err := buildReportFormViewSQL(form, `sreeugcl"; DROP`, "report_form_v_dairy_report", nil); err == nil {
		t.Error("buildReportFormViewSQL accepted an invalid schema name")
	}
}
