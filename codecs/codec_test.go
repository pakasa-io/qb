package codecs_test

import (
	"testing"

	"github.com/pakasa-io/qb/codecs"
)

func TestParseWrapper(t *testing.T) {
	query, err := codecs.Parse(map[string]any{
		"$where": map[string]any{"status": "active"},
		"$sort":  "-created_at",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if query.Filter == nil || len(query.Sorts) != 1 {
		t.Fatalf("unexpected parsed query: %#v", query)
	}
}
