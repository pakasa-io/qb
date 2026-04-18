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

	if _, _, _, err := renderer.CompileProjectionList([]qb.Projection{{}}, 1); err == nil {
		t.Fatal("expected nil projection expression to fail")
	}

	if _, _, _, err := renderer.CompileScalarList([]qb.Scalar{nil}, "group_by", 1); err == nil {
		t.Fatal("expected nil scalar expression to fail")
	}

	if _, _, _, err := renderer.CompileSorts([]qb.Sort{{Expr: nil}}, 1); err == nil {
		t.Fatal("expected nil sort expression to fail")
	}

	if _, _, _, err := renderer.CompileSorts([]qb.Sort{{Expr: qb.F("name"), Direction: qb.Direction("sideways")}}, 1); err == nil {
		t.Fatal("expected invalid sort direction to fail")
	}

	if _, _, _, err := renderer.CompileExpr(qb.Group{Kind: qb.AndGroup}, 1); err == nil {
		t.Fatal("expected empty group to fail")
	}

	if _, _, _, err := renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpIn, Right: qb.ListOperand{}}, 1); err == nil {
		t.Fatal("expected empty IN list to fail")
	}

	if _, _, _, err := renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpLike, Right: qb.ListOperand{}}, 1); err == nil {
		t.Fatal("expected LIKE list operand to fail")
	}

	if _, _, _, err := renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpILike, Right: qb.ScalarOperand{Expr: qb.V("a")}}, 1); err == nil {
		t.Fatal("expected dialect predicate compiler error")
	}

	if _, _, _, err := renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpGt, Right: qb.ListOperand{Items: []qb.Scalar{qb.V(1)}}}, 1); err == nil {
		t.Fatal("expected binary predicate with list operand to fail")
	}

	noCapability := New(noopDialect{}, qb.StageCompile)
	if _, _, _, err := noCapability.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpRegexp, Right: qb.ScalarOperand{Expr: qb.V("a")}}, 1); err == nil {
		t.Fatal("expected unsupported operator to fail")
	}

	if _, _, _, err := renderer.CompileExpr(qb.Predicate{Left: qb.F("name"), Op: qb.OpEq, Right: qb.ScalarOperand{Expr: nil}}, 1); err == nil {
		t.Fatal("expected nil right scalar to fail")
	}

	if _, _, _, err := renderer.CompileScalar(nil, 1); err == nil {
		t.Fatal("expected nil scalar to fail")
	}

	if _, _, _, err := renderer.CompileScalar(qb.F(""), 1); err == nil {
		t.Fatal("expected empty ref to fail")
	}

	if sql, args, nextArg, err := renderer.CompileScalar(qb.V(nil), 3); err != nil || sql != "NULL" || len(args) != 0 || nextArg != 3 {
		t.Fatalf("unexpected nil literal compile: sql=%q args=%#v next=%d err=%v", sql, args, nextArg, err)
	}

	if _, _, _, err := renderer.CompileScalar(qb.Func("explode"), 1); err == nil {
		t.Fatal("expected function compiler error")
	}

	if _, _, _, err := renderer.CompileScalar(qb.F("score").Cast("bad"), 1); err == nil {
		t.Fatal("expected cast compiler error")
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
