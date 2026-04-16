package schema_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/codec/model"
	"github.com/pakasa-io/qb/schema"
)

func TestSchemaDrivenParsingAndCompilation(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define(
			"status",
			schema.Storage("users.status"),
			schema.Aliases("state"),
			schema.Operators(qb.OpEq, qb.OpIn),
		),
		schema.Define(
			"age",
			schema.Storage("users.age"),
			schema.Aliases("minAge"),
			schema.Operators(qb.OpEq, qb.OpGte, qb.OpLte),
			schema.Decode(func(_ qb.Operator, value any) (any, error) {
				switch typed := value.(type) {
				case string:
					return strconv.Atoi(typed)
				default:
					return value, nil
				}
			}),
		),
		schema.Define(
			"created_at",
			schema.Storage("users.created_at"),
			schema.Aliases("createdAt"),
			schema.Sortable(),
			schema.DisableFiltering(),
		),
	)

	query, err := model.Parse(
		map[string]any{
			"$where": map[string]any{
				"state":  "active",
				"minAge": map[string]any{"$gte": "21"},
			},
			"$sort": "-createdAt",
		},
		model.WithFilterFieldResolver(userSchema.ResolveFilterField),
		model.WithSortFieldResolver(userSchema.ResolveSortField),
		model.WithValueDecoder(userSchema.DecodeValue),
	)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(userSchema.ToStorage),
	).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE ("users"."age" >= $1 AND "users"."status" = $2) ORDER BY "users"."created_at" DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	assertArgsEqual(t, statement.Args, []any{21, "active"})
}

func TestSchemaDrivenGroupingDoesNotRequireSortableFields(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define("status", schema.Storage("users.status"), schema.Aliases("state")),
		schema.Define("role", schema.Storage("users.role")),
	)

	query, err := model.Parse(
		map[string]any{
			"$select": "state,role",
			"$group":  "state,role",
		},
		model.WithGroupFieldResolver(userSchema.ResolveGroupField),
	)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	projected, err := userSchema.ToStorage(query)
	if err != nil {
		t.Fatalf("ToStorage() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(projected)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT "users"."status", "users"."role" GROUP BY "users"."status", "users"."role"`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}
}

func TestNormalizeBuilderQuery(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define("status", schema.Aliases("state"), schema.Sortable()),
		schema.Define("created_at", schema.Aliases("createdAt"), schema.Sortable()),
	)

	query, err := qb.New().
		Select("state").
		GroupBy("state").
		Where(qb.Field("state").Eq("active")).
		SortBy("createdAt", qb.Desc).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	normalized, err := userSchema.Normalize(query)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(normalized)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT "status" WHERE "status" = $1 GROUP BY "status" ORDER BY "created_at" DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}
}

func TestNormalizeRejectsUnsupportedOperator(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define("age", schema.Operators(qb.OpEq, qb.OpGte)),
	)

	query, err := qb.New().
		Where(qb.Field("age").Lt(18)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if _, err := userSchema.Normalize(query); err == nil {
		t.Fatal("expected unsupported operator error")
	}
}

func TestNormalizeDecodesBuilderValue(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define(
			"age",
			schema.Operators(qb.OpEq, qb.OpGte),
			schema.Decode(func(_ qb.Operator, value any) (any, error) {
				switch typed := value.(type) {
				case string:
					return strconv.Atoi(typed)
				default:
					return value, nil
				}
			}),
		),
	)

	query, err := qb.New().
		Where(qb.Field("age").Gte("21")).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	normalized, err := userSchema.Normalize(query)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(normalized)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	assertArgsEqual(t, statement.Args, []any{21})
}

func TestNormalizeCursorAppliesAliasesAndDecoding(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define(
			"created_at",
			schema.Aliases("createdAt"),
			schema.Decode(func(_ qb.Operator, value any) (any, error) {
				switch typed := value.(type) {
				case string:
					return "parsed:" + typed, nil
				default:
					return value, nil
				}
			}),
		),
		schema.Define(
			"id",
			schema.Decode(func(_ qb.Operator, value any) (any, error) {
				switch typed := value.(type) {
				case string:
					return strconv.Atoi(typed)
				default:
					return value, nil
				}
			}),
		),
	)

	query, err := qb.New().
		Size(25).
		CursorValues(map[string]any{
			"createdAt": "2026-04-11T12:00:00Z",
			"id":        "981",
		}).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	normalized, err := userSchema.Normalize(query)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if normalized.Cursor == nil {
		t.Fatal("expected normalized cursor")
	}

	if _, ok := normalized.Cursor.Values["createdAt"]; ok {
		t.Fatalf("expected cursor alias to be normalized away: %#v", normalized.Cursor.Values)
	}

	if got := normalized.Cursor.Values["created_at"]; got != "parsed:2026-04-11T12:00:00Z" {
		t.Fatalf("unexpected created_at cursor value: %#v", got)
	}

	if got := normalized.Cursor.Values["id"]; got != 981 {
		t.Fatalf("unexpected id cursor value: %#v", got)
	}
}

