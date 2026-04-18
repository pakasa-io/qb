package qb

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type customScalar struct {
	label string
}

func (customScalar) scalarNode() {}

type customOperand struct{}

func (customOperand) operandNode() {}

type customExpr struct{}

func (customExpr) exprNode() {}

func TestBuilderAliasesAndValidation(t *testing.T) {
	query, err := New().
		Pick("id").
		PickExpr(Lower(F("name"))).
		PickProjection(Project(F("status")).As("state")).
		Include("orders.items").
		GroupByExpr(F("status")).
		SortByExpr(F("created_at"), "").
		Limit(10).
		Offset(5).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(query.Projections) != 3 {
		t.Fatalf("unexpected projections: %#v", query.Projections)
	}

	if query.Projections[2].Alias != "state" {
		t.Fatalf("unexpected projection alias: %#v", query.Projections[2])
	}

	if len(query.Includes) != 1 || query.Includes[0] != "orders.items" {
		t.Fatalf("unexpected includes: %#v", query.Includes)
	}

	if len(query.GroupBy) != 1 {
		t.Fatalf("unexpected group by: %#v", query.GroupBy)
	}

	if len(query.Sorts) != 1 || query.Sorts[0].Direction != Asc {
		t.Fatalf("unexpected sorts: %#v", query.Sorts)
	}

	if query.Limit == nil || *query.Limit != 10 || query.Offset == nil || *query.Offset != 5 {
		t.Fatalf("unexpected pagination fields: %+v", query)
	}

	if _, err := New().Size(3).CursorToken("opaque").Query(); err != nil {
		t.Fatalf("CursorToken() query error = %v", err)
	}

	tests := []struct {
		name  string
		build func() Builder
	}{
		{name: "empty pick field", build: func() Builder { return New().Pick("") }},
		{name: "nil projection", build: func() Builder { return New().PickProjection(Projection{}) }},
		{name: "nil group expr", build: func() Builder { return New().GroupByExpr(nil) }},
		{name: "empty include", build: func() Builder { return New().Include("") }},
		{name: "nil sort expr", build: func() Builder { return New().SortByExpr(nil, Asc) }},
		{name: "invalid sort direction", build: func() Builder { return New().SortByExpr(F("id"), Direction("sideways")) }},
		{name: "page too small", build: func() Builder { return New().Page(0) }},
		{name: "size too small", build: func() Builder { return New().Size(0) }},
		{name: "negative limit", build: func() Builder { return New().Limit(-1) }},
		{name: "negative offset", build: func() Builder { return New().Offset(-1) }},
		{name: "empty cursor token", build: func() Builder { return New().CursorToken("") }},
		{name: "empty cursor values", build: func() Builder { return New().CursorValues(nil) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.build().Query(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestQueryPaginationAndTransformers(t *testing.T) {
	limit, offset, err := (Query{Size: intPtr(7)}).ResolvedPagination()
	if err != nil {
		t.Fatalf("ResolvedPagination() error = %v", err)
	}
	if limit == nil || *limit != 7 || offset != nil {
		t.Fatalf("unexpected cursor-style pagination: limit=%v offset=%v", limit, offset)
	}

	tests := []struct {
		name  string
		query Query
	}{
		{name: "page and cursor", query: Query{Page: intPtr(1), Size: intPtr(10), Cursor: &Cursor{Token: "x"}}},
		{name: "cursor and limit", query: Query{Cursor: &Cursor{Token: "x"}, Size: intPtr(10), Limit: intPtr(5)}},
		{name: "page and limit", query: Query{Page: intPtr(1), Size: intPtr(10), Limit: intPtr(5)}},
		{name: "size and limit", query: Query{Size: intPtr(10), Limit: intPtr(5)}},
		{name: "page without size", query: Query{Page: intPtr(2)}},
		{name: "cursor without size", query: Query{Cursor: &Cursor{Token: "x"}}},
		{name: "page less than one", query: Query{Page: intPtr(0), Size: intPtr(10)}},
		{name: "size less than one", query: Query{Size: intPtr(0)}},
		{name: "negative limit", query: Query{Limit: intPtr(-1)}},
		{name: "negative offset", query: Query{Offset: intPtr(-1)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := tc.query.ResolvedPagination(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}

	original := Query{
		Projections: []Projection{Project(F("status"))},
		Includes:    []string{"orders"},
		Filter:      F("status").Eq("active"),
	}

	transformed, err := TransformQuery(
		original,
		nil,
		ComposeTransformers(
			func(query Query) (Query, error) {
				query.Projections[0] = Project(F("state"))
				return query, nil
			},
			func(query Query) (Query, error) {
				query.Includes[0] = "mutated"
				return query, nil
			},
		),
	)
	if err != nil {
		t.Fatalf("TransformQuery() error = %v", err)
	}

	if got := transformed.Projections[0].Expr.(Ref).Name; got != "state" {
		t.Fatalf("unexpected transformed projection: %#v", transformed.Projections)
	}

	if transformed.Includes[0] != "mutated" {
		t.Fatalf("unexpected transformed includes: %#v", transformed.Includes)
	}

	if got := original.Projections[0].Expr.(Ref).Name; got != "status" || original.Includes[0] != "orders" {
		t.Fatalf("expected TransformQuery to clone input, got %#v", original)
	}

	if got := ErrInvalidPagination("bad").Error(); got != "bad" {
		t.Fatalf("unexpected ErrInvalidPagination message: %q", got)
	}
}

func TestExpressionsErrorsCapabilitiesAndWalkers(t *testing.T) {
	filter := And(
		F("status").Eq("active"),
		And(F("role").Eq("admin"), F("tenant_id").Eq(7)),
	)

	group, ok := filter.(Group)
	if !ok || group.Kind != AndGroup || len(group.Terms) != 3 {
		t.Fatalf("unexpected flattened group: %#v", filter)
	}

	if got := Or(F("status").Eq("active")); reflect.TypeOf(got) != reflect.TypeOf(Predicate{}) {
		t.Fatalf("expected single Or() term to unwrap, got %T", got)
	}

	if Not(nil) != nil {
		t.Fatal("expected Not(nil) to return nil")
	}

	var walkOrder []string
	if err := Walk(Not(filter), func(expr Expr) error {
		walkOrder = append(walkOrder, fmt.Sprintf("%T", expr))
		return nil
	}); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if len(walkOrder) != 5 || walkOrder[0] != "qb.Negation" || walkOrder[1] != "qb.Group" {
		t.Fatalf("unexpected walk order: %#v", walkOrder)
	}

	wantStop := errors.New("stop")
	if err := Walk(filter, func(expr Expr) error {
		if _, ok := expr.(Predicate); ok {
			return wantStop
		}
		return nil
	}); !errors.Is(err, wantStop) {
		t.Fatalf("expected early walk stop, got %v", err)
	}

	cloned := CloneExpr(filter).(Group)
	cloned.Terms[0] = F("mutated").Eq("inactive")
	first := group.Terms[0].(Predicate)
	if got := first.Left.(Ref).Name; got != "status" {
		t.Fatalf("expected CloneExpr() to protect original filter, got %q", got)
	}

	rewritten, err := RewriteExpr(filter, func(expr Expr) (Expr, error) {
		predicate, ok := expr.(Predicate)
		if !ok {
			return expr, nil
		}

		ref := predicate.Left.(Ref)
		if ref.Name == "status" {
			predicate.Left = F("state")
		}
		return predicate, nil
	})
	if err != nil {
		t.Fatalf("RewriteExpr() error = %v", err)
	}

	rewrittenGroup := rewritten.(Group)
	if got := rewrittenGroup.Terms[0].(Predicate).Left.(Ref).Name; got != "state" {
		t.Fatalf("unexpected rewritten predicate: %#v", rewrittenGroup.Terms[0])
	}

	queryClone, err := RewriteQuery(Query{Includes: []string{"orders"}}, nil)
	if err != nil {
		t.Fatalf("RewriteQuery() error = %v", err)
	}
	queryClone.Includes[0] = "mutated"

	query := Query{
		Projections: []Projection{Project(Lower(F("name")))},
		GroupBy:     []Scalar{F("group_field")},
		Sorts:       []Sort{{Expr: F("sort_field"), Direction: Desc}},
		Filter: And(
			F("status").Eq("active"),
			F("role").In("admin", "owner"),
		),
	}

	var scalarKinds []string
	if err := WalkQueryScalars(query, func(expr Scalar) error {
		scalarKinds = append(scalarKinds, fmt.Sprintf("%T", expr))
		return nil
	}); err != nil {
		t.Fatalf("WalkQueryScalars() error = %v", err)
	}

	if len(scalarKinds) == 0 || scalarKinds[0] != "qb.Call" {
		t.Fatalf("unexpected scalar walk: %#v", scalarKinds)
	}

	caps := Capabilities{
		Functions:       map[string]struct{}{"lower": {}},
		Operators:       map[Operator]struct{}{OpEq: {}},
		SupportsSelect:  true,
		SupportsGroupBy: true,
		SupportsSort:    true,
		SupportsLimit:   true,
		SupportsOffset:  true,
		SupportsPage:    true,
		SupportsSize:    true,
	}

	if !caps.SupportsFunction("LOWER") || !caps.SupportsOperator(OpEq) {
		t.Fatalf("unexpected capability support: %+v", caps)
	}

	if err := caps.Validate(StageCompile, query); err == nil {
		t.Fatal("expected unsupported capability error")
	}

	if got := scalarField(F("status")); got != "status" {
		t.Fatalf("unexpected scalarField() result: %q", got)
	}

	validatorQuery := Query{Projections: []Projection{Project(F("id"))}}
	validated, err := (Capabilities{SupportsSelect: true}).Validator(StageCompile)(validatorQuery)
	if err != nil {
		t.Fatalf("Validator() error = %v", err)
	}
	validated.Projections[0] = Project(F("mutated"))
	if got := validatorQuery.Projections[0].Expr.(Ref).Name; got != "id" {
		t.Fatalf("expected Validator() to clone query, got %q", got)
	}

	var diagnostic *Error
	if err := (Capabilities{}).Validate(StageApply, Query{Includes: []string{"orders"}}); !errors.As(err, &diagnostic) {
		t.Fatalf("expected structured capability error, got %v", err)
	}

	var nilDiagnostic *Error
	if nilDiagnostic.Error() != "" || nilDiagnostic.Unwrap() != nil {
		t.Fatal("expected nil diagnostic methods to be safe")
	}

	err = NewError(
		errors.New("boom"),
		WithStage(StageParse),
		WithCode(CodeInvalidInput),
		WithPath("$.where"),
		WithField("status"),
		WithOperator(OpEq),
		WithFunction("lower"),
	)

	var structured *Error
	if !errors.As(err, &structured) {
		t.Fatalf("expected structured error, got %T", err)
	}

	message := structured.Error()
	for _, want := range []string{"parse", "invalid_input", "path=$.where", "field=status", "op=eq", "fn=lower", "boom"} {
		if !strings.Contains(message, want) {
			t.Fatalf("missing %q from error message %q", want, message)
		}
	}

	wrapped := WrapError(err, WithDefaultStage(StageCompile), WithDefaultCode(CodeInvalidQuery), WithPath("$.filter"))
	if !errors.As(wrapped, &structured) {
		t.Fatalf("expected wrapped structured error, got %T", wrapped)
	}

	if structured.Stage != StageParse || structured.Code != CodeInvalidInput || structured.Path != "$.filter" {
		t.Fatalf("unexpected wrapped error: %+v", structured)
	}

	plainWrapped := WrapError(errors.New("plain"), WithDefaultStage(StageRewrite), WithDefaultCode(CodeInvalidQuery))
	if !errors.As(plainWrapped, &structured) {
		t.Fatalf("expected wrapped plain error to become structured, got %T", plainWrapped)
	}

	if structured.Stage != StageRewrite || structured.Code != CodeInvalidQuery {
		t.Fatalf("unexpected wrapped plain error metadata: %+v", structured)
	}

	if WrapError(nil) != nil || NewError(nil) != nil {
		t.Fatal("expected nil errors to stay nil")
	}

	unsupported := UnsupportedFunction(StageCompile, "sqlite", "extract")
	if !errors.As(unsupported, &structured) {
		t.Fatalf("expected UnsupportedFunction() to return qb.Error, got %T", unsupported)
	}

	if structured.Stage != StageCompile || structured.Code != CodeUnsupportedFunction || structured.Function != "extract" {
		t.Fatalf("unexpected unsupported function error: %+v", structured)
	}
}

func TestScalarsOperandsAndHelpers(t *testing.T) {
	if _, ok := AsScalar(42); ok {
		t.Fatal("expected plain integer to not be a scalar")
	}

	if _, ok := AsScalar(F("name")); !ok {
		t.Fatal("expected ref to satisfy AsScalar")
	}

	original := Func("concat", F("name"), []string{"a", "b"}).Cast("string")

	var walked []string
	if err := WalkScalar(original, func(expr Scalar) error {
		walked = append(walked, fmt.Sprintf("%T", expr))
		return nil
	}); err != nil {
		t.Fatalf("WalkScalar() error = %v", err)
	}

	if len(walked) != 4 || walked[0] != "qb.Cast" {
		t.Fatalf("unexpected scalar walk order: %#v", walked)
	}

	wantStop := errors.New("stop")
	if err := WalkScalar(original, func(expr Scalar) error {
		if _, ok := expr.(Literal); ok {
			return wantStop
		}
		return nil
	}); !errors.Is(err, wantStop) {
		t.Fatalf("expected WalkScalar() to stop early, got %v", err)
	}

	rewritten, err := RewriteScalar(original, func(expr Scalar) (Scalar, error) {
		switch typed := expr.(type) {
		case Ref:
			return F("display_name"), nil
		case Literal:
			if values, ok := typed.Value.([]any); ok {
				values[0] = "changed"
			}
			return typed, nil
		default:
			return expr, nil
		}
	})
	if err != nil {
		t.Fatalf("RewriteScalar() error = %v", err)
	}

	rewrittenCast := rewritten.(Cast)
	rewrittenCall := rewrittenCast.Expr.(Call)
	if got := rewrittenCall.Args[0].(Ref).Name; got != "display_name" {
		t.Fatalf("unexpected rewritten call: %#v", rewrittenCall)
	}

	cloned := CloneScalar(original).(Cast)
	clonedArgs := cloned.Expr.(Call).Args
	clonedArgs[0] = F("mutated")
	origArgs := original.Expr.(Call).Args
	if got := origArgs[0].(Ref).Name; got != "name" {
		t.Fatalf("expected CloneScalar() to protect original, got %q", got)
	}

	if got, ok := SingleRef(F("status")); !ok || got != "status" {
		t.Fatalf("unexpected SingleRef() result: %q %v", got, ok)
	}

	if _, ok := SingleRef(Func("concat", F("first"), F("last"))); ok {
		t.Fatal("expected SingleRef() to reject multiple refs")
	}

	if got := CloneValue([]any{F("status"), []string{"a", "b"}}); reflect.TypeOf(got) == nil {
		t.Fatal("expected CloneValue() to return a value")
	}

	if got := prependCallArg(F("name"), Concat, "suffix"); got.Name != "concat" || len(got.Args) != 2 {
		t.Fatalf("unexpected prependCallArg() result: %#v", got)
	}

	flat := flattenScalars([]any{[]string{"a", "b"}})
	if len(flat) != 2 || flat[0].(Literal).Value != "a" {
		t.Fatalf("unexpected flattenScalars() result: %#v", flat)
	}

	if got, ok := anySlice([]int{1, 2}); !ok || len(got) != 2 || got[1] != 2 {
		t.Fatalf("unexpected anySlice([]int) result: %#v %v", got, ok)
	}

	if got, ok := anySlice([2]int{3, 4}); !ok || len(got) != 2 || got[0] != 3 {
		t.Fatalf("unexpected anySlice(array) result: %#v %v", got, ok)
	}

	if _, ok := anySlice([]byte("abc")); ok {
		t.Fatal("expected []byte to stay scalar")
	}

	operand := CloneOperand(ListOperand{Items: []Scalar{F("status"), V(1)}}).(ListOperand)
	operand.Items[0] = F("mutated")

	rewrittenOperand, err := RewriteOperand(ScalarOperand{Expr: F("status")}, func(expr Scalar) (Scalar, error) {
		if ref, ok := expr.(Ref); ok {
			return F(ref.Name + "_rewritten"), nil
		}
		return expr, nil
	})
	if err != nil {
		t.Fatalf("RewriteOperand() error = %v", err)
	}

	if got := rewrittenOperand.(ScalarOperand).Expr.(Ref).Name; got != "status_rewritten" {
		t.Fatalf("unexpected rewritten operand: %#v", rewrittenOperand)
	}

	if got, err := RewriteOperand(customOperand{}, nil); err != nil || reflect.TypeOf(got) != reflect.TypeOf(customOperand{}) {
		t.Fatalf("unexpected RewriteOperand() passthrough: %#v %v", got, err)
	}

	if cloned := CloneScalar(customScalar{label: "x"}); cloned.(customScalar).label != "x" {
		t.Fatalf("unexpected CloneScalar() passthrough: %#v", cloned)
	}

	if rewritten, err := RewriteScalar(customScalar{label: "x"}, nil); err != nil || rewritten.(customScalar).label != "x" {
		t.Fatalf("unexpected RewriteScalar() passthrough: %#v %v", rewritten, err)
	}

	if cloned := CloneExpr(customExpr{}); reflect.TypeOf(cloned) != reflect.TypeOf(customExpr{}) {
		t.Fatalf("unexpected CloneExpr() passthrough: %#v", cloned)
	}
}
