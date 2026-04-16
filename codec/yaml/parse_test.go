package yamlcodec_test

import (
	"errors"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	yamlcodec "github.com/pakasa-io/qb/codec/yaml"
)

func TestParse(t *testing.T) {
	payload := []byte(`
$select:
  - "lower(users.name) as normalized_name"
  - users.age
$where:
  status: active
  age:
    $gte: 18
  $expr:
    $eq:
      - "@users.role"
      - "'admin'"
$group:
  - "lower(users.name)"
$sort:
  - "lower(users.name) asc"
$page: 2
$size: 10
`)

	query, err := yamlcodec.Parse(payload)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(query.Projections) != 2 || query.Projections[0].Alias != "normalized_name" {
		t.Fatalf("unexpected projections: %#v", query.Projections)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT LOWER("users"."name") AS "normalized_name", "users"."age" WHERE ("users"."role" = $1 AND "age" >= $2 AND "status" = $3) GROUP BY LOWER("users"."name") ORDER BY LOWER("users"."name") ASC LIMIT 10 OFFSET 10`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	assertArgsEqual(t, statement.Args, []any{"admin", 18, "active"})
}

func TestParseRejectsNonObjectRoot(t *testing.T) {
	_, err := yamlcodec.Parse([]byte(`- users.id`))
	if err == nil {
		t.Fatal("expected root object error")
	}
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

func TestParseYAMLReturnsStructuredError(t *testing.T) {
	_, err := yamlcodec.Parse([]byte("$size: nope"))
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
