package main

import (
	"testing"

	"github.com/pakasa-io/qb"
)

func TestExample(t *testing.T) {
	main()
}

func TestTransformersWithEmptyFilter(t *testing.T) {
	query, err := tenantFilter(42)(qb.Query{})
	if err != nil {
		t.Fatalf("tenantFilter() error = %v", err)
	}

	predicate, ok := query.Filter.(qb.Predicate)
	if !ok || predicate.Left.(qb.Ref).Name != "tenant_id" {
		t.Fatalf("unexpected tenant filter: %#v", query.Filter)
	}

	query, err = softDeleteFilter(qb.Query{})
	if err != nil {
		t.Fatalf("softDeleteFilter() error = %v", err)
	}

	predicate, ok = query.Filter.(qb.Predicate)
	if !ok || predicate.Op != qb.OpIsNull {
		t.Fatalf("unexpected soft delete filter: %#v", query.Filter)
	}
}
