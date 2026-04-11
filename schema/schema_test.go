package schema_test

import (
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
			schema.Aliases("state"),
			schema.Operators(qb.OpEq, qb.OpIn),
		),
		schema.Define(
			"age",
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
		sqladapter.WithQueryTransformer(userSchema.Normalize),
	).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE ("age" >= ? AND "status" = ?) ORDER BY "created_at" DESC`
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
		schema.Define("status", schema.Aliases("state")),
		schema.Define("created_at", schema.Aliases("createdAt"), schema.Sortable()),
	)

	query, err := qb.New().
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

	wantSQL := `WHERE "status" = ? ORDER BY "created_at" DESC`
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
