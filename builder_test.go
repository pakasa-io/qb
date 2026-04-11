package qb_test

import (
	"testing"

	"github.com/pakasa-io/qb"
)

func TestBuilderProducesIndependentQuery(t *testing.T) {
	query, err := qb.New().
		Where(qb.And(
			qb.Field("status").Eq("active"),
			qb.Or(
				qb.Field("role").Eq("admin"),
				qb.Field("role").Eq("owner"),
			),
		)).
		SortBy("created_at", qb.Desc).
		Limit(25).
		Offset(50).
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

	if query.Sorts[0].Field != "created_at" || query.Sorts[0].Direction != qb.Desc {
		t.Fatalf("unexpected sort: %+v", query.Sorts[0])
	}

	if query.Limit == nil || *query.Limit != 25 {
		t.Fatalf("unexpected limit: %v", query.Limit)
	}

	if query.Offset == nil || *query.Offset != 50 {
		t.Fatalf("unexpected offset: %v", query.Offset)
	}

	clone := query.Clone()
	clone.Sorts[0].Field = "mutated"

	if query.Sorts[0].Field != "created_at" {
		t.Fatal("expected Clone to protect sort slice")
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

	if predicate.Value != "active" {
		t.Fatalf("expected original filter tree to remain unchanged, got %#v", predicate.Value)
	}
}

func TestBuilderRejectsNegativeLimit(t *testing.T) {
	_, err := qb.New().Limit(-1).Query()
	if err == nil {
		t.Fatal("expected negative limit error")
	}
}
