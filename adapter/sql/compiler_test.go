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

func TestCompileCommonFunctionHelpersAcrossDialects(t *testing.T) {
	query, err := qb.New().
		SelectExpr(
			qb.Count(),
			qb.F("users.amount").Sum(),
			qb.F("users.rating").Avg(),
			qb.F("users.age").Min(),
			qb.F("users.age").Max(),
			qb.F("users.name").LTrim(),
			qb.F("users.name").RTrim(),
			qb.F("users.score").Mod(10),
			qb.F("users.code").Left(3),
			qb.F("users.code").Right(2),
			qb.CurrentDate(),
			qb.CurrentTime(),
			qb.CurrentTimestamp(),
			qb.F("users.profile").JsonExtract("$.nickname"),
			qb.F("users.profile").JsonValue("$.nickname"),
		).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	tests := []struct {
		name    string
		dialect sqladapter.Dialect
		wantSQL string
	}{
		{
			name:    "postgres",
			dialect: sqladapter.PostgresDialect{},
			wantSQL: `SELECT COUNT(*), SUM("users"."amount"), AVG("users"."rating"), MIN("users"."age"), MAX("users"."age"), LTRIM("users"."name"), RTRIM("users"."name"), ("users"."score" % $1), LEFT("users"."code", $2), RIGHT("users"."code", $3), CURRENT_DATE, CURRENT_TIME, CURRENT_TIMESTAMP, JSON_QUERY("users"."profile", $4), JSON_VALUE("users"."profile", $5)`,
		},
		{
			name:    "mysql",
			dialect: sqladapter.MySQLDialect{},
			wantSQL: "SELECT COUNT(*), SUM(`users`.`amount`), AVG(`users`.`rating`), MIN(`users`.`age`), MAX(`users`.`age`), LTRIM(`users`.`name`), RTRIM(`users`.`name`), (`users`.`score` % ?), LEFT(`users`.`code`, ?), RIGHT(`users`.`code`, ?), CURRENT_DATE, CURRENT_TIME, CURRENT_TIMESTAMP, JSON_EXTRACT(`users`.`profile`, ?), JSON_VALUE(`users`.`profile`, ?)",
		},
		{
			name:    "sqlite",
			dialect: sqladapter.SQLiteDialect{},
			wantSQL: `SELECT COUNT(*), SUM("users"."amount"), AVG("users"."rating"), MIN("users"."age"), MAX("users"."age"), LTRIM("users"."name"), RTRIM("users"."name"), ("users"."score" % ?), SUBSTR("users"."code", 1, ?), SUBSTR("users"."code", -?), CURRENT_DATE, CURRENT_TIME, CURRENT_TIMESTAMP, json_extract("users"."profile", ?), json_extract("users"."profile", ?)`,
		},
	}

	wantArgs := []any{10, 3, 2, "$.nickname", "$.nickname"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, err := sqladapter.New(sqladapter.WithDialect(tt.dialect)).Compile(query)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}

			if statement.SQL != tt.wantSQL {
				t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", tt.wantSQL, statement.SQL)
			}

			if len(statement.Args) != len(wantArgs) {
				t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
			}

			for i := range wantArgs {
				if statement.Args[i] != wantArgs[i] {
					t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
				}
			}
		})
	}
}

func TestCompileDateAndJSONHelpersAcrossDialects(t *testing.T) {
	query, err := qb.New().
		SelectExpr(
			qb.Now(),
			qb.LocalTime(),
			qb.LocalTimestamp(),
			qb.F("users.profile").JsonQuery("$.address"),
			qb.F("users.profile").JsonExists("$.address"),
			qb.F("users.profile").JsonArrayLength("$.tags"),
			qb.F("users.profile").JsonType("$.address"),
			qb.JsonArray("admin", "owner"),
			qb.JsonObject("id", qb.F("users.id"), "name", qb.F("users.name")),
		).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	tests := []struct {
		name    string
		dialect sqladapter.Dialect
		wantSQL string
	}{
		{
			name:    "postgres",
			dialect: sqladapter.PostgresDialect{},
			wantSQL: `SELECT NOW(), LOCALTIME, LOCALTIMESTAMP, JSON_QUERY("users"."profile", $1), JSON_EXISTS("users"."profile", $2), JSON_ARRAY_LENGTH(JSON_QUERY("users"."profile", $3)), JSON_TYPEOF(JSON_QUERY("users"."profile", $4)), JSON_ARRAY($5, $6), JSON_OBJECT($7 VALUE "users"."id", $8 VALUE "users"."name")`,
		},
		{
			name:    "mysql",
			dialect: sqladapter.MySQLDialect{},
			wantSQL: "SELECT NOW(), LOCALTIME, LOCALTIMESTAMP, JSON_EXTRACT(`users`.`profile`, ?), JSON_CONTAINS_PATH(`users`.`profile`, 'one', ?), JSON_LENGTH(`users`.`profile`, ?), JSON_TYPE(JSON_EXTRACT(`users`.`profile`, ?)), JSON_ARRAY(?, ?), JSON_OBJECT(?, `users`.`id`, ?, `users`.`name`)",
		},
		{
			name:    "sqlite",
			dialect: sqladapter.SQLiteDialect{},
			wantSQL: "SELECT CURRENT_TIMESTAMP, TIME('now', 'localtime'), DATETIME('now', 'localtime'), json_extract(\"users\".\"profile\", ?), (json_type(\"users\".\"profile\", ?) IS NOT NULL), json_array_length(\"users\".\"profile\", ?), json_type(\"users\".\"profile\", ?), json_array(?, ?), json_object(?, \"users\".\"id\", ?, \"users\".\"name\")",
		},
	}

	wantArgs := []any{"$.address", "$.address", "$.tags", "$.address", "admin", "owner", "id", "name"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, err := sqladapter.New(sqladapter.WithDialect(tt.dialect)).Compile(query)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}

			if statement.SQL != tt.wantSQL {
				t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", tt.wantSQL, statement.SQL)
			}

			if len(statement.Args) != len(wantArgs) {
				t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
			}

			for i := range wantArgs {
				if statement.Args[i] != wantArgs[i] {
					t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
				}
			}
		})
	}
}

