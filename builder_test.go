package qb_test

import (
	"errors"
	"testing"

	"github.com/pakasa-io/qb"
)

func TestBuilderProducesIndependentQuery(t *testing.T) {
	query, err := qb.New().
		Select("id", "status").
		Include("Customer", "Orders.Items").
		GroupBy("status").
		Where(qb.And(
			qb.Field("status").Eq("active"),
			qb.Or(
				qb.Field("role").Eq("admin"),
				qb.Field("role").Eq("owner"),
			),
		)).
		SortBy("created_at", qb.Desc).
		Page(3).
		Size(25).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if query.Filter == nil {
		t.Fatal("expected filter to be set")
	}

	if len(query.Sorts) != 1 {
		t.Fatalf("expected 1 sort, got %d", len(query.Sorts))
	}

	if len(query.Projections) != 2 || projectionRefName(query.Projections[0]) != "id" || projectionRefName(query.Projections[1]) != "status" {
		t.Fatalf("unexpected selects: %#v", query.Projections)
	}

	if len(query.Includes) != 2 || query.Includes[0] != "Customer" || query.Includes[1] != "Orders.Items" {
		t.Fatalf("unexpected includes: %#v", query.Includes)
	}

	if len(query.GroupBy) != 1 || refName(query.GroupBy[0]) != "status" {
		t.Fatalf("unexpected group_by: %#v", query.GroupBy)
	}

	if refName(query.Sorts[0].Expr) != "created_at" || query.Sorts[0].Direction != qb.Desc {
		t.Fatalf("unexpected sort: %+v", query.Sorts[0])
	}

	if query.Page == nil || *query.Page != 3 {
		t.Fatalf("unexpected page: %v", query.Page)
	}

	if query.Size == nil || *query.Size != 25 {
		t.Fatalf("unexpected size: %v", query.Size)
	}

	limit, offset, err := query.ResolvedPagination()
	if err != nil {
		t.Fatalf("ResolvedPagination() error = %v", err)
	}

	if limit == nil || *limit != 25 {
		t.Fatalf("unexpected resolved limit: %v", limit)
	}

	if offset == nil || *offset != 50 {
		t.Fatalf("unexpected resolved offset: %v", offset)
	}

	clone := query.Clone()
	clone.Sorts[0].Expr = qb.F("mutated")
	clone.Projections[0] = qb.Project(qb.F("mutated"))
	clone.Includes[0] = "mutated"
	clone.GroupBy[0] = qb.F("mutated")

	if refName(query.Sorts[0].Expr) != "created_at" {
		t.Fatal("expected Clone to protect sort slice")
	}

	if projectionRefName(query.Projections[0]) != "id" || query.Includes[0] != "Customer" || refName(query.GroupBy[0]) != "status" {
		t.Fatal("expected Clone to protect metadata slices")
	}

	group, ok := clone.Filter.(qb.Group)
	if !ok {
		t.Fatalf("expected cloned filter to be a group, got %T", clone.Filter)
	}

	group.Terms[0] = qb.Field("status").Eq("mutated")
	clone.Filter = group

	originalGroup, ok := query.Filter.(qb.Group)
	if !ok {
		t.Fatalf("expected original filter to be a group, got %T", query.Filter)
	}

	predicate, ok := originalGroup.Terms[0].(qb.Predicate)
	if !ok {
		t.Fatalf("expected original first term to be a predicate, got %T", originalGroup.Terms[0])
	}

	operand, ok := predicate.Right.(qb.ScalarOperand)
	if !ok {
		t.Fatalf("expected scalar operand, got %T", predicate.Right)
	}

	literal, ok := operand.Expr.(qb.Literal)
	if !ok || literal.Value != "active" {
		t.Fatalf("expected original filter tree to remain unchanged, got %#v", predicate.Right)
	}
}

func TestBuilderRejectsNegativeLimit(t *testing.T) {
	_, err := qb.New().Limit(-1).Query()
	if err == nil {
		t.Fatal("expected negative limit error")
	}

	if err.Error() != "qb: limit cannot be negative" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestBuilderRejectsPageWithoutSize(t *testing.T) {
	_, err := qb.New().Page(2).Query()
	if err == nil {
		t.Fatal("expected page without size error")
	}

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected structured pagination error, got %T", err)
	}

	if diagnostic.Code != qb.CodeInvalidQuery || diagnostic.Error() != "invalid_query: page requires size" {
		t.Fatalf("unexpected pagination diagnostic: %+v", diagnostic)
	}
}

func TestBuilderSupportsCursorPaginationMetadata(t *testing.T) {
	query, err := qb.New().
		SortBy("created_at", qb.Desc).
		Size(25).
		CursorValues(map[string]any{
			"created_at": "2026-04-11T12:00:00Z",
			"id":         981,
		}).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if query.Cursor == nil {
		t.Fatal("expected cursor to be set")
	}

	if got := query.Cursor.Values["id"]; got != 981 {
		t.Fatalf("unexpected cursor value: %#v", got)
	}

	limit, offset, err := query.ResolvedPagination()
	if err != nil {
		t.Fatalf("ResolvedPagination() error = %v", err)
	}

	if limit == nil || *limit != 25 {
		t.Fatalf("unexpected resolved limit: %v", limit)
	}

	if offset != nil {
		t.Fatalf("expected nil resolved offset, got %v", *offset)
	}
}

