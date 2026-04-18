package sqladapter

import (
	"errors"
	"reflect"
	"testing"

	"github.com/pakasa-io/qb"
)

func TestDefaultDialectLookupAndRegistration(t *testing.T) {
	previous := DefaultDialect()
	defer SetDefaultDialect(previous)

	if DefaultDialect() == nil {
		t.Fatal("expected a default dialect")
	}

	SetDefaultDialect(nil)
	if DefaultDialect() == nil {
		t.Fatal("expected SetDefaultDialect(nil) to be ignored")
	}

	if err := SetDefaultDialectByName("mysql"); err != nil {
		t.Fatalf("SetDefaultDialectByName() error = %v", err)
	}

	if got := DefaultDialect().Name(); got != "mysql" {
		t.Fatalf("unexpected default dialect name: %q", got)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"postgres", "postgres"},
		{"postgresql", "postgres"},
		{"pg", "postgres"},
		{"mysql", "mysql"},
		{"sqlite3", "sqlite"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			dialect, err := LookupDialect(tc.input)
			if err != nil {
				t.Fatalf("LookupDialect() error = %v", err)
			}
			if dialect.Name() != tc.want {
				t.Fatalf("unexpected dialect for %q: %q", tc.input, dialect.Name())
			}
		})
	}

	if _, err := LookupDialect("oracle"); err == nil {
		t.Fatal("expected LookupDialect() to reject unknown dialects")
	}

	if MustDialect("sqlite").Name() != "sqlite" {
		t.Fatal("expected MustDialect() to return a dialect")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected MustDialect() to panic on an unknown dialect")
		}
	}()
	_ = MustDialect("oracle")
}

