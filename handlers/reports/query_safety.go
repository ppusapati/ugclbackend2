package reports

import (
	"fmt"
	"regexp"
	"strings"

	"p9e.in/ugcl/models"
)

// Report definitions (and the runtime filters sent alongside them) are
// user-supplied JSON, so no value from them may reach the SQL text verbatim.
// The helpers below turn the free-form pieces — aggregate names, join types,
// join conditions, data source references — into SQL only after checking them
// against a fixed grammar or whitelist.

// aggregateExpr returns a whitelisted aggregate over ref.
func aggregateExpr(function, ref string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(function)) {
	case "COUNT":
		return "COUNT(" + ref + ")", nil
	case "COUNT_DISTINCT":
		return "COUNT(DISTINCT " + ref + ")", nil
	case "SUM":
		return "SUM(" + ref + ")", nil
	case "AVG":
		return "AVG(" + ref + ")", nil
	case "MIN":
		return "MIN(" + ref + ")", nil
	case "MAX":
		return "MAX(" + ref + ")", nil
	}
	return "", fmt.Errorf("unsupported aggregate function %q", function)
}

// joinKeyword maps a data source's join_type to SQL; empty means LEFT.
func joinKeyword(joinType string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(joinType)) {
	case "", "LEFT":
		return "LEFT JOIN", nil
	case "INNER":
		return "INNER JOIN", nil
	case "RIGHT":
		return "RIGHT JOIN", nil
	case "FULL":
		return "FULL JOIN", nil
	}
	return "", fmt.Errorf("unsupported join type %q", joinType)
}

// joinIdent matches one identifier, either "quoted" (case kept) or bare
// (folded to lower case by Postgres). Only letters, digits and underscores
// are allowed either way, so a matched identifier cannot carry SQL.
const joinIdent = `(?:"([A-Za-z_][A-Za-z0-9_]*)"|([A-Za-z_][A-Za-z0-9_]*))`

// joinOperandPattern matches alias.column with an optional ::type cast.
var joinOperandPattern = regexp.MustCompile(`^` + joinIdent + `\s*\.\s*` + joinIdent + `(?:\s*::\s*([A-Za-z]+))?$`)

var joinConditionSeparator = regexp.MustCompile(`(?i)\s+AND\s+`)

// joinCasts are the types a join operand may be cast to, so text columns from
// form views can be compared with uuid/numeric keys.
var joinCasts = map[string]bool{
	"text": true, "varchar": true, "uuid": true, "int": true, "integer": true, "bigint": true,
	"numeric": true, "date": true, "timestamp": true, "timestamptz": true, "boolean": true,
}

// renderJoinCondition rebuilds a join_on string such as
// "projects.id = tasks.project_id AND projects.site_id = tasks.site_id" from
// validated parts. Each condition must be an equality between two
// alias.column operands whose aliases are in scope (this data source or one
// joined before it); anything else is rejected.
func renderJoinCondition(joinOn string, inScope map[string]bool) (string, error) {
	joinOn = trimOuterParens(joinOn)
	if joinOn == "" {
		return "", fmt.Errorf("join condition is required: add one in the report builder (e.g. base.id = joined.base_id) or remove this data source")
	}

	conditions := joinConditionSeparator.Split(joinOn, -1)
	rendered := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		condition = trimOuterParens(condition)

		sides := strings.Split(condition, "=")
		if len(sides) != 2 {
			return "", fmt.Errorf("join condition %q must be an equality like a.id = b.a_id", condition)
		}
		left, err := renderJoinOperand(sides[0], inScope)
		if err != nil {
			return "", err
		}
		right, err := renderJoinOperand(sides[1], inScope)
		if err != nil {
			return "", err
		}
		rendered = append(rendered, left+" = "+right)
	}
	return strings.Join(rendered, " AND "), nil
}

// trimOuterParens removes one pair of parentheses only when they enclose the
// whole value, so "(a = b) AND (c = d)" is left intact.
func trimOuterParens(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return value
	}
	depth := 0
	for i, r := range value {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(value)-1 {
				return value
			}
		}
	}
	return strings.TrimSpace(value[1 : len(value)-1])
}

func renderJoinOperand(operand string, inScope map[string]bool) (string, error) {
	operand = strings.TrimSpace(operand)
	match := joinOperandPattern.FindStringSubmatch(operand)
	if match == nil {
		return "", fmt.Errorf("join operand %q must look like alias.column", operand)
	}

	alias, aliasKey := joinIdentifierSQL(match[1], match[2])
	if !inScope[aliasKey] {
		return "", fmt.Errorf("join operand %q refers to data source %q, which is not joined at this point", operand, aliasKey)
	}
	column, _ := joinIdentifierSQL(match[3], match[4])

	sql := alias + "." + column
	if cast := strings.ToLower(match[5]); cast != "" {
		if !joinCasts[cast] {
			return "", fmt.Errorf("join operand %q uses unsupported cast %q", operand, cast)
		}
		sql += "::" + cast
	}
	return sql, nil
}

