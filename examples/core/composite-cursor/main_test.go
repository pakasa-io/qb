package main

import (
	"testing"

	"github.com/pakasa-io/qb"
)

func TestExample(t *testing.T) {
	main()
}

func TestRewriteCompositeCursorWithoutCursor(t *testing.T) {
	query, err := rewriteCompositeCursor(qb.Query{})
	if err != nil {
		t.Fatalf("rewriteCompositeCursor() error = %v", err)
	}

	if query.Filter != nil || query.Cursor != nil {
		t.Fatalf("expected empty query to pass through unchanged, got %#v", query)
	}
}