func TestToStorageMapsCanonicalFields(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define("status", schema.Storage("users.status"), schema.Sortable()),
		schema.Define("created_at", schema.Storage("users.created_at"), schema.Sortable()),
	)

	query, err := qb.New().
		Select("status").
		GroupBy("status").
		Where(qb.Field("status").Eq("active")).
		SortBy("created_at", qb.Desc).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	projected, err := userSchema.ToStorage(query)
	if err != nil {
		t.Fatalf("ToStorage() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(projected)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT "users"."status" WHERE "users"."status" = $1 GROUP BY "users"."status" ORDER BY "users"."created_at" DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}
}

func TestToStorageProjectsStructuredCursorFields(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define("created_at", schema.Storage("users.created_at"), schema.Aliases("createdAt")),
		schema.Define("id", schema.Storage("users.id")),
	)

	query, err := qb.New().
		Size(25).
		CursorValues(map[string]any{
			"createdAt": "2026-04-11T12:00:00Z",
			"id":        981,
		}).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	projected, err := userSchema.ToStorage(query)
	if err != nil {
		t.Fatalf("ToStorage() error = %v", err)
	}

	if projected.Cursor == nil {
		t.Fatal("expected projected cursor")
	}

	if _, ok := projected.Cursor.Values["created_at"]; ok {
		t.Fatalf("expected canonical cursor key to be projected away: %#v", projected.Cursor.Values)
	}

	if got := projected.Cursor.Values["users.created_at"]; got != "2026-04-11T12:00:00Z" {
		t.Fatalf("unexpected storage cursor value: %#v", got)
	}

	if got := projected.Cursor.Values["users.id"]; got != 981 {
		t.Fatalf("unexpected storage cursor id: %#v", got)
	}
}

func TestNormalizeReturnsStructuredError(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define("age", schema.Operators(qb.OpEq)),
	)

	query, err := qb.New().
		Where(qb.Field("age").Gt(21)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	_, err = userSchema.Normalize(query)
	if err == nil {
		t.Fatal("expected normalize error")
	}

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected qb.Error, got %T", err)
	}

	if diagnostic.Stage != qb.StageNormalize || diagnostic.Code != qb.CodeUnsupportedOperator {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}
}

func TestToStorageRewritesFunctionExpressions(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define("name", schema.Storage("users.name"), schema.Sortable()),
		schema.Define("age", schema.Storage("users.age"), schema.Sortable()),
	)

	query, err := qb.New().
		SelectProjection(
			qb.F("name").Lower().As("normalized_name"),
			qb.Round(qb.F("age").Cast("decimal"), 2).As("rounded_age"),
			qb.RoundDouble(qb.F("age").Cast("double"), 2).As("rounded_age_double"),
		).
		GroupByExpr(qb.Lower(qb.F("name")), qb.F("age").Cast("decimal"), qb.F("age").Cast("double")).
		SortByExpr(qb.F("age").Cast("double"), qb.Desc).
		Where(qb.And(
			qb.Lower(qb.F("name")).Eq("john"),
			qb.F("age").Cast("decimal").Gte(18),
		)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	projected, err := userSchema.ToStorage(query)
	if err != nil {
		t.Fatalf("ToStorage() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(projected)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT LOWER("users"."name") AS "normalized_name", ROUND(CAST("users"."age" AS NUMERIC), $1) AS "rounded_age", CAST(ROUND(CAST(CAST("users"."age" AS DOUBLE PRECISION) AS NUMERIC), $2) AS DOUBLE PRECISION) AS "rounded_age_double" WHERE (LOWER("users"."name") = $3 AND CAST("users"."age" AS NUMERIC) >= $4) GROUP BY LOWER("users"."name"), CAST("users"."age" AS NUMERIC), CAST("users"."age" AS DOUBLE PRECISION) ORDER BY CAST("users"."age" AS DOUBLE PRECISION) DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	assertArgsEqual(t, statement.Args, []any{2, 2, "john", 18})
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
