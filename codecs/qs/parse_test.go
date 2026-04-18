package querystring_test

import (
	"net/url"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
	"github.com/pakasa-io/qb/codecs/querystring"
)

func TestParse(t *testing.T) {
	values := url.Values{
		"$where[$or][0][status]": {"active"},
		"$where[$or][1][status]": {"trial"},
		"$where[age][$gte]":      {"21"},
		"$sort":                  {"-created_at,name"},
		"$size":                  {"10"},
	}

	query, err := querystring.Parse(values)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.DollarDialect{})).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE (("status" = $1 OR "status" = $2) AND "age" >= $3) ORDER BY "created_at" DESC, "name" ASC LIMIT 10`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	assertArgsEqual(t, statement.Args, []any{"active", "trial", "21"})
}

func TestParseTopLevelConstructs(t *testing.T) {
	values := url.Values{
		"$select":  {"id,status"},
		"$include": {"Customer,Orders"},
		"$group":   {"status"},
		"$page":    {"2"},
		"$size":    {"10"},
	}

	query, err := querystring.Parse(values)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.Projections) != 2 || projectionRefName(query.Projections[0]) != "id" || projectionRefName(query.Projections[1]) != "status" {
		t.Fatalf("unexpected projections: %#v", query.Projections)
	}

	if len(query.Includes) != 2 || query.Includes[0] != "Customer" || query.Includes[1] != "Orders" {
		t.Fatalf("unexpected includes: %#v", query.Includes)
	}

	if len(query.GroupBy) != 1 || refName(query.GroupBy[0]) != "status" {
		t.Fatalf("unexpected group: %#v", query.GroupBy)
	}

	limit, offset, err := query.ResolvedPagination()
	if err != nil {
		t.Fatalf("ResolvedPagination() error = %v", err)
	}

	if limit == nil || *limit != 10 {
		t.Fatalf("unexpected resolved limit: %v", limit)
	}

	if offset == nil || *offset != 10 {
		t.Fatalf("unexpected resolved offset: %v", offset)
	}
}

func TestParseExpressionListsAndExpressionFilters(t *testing.T) {
	values := url.Values{
		"$select[0]":             {"lower(users.name) as normalized_name"},
		"$select[1]":             {"users.age"},
		"$group[0]":              {"lower(users.name)"},
		"$sort[0]":               {"lower(users.name) asc"},
		"$where[$expr][$eq][0]":  {"lower(@users.name)"},
		"$where[$expr][$eq][1]":  {"lower('john')"},
		"$where[$expr][$gte][0]": {"round(@users.age::decimal, 2)"},
		"$where[$expr][$gte][1]": {"18"},
		"$where[$expr][$lte][0]": {"round_double(@users.score::double, 2)"},
		"$where[$expr][$lte][1]": {"100"},
	}

	query, err := querystring.Parse(values)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.Projections) != 2 {
		t.Fatalf("unexpected projections: %#v", query.Projections)
	}
	if query.Projections[0].Alias != "normalized_name" {
		t.Fatalf("unexpected alias: %#v", query.Projections[0])
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.DollarDialect{})).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT LOWER("users"."name") AS "normalized_name", "users"."age" WHERE (LOWER("users"."name") = LOWER($1) AND ROUND(CAST("users"."age" AS NUMERIC), $2) >= $3 AND CAST(ROUND(CAST(CAST("users"."score" AS DOUBLE PRECISION) AS NUMERIC), $4) AS DOUBLE PRECISION) <= $5) GROUP BY LOWER("users"."name") ORDER BY LOWER("users"."name") ASC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	assertArgsEqual(t, statement.Args, []any{"john", int64(2), "18", int64(2), "100"})
}

func TestParseCursor(t *testing.T) {
	values := url.Values{
		"$cursor[created_at]": {"2026-04-11T12:00:00Z"},
		"$cursor[id]":         {"981"},
		"$size":               {"25"},
	}

	query, err := querystring.Parse(values)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if query.Cursor == nil {
		t.Fatal("expected cursor to be set")
	}

	if got := query.Cursor.Values["id"]; got != "981" {
		t.Fatalf("unexpected cursor id: %#v", got)
	}

	limit, offset, err := query.ResolvedPagination()
	if err != nil {
		t.Fatalf("ResolvedPagination() error = %v", err)
	}

	if limit == nil || *limit != 25 {
		t.Fatalf("unexpected resolved limit: %v", limit)
	}

	if offset != nil {
		t.Fatalf("expected nil offset, got %v", offset)
	}
}

func TestParseRejectsExpressionBearingStringSelect(t *testing.T) {
	values := url.Values{
		"$select": {"lower(users.name) as normalized_name,users.age"},
	}

	if _, err := querystring.Parse(values); err == nil {
		t.Fatal("expected expression shorthand rejection")
	}
}

func TestParseRejectsSparseArrays(t *testing.T) {
	values := url.Values{
		"$select[1]": {"users.age"},
	}

	if _, err := querystring.Parse(values); err == nil {
		t.Fatal("expected sparse array rejection")
	}
}

func TestParseRejectsInvalidIsNullOperand(t *testing.T) {
	values := url.Values{
		"$where[deleted_at][$isnull]": {"false"},
	}

	if _, err := querystring.Parse(values); err == nil {
		t.Fatal("expected invalid $isnull operand error")
	}
}

func refName(expr qb.Scalar) string {
	ref, ok := expr.(qb.Ref)
	if !ok {
		return ""
	}
	return ref.Name
}

func projectionRefName(projection qb.Projection) string {
	return refName(projection.Expr)
}

func assertArgsEqual(t *testing.T, got []any, want []any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, want[i], got[i])
		}
	}
}