func TestDialectHelpersAndCompilers(t *testing.T) {
	if got, err := keyword("CURRENT_DATE")(nil); err != nil || got != "CURRENT_DATE" {
		t.Fatalf("unexpected keyword() result: %q %v", got, err)
	}

	if _, err := keyword("CURRENT_DATE")([]string{"x"}); err == nil {
		t.Fatal("expected keyword() to reject arguments")
	}

	if got, err := zeroOrOne("count", "COUNT")(nil); err != nil || got != "COUNT(*)" {
		t.Fatalf("unexpected zeroOrOne() zero-arg result: %q %v", got, err)
	}

	if got, err := zeroOrOne("count", "COUNT")([]string{"field"}); err != nil || got != "COUNT(field)" {
		t.Fatalf("unexpected zeroOrOne() one-arg result: %q %v", got, err)
	}

	if _, err := zeroOrOne("count", "COUNT")([]string{"a", "b"}); err == nil {
		t.Fatal("expected zeroOrOne() to reject too many arguments")
	}

	if got, err := exactArity("sum", "SUM", 1)([]string{"field"}); err != nil || got != "SUM(field)" {
		t.Fatalf("unexpected exactArity() result: %q %v", got, err)
	}

	if _, err := exactArity("sum", "SUM", 1)(nil); err == nil {
		t.Fatal("expected exactArity() to enforce arity")
	}

	if got, err := arityRange("round", "ROUND", 1, 2)([]string{"field", "2"}); err != nil || got != "ROUND(field, 2)" {
		t.Fatalf("unexpected arityRange() result: %q %v", got, err)
	}

	if _, err := arityRange("round", "ROUND", 1, 2)([]string{}); err == nil {
		t.Fatal("expected arityRange() to reject too few arguments")
	}

	if got, err := variadic("coalesce", "COALESCE", 1)([]string{"a", "b"}); err != nil || got != "COALESCE(a, b)" {
		t.Fatalf("unexpected variadic() result: %q %v", got, err)
	}

	if _, err := variadic("coalesce", "COALESCE", 1)(nil); err == nil {
		t.Fatal("expected variadic() to reject too few arguments")
	}

	if _, err := unsupportedfunctionCompiler("sqlite", "extract")(nil); err == nil {
		t.Fatal("expected unsupportedfunctionCompiler() to fail")
	}

	if got, err := concatCompiler("postgres")([]string{"a", "b"}); err != nil || got != "(a || b)" {
		t.Fatalf("unexpected postgres concat: %q %v", got, err)
	}

	if got, err := concatCompiler("mysql")([]string{"a", "b"}); err != nil || got != "CONCAT(a, b)" {
		t.Fatalf("unexpected mysql concat: %q %v", got, err)
	}

	if _, err := concatCompiler("postgres")(nil); err == nil {
		t.Fatal("expected concatCompiler() to reject zero args")
	}

	if got, err := substringCompiler("sqlite")([]string{"a", "1"}); err != nil || got != "SUBSTR(a, 1)" {
		t.Fatalf("unexpected sqlite substring: %q %v", got, err)
	}

	if _, err := substringCompiler("postgres")([]string{"a"}); err == nil {
		t.Fatal("expected substringCompiler() to enforce arity")
	}

	if _, err := ceilCompiler("sqlite")([]string{"a"}); err == nil {
		t.Fatal("expected sqlite ceilCompiler() to be unsupported")
	}

	if _, err := floorCompiler("sqlite")([]string{"a"}); err == nil {
		t.Fatal("expected sqlite floorCompiler() to be unsupported")
	}

	if got, err := modCompiler()([]string{"a", "b"}); err != nil || got != "(a % b)" {
		t.Fatalf("unexpected modCompiler() result: %q %v", got, err)
	}

	if got, err := roundDoubleCompiler("postgres")([]string{"score", "2"}); err != nil || got == "" {
		t.Fatalf("unexpected roundDoubleCompiler() result: %q %v", got, err)
	}

	if got, err := leftCompiler("sqlite")([]string{"name", "2"}); err != nil || got != "SUBSTR(name, 1, 2)" {
		t.Fatalf("unexpected leftCompiler() result: %q %v", got, err)
	}

	if got, err := rightCompiler("sqlite")([]string{"name", "2"}); err != nil || got != "SUBSTR(name, -2)" {
		t.Fatalf("unexpected rightCompiler() result: %q %v", got, err)
	}

	if got, err := nowCompiler("sqlite")(nil); err != nil || got != "CURRENT_TIMESTAMP" {
		t.Fatalf("unexpected nowCompiler() result: %q %v", got, err)
	}

	if got, err := localTimeCompiler("sqlite")(nil); err != nil || got != "TIME('now', 'localtime')" {
		t.Fatalf("unexpected localTimeCompiler() result: %q %v", got, err)
	}

	if got, err := localTimestampCompiler("sqlite")(nil); err != nil || got != "DATETIME('now', 'localtime')" {
		t.Fatalf("unexpected localTimestampCompiler() result: %q %v", got, err)
	}

	if got, err := postgresOnly("extract", "DATE_PART", 2)([]string{"year", "created_at"}); err != nil || got != "DATE_PART(year, created_at)" {
		t.Fatalf("unexpected postgresOnly() result: %q %v", got, err)
	}

	if got, err := jsonArrayLengthPostgres()([]string{"payload", "'$.items'"}); err != nil || got == "" {
		t.Fatalf("unexpected jsonArrayLengthPostgres() result: %q %v", got, err)
	}

	if got, err := jsonTypePostgres()([]string{"payload"}); err != nil || got != "JSON_TYPEOF(payload)" {
		t.Fatalf("unexpected jsonTypePostgres() result: %q %v", got, err)
	}

	if got, err := jsonObjectPostgres()([]string{"'a'", "1"}); err != nil || got != "JSON_OBJECT('a' VALUE 1)" {
		t.Fatalf("unexpected jsonObjectPostgres() result: %q %v", got, err)
	}

	if got, err := jsonExistsMySQL()([]string{"payload", "'$.a'"}); err != nil || got == "" {
		t.Fatalf("unexpected jsonExistsMySQL() result: %q %v", got, err)
	}

	if got, err := jsonExistsSQLite()([]string{"payload", "'$.a'"}); err != nil || got == "" {
		t.Fatalf("unexpected jsonExistsSQLite() result: %q %v", got, err)
	}

	if got, err := jsonTypeMySQL()([]string{"payload", "'$.a'"}); err != nil || got == "" {
		t.Fatalf("unexpected jsonTypeMySQL() result: %q %v", got, err)
	}

	if got, err := jsonObjectSimple("JSON_OBJECT")([]string{"'a'", "1"}); err != nil || got != "JSON_OBJECT('a', 1)" {
		t.Fatalf("unexpected jsonObjectSimple() result: %q %v", got, err)
	}

	if got, err := regexOp("~")("left", "right"); err != nil || got != "left ~ right" {
		t.Fatalf("unexpected regexOp() result: %q %v", got, err)
	}

	if got, err := regexLikeMySQL()("left", "right"); err != nil || got != "REGEXP_LIKE(left, right)" {
		t.Fatalf("unexpected regexLikeMySQL() result: %q %v", got, err)
	}
}

