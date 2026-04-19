package json_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	"github.com/pakasa-io/qb/codecs"
	jsoncodec "github.com/pakasa-io/qb/codecs/json"
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

	group, ok := parsed.Filter.(qb.Group)
	if !ok {
		t.Fatalf("expected grouped filter, got %T", parsed.Filter)
	}

	if len(group.Terms) != 2 {
		t.Fatalf("unexpected filter terms: %#v", group.Terms)
	}

	var joinedAtPredicate qb.Predicate
	foundJoinedAt := false
	for _, term := range group.Terms {
		predicate, ok := term.(qb.Predicate)
		if !ok {
			t.Fatalf("expected grouped predicates, got %T", term)
		}

		ref, ok := predicate.Left.(qb.Ref)
		if ok && ref.Name == "users.joined_at" {
			joinedAtPredicate = predicate
			foundJoinedAt = true
			break
		}
	}

	if !foundJoinedAt {
		t.Fatalf("expected joined_at predicate in round-tripped filter, got %#v", group.Terms)
	}

	operand, ok := joinedAtPredicate.Right.(qb.ScalarOperand)
	if !ok {
		t.Fatalf("expected scalar operand, got %T", joinedAtPredicate.Right)
	}

	literal, ok := operand.Expr.(qb.Literal)
	if !ok {
		t.Fatalf("expected literal rhs, got %T", operand.Expr)
	}

	parsedJoinedAt, ok := literal.Value.(time.Time)
	if !ok || !parsedJoinedAt.Equal(joinedAt) {
		t.Fatalf("expected time literal to round-trip, got %#v", literal.Value)
	}

	statement, err := sqladapter.New().Compile(parsed)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `SELECT "users"."id", "users"."status" WHERE ("users"."joined_at" >= $1 AND "users"."status" = $2) ORDER BY "users"."created_at" DESC LIMIT 10`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	if len(statement.Args) != 2 || statement.Args[1] != "active" {
		t.Fatalf("unexpected SQL args: %#v", statement.Args)
	}

	compiledJoinedAt, ok := statement.Args[0].(time.Time)
	if !ok || !compiledJoinedAt.Equal(joinedAt) {
		t.Fatalf("expected compiled time arg to round-trip, got %#v", statement.Args[0])
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
	if err.Error() != "boom" {
		t.Fatalf("expected codec error to propagate without wrapping, got %v", err)
	}
}