// joinIdentifierSQL renders an identifier the way the user wrote it — quoted
// stays quoted, bare stays bare — and returns the name Postgres resolves it to.
func joinIdentifierSQL(quoted, bare string) (sql string, resolved string) {
	if quoted != "" {
		return `"` + quoted + `"`, quoted
	}
	return bare, strings.ToLower(bare)
}

// joinAliasPattern finds the alias part of each alias.column operand.
var joinAliasPattern = regexp.MustCompile(joinIdent + `\s*\.`)

// joinConditionAliases returns the aliases a join condition refers to, as
// Postgres resolves them (bare names folded to lower case).
func joinConditionAliases(joinOn string) []string {
	var aliases []string
	for _, match := range joinAliasPattern.FindAllStringSubmatch(joinOn, -1) {
		_, resolved := joinIdentifierSQL(match[1], match[2])
		aliases = append(aliases, resolved)
	}
	return aliases
}

// pruneUnusedDataSources drops data sources that cannot affect the result:
// nothing in the report reads from them, no join condition refers to them,
// and (unless they are the base) they have no join condition of their own.
// The report builder appends every table the user clicks as another source
// with an empty join, so saved reports often carry such leftovers, and joining
// them would need a condition nobody wrote. Sources with a join condition are
// always kept, since an INNER or one-to-many join changes the rows returned.
func pruneUnusedDataSources(
	dataSources []models.DataSource,
	fields []models.ReportField,
	filters []models.ReportFilter,
	groupings []models.ReportGrouping,
	aggregations []models.ReportAggregation,
	sortings []models.ReportSorting,
) []models.DataSource {
	used := make(map[string]bool)
	markUsed := func(dataSource string) {
		if dataSource != "" {
			used[strings.ToLower(dataSource)] = true
		}
	}
	for _, field := range fields {
		markUsed(field.DataSource)
	}
	for _, filter := range filters {
		markUsed(filter.DataSource)
	}
	for _, group := range groupings {
		markUsed(group.DataSource)
	}
	for _, agg := range aggregations {
		markUsed(agg.DataSource)
	}
	for _, sort := range sortings {
		markUsed(sort.DataSource)
	}
	for _, ds := range dataSources {
		for _, alias := range joinConditionAliases(ds.JoinOn) {
			markUsed(alias)
		}
	}

	kept := make([]models.DataSource, 0, len(dataSources))
	for i, ds := range dataSources {
		hasJoin := i > 0 && strings.TrimSpace(ds.JoinOn) != ""
		// Alias-less sources get positional default aliases (t1, t2, ...) later,
		// so removing any of them would shift what those names point at.
		unnamed := strings.TrimSpace(ds.Alias) == ""
		if unnamed || used[strings.ToLower(ds.Alias)] || hasJoin {
			kept = append(kept, ds)
		}
	}
	if len(kept) == 0 {
		// Nothing references any source; keep them so the usual errors surface.
		return dataSources
	}
	return kept
}

// validateDataSourceRefs ensures every field, filter, grouping, aggregation
// and sort refers to a declared data source alias. Beyond catching typos, this
// keeps data_source values out of the SQL text: they become table qualifiers
// and are written into the auto-added joins in collectRequiredJoins. Sorts may
// leave data_source empty to order by an output column.
func validateDataSourceRefs(
	dataSources []models.DataSource,
	fields []models.ReportField,
	filters []models.ReportFilter,
	groupings []models.ReportGrouping,
	aggregations []models.ReportAggregation,
	sortings []models.ReportSorting,
) error {
	declared := make(map[string]bool, len(dataSources))
	for _, ds := range dataSources {
		declared[ds.Alias] = true
	}
	check := func(kind, fieldName, dataSource string) error {
		if !declared[dataSource] {
			return fmt.Errorf("%s %q refers to unknown data source %q", kind, fieldName, dataSource)
		}
		return nil
	}

	for _, field := range fields {
		if err := check("field", field.FieldName, field.DataSource); err != nil {
			return err
		}
	}
	for _, filter := range filters {
		if err := check("filter", filter.FieldName, filter.DataSource); err != nil {
			return err
		}
	}
	for _, group := range groupings {
		if err := check("grouping", group.FieldName, group.DataSource); err != nil {
			return err
		}
	}
	for _, agg := range aggregations {
		if err := check("aggregation", agg.FieldName, agg.DataSource); err != nil {
			return err
		}
	}
	for _, sort := range sortings {
		if sort.DataSource == "" {
			continue
		}
		if err := check("sort", sort.FieldName, sort.DataSource); err != nil {
			return err
		}
	}
	return nil
}
