package qs_test

import (
	"strings"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	querystring "github.com/pakasa-io/qb/codecs/qs"
)

func TestEncodeParseStringRoundTrip(t *testing.T) {
	query, err := qb.New().
		Select("users.id", "users.status").
		Where(qb.F("users.status").Eq("active")).
		Where(qb.F("users.age").Gte(21)).
		SortBy("users.created_at", qb.Desc).
		Page(2).
		Size(10).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	raw, err := querystring.Encode(query)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !strings.Contains(raw, "%24where%5Busers.age%5D%5B%24gte%5D=21") {
		t.Fatalf("unexpected encoded query string: %s", raw)
	}

	parsed, err := querystring.ParseString(raw)
	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(parsed)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(statement.SQL, `"users"."created_at" DESC LIMIT 10 OFFSET 10`) {
		t.Fatalf("unexpected SQL: %s", statement.SQL)
	}
}