func TestCompilePostgresDateHelpers(t *testing.T) {
	query, err := qb.New().
		SelectExpr(
			qb.F("users.created_at").DateTrunc("day"),
			qb.F("users.created_at").Extract("year"),
			qb.DateBin("1 hour", qb.F("users.created_at"), qb.F("users.origin_at")),
		).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.PostgresDialect{})).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT DATE_TRUNC($1, "users"."created_at"), DATE_PART($2, "users"."created_at"), DATE_BIN($3, "users"."created_at", "users"."origin_at")`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{"day", "year", "1 hour"}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
	}
}

func TestCompileDialectSpecificPatternOperators(t *testing.T) {
	postgresQuery, err := qb.New().
		Where(qb.And(
			qb.F("users.name").ILike("jo%"),
			qb.F("users.email").Regexp(`.+@example\.com`),
		)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.PostgresDialect{})).Compile(postgresQuery)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantPostgres := `WHERE ("users"."name" ILIKE $1 AND "users"."email" ~ $2)`
	if statement.SQL != wantPostgres {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantPostgres, statement.SQL)
	}

	mysqlQuery, err := qb.New().
		Where(qb.F("users.email").Regexp(`.+@example\.com`)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	mysqlStatement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.MySQLDialect{})).Compile(mysqlQuery)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantMySQL := "WHERE REGEXP_LIKE(`users`.`email`, ?)"
	if mysqlStatement.SQL != wantMySQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantMySQL, mysqlStatement.SQL)
	}
}

func TestCompileUnsupportedFunctionsAndOperators(t *testing.T) {
	tests := []struct {
		name    string
		query   qb.Query
		dialect sqladapter.Dialect
	}{
		{
			name: "ceil sqlite",
			query: mustQuery(t, qb.New().
				SelectExpr(qb.F("users.score").Ceil())),
			dialect: sqladapter.SQLiteDialect{},
		},
		{
			name: "floor sqlite",
			query: mustQuery(t, qb.New().
				SelectExpr(qb.F("users.score").Floor())),
			dialect: sqladapter.SQLiteDialect{},
		},
		{
			name: "ilike mysql",
			query: mustQuery(t, qb.New().
				Where(qb.F("users.name").ILike("jo%"))),
			dialect: sqladapter.MySQLDialect{},
		},
		{
			name: "regexp sqlite",
			query: mustQuery(t, qb.New().
				Where(qb.F("users.name").Regexp("jo.*"))),
			dialect: sqladapter.SQLiteDialect{},
		},
		{
			name: "date_trunc mysql",
			query: mustQuery(t, qb.New().
				SelectExpr(qb.F("users.created_at").DateTrunc("day"))),
			dialect: sqladapter.MySQLDialect{},
		},
		{
			name: "extract sqlite",
			query: mustQuery(t, qb.New().
				SelectExpr(qb.F("users.created_at").Extract("year"))),
			dialect: sqladapter.SQLiteDialect{},
		},
		{
			name: "date_bin mysql",
			query: mustQuery(t, qb.New().
				SelectExpr(qb.DateBin("1 hour", qb.F("users.created_at"), qb.F("users.origin_at")))),
			dialect: sqladapter.MySQLDialect{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sqladapter.New(sqladapter.WithDialect(tt.dialect)).Compile(tt.query)
			if err == nil {
				t.Fatal("expected compile error")
			}

			var diagnostic *qb.Error
			if !errors.As(err, &diagnostic) {
				t.Fatalf("expected qb.Error, got %T", err)
			}

			if diagnostic.Code != qb.CodeUnsupportedFeature {
				t.Fatalf("unexpected diagnostic: %+v", diagnostic)
			}
		})
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

func mustQuery(t *testing.T, builder qb.Builder) qb.Query {
	t.Helper()

	query, err := builder.Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	return query
}
