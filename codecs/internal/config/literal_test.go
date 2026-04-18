package codecconfig

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pakasa-io/qb"
)

type reversibleTextValue struct {
	value string
}

func (v *reversibleTextValue) MarshalText() ([]byte, error) {
	if v.value == "marshal-error" {
		return nil, errors.New("marshal failed")
	}
	return []byte(v.value), nil
}

func (v *reversibleTextValue) UnmarshalText(text []byte) error {
	if string(text) == "bad" {
		return errors.New("unmarshal failed")
	}
	v.value = string(text)
	return nil
}

type fakeLiteralCodec struct{}

func (fakeLiteralCodec) FormatLiteral(value any) (any, string, bool, error) {
	return value, "", true, nil
}

func (fakeLiteralCodec) ParseLiteral(codec string, literal any) (any, bool, error) {
	return literal, true, nil
}

func TestApplyOptionsAndOptionHelpers(t *testing.T) {
	valueDecoder := func(field string, op qb.Operator, value any) (any, error) {
		return field + string(op), nil
	}
	filterResolver := func(field string, op qb.Operator) (string, error) { return field + string(op), nil }
	groupResolver := func(field string) (string, error) { return field + "_group", nil }
	sortResolver := func(field string) (string, error) { return field + "_sort", nil }

	config := ApplyOptions(
		WithValueDecoder(valueDecoder),
		WithFilterFieldResolver(filterResolver),
		WithGroupFieldResolver(groupResolver),
		WithSortFieldResolver(sortResolver),
		WithLiteralCodec(fakeLiteralCodec{}),
		WithMode(Compact),
	)

	if reflect.ValueOf(config.ValueDecoder).Pointer() != reflect.ValueOf(valueDecoder).Pointer() {
		t.Fatal("expected value decoder to be applied")
	}

	if reflect.ValueOf(config.FilterFieldResolver).Pointer() != reflect.ValueOf(filterResolver).Pointer() {
		t.Fatal("expected filter resolver to be applied")
	}

	if reflect.ValueOf(config.GroupFieldResolver).Pointer() != reflect.ValueOf(groupResolver).Pointer() {
		t.Fatal("expected group resolver to be applied")
	}

	if reflect.ValueOf(config.SortFieldResolver).Pointer() != reflect.ValueOf(sortResolver).Pointer() {
		t.Fatal("expected sort resolver to be applied")
	}

	if _, ok := config.LiteralCodec.(fakeLiteralCodec); !ok || config.Mode != Compact {
		t.Fatalf("unexpected config: %#v", config)
	}

	defaults := ApplyOptions(WithLiteralCodec(nil), WithMode(""))
	if defaults.LiteralCodec == nil || defaults.Mode != Canonical {
		t.Fatalf("unexpected default config: %#v", defaults)
	}
}

func TestDefaultLiteralCodecBuiltinsAndModeParsing(t *testing.T) {
	previousMode := DefaultLiteralCodecModeValue()
	defer func() {
		_ = SetDefaultLiteralCodecMode(previousMode)
	}()

	if err := SetDefaultLiteralCodecMode(" reversible-text "); err != nil {
		t.Fatalf("SetDefaultLiteralCodecMode() error = %v", err)
	}

	if got := DefaultLiteralCodecModeValue(); got != LiteralCodecModeReversibleText {
		t.Fatalf("unexpected default mode: %q", got)
	}

	if err := SetDefaultLiteralCodecMode("nope"); err == nil {
		t.Fatal("expected invalid mode error")
	}

	t.Setenv(DefaultLiteralCodecModeEnv, "strict")
	if err := ResetDefaultLiteralCodecModeFromEnv(); err != nil {
		t.Fatalf("ResetDefaultLiteralCodecModeFromEnv() error = %v", err)
	}

	t.Setenv(DefaultLiteralCodecModeEnv, "broken")
	if err := ResetDefaultLiteralCodecModeFromEnv(); err == nil {
		t.Fatal("expected invalid env mode error")
	}

	if got := loadDefaultLiteralCodecModeFromEnv(); got != LiteralCodecModeStrict {
		t.Fatalf("unexpected env fallback mode: %q", got)
	}

	if got, err := parseLiteralCodecMode(""); err != nil || got != LiteralCodecModeStrict {
		t.Fatalf("unexpected parseLiteralCodecMode() result: %q %v", got, err)
	}

	if got, err := normalizeLiteralCodecMode("reversible-text"); err != nil || got != LiteralCodecModeReversibleText {
		t.Fatalf("unexpected normalizeLiteralCodecMode() result: %q %v", got, err)
	}

	codec := DefaultLiteralCodec{}
	now := time.Date(2026, 4, 18, 12, 34, 56, 789, time.UTC)

	tests := []struct {
		name      string
		value     any
		wantCodec string
	}{
		{"time", now, "time"},
		{"duration", 3 * time.Minute, "duration"},
		{"uuid", uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"), "uuid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			literal, codecName, handled, err := codec.FormatLiteral(tc.value)
			if err != nil || !handled || codecName != tc.wantCodec {
				t.Fatalf("unexpected FormatLiteral() result: literal=%#v codec=%q handled=%v err=%v", literal, codecName, handled, err)
			}

			parsed, handled, err := codec.ParseLiteral(codecName, literal)
			if err != nil || !handled {
				t.Fatalf("unexpected ParseLiteral() result: value=%#v handled=%v err=%v", parsed, handled, err)
			}
		})
	}

	if value, handled, err := codec.ParseLiteral("", "x"); value != nil || handled || err != nil {
		t.Fatalf("unexpected ParseLiteral(empty) result: %#v %v %v", value, handled, err)
	}
}

