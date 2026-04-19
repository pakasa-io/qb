package sqlrender

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pakasa-io/qb"
)

type fakeDialect struct{}

func (fakeDialect) Name() string { return "fake" }

func (fakeDialect) QuoteIdentifier(identifier string) string {
	return "[" + identifier + "]"
}

func (fakeDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (fakeDialect) CompileFunction(name string, args []string) (string, error) {
	if strings.EqualFold(name, "explode") {
		return "", errors.New("explode failed")
	}
	return strings.ToUpper(name) + "(" + strings.Join(args, ", ") + ")", nil
}

func (fakeDialect) CompileCast(expr string, typeName string) (string, error) {
	if typeName == "bad" {
		return "", errors.New("cast failed")
	}
	return "CAST(" + expr + " AS " + strings.ToUpper(typeName) + ")", nil
}

func (fakeDialect) CompilePredicate(op qb.Operator, left string, right string) (string, bool, error) {
	switch op {
	case qb.OpRegexp:
		return left + " ~ " + right, true, nil
	case qb.OpILike:
		return "", true, errors.New("predicate failed")
	default:
		return "", false, nil
	}
}

func (fakeDialect) Capabilities() qb.Capabilities {
	return qb.Capabilities{
		Operators: map[qb.Operator]struct{}{
			qb.OpRegexp: {},
			qb.OpILike:  {},
		},
	}
}

func TestRendererCompilesListsAndExpressions(t *testing.T) {
	renderer := New(fakeDialect{}, qb.StageCompile)

	if renderer.Dialect().Name() != "fake" {
		t.Fatalf("unexpected dialect: %s", renderer.Dialect().Name())
	}

	if !renderer.Capabilities().SupportsOperator(qb.OpRegexp) {
		t.Fatalf("unexpected capabilities: %+v", renderer.Capabilities())
	}

	projections, args, nextArg, err := renderer.CompileProjectionList(
		[]qb.Projection{
			qb.Project(qb.F("users.name")).As("name"),
			qb.Project(qb.V(7)),
		},
		1,
	)
	if err != nil {
		t.Fatalf("CompileProjectionList() error = %v", err)
	}

	if projections != "[users.name] AS [name], $1" || nextArg != 2 || len(args) != 1 || args[0] != 7 {
		t.Fatalf("unexpected compiled projections: sql=%q args=%#v next=%d", projections, args, nextArg)
	}

	scalars, args, nextArg, err := renderer.CompileScalarList(
		[]qb.Scalar{qb.F("users.name"), qb.Lower(qb.F("users.name"))},
		"group_by",
		2,
	)
	if err != nil {
		t.Fatalf("CompileScalarList() error = %v", err)
	}

	if scalars != "[users.name], LOWER([users.name])" || nextArg != 2 || len(args) != 0 {
		t.Fatalf("unexpected compiled scalar list: sql=%q args=%#v next=%d", scalars, args, nextArg)
	}

	sorts, args, nextArg, err := renderer.CompileSorts(
		[]qb.Sort{{Expr: qb.F("users.name"), Direction: ""}},
		1,
	)
	if err != nil {
		t.Fatalf("CompileSorts() error = %v", err)
	}

	if sorts != "[users.name] ASC" || len(args) != 0 || nextArg != 1 {
		t.Fatalf("unexpected compiled sorts: sql=%q args=%#v next=%d", sorts, args, nextArg)
	}

	expr, args, nextArg, err := renderer.CompileExpr(
		qb.And(
			qb.F("deleted_at").Eq(nil),
			qb.Not(qb.F("role").In("admin", "owner")),
			qb.F("name").Contains("adm"),
			qb.F("name").Prefix(qb.F("suffix_source")),
			qb.F("name").Regexp("a.*"),
		),
		1,
	)
	if err != nil {
		t.Fatalf("CompileExpr() error = %v", err)
	}

	wantSQL := `([deleted_at] IS NULL AND NOT ([role] IN ($1, $2)) AND [name] LIKE $3 AND [name] LIKE CONCAT([suffix_source], $4) AND [name] ~ $5)`
	if expr != wantSQL || nextArg != 6 {
		t.Fatalf("unexpected compiled expr:\nwant: %s\ngot:  %s\nargs: %#v next: %d", wantSQL, expr, args, nextArg)
	}

	wantArgs := []any{"admin", "owner", "%adm%", "%", "a.*"}
	if len(args) != len(wantArgs) {
		t.Fatalf("unexpected arg count: got %#v want %#v", args, wantArgs)
	}

	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: got %#v want %#v", i, args[i], wantArgs[i])
		}
	}
}

