package jsoncodec_test

import (
	"strings"
	"testing"
	"time"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	jsoncodec "github.com/pakasa-io/qb/codec/json"
)

func TestMarshalParseRoundTrip(t *testing.T) {
	joinedAt := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	query, err := qb.New().
		Select("users.id", "users.status").
		Where(qb.F("users.status").Eq("active")).
		Where(qb.F("users.joined_at").Gte(joinedAt)).
		SortBy("users.created_at", qb.Desc).
		Size(10).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	payload, err := jsoncodec.Marshal(query)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(payload), `"$codec": "time"`) {
		t.Fatalf("Marshal() missing expected typed literal wrapper:\n%s", string(payload))
	}

	parsed, err := jsoncodec.Parse(payload)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(parsed)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(statement.SQL, `"users"."joined_at" >= $1`) {
		t.Fatalf("unexpected SQL: %s", statement.SQL)
	}
}
