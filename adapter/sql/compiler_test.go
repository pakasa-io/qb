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

	wantSQL := `WHERE "status" = $1 ORDER BY "created_at" DESC`
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

func TestCompileReturnsStructuredError(t *testing.T) {
	query := qb.Query{
		Filter: qb.Predicate{
			Left:  qb.F("status"),
			Op:    qb.Operator("bogus"),
			Right: qb.ScalarOperand{Expr: qb.V("active")},
		},
	}

	_, err := sqladapter.New().Compile(query)
	if err == nil {
		t.Fatal("expected compile error")
	}

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected qb.Error, got %T", err)
	}

	if diagnostic.Stage != qb.StageCompile || diagnostic.Code != qb.CodeUnsupportedOperator {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}
}

func TestCompileSelectGroupByAndPageSize(t *testing.T) {
	query, err := qb.New().
		Select("status", "role").
		Where(qb.Field("tenant_id").Eq(42)).
		GroupBy("status", "role").
		SortBy("status", qb.Asc).
		Page(2).
		Size(10).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT "status", "role" WHERE "tenant_id" = $1 GROUP BY "status", "role" ORDER BY "status" ASC LIMIT 10 OFFSET 10`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}
}

func TestCompileCursorQueryWithTransformer(t *testing.T) {
	query, err := qb.New().
		SortBy("created_at", qb.Desc).
		Size(25).
		CursorValues(map[string]any{
			"created_at": "2026-04-11T12:00:00Z",
			"id":         981,
		}).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(func(query qb.Query) (qb.Query, error) {
			if query.Cursor == nil {
				return query, nil
			}

			query.Filter = qb.Or(
				qb.Field("created_at").Lt(query.Cursor.Values["created_at"]),
				qb.And(
					qb.Field("created_at").Eq(query.Cursor.Values["created_at"]),
					qb.Field("id").Lt(query.Cursor.Values["id"]),
				),
			)
			query.Cursor = nil
			return query, nil
		}),
	).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE ("created_at" < $1 OR ("created_at" = $2 AND "id" < $3)) ORDER BY "created_at" DESC LIMIT 25`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{"2026-04-11T12:00:00Z", "2026-04-11T12:00:00Z", 981}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
	}
}

func TestCompileFunctionExpressions(t *testing.T) {
	query, err := qb.New().
		SelectExpr(qb.Lower(qb.F("users.name")), qb.F("users.age")).
		GroupByExpr(qb.Lower(qb.F("users.name"))).
		SortByExpr(qb.Lower(qb.F("users.name")), qb.Asc).
		Where(qb.And(
			qb.Lower(qb.F("users.name")).Eq("john"),
			qb.F("users.name").Eq(qb.Lower("JOHN")),
		)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT LOWER("users"."name"), "users"."age" WHERE (LOWER("users"."name") = $1 AND "users"."name" = LOWER($2)) GROUP BY LOWER("users"."name") ORDER BY LOWER("users"."name") ASC`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{"john", "JOHN"}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
	}
}

func TestDefaultDialectCanBeChangedGloballyAndOverridden(t *testing.T) {
	original := sqladapter.DefaultDialect()
	defer sqladapter.SetDefaultDialect(original)

	query, err := qb.New().
		Where(qb.Lower(qb.F("users.name")).Eq("john")).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	sqladapter.SetDefaultDialect(sqladapter.MySQLDialect{})

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantGlobal := "WHERE LOWER(`users`.`name`) = ?"
	if statement.SQL != wantGlobal {
		t.Fatalf("global dialect SQL mismatch\nwant: %s\ngot:  %s", wantGlobal, statement.SQL)
	}

	override, err := sqladapter.New(sqladapter.WithDialect(sqladapter.SQLiteDialect{})).Compile(query)
	if err != nil {
		t.Fatalf("Compile() with override error = %v", err)
	}

	wantOverride := `WHERE LOWER("users"."name") = ?`
	if override.SQL != wantOverride {
		t.Fatalf("override dialect SQL mismatch\nwant: %s\ngot:  %s", wantOverride, override.SQL)
	}
}