func TestDialectImplementationsAndCasts(t *testing.T) {
	tests := []struct {
		dialect         Dialect
		wantName        string
		wantQuote       string
		wantPlaceholder string
	}{
		{dialect: PostgresDialect{}, wantName: "postgres", wantQuote: `"users"."name"`, wantPlaceholder: "$2"},
		{dialect: MySQLDialect{}, wantName: "mysql", wantQuote: "`users`.`name`", wantPlaceholder: "?"},
		{dialect: SQLiteDialect{}, wantName: "sqlite", wantQuote: `"users"."name"`, wantPlaceholder: "?"},
	}

	for _, tc := range tests {
		t.Run(tc.wantName, func(t *testing.T) {
			if tc.dialect.Name() != tc.wantName {
				t.Fatalf("unexpected dialect name: %q", tc.dialect.Name())
			}

			if got := tc.dialect.QuoteIdentifier("users.name"); got != tc.wantQuote {
				t.Fatalf("unexpected quoted identifier: %q", got)
			}

			if got := tc.dialect.Placeholder(2); got != tc.wantPlaceholder {
				t.Fatalf("unexpected placeholder: %q", got)
			}

			if !tc.dialect.Capabilities().SupportsFunction("lower") {
				t.Fatalf("expected %s to support lower()", tc.wantName)
			}
		})
	}

	if got := quoteDottedIdentifier(` users."name `, `"`); got != `"users"."""name"` {
		t.Fatalf("unexpected quoteDottedIdentifier() result: %q", got)
	}

	castTests := []struct {
		dialect string
		expr    string
		typeName string
	}{
		{"postgres", "score", "double"},
		{"postgres", "score", "json"},
		{"mysql", "score", "timestamp"},
		{"sqlite", "score", "date"},
	}

	for _, tc := range castTests {
		t.Run(tc.dialect+"_"+tc.typeName, func(t *testing.T) {
			sql, err := castCompilerForDialect(tc.dialect)(tc.expr, tc.typeName)
			if err != nil || sql == "" {
				t.Fatalf("unexpected castCompilerForDialect() result: %q %v", sql, err)
			}
		})
	}

	if _, err := castCompilerForDialect("postgres")("", "string"); err == nil {
		t.Fatal("expected castCompilerForDialect() to reject empty expressions")
	}

	if _, err := castCompilerForDialect("postgres")("score", "bogus"); err == nil {
		t.Fatal("expected castCompilerForDialect() to reject unsupported types")
	}

	sql, handled, err := PostgresDialect{}.CompilePredicate(qb.OpRegexp, `"name"`, "$1")
	if err != nil || !handled || sql != `"name" ~ $1` {
		t.Fatalf("unexpected postgres predicate result: %q %v %v", sql, handled, err)
	}

	sql, handled, err = MySQLDialect{}.CompilePredicate(qb.OpRegexp, "`name`", "?")
	if err != nil || !handled || sql != "REGEXP_LIKE(`name`, ?)" {
		t.Fatalf("unexpected mysql predicate result: %q %v %v", sql, handled, err)
	}

	sql, handled, err = SQLiteDialect{}.CompilePredicate(qb.OpRegexp, `"name"`, "?")
	if err != nil || handled || sql != "" {
		t.Fatalf("unexpected sqlite predicate result: %q %v %v", sql, handled, err)
	}

	if _, err := (PostgresDialect{}).CompileCast("score", "bogus"); err == nil {
		t.Fatal("expected unsupported cast to fail")
	}

	if _, err := (SQLiteDialect{}).CompileFunction("ceil", []string{"score"}); err == nil {
		t.Fatal("expected unsupported sqlite function to fail")
	}

	registered := registeredDialect{
		name:        "test",
		quote:       `"`,
		placeholder: func(index int) string { return "$" },
		functions: map[string]functionCompiler{
			"lower": exactArity("lower", "LOWER", 1),
		},
		cast:               castCompilerForDialect("postgres"),
		predicateCompilers: map[qb.Operator]predicateCompiler{qb.OpRegexp: regexOp("~")},
		capabilities:       newCapabilities(map[string]functionCompiler{"lower": exactArity("lower", "LOWER", 1)}, qb.OpRegexp),
	}

	if _, err := registered.CompileFunction("missing", nil); err == nil {
		t.Fatal("expected registeredDialect.CompileFunction() to reject unknown functions")
	}

	if _, handled, err := registered.CompilePredicate(qb.OpEq, "left", "right"); err != nil || handled {
		t.Fatalf("unexpected unhandled predicate result: handled=%v err=%v", handled, err)
	}

	if got := registered.Capabilities(); !reflect.DeepEqual(got.Functions, map[string]struct{}{"lower": {}}) {
		t.Fatalf("unexpected registered capabilities: %+v", got)
	}

	if _, err := registered.CompileCast("score", "string"); err != nil {
		t.Fatalf("expected registered cast to compile, got %v", err)
	}

	err = func() error {
		_, err := registered.CompileFunction("missing", nil)
		return err
	}()
	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected missing function to return qb.Error, got %T", err)
	}
}
