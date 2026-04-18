package model_test

import (
	"strings"
	"testing"

	"github.com/pakasa-io/qb"
	jsoncodec "github.com/pakasa-io/qb/codec/json"
	"github.com/pakasa-io/qb/codec/model"
)

type reversibleStatus string

func (s reversibleStatus) MarshalText() ([]byte, error) {
	return []byte("X-" + string(s)), nil
}

func (s *reversibleStatus) UnmarshalText(text []byte) error {
	*s = reversibleStatus(strings.TrimPrefix(string(text), "X-"))
	return nil
}

func TestDefaultLiteralCodecModeCanBeChangedByFunctionAndEnv(t *testing.T) {
	const codecName = "reversible_status_test"

	if err := model.RegisterReversibleTextType(codecName, new(reversibleStatus)); err != nil && !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("RegisterReversibleTextType() error = %v", err)
	}

	previous := model.DefaultLiteralCodecModeValue()
	t.Cleanup(func() {
		if err := model.SetDefaultLiteralCodecMode(previous); err != nil {
			t.Fatalf("restore literal codec mode: %v", err)
		}
		t.Setenv(model.DefaultLiteralCodecModeEnv, "")
	})

	if err := model.SetDefaultLiteralCodecMode(model.LiteralCodecModeStrict); err != nil {
		t.Fatalf("SetDefaultLiteralCodecMode(strict) error = %v", err)
	}

	query, err := qb.New().
		Where(qb.F("status").Eq(reversibleStatus("active"))).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	payload, err := jsoncodec.Marshal(query)
	if err != nil {
		t.Fatalf("Marshal(strict) error = %v", err)
	}
	if strings.Contains(string(payload), `"$codec": "`+codecName+`"`) {
		t.Fatalf("strict mode unexpectedly emitted reversible text codec wrapper:\n%s", string(payload))
	}

	t.Setenv(model.DefaultLiteralCodecModeEnv, string(model.LiteralCodecModeReversibleText))
	if err := model.ResetDefaultLiteralCodecModeFromEnv(); err != nil {
		t.Fatalf("ResetDefaultLiteralCodecModeFromEnv() error = %v", err)
	}

	payload, err = jsoncodec.Marshal(query)
	if err != nil {
		t.Fatalf("Marshal(reversible_text) error = %v", err)
	}
	if !strings.Contains(string(payload), `"$codec": "`+codecName+`"`) {
		t.Fatalf("reversible_text mode did not emit codec wrapper:\n%s", string(payload))
	}

	parsed, err := jsoncodec.Parse(payload)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	predicate, ok := parsed.Filter.(qb.Predicate)
	if !ok {
		t.Fatalf("expected predicate filter, got %#v", parsed.Filter)
	}
	right, ok := predicate.Right.(qb.ScalarOperand)
	if !ok {
		t.Fatalf("expected scalar operand, got %#v", predicate.Right)
	}
	literal, ok := right.Expr.(qb.Literal)
	if !ok {
		t.Fatalf("expected literal rhs, got %#v", right.Expr)
	}
	value, ok := literal.Value.(reversibleStatus)
	if !ok {
		t.Fatalf("expected reversibleStatus, got %T (%#v)", literal.Value, literal.Value)
	}
	if value != "active" {
		t.Fatalf("unexpected reversibleStatus value: %q", value)
	}
}

func TestSetDefaultLiteralCodecModeRejectsUnknownValue(t *testing.T) {
	if err := model.SetDefaultLiteralCodecMode("unsupported"); err == nil {
		t.Fatalf("SetDefaultLiteralCodecMode() unexpectedly accepted unsupported mode")
	}
}
