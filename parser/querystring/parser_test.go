package querystring_test

import (
	"net/url"
	"strconv"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/mapinput"
	"github.com/pakasa-io/qb/parser/querystring"
)

func TestParse(t *testing.T) {
	values := url.Values{
		"where[$or][0][status][$eq]": {"active"},
		"where[$or][1][status][$eq]": {"trial"},
		"where[age][$gte]":           {"21"},
		"sort":                       {"-created_at,name"},
		"limit":                      {"10"},
	}

	query, err := querystring.Parse(values, mapinput.WithValueDecoder(func(field string, op qb.Operator, value any) (any, error) {
		if field == "age" && op == qb.OpGte {
			return strconv.Atoi(value.(string))
		}
		return value, nil
	}))
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

	wantArgs := []any{"active", "trial", 21}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
	}
}

func TestParseTopLevelConstructs(t *testing.T) {
	values := url.Values{
		"pick":     {"id,status"},
		"include":  {"Customer,Orders"},
		"group_by": {"status"},
		"page":     {"2"},
		"size":     {"10"},
	}

	query, err := querystring.Parse(values)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.Projections) != 2 || projectionRefName(query.Projections[0]) != "id" || projectionRefName(query.Projections[1]) != "status" {
		t.Fatalf("unexpected selects: %#v", query.Projections)
	}

	if len(query.Includes) != 2 || query.Includes[0] != "Customer" || query.Includes[1] != "Orders" {
		t.Fatalf("unexpected includes: %#v", query.Includes)
	}

	if len(query.GroupBy) != 1 || refName(query.GroupBy[0]) != "status" {
		t.Fatalf("unexpected group_by: %#v", query.GroupBy)
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

func TestParseCursor(t *testing.T) {
	values := url.Values{
		"cursor[created_at]": {"2026-04-11T12:00:00Z"},
		"cursor[id]":         {"981"},
		"size":               {"25"},
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
