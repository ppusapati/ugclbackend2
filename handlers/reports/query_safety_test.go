package reports

import (
	"strings"
	"testing"

	"p9e.in/ugcl/models"
)

func TestRenderJoinCondition(t *testing.T) {
	scope := map[string]bool{"projects": true, "tasks": true}

	valid := []struct {
		name     string
		joinOn   string
		expected string
	}{
		{"documented example", "projects.id = tasks.project_id", "projects.id = tasks.project_id"},
		{"multiple conditions", "projects.id = tasks.project_id and projects.site_id=tasks.site_id", "projects.id = tasks.project_id AND projects.site_id = tasks.site_id"},
		{"quoted identifiers kept quoted", `"projects"."id" = "tasks"."project_id"`, `"projects"."id" = "tasks"."project_id"`},
		{"bare alias folds to lower case", "Projects.id = tasks.project_id", "Projects.id = tasks.project_id"},
		{"whitelisted cast", "projects.id::text = tasks.project_id", "projects.id::text = tasks.project_id"},
		{"wrapped in parentheses", "(projects.id = tasks.project_id)", "projects.id = tasks.project_id"},
		{"each condition parenthesised", "(projects.id = tasks.project_id) AND (projects.site_id = tasks.site_id)", "projects.id = tasks.project_id AND projects.site_id = tasks.site_id"},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderJoinCondition(tt.joinOn, scope)
			if err != nil {
				t.Fatalf("renderJoinCondition(%q) error: %v", tt.joinOn, err)
			}
			if got != tt.expected {
				t.Errorf("renderJoinCondition(%q) = %q, want %q", tt.joinOn, got, tt.expected)
			}
		})
	}

	rejected := []struct {
		name   string
		joinOn string
	}{
		{"empty", "  "},
		{"subquery", "projects.id = (SELECT id FROM othertenant.users LIMIT 1)"},
		{"always true with OR", "projects.id = tasks.project_id OR 1=1"},
		{"statement break", "projects.id = tasks.project_id; DROP TABLE tasks"},
		{"comment", "projects.id = tasks.project_id --"},
		{"function call", "projects.id = pg_sleep(10)"},
		{"literal", "projects.id = 1"},
		{"non-equality", "projects.id > tasks.project_id"},
		{"alias not in scope", "projects.id = othertenant.project_id"},
		{"quoted alias with wrong case", `"Projects".id = tasks.project_id`},
		{"three-part name", "sreeugcl.projects.id = tasks.project_id"},
		{"unsupported cast", "projects.id::regclass = tasks.project_id"},
		{"quote breakout", `"projects"" OR ""x".id = tasks.project_id`},
	}
	for _, tt := range rejected {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			if got, err := renderJoinCondition(tt.joinOn, scope); err == nil {
				t.Errorf("renderJoinCondition(%q) accepted, rendered %q", tt.joinOn, got)
			}
		})
	}
}