func TestRendererValidationAndFailurePaths(t *testing.T) {
	renderer := New(fakeDialect{}, qb.StageCompile)
	tests := []struct {
		name         string
		compile      func() (string, []any, int, error)
		wantCode     qb.ErrorCode
		wantField    string
		wantOperator qb.Operator
		wantMessage  string
	}{
		{
			name:        "nil projection expression",
			compile:     func() (string, []any, int, error) { return renderer.CompileProjectionList([]qb.Projection{{}}, 1) },
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: select expression cannot be nil",
		},
		{
			name:        "nil scalar expression",
			compile:     func() (string, []any, int, error) { return renderer.CompileScalarList([]qb.Scalar{nil}, "group_by", 1) },
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: scalar expression cannot be nil",
		},
		{
			name:        "nil sort expression",
			compile:     func() (string, []any, int, error) { return renderer.CompileSorts([]qb.Sort{{Expr: nil}}, 1) },
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: sort expression cannot be nil",
		},
		{
			name: "invalid sort direction",
			compile: func() (string, []any, int, error) {
				return renderer.CompileSorts([]qb.Sort{{Expr: qb.F("name"), Direction: qb.Direction("sideways")}}, 1)
			},
			wantCode:    qb.CodeInvalidQuery,
			wantField:   "name",
			wantMessage: `compile invalid_query field=name: unsupported sort direction "sideways"`,
		},
		{
			name:        "empty group",
			compile:     func() (string, []any, int, error) { return renderer.CompileExpr(qb.Group{Kind: qb.AndGroup}, 1) },
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: empty and group",
		},
		{
			name: "empty IN list",
			compile: func() (string, []any, int, error) {
				return renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpIn, Right: qb.ListOperand{}}, 1)
			},
			wantCode:     qb.CodeInvalidValue,
			wantField:    "name",
			wantOperator: qb.OpIn,
			wantMessage:  "compile invalid_value field=name op=in: in requires a non-empty list",
		},
		{
			name: "LIKE list operand",
			compile: func() (string, []any, int, error) {
				return renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpLike, Right: qb.ListOperand{}}, 1)
			},
			wantCode:     qb.CodeInvalidValue,
			wantField:    "name",
			wantOperator: qb.OpLike,
			wantMessage:  "compile invalid_value field=name op=like: like requires a scalar operand",
		},
		{
			name: "dialect predicate compiler error",
			compile: func() (string, []any, int, error) {
				return renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpILike, Right: qb.ScalarOperand{Expr: qb.V("a")}}, 1)
			},
			wantCode:     qb.CodeInvalidQuery,
			wantField:    "name",
			wantOperator: qb.OpILike,
			wantMessage:  "compile invalid_query field=name op=ilike: predicate failed",
		},
		{
			name: "binary predicate with list operand",
			compile: func() (string, []any, int, error) {
				return renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpGt, Right: qb.ListOperand{Items: []qb.Scalar{qb.V(1)}}}, 1)
			},
			wantCode:     qb.CodeInvalidValue,
			wantField:    "name",
			wantOperator: qb.OpGt,
			wantMessage:  "compile invalid_value field=name op=gt: gt requires a scalar operand",
		},
		{
			name: "unsupported operator",
			compile: func() (string, []any, int, error) {
				return New(noopDialect{}, qb.StageCompile).CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpRegexp, Right: qb.ScalarOperand{Expr: qb.V("a")}}, 1)
			},
			wantCode:     qb.CodeUnsupportedOperator,
			wantField:    "name",
			wantOperator: qb.OpRegexp,
			wantMessage:  `compile unsupported_operator field=name op=regexp: operator "regexp" is not supported`,
		},
		{
			name: "nil right scalar",
			compile: func() (string, []any, int, error) {
				return renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpEq, Right: qb.ScalarOperand{Expr: nil}}, 1)
			},
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: scalar expression cannot be nil",
		},
		{
			name:        "nil scalar",
			compile:     func() (string, []any, int, error) { return renderer.CompileScalar(nil, 1) },
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: scalar expression cannot be nil",
		},
		{
			name:        "empty ref",
			compile:     func() (string, []any, int, error) { return renderer.CompileScalar(qb.F(""), 1) },
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: field reference cannot be empty",
		},
		{
			name:        "function compiler error",
			compile:     func() (string, []any, int, error) { return renderer.CompileScalar(qb.Func("explode"), 1) },
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: explode failed",
		},
		{
			name:        "cast compiler error",
			compile:     func() (string, []any, int, error) { return renderer.CompileScalar(qb.F("score").Cast("bad"), 1) },
			wantCode:    qb.CodeInvalidQuery,
			wantMessage: "compile invalid_query: cast failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sql, args, nextArg, err := tc.compile()
			if err == nil {
				t.Fatalf("expected %s to fail", tc.name)
			}

			if sql != "" || args != nil || nextArg != 1 {
				t.Fatalf("expected no partial compile output, got sql=%q args=%#v next=%d", sql, args, nextArg)
			}

			assertRendererDiagnostic(t, err, tc.wantCode, tc.wantField, tc.wantOperator, tc.wantMessage)
		})
	}

	if sql, args, nextArg, err := renderer.CompileScalar(qb.V(nil), 3); err != nil || sql != "NULL" || len(args) != 0 || nextArg != 3 {
		t.Fatalf("unexpected nil literal compile: sql=%q args=%#v next=%d err=%v", sql, args, nextArg, err)
	}

	if !operandIsNull(qb.ScalarOperand{Expr: qb.V(nil)}) || operandIsNull(qb.ListOperand{}) {
		t.Fatal("unexpected operandIsNull() behavior")
	}

	if got := predicateField(qb.Func("concat", qb.F("first"), qb.F("last"))); got != "" {
		t.Fatalf("unexpected predicateField() result: %q", got)
	}
}

type noopDialect struct{}

func (noopDialect) Name() string { return "noop" }
func (noopDialect) QuoteIdentifier(identifier string) string {
	return identifier
}
func (noopDialect) Placeholder(index int) string { return fmt.Sprintf("$%d", index) }
func (noopDialect) CompileFunction(name string, args []string) (string, error) {
	return name, nil
}
func (noopDialect) CompileCast(expr string, typeName string) (string, error) {
	return expr, nil
}
func (noopDialect) CompilePredicate(op qb.Operator, left string, right string) (string, bool, error) {
	return "", false, nil
}
func (noopDialect) Capabilities() qb.Capabilities { return qb.Capabilities{} }

func assertRendererDiagnostic(t *testing.T, err error, wantCode qb.ErrorCode, wantField string, wantOperator qb.Operator, wantMessage string) {
	t.Helper()

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected qb.Error, got %T", err)
	}

	if diagnostic.Stage != qb.StageCompile || diagnostic.Code != wantCode || diagnostic.Field != wantField || diagnostic.Operator != wantOperator {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}

	if diagnostic.Error() != wantMessage {
		t.Fatalf("unexpected diagnostic message: %q", diagnostic.Error())
	}
}
