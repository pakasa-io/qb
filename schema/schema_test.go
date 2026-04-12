package schema_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/mapinput"
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

	query, err := mapinput.Parse(
		map[string]any{
			"where": map[string]any{
				"state":  "active",
				"minAge": map[string]any{"$gte": "21"},
			},
			"sort": "-createdAt",
		},
		mapinput.WithFilterFieldResolver(userSchema.ResolveFilterField),
		mapinput.WithSortFieldResolver(userSchema.ResolveSortField),
		mapinput.WithValueDecoder(userSchema.DecodeValue),
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

	wantSQL := `WHERE ("users"."age" >= ? AND "users"."status" = ?) ORDER BY "users"."created_at" DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{21, "active"}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
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

	wantSQL := `SELECT "status" WHERE "status" = ? GROUP BY "status" ORDER BY "created_at" DESC`
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

	if len(statement.Args) != 1 || statement.Args[0] != 21 {
		t.Fatalf("unexpected args: %#v", statement.Args)
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

	wantSQL := `SELECT "users"."status" WHERE "users"."status" = ? GROUP BY "users"."status" ORDER BY "users"."created_at" DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
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
		schema.Define("name", schema.Storage("users.name")),
		schema.Define("age", schema.Storage("users.age")),
	)

	query, err := qb.New().
		SelectExpr(qb.Lower(qb.Field("name")), qb.Field("age")).
		GroupByExpr(qb.Lower(qb.Field("name"))).
		Where(qb.Lower(qb.Field("name")).Eq(qb.Lower("JOHN"))).
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

	wantSQL := `SELECT LOWER("users"."name"), "users"."age" WHERE LOWER("users"."name") = LOWER(?) GROUP BY LOWER("users"."name")`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	if len(statement.Args) != 1 || statement.Args[0] != "JOHN" {
		t.Fatalf("unexpected args: %#v", statement.Args)
	}
}
