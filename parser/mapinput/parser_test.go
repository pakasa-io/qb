package mapinput_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/mapinput"
)

func TestParse(t *testing.T) {
	input := map[string]any{
		"where": map[string]any{
			"$or": []any{
				map[string]any{"role": "admin"},
				map[string]any{"role": "owner"},
			},
			"age":    map[string]any{"$gte": json.Number("18")},
			"status": "active",
		},
		"sort":   []any{"-created_at", "name"},
		"limit":  "20",
		"offset": json.Number("40"),
	}

	query, err := mapinput.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE (("role" = ? OR "role" = ?) AND "age" >= ? AND "status" = ?) ORDER BY "created_at" DESC, "name" ASC LIMIT 20 OFFSET 40`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{"admin", "owner", int64(18), "active"}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
	}
}

func TestParseWithValueDecoder(t *testing.T) {
	input := map[string]any{
		"where": map[string]any{
			"age": map[string]any{"$gte": "21"},
		},
	}

	query, err := mapinput.Parse(input, mapinput.WithValueDecoder(func(field string, _ qb.Operator, value any) (any, error) {
		if field != "age" {
			return value, nil
		}
		return strconv.Atoi(value.(string))
	}))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if len(statement.Args) != 1 || statement.Args[0] != 21 {
		t.Fatalf("unexpected args: %#v", statement.Args)
	}
}
