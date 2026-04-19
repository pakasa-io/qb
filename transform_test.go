package qb_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/pakasa-io/qb"
)

func TestRewriteQuery(t *testing.T) {
	query, err := qb.New().
		Where(qb.And(
			qb.Field("state").Eq("active"),
			qb.Field("age").Gte(21),
		)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	rewritten, err := qb.RewriteQuery(query, func(expr qb.Expr) (qb.Expr, error) {
		predicate, ok := expr.(qb.Predicate)
		if !ok {
			return expr, nil
		}

		ref, ok := predicate.Left.(qb.Ref)
		if ok && ref.Name == "state" {
			predicate.Left = qb.F("status")
		}

		return predicate, nil
	})
	if err != nil {
		t.Fatalf("RewriteQuery() error = %v", err)
	}

	group, ok := rewritten.Filter.(qb.Group)
	if !ok {
		t.Fatalf("expected group filter, got %T", rewritten.Filter)
	}

	first, ok := group.Terms[0].(qb.Predicate)
	if !ok {
		t.Fatalf("expected predicate term, got %T", group.Terms[0])
	}

	ref, ok := first.Left.(qb.Ref)
	if !ok || ref.Name != "status" {
		t.Fatalf("expected rewritten field to be status, got %#v", first.Left)
	}

	originalGroup, ok := query.Filter.(qb.Group)
	if !ok {
		t.Fatalf("expected original filter to remain grouped, got %T", query.Filter)
	}

	originalFirst := originalGroup.Terms[0].(qb.Predicate)
	if got := originalFirst.Left.(qb.Ref).Name; got != "state" {
		t.Fatalf("expected RewriteQuery to preserve original filter, got %#v", query.Filter)
	}
}

func TestCapabilitiesValidateStructuredError(t *testing.T) {
	capabilities := qb.Capabilities{
		Operators:      map[qb.Operator]struct{}{qb.OpEq: {}},
		SupportsSort:   true,
		SupportsLimit:  true,
		SupportsOffset: true,
	}

	query, err := qb.New().
		Where(qb.Field("age").Gt(18)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	err = capabilities.Validate(qb.StageCompile, query)
	if err == nil {
		t.Fatal("expected capability validation error")
	}

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected qb.Error, got %T", err)
	}

	if diagnostic.Stage != qb.StageCompile || diagnostic.Code != qb.CodeUnsupportedOperator || diagnostic.Field != "age" || diagnostic.Operator != qb.OpGt {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}

	if diagnostic.Error() != `compile unsupported_operator field=age op=gt: operator "gt" is not supported` {
		t.Fatalf("unexpected diagnostic message: %q", diagnostic.Error())
	}
}

func TestTransformQueryReturnsUnderlyingError(t *testing.T) {
	wantErr := fmt.Errorf("boom")
	_, err := qb.TransformQuery(
		qb.Query{},
		func(query qb.Query) (qb.Query, error) {
			return qb.Query{}, wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected transform error, got %v", err)
	}

	var diagnostic *qb.Error
	if errors.As(err, &diagnostic) {
		t.Fatalf("expected TransformQuery to return the underlying error directly, got %+v", diagnostic)
	}
}
