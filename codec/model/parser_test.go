package model_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/codec/model"
)

func TestParse(t *testing.T) {
	input := map[string]any{
		"$where": map[string]any{
			"$or": []any{
				map[string]any{"status": "active"},
				map[string]any{"status": "trial"},
			},
			"age": map[string]any{"$gte": json.Number("21")},
		},
		"$sort": "-created_at,name",
		"$size": "10",
	}

	query, err := model.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE (("status" = $1 OR "status" = $2) AND "age" >= $3) ORDER BY "created_at" DESC, "name" ASC LIMIT 10`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{"active", "trial", int64(21)}
	assertArgsEqual(t, statement.Args, wantArgs)
}

func TestParseWithValueDecoder(t *testing.T) {
	input := map[string]any{
		"$where": map[string]any{
			"age": map[string]any{"$gte": "21"},
		},
	}

	query, err := model.Parse(input, model.WithValueDecoder(func(field string, _ qb.Operator, value any) (any, error) {
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

	assertArgsEqual(t, statement.Args, []any{21})
}

func TestParseReturnsStructuredError(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$size": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected parse error")
	}

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected qb.Error, got %T", err)
	}

	if diagnostic.Stage != qb.StageParse || diagnostic.Code != qb.CodeInvalidValue || diagnostic.Path != "$size" {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}
}

func TestParseSelectIncludeGroupAndPageSize(t *testing.T) {
	input := map[string]any{
		"$select":  "id,status",
		"$include": []any{"Customer", "Orders.Items"},
		"$group":   "status",
		"$where": map[string]any{
			"status": "active",
		},
		"$sort": "-created_at",
		"$page": "3",
		"$size": json.Number("25"),
	}

	query, err := model.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.Projections) != 2 || projectionRefName(query.Projections[0]) != "id" || projectionRefName(query.Projections[1]) != "status" {
		t.Fatalf("unexpected projections: %#v", query.Projections)
	}

	if len(query.Includes) != 2 || query.Includes[0] != "Customer" || query.Includes[1] != "Orders.Items" {
		t.Fatalf("unexpected includes: %#v", query.Includes)
	}

	if len(query.GroupBy) != 1 || refName(query.GroupBy[0]) != "status" {
		t.Fatalf("unexpected group: %#v", query.GroupBy)
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

func TestParseGroupUsesDedicatedGroupResolver(t *testing.T) {
	query, err := model.Parse(
		map[string]any{
			"$group": []any{"state", "lower(state)"},
		},
		model.WithGroupFieldResolver(func(field string) (string, error) {
			switch field {
			case "state":
				return "status", nil
			default:
				return "", errors.New("unknown group field")
			}
		}),
	)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.GroupBy) != 2 {
		t.Fatalf("unexpected group expressions: %#v", query.GroupBy)
	}

	if refName(query.GroupBy[0]) != "status" {
		t.Fatalf("unexpected grouped field: %#v", query.GroupBy[0])
	}

	call, ok := query.GroupBy[1].(qb.Call)
	if !ok || len(call.Args) != 1 || refName(call.Args[0]) != "status" {
		t.Fatalf("unexpected grouped call: %#v", query.GroupBy[1])
	}
}

func TestParseDSLExpressions(t *testing.T) {
	input := map[string]any{
		"$select": []any{
			"lower(users.name) as normalized_name",
			"round(users.age::decimal, 2) as rounded_age",
			"round_double(users.score::double, 2) as rounded_score",
			"users.age",
		},
		"$group": []any{
			"lower(users.name)",
			"users.age::decimal",
			"users.score::double",
		},
		"$sort": []any{
			"lower(users.name) asc",
			"users.age::decimal desc",
		},
	}

	query, err := model.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.Projections) != 4 {
		t.Fatalf("unexpected projections: %#v", query.Projections)
	}
	if query.Projections[0].Alias != "normalized_name" || query.Projections[1].Alias != "rounded_age" {
		t.Fatalf("unexpected aliases: %#v", query.Projections)
	}
	if _, ok := query.Projections[0].Expr.(qb.Call); !ok {
		t.Fatalf("expected function projection, got %T", query.Projections[0].Expr)
	}
	if _, ok := query.Projections[1].Expr.(qb.Call); !ok {
		t.Fatalf("expected function projection, got %T", query.Projections[1].Expr)
	}
	if _, ok := query.GroupBy[1].(qb.Cast); !ok {
		t.Fatalf("expected cast group expression, got %T", query.GroupBy[1])
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT LOWER("users"."name") AS "normalized_name", ROUND(CAST("users"."age" AS NUMERIC), $1) AS "rounded_age", CAST(ROUND(CAST(CAST("users"."score" AS DOUBLE PRECISION) AS NUMERIC), $2) AS DOUBLE PRECISION) AS "rounded_score", "users"."age" GROUP BY LOWER("users"."name"), CAST("users"."age" AS NUMERIC), CAST("users"."score" AS DOUBLE PRECISION) ORDER BY LOWER("users"."name") ASC, CAST("users"."age" AS NUMERIC) DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	assertArgsEqual(t, statement.Args, []any{int64(2), int64(2)})
}

func TestParseExpressionPredicates(t *testing.T) {
	input := map[string]any{
		"$where": map[string]any{
			"$expr": map[string]any{
				"$eq": []any{"lower(@users.name)", "lower('JOHN')"},
				"$gte": []any{
					"round(@users.age::decimal, 2)",
					18,
				},
				"$lte": []any{
					"round_double(@users.score::double, 2)",
					100,
				},
			},
		},
	}

	query, err := model.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE (LOWER("users"."name") = LOWER($1) AND ROUND(CAST("users"."age" AS NUMERIC), $2) >= $3 AND CAST(ROUND(CAST(CAST("users"."score" AS DOUBLE PRECISION) AS NUMERIC), $4) AS DOUBLE PRECISION) <= $5)`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	assertArgsEqual(t, statement.Args, []any{"JOHN", int64(2), 18, int64(2), 100})
}

