package schema

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/pakasa-io/qb"
)

func TestSchemaConstructionLookupsAndPanics(t *testing.T) {
	if _, err := New(Field{}); err == nil {
		t.Fatal("expected empty field name to fail")
	}

	if _, err := New(Define("id"), Define("id")); err == nil {
		t.Fatal("expected duplicate field to fail")
	}

	if _, err := New(Define("id", Aliases(""))); err == nil {
		t.Fatal("expected empty alias to fail")
	}

	if _, err := New(Define("id", Aliases("alias")), Define("status", Aliases("alias"))); err == nil {
		t.Fatal("expected duplicate alias to fail")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected MustNew() to panic on invalid schemas")
		}
	}()
	_ = MustNew(Field{})
}

func TestSchemaResolversAndHelpers(t *testing.T) {
	s := MustNew(
		Define("status", Storage("users.status"), Aliases("state"), Sortable(), Operators(qb.OpEq, qb.OpIn)),
		Define("age", Storage("users.age"), Sortable(), Decode(func(_ qb.Operator, value any) (any, error) {
			switch typed := value.(type) {
			case string:
				return strconv.Atoi(typed)
			default:
				return value, nil
			}
		})),
		Define("hidden", DisableFiltering()),
	)

	if fields := s.Fields(); !reflect.DeepEqual(fields, []string{"age", "hidden", "status"}) {
		t.Fatalf("unexpected Fields() result: %#v", fields)
	}

	if got, err := s.ResolveFilterField("state", qb.OpEq); err != nil || got != "status" {
		t.Fatalf("unexpected ResolveFilterField() result: %q %v", got, err)
	}

	if _, err := s.ResolveFilterField("hidden", qb.OpEq); err == nil {
		t.Fatal("expected ResolveFilterField() to reject disabled filtering")
	}

	if _, err := s.ResolveFilterField("status", qb.OpGt); err == nil {
		t.Fatal("expected ResolveFilterField() to reject unsupported operators")
	}

	if got, err := s.ResolveSortField("state"); err != nil || got != "status" {
		t.Fatalf("unexpected ResolveSortField() result: %q %v", got, err)
	}

	if _, err := s.ResolveSortField("hidden"); err == nil {
		t.Fatal("expected ResolveSortField() to reject unsortable fields")
	}

	for _, resolver := range []struct {
		name string
		fn   func(string) (string, error)
		want string
	}{
		{name: "group", fn: s.ResolveGroupField, want: "status"},
		{name: "cursor", fn: s.ResolveCursorField, want: "status"},
		{name: "storage", fn: s.ResolveStorageField, want: "users.status"},
		{name: "field", fn: s.ResolveField, want: "status"},
	} {
		t.Run(resolver.name, func(t *testing.T) {
			got, err := resolver.fn("state")
			if err != nil || got != resolver.want {
				t.Fatalf("unexpected resolver result: %q %v", got, err)
			}
		})
	}

	if value, err := s.DecodeValue("age", qb.OpEq, "21"); err != nil || value != 21 {
		t.Fatalf("unexpected DecodeValue() result: %#v %v", value, err)
	}

	if _, err := s.DecodeValue("missing", qb.OpEq, "21"); err == nil {
		t.Fatal("expected DecodeValue() to reject unknown fields")
	}

	badSchema := MustNew(
		Define("age", Decode(func(_ qb.Operator, value any) (any, error) {
			return nil, errors.New("decode failed")
		})),
	)
	if _, err := badSchema.DecodeValue("age", qb.OpEq, "21"); err == nil {
		t.Fatal("expected DecodeValue() to wrap decoder failures")
	}
}

func TestSchemaNormalizationAndProjectionHelpers(t *testing.T) {
	s := MustNew(
		Define("status", Storage("users.status"), Aliases("state"), Sortable()),
		Define("created_at", Storage("users.created_at"), Aliases("createdAt"), Decode(func(_ qb.Operator, value any) (any, error) {
			if text, ok := value.(string); ok {
				return "parsed:" + text, nil
			}
			return value, nil
		})),
		Define("age", Decode(func(_ qb.Operator, value any) (any, error) {
			if text, ok := value.(string); ok {
				return strconv.Atoi(text)
			}
			return value, nil
		})),
	)

	operand, err := s.normalizeOperand(qb.ListOperand{Items: []qb.Scalar{qb.V("21"), qb.F("state")}}, "age", qb.OpIn)
	if err != nil {
		t.Fatalf("normalizeOperand() error = %v", err)
	}

	list := operand.(qb.ListOperand)
	if list.Items[0].(qb.Literal).Value != 21 || list.Items[1].(qb.Ref).Name != "status" {
		t.Fatalf("unexpected normalized operand: %#v", list)
	}

	projected, err := s.projectOperand(qb.ScalarOperand{Expr: qb.F("state")})
	if err != nil {
		t.Fatalf("projectOperand() error = %v", err)
	}

	if got := projected.(qb.ScalarOperand).Expr.(qb.Ref).Name; got != "users.status" {
		t.Fatalf("unexpected projected operand: %#v", projected)
	}

	if got, err := s.normalizeCursor(&qb.Cursor{Values: map[string]any{"createdAt": "2026-04-18T00:00:00Z"}}); err != nil || got.Values["created_at"] != "parsed:2026-04-18T00:00:00Z" {
		t.Fatalf("unexpected normalizeCursor() result: %#v %v", got, err)
	}

	if _, err := s.normalizeCursor(&qb.Cursor{Values: map[string]any{"createdAt": "x", "created_at": "y"}}); err == nil {
		t.Fatal("expected normalizeCursor() to detect duplicate normalized fields")
	}

	duplicateStorage := MustNew(
		Define("state", Storage("users.status")),
		Define("status", Storage("users.status")),
	)
	if _, err := duplicateStorage.projectCursor(&qb.Cursor{Values: map[string]any{"state": "x", "status": "y"}}); err == nil {
		t.Fatal("expected projectCursor() to detect duplicate storage fields")
	}

	if _, err := s.rewriteScalars([]qb.Scalar{qb.F("missing")}, s.ResolveField, qb.StageNormalize); err == nil {
		t.Fatal("expected rewriteScalars() to fail on unknown fields")
	}

	if _, err := s.rewriteProjections([]qb.Projection{{Expr: qb.F("missing")}}, s.ResolveStorageField, qb.StageRewrite); err == nil {
		t.Fatal("expected rewriteProjections() to fail on unknown fields")
	}

	rewritten, err := s.rewriteScalar(qb.F("state"), s.ResolveField, qb.StageNormalize)
	if err != nil || rewritten.(qb.Ref).Name != "status" {
		t.Fatalf("unexpected rewriteScalar() result: %#v %v", rewritten, err)
	}

	if got := predicatePrimaryField(qb.F("status")); got != "status" {
		t.Fatalf("unexpected predicatePrimaryField() result: %q", got)
	}

	if got, ok := anySlice([]string{"a", "b"}); !ok || len(got) != 2 || got[1] != "b" {
		t.Fatalf("unexpected anySlice([]string) result: %#v %v", got, ok)
	}

	if _, ok := anySlice([]byte("abc")); ok {
		t.Fatal("expected anySlice([]byte) to return false")
	}
}
