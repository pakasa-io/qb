package codec_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	"github.com/pakasa-io/qb/codecs"
	jsoncodec "github.com/pakasa-io/qb/codecs/jsoncodec"
)

type failingLiteralCodec struct{}

func (failingLiteralCodec) FormatLiteral(value any) (any, string, bool, error) {
	return nil, "", true, errors.New("boom")
}

func (failingLiteralCodec) ParseLiteral(codec string, literal any) (any, bool, error) {
	return nil, false, nil
}

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

func TestMarshalFailsWhenLiteralCodecReturnsError(t *testing.T) {
	query, err := qb.New().
		Where(qb.F("users.status").Eq("active")).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	_, err = jsoncodec.Marshal(query, codecs.WithLiteralCodec(failingLiteralCodec{}))
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected codec error to propagate, got %v", err)
	}
}
