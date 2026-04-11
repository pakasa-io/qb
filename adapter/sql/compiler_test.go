package sqladapter_test

import (
	"errors"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
)

func TestCompile(t *testing.T) {
	query, err := qb.New().
		Where(qb.And(
			qb.Field("users.status").Eq("active"),
			qb.Not(qb.Field("deleted_at").IsNull()),
			qb.Field("role").In("admin", "owner"),
		)).
		SortBy("users.created_at", qb.Desc).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.DollarDialect{})).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE ("users"."status" = $1 AND NOT ("deleted_at" IS NULL) AND "role" IN ($2, $3)) ORDER BY "users"."created_at" DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{"active", "admin", "owner"}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
	}
}

func TestCompileWithTransformer(t *testing.T) {
	query, err := qb.New().
		Where(qb.Field("status").Eq("active")).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(func(query qb.Query) (qb.Query, error) {
			return qb.New().
				Where(query.Filter).
				SortBy("created_at", qb.Desc).
				Query()
		}),
	).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE "status" = ? ORDER BY "created_at" DESC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}
}

func TestCompileWithTransformerError(t *testing.T) {
	query, err := qb.New().
		Where(qb.Field("status").Eq("active")).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	wantErr := errors.New("boom")
	_, err = sqladapter.New(
		sqladapter.WithQueryTransformer(func(query qb.Query) (qb.Query, error) {
			return qb.Query{}, wantErr
		}),
	).Compile(query)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected transformer error, got %v", err)
	}
}