func TestBuilderRejectsCursorWithoutSize(t *testing.T) {
	_, err := qb.New().CursorToken("opaque-cursor").Query()
	if err == nil {
		t.Fatal("expected cursor without size error")
	}

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected structured pagination error, got %T", err)
	}

	if diagnostic.Code != qb.CodeInvalidQuery || diagnostic.Error() != "invalid_query: cursor requires size" {
		t.Fatalf("unexpected pagination diagnostic: %+v", diagnostic)
	}
}

func TestBuilderSupportsFunctionExpressions(t *testing.T) {
	query, err := qb.New().
		SelectExpr(qb.Lower(qb.F("users.name")), qb.F("users.age")).
		GroupByExpr(qb.Lower(qb.F("users.name"))).
		SortByExpr(qb.Lower(qb.F("users.name")), qb.Asc).
		Where(qb.And(
			qb.Lower(qb.F("users.name")).Eq("john"),
			qb.F("users.name").Eq(qb.Lower("JOHN")),
		)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(query.Projections) != 2 {
		t.Fatalf("expected 2 select expressions, got %d", len(query.Projections))
	}

	if _, ok := query.Projections[0].Expr.(qb.Call); !ok {
		t.Fatalf("expected first select to be a function call, got %T", query.Projections[0].Expr)
	}

	if len(query.GroupBy) != 1 {
		t.Fatalf("expected 1 group expression, got %d", len(query.GroupBy))
	}

	if _, ok := query.Sorts[0].Expr.(qb.Call); !ok {
		t.Fatalf("expected sort expression to be a function call, got %T", query.Sorts[0].Expr)
	}

	group, ok := query.Filter.(qb.Group)
	if !ok {
		t.Fatalf("expected grouped filter, got %T", query.Filter)
	}

	predicate, ok := group.Terms[1].(qb.Predicate)
	if !ok {
		t.Fatalf("expected predicate, got %T", group.Terms[1])
	}

	if _, ok := predicate.Right.(qb.ScalarOperand); !ok {
		t.Fatalf("expected scalar operand, got %T", predicate.Right)
	}
}

func TestBuilderWhereIgnoresNilAndAccumulatesFilters(t *testing.T) {
	query, err := qb.New().
		Where(nil).
		Where(qb.F("status").Eq("active")).
		Where(qb.F("role").Eq("admin")).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	group, ok := query.Filter.(qb.Group)
	if !ok {
		t.Fatalf("expected grouped filter, got %T", query.Filter)
	}

	if group.Kind != qb.AndGroup || len(group.Terms) != 2 {
		t.Fatalf("unexpected accumulated filter: %#v", group)
	}

	first, ok := group.Terms[0].(qb.Predicate)
	if !ok {
		t.Fatalf("expected first term to be predicate, got %T", group.Terms[0])
	}

	second, ok := group.Terms[1].(qb.Predicate)
	if !ok {
		t.Fatalf("expected second term to be predicate, got %T", group.Terms[1])
	}

	if refName(first.Left) != "status" || refName(second.Left) != "role" {
		t.Fatalf("unexpected filter terms: %#v", group.Terms)
	}
}

func TestBuilderClonesCallerOwnedInputs(t *testing.T) {
	nameExpr := qb.Concat(qb.F("first_name"), " ", qb.F("last_name"))
	cursorValues := map[string]any{
		"id":    981,
		"token": "before",
	}

	query, err := qb.New().
		SelectExpr(nameExpr).
		SortByExpr(nameExpr, qb.Desc).
		Size(10).
		CursorValues(cursorValues).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	nameExpr.Args[0] = qb.F("mutated")
	cursorValues["id"] = 0
	cursorValues["token"] = "after"

	selectCall, ok := query.Projections[0].Expr.(qb.Call)
	if !ok {
		t.Fatalf("expected projected call, got %T", query.Projections[0].Expr)
	}

	sortCall, ok := query.Sorts[0].Expr.(qb.Call)
	if !ok {
		t.Fatalf("expected sort call, got %T", query.Sorts[0].Expr)
	}

	if refName(selectCall.Args[0]) != "first_name" || refName(sortCall.Args[0]) != "first_name" {
		t.Fatalf("builder retained caller-owned scalar slices: select=%#v sort=%#v", selectCall, sortCall)
	}

	if query.Cursor == nil {
		t.Fatal("expected cursor metadata")
	}

	if got := query.Cursor.Values["id"]; got != 981 {
		t.Fatalf("cursor value changed with caller mutation: %#v", got)
	}

	if got := query.Cursor.Values["token"]; got != "before" {
		t.Fatalf("cursor token changed with caller mutation: %#v", got)
	}
}

func TestBuilderPreservesFirstError(t *testing.T) {
	_, err := qb.New().
		Limit(-1).
		Select("id").
		Page(2).
		Query()
	if err == nil {
		t.Fatal("expected builder error")
	}

	if err.Error() != "qb: limit cannot be negative" {
		t.Fatalf("expected first validation error to win, got %v", err)
	}
}

func refName(expr qb.Scalar) string {
	ref, ok := expr.(qb.Ref)
	if !ok {
		return ""
	}
	return ref.Name
}

func projectionRefName(projection qb.Projection) string {
	return refName(projection.Expr)
}
