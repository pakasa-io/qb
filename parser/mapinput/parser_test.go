package mapinput_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/mapinput"
)

func TestParse(t *testing.T) {
	input := map[string]any{
		"where": map[string]any{
			"$or": []any{
				map[string]any{"role": "admin"},
				map[string]any{"role": "owner"},
			},
			"age":    map[string]any{"$gte": json.Number("18")},
			"status": "active",
		},
		"sort":   []any{"-created_at", "name"},
		"limit":  "20",
		"offset": json.Number("40"),
	}

	query, err := mapinput.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE (("role" = $1 OR "role" = $2) AND "age" >= $3 AND "status" = $4) ORDER BY "created_at" DESC, "name" ASC LIMIT 20 OFFSET 40`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{"admin", "owner", int64(18), "active"}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
	}
}

func TestParseWithValueDecoder(t *testing.T) {
	input := map[string]any{
		"where": map[string]any{
			"age": map[string]any{"$gte": "21"},
		},
	}

	query, err := mapinput.Parse(input, mapinput.WithValueDecoder(func(field string, _ qb.Operator, value any) (any, error) {
		if field != "age" {
			return value, nil
		}
		return strconv.Atoi(value.(string))
	}))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if len(statement.Args) != 1 || statement.Args[0] != 21 {
		t.Fatalf("unexpected args: %#v", statement.Args)
	}
}

func TestParseReturnsStructuredError(t *testing.T) {
	_, err := mapinput.Parse(map[string]any{
		"limit": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected parse error")
	}

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected qb.Error, got %T", err)
	}

	if diagnostic.Stage != qb.StageParse || diagnostic.Code != qb.CodeInvalidValue || diagnostic.Path != "limit" {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}
}

func TestParseSelectIncludeGroupByAndPageSize(t *testing.T) {
	input := map[string]any{
		"pick":     "id,status",
		"include":  []any{"Customer", "Orders.Items"},
		"group_by": []any{"status"},
		"where": map[string]any{
			"status": "active",
		},
		"sort": []any{"-created_at"},
		"page": "3",
		"size": json.Number("25"),
	}

	query, err := mapinput.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.Projections) != 2 || projectionRefName(query.Projections[0]) != "id" || projectionRefName(query.Projections[1]) != "status" {
		t.Fatalf("unexpected selects: %#v", query.Projections)
	}

	if len(query.Includes) != 2 || query.Includes[0] != "Customer" || query.Includes[1] != "Orders.Items" {
		t.Fatalf("unexpected includes: %#v", query.Includes)
	}

	if len(query.GroupBy) != 1 || refName(query.GroupBy[0]) != "status" {
		t.Fatalf("unexpected group_by: %#v", query.GroupBy)
	}

	limit, offset, err := query.ResolvedPagination()
	if err != nil {
		t.Fatalf("ResolvedPagination() error = %v", err)
	}

	if limit == nil || *limit != 25 {
		t.Fatalf("unexpected resolved limit: %v", limit)
	}

	if offset == nil || *offset != 50 {
		t.Fatalf("unexpected resolved offset: %v", offset)
	}
}

func TestParseStructuredProjectionAliases(t *testing.T) {
	input := map[string]any{
		"select": []any{
			map[string]any{
				"$as": "normalized_name",
				"$expr": map[string]any{
					"$call": "lower",
					"args": []any{
						map[string]any{"$field": "users.name"},
					},
				},
			},
			"users.age",
		},
		"group_by": []any{
			map[string]any{
				"$call": "lower",
				"args": []any{
					map[string]any{"$field": "users.name"},
				},
			},
		},
	}

	query, err := mapinput.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.Projections) != 2 {
		t.Fatalf("unexpected projections: %#v", query.Projections)
	}

	if query.Projections[0].Alias != "normalized_name" {
		t.Fatalf("unexpected projection alias: %#v", query.Projections[0])
	}

	if _, ok := query.Projections[0].Expr.(qb.Call); !ok {
		t.Fatalf("expected function projection, got %T", query.Projections[0].Expr)
	}

	if len(query.GroupBy) != 1 {
		t.Fatalf("unexpected group_by: %#v", query.GroupBy)
	}

	if _, ok := query.GroupBy[0].(qb.Call); !ok {
		t.Fatalf("expected function group_by expression, got %T", query.GroupBy[0])
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT LOWER("users"."name") AS "normalized_name", "users"."age" GROUP BY LOWER("users"."name")`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}
}

func TestParseCursor(t *testing.T) {
	query, err := mapinput.Parse(map[string]any{
		"cursor": map[string]any{
			"created_at": "2026-04-11T12:00:00Z",
			"id":         json.Number("981"),
		},
		"size": "25",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if query.Cursor == nil {
		t.Fatal("expected cursor to be set")
	}

	if got := query.Cursor.Values["id"]; got != int64(981) {
		t.Fatalf("unexpected cursor value: %#v", got)
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

func TestParseRejectsSelectAndPickTogether(t *testing.T) {
	_, err := mapinput.Parse(map[string]any{
		"select": "id",
		"pick":   "status",
	})
	if err == nil {
		t.Fatal("expected select/pick conflict error")
	}
}

func TestParseRejectsCursorWithoutSize(t *testing.T) {
	_, err := mapinput.Parse(map[string]any{
		"cursor": "opaque-cursor",
	})
	if err == nil {
		t.Fatal("expected cursor without size error")
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