func TestJoinKeyword(t *testing.T) {
	for input, want := range map[string]string{"": "LEFT JOIN", "left": "LEFT JOIN", "INNER": "INNER JOIN", "RIGHT": "RIGHT JOIN", "FULL": "FULL JOIN"} {
		if got, err := joinKeyword(input); err != nil || got != want {
			t.Errorf("joinKeyword(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"CROSS", "LEFT JOIN users ON true --", "NATURAL"} {
		if _, err := joinKeyword(input); err == nil {
			t.Errorf("joinKeyword(%q) accepted", input)
		}
	}
}

func TestAggregateExpr(t *testing.T) {
	for function, want := range map[string]string{
		"COUNT": `COUNT("t"."x")`, "sum": `SUM("t"."x")`, "Avg": `AVG("t"."x")`,
		"MIN": `MIN("t"."x")`, "MAX": `MAX("t"."x")`, "COUNT_DISTINCT": `COUNT(DISTINCT "t"."x")`,
	} {
		if got, err := aggregateExpr(function, `"t"."x"`); err != nil || got != want {
			t.Errorf("aggregateExpr(%q) = %q, %v; want %q", function, got, err, want)
		}
	}
	for _, function := range []string{"", "pg_sleep", "(SELECT password FROM users) || MAX", "COUNT(*) FROM users --"} {
		if _, err := aggregateExpr(function, `"t"."x"`); err == nil {
			t.Errorf("aggregateExpr(%q) accepted", function)
		}
	}
}

func TestPruneUnusedDataSources(t *testing.T) {
	aliases := func(sources []models.DataSource) []string {
		out := make([]string, 0, len(sources))
		for _, ds := range sources {
			out = append(out, ds.Alias)
		}
		return out
	}
	fieldsOn := func(alias string) []models.ReportField {
		return []models.ReportField{{FieldName: "x", DataSource: alias}}
	}

	tests := []struct {
		name    string
		sources []models.DataSource
		fields  []models.ReportField
		want    []string
	}{
		{
			"leftover base clicked before the real table is dropped",
			[]models.DataSource{{Alias: "site_visit"}, {Alias: "dairy_report", JoinType: "LEFT"}},
			fieldsOn("dairy_report"),
			[]string{"dairy_report"},
		},
		{
			"leftover extra table with no join is dropped",
			[]models.DataSource{{Alias: "dairy_report"}, {Alias: "site_visit", JoinType: "LEFT"}},
			fieldsOn("dairy_report"),
			[]string{"dairy_report"},
		},
		{
			"used source without a join is kept so the error surfaces",
			[]models.DataSource{{Alias: "projects"}, {Alias: "tasks"}},
			append(fieldsOn("projects"), fieldsOn("tasks")...),
			[]string{"projects", "tasks"},
		},
		{
			"base referenced only by a join condition is kept",
			[]models.DataSource{{Alias: "projects"}, {Alias: "tasks", JoinOn: "projects.id = tasks.project_id"}},
			fieldsOn("tasks"),
			[]string{"projects", "tasks"},
		},
		{
			"unreferenced source with a join condition is kept (it can filter rows)",
			[]models.DataSource{{Alias: "tasks"}, {Alias: "projects", JoinType: "INNER", JoinOn: "projects.id = tasks.project_id"}},
			fieldsOn("tasks"),
			[]string{"tasks", "projects"},
		},
		{
			"alias-less sources are never dropped",
			[]models.DataSource{{Alias: ""}, {Alias: "dairy_report"}},
			fieldsOn("dairy_report"),
			[]string{"", "dairy_report"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aliases(pruneUnusedDataSources(tt.sources, tt.fields, nil, nil, nil, nil))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("kept %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateDataSourceRefs(t *testing.T) {
	dataSources := []models.DataSource{{Alias: "dairy_report"}, {Alias: "sites"}}

	if err := validateDataSourceRefs(dataSources,
		[]models.ReportField{{FieldName: "milk_qty", DataSource: "dairy_report"}},
		[]models.ReportFilter{{FieldName: "site_name", DataSource: "sites"}},
		[]models.ReportGrouping{{FieldName: "site_name", DataSource: "dairy_report"}},
		[]models.ReportAggregation{{Function: "SUM", FieldName: "milk_qty", DataSource: "dairy_report"}},
		[]models.ReportSorting{{FieldName: "milk_qty"}},
	); err != nil {
		t.Fatalf("validateDataSourceRefs rejected a valid report: %v", err)
	}

	// The auto-join for site_name writes data_source into SQL, so a crafted
	// runtime filter must be rejected before it gets there.
	injected := "dairy_report.site_id OR true) LEFT JOIN othertenant.users u ON (true"
	cases := map[string]error{
		"field": validateDataSourceRefs(dataSources, []models.ReportField{{FieldName: "site_name", DataSource: injected}}, nil, nil, nil, nil),
		"filter": validateDataSourceRefs(dataSources, nil, []models.ReportFilter{{FieldName: "site_name", DataSource: injected}}, nil, nil, nil),
		"grouping": validateDataSourceRefs(dataSources, nil, nil, []models.ReportGrouping{{FieldName: "site_name", DataSource: injected}}, nil, nil),
		"aggregation": validateDataSourceRefs(dataSources, nil, nil, nil, []models.ReportAggregation{{Function: "COUNT", FieldName: "id", DataSource: injected}}, nil),
		"sort": validateDataSourceRefs(dataSources, nil, nil, nil, nil, []models.ReportSorting{{FieldName: "site_name", DataSource: injected}}),
		"empty filter source": validateDataSourceRefs(dataSources, nil, []models.ReportFilter{{FieldName: "submitted_by_name"}}, nil, nil, nil),
	}
	for name, err := range cases {
		if err == nil {
			t.Errorf("validateDataSourceRefs accepted an undeclared data source in %s", name)
		}
	}
}