func TestParseCursor(t *testing.T) {
	query, err := model.Parse(map[string]any{
		"$cursor": map[string]any{
			"created_at": "2026-04-11T12:00:00Z",
			"id":         json.Number("981"),
		},
		"$size": "25",
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

func TestParseRejectsUnknownTopLevelKey(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$select": "id",
		"$pick":   "status",
	})
	if err == nil {
		t.Fatal("expected unknown top-level key error")
	}
}

func TestParseRejectsExpressionBearingStringSelect(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$select": "lower(users.name) as normalized_name,users.age",
	})
	if err == nil {
		t.Fatal("expected expression shorthand rejection")
	}
}

func TestParseRejectsAliasesInGroup(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$group": []any{"lower(users.name) as normalized_name"},
	})
	if err == nil {
		t.Fatal("expected alias rejection in $group")
	}
}

func TestParseRejectsCursorWithoutSize(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$cursor": "opaque-cursor",
	})
	if err == nil {
		t.Fatal("expected cursor without size error")
	}
}

func TestParseRejectsInvalidIsNullOperand(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$where": map[string]any{
			"deleted_at": map[string]any{"$isnull": false},
		},
	})
	if err == nil {
		t.Fatal("expected invalid $isnull operand error")
	}
}

func TestParseRejectsInvalidExprIsNullOperandCount(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$where": map[string]any{
			"$expr": map[string]any{
				"$isnull": []any{"@users.deleted_at", "@users.archived_at"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected invalid $expr $isnull operand count error")
	}
}

func TestParseRejectsUnknownProjectionWrapperKey(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$select": []any{
			map[string]any{
				"$expr": "users.id",
				"$as":   "id",
				"$junk": true,
			},
		},
	})
	if err == nil {
		t.Fatal("expected unknown projection wrapper key error")
	}
}

func TestParseRejectsUnknownSortWrapperKey(t *testing.T) {
	_, err := model.Parse(map[string]any{
		"$sort": []any{
			map[string]any{
				"$expr": "users.name",
				"$dir":  "asc",
				"$junk": true,
			},
		},
	})
	if err == nil {
		t.Fatal("expected unknown sort wrapper key error")
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