func TestReversibleTextRegistrationAndHelpers(t *testing.T) {
	const codecName = "codecconfig_test_text"

	if err := RegisterReversibleTextType(codecName, &reversibleTextValue{}); err != nil {
		t.Fatalf("RegisterReversibleTextType() error = %v", err)
	}

	tests := []struct {
		name      string
		codec     string
		prototype any
	}{
		{name: "empty codec", codec: "", prototype: &reversibleTextValue{}},
		{name: "nil prototype", codec: "nil_proto", prototype: nil},
		{name: "non pointer", codec: "non_ptr", prototype: reversibleTextValue{}},
		{name: "missing interfaces", codec: "bad_proto", prototype: new(struct{})},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := RegisterReversibleTextType(tc.codec, tc.prototype); err == nil {
				t.Fatalf("expected RegisterReversibleTextType() to fail for %s", tc.name)
			}
		})
	}

	codec := DefaultLiteralCodec{Mode: LiteralCodecModeReversibleText}
	literal, gotCodec, handled, err := codec.FormatLiteral(reversibleTextValue{value: "hello"})
	if err != nil || !handled || gotCodec != codecName || literal != "hello" {
		t.Fatalf("unexpected reversible FormatLiteral() result: %#v %q %v %v", literal, gotCodec, handled, err)
	}

	parsed, handled, err := codec.ParseLiteral(codecName, literal)
	if err != nil || !handled || parsed.(reversibleTextValue).value != "hello" {
		t.Fatalf("unexpected reversible ParseLiteral() result: %#v %v %v", parsed, handled, err)
	}

	if _, handled, err := codec.ParseLiteral(codecName, "bad"); !handled || err == nil {
		t.Fatalf("expected reversible ParseLiteral() to fail on bad input, got handled=%v err=%v", handled, err)
	}

	if _, codecName, handled, err := codec.FormatLiteral(reversibleTextValue{value: "marshal-error"}); !handled || codecName != "" || err == nil {
		t.Fatalf("expected reversible FormatLiteral() marshal failure, got codec=%q handled=%v err=%v", codecName, handled, err)
	}

	if _, handled, err := parseRegisteredReversibleText("unknown_codec", "value"); handled || err != nil {
		t.Fatalf("unexpected parseRegisteredReversibleText() result: handled=%v err=%v", handled, err)
	}

	if _, _, ok := lookupReversibleTextSpec(nil); ok {
		t.Fatal("expected lookupReversibleTextSpec(nil) to fail")
	}

	if codecName, _, ok := lookupReversibleTextSpec(reversibleTextValue{value: "hello"}); !ok || codecName != "codecconfig_test_text" {
		t.Fatalf("unexpected lookupReversibleTextSpec() result: %q %v", codecName, ok)
	}

	if _, err := makeTextMarshaler(nil, reversibleTextSpec{}); err == nil {
		t.Fatal("expected makeTextMarshaler(nil) to fail")
	}

	if _, err := makeTextMarshaler(&struct{}{}, reversibleTextSpec{pointerType: reflect.TypeOf(&reversibleTextValue{})}); err == nil {
		t.Fatal("expected makeTextMarshaler(pointer without marshaler) to fail")
	}
}
