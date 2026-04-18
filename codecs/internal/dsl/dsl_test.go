package dsl

import (
	"errors"
	"testing"

	"github.com/pakasa-io/qb"
)

func TestParseStandaloneScalarAndSort(t *testing.T) {
	scalar, alias, err := ParseStandaloneScalar("lower(user.name)::string as normalized_name", true, nil)
	if err != nil {
		t.Fatalf("ParseStandaloneScalar() error = %v", err)
	}

	cast, ok := scalar.(qb.Cast)
	if !ok || alias != "normalized_name" {
		t.Fatalf("unexpected parsed scalar: %#v alias=%q", scalar, alias)
	}

	call, ok := cast.Expr.(qb.Call)
	if !ok || call.Name != "lower" || len(call.Args) != 1 {
		t.Fatalf("unexpected parsed call: %#v", cast.Expr)
	}

	if ref := call.Args[0].(qb.Ref); ref.Name != "user.name" {
		t.Fatalf("unexpected parsed ref: %#v", ref)
	}

	boolScalar, _, err := ParseStandaloneScalar("true", false, nil)
	if err != nil || boolScalar.(qb.Literal).Value != true {
		t.Fatalf("unexpected bool scalar: %#v %v", boolScalar, err)
	}

	nullScalar, _, err := ParseStandaloneScalar("null", false, nil)
	if err != nil || nullScalar.(qb.Literal).Value != nil {
		t.Fatalf("unexpected null scalar: %#v %v", nullScalar, err)
	}

	typedLiteral, _, err := ParseStandaloneScalar("!#:uuid:abc123", false, func(codec string, literal string) (any, error) {
		return codec + ":" + literal, nil
	})
	if err != nil {
		t.Fatalf("ParseStandaloneScalar(typed literal) error = %v", err)
	}

	if got := typedLiteral.(qb.Literal).Value; got != "uuid:abc123" {
		t.Fatalf("unexpected typed literal decoding: %#v", got)
	}

	sortExpr, direction, err := ParseSort("score desc", nil)
	if err != nil {
		t.Fatalf("ParseSort() error = %v", err)
	}

	if direction != qb.Desc || sortExpr.(qb.Ref).Name != "score" {
		t.Fatalf("unexpected parsed sort: expr=%#v direction=%q", sortExpr, direction)
	}

	sortExpr, direction, err = ParseSort("score", nil)
	if err != nil || direction != qb.Asc || sortExpr.(qb.Ref).Name != "score" {
		t.Fatalf("unexpected default sort direction: expr=%#v direction=%q err=%v", sortExpr, direction, err)
	}

	tests := []string{
		"lower(name) trailing",
		"name as",
		"(",
		"name ::",
		"unterminated'",
	}

	for _, input := range tests {
		if _, _, err := ParseStandaloneScalar(input, true, nil); err == nil {
			t.Fatalf("expected ParseStandaloneScalar(%q) to fail", input)
		}
	}

	if _, _, err := ParseSort("score desc trailing", nil); err == nil {
		t.Fatal("expected ParseSort() to reject trailing tokens")
	}
}

func TestFormatScalarProjectionAndSort(t *testing.T) {
	mixed, err := FormatScalar(qb.F("user.name"), FormatOptions{Context: MixedContext})
	if err != nil || mixed != "@user.name" {
		t.Fatalf("unexpected mixed context scalar: %q %v", mixed, err)
	}

	projection, err := FormatProjection(qb.Project(qb.F("user.name")).As("name"), FormatOptions{})
	if err != nil || projection != "user.name as name" {
		t.Fatalf("unexpected projection formatting: %q %v", projection, err)
	}

	sortText, err := FormatSort(qb.Sort{Expr: qb.F("user.name")}, FormatOptions{})
	if err != nil || sortText != "user.name asc" {
		t.Fatalf("unexpected sort formatting: %q %v", sortText, err)
	}

	typedText, err := FormatScalar(
		qb.V("abc123"),
		FormatOptions{
			Mode: CompactMode,
			LiteralFormatter: func(value any) (string, string, bool, error) {
				return value.(string), "uuid", true, nil
			},
		},
	)
	if err != nil || typedText != "!#:uuid:abc123" {
		t.Fatalf("unexpected compact typed literal: %q %v", typedText, err)
	}

	if _, err := FormatScalar(
		qb.V("not safe "),
		FormatOptions{
			Mode: CompactMode,
			LiteralFormatter: func(value any) (string, string, bool, error) {
				return value.(string), "uuid", true, nil
			},
		},
	); !errors.Is(err, ErrStructuredRequired) {
		t.Fatalf("expected structured-required error, got %v", err)
	}

	if _, err := FormatScalar(
		qb.V(map[string]string{"a": "b"}),
		FormatOptions{},
	); !errors.Is(err, ErrStructuredRequired) {
		t.Fatalf("expected unsupported literal to require structure, got %v", err)
	}

}

func TestTokenizerAndHelperUtilities(t *testing.T) {
	tokens, err := tokenize("@user.name, 'it''s', !#:uuid:abc")
	if err != nil {
		t.Fatalf("tokenize() error = %v", err)
	}

	if len(tokens) < 8 || tokens[0].kind != tokenAt || tokens[1].text != "user" || tokens[5].kind != tokenString {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}

	if _, err := tokenize(":"); err == nil {
		t.Fatal("expected tokenize() to reject a bare colon")
	}

	if got := quoteString("it's"); got != "'it''s'" {
		t.Fatalf("unexpected quoteString() result: %q", got)
	}

	if codec, literal, err := splitTypedLiteralToken("!#:uuid:abc"); err != nil || codec != "uuid" || literal != "abc" {
		t.Fatalf("unexpected splitTypedLiteralToken() result: %q %q %v", codec, literal, err)
	}

	if _, _, err := splitTypedLiteralToken("broken"); err == nil {
		t.Fatal("expected splitTypedLiteralToken() to reject invalid input")
	}

	if inlineSafeLiteral("") || inlineSafeLiteral(" needs-space") || inlineSafeLiteral("a,b") {
		t.Fatal("expected inlineSafeLiteral() to reject unsafe values")
	}

	if !inlineSafeLiteral("abc123") {
		t.Fatal("expected inlineSafeLiteral() to accept simple values")
	}

	if !isIdentStart('_') || !isIdentContinue('9') || isIdentStart('9') {
		t.Fatal("unexpected identifier helpers behavior")
	}

	parser, err := newParser("@user.name", nil)
	if err != nil {
		t.Fatalf("newParser() error = %v", err)
	}

	if parser.peek().kind != tokenAt || parser.peekN(1).kind != tokenIdent {
		t.Fatalf("unexpected parser peek state: %#v", parser.tokens)
	}

	if parser.next().kind != tokenAt || parser.next().text != "user" {
		t.Fatalf("unexpected parser next behavior: pos=%d tokens=%#v", parser.pos, parser.tokens)
	}
}
