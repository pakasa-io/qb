package querystring_test

import (
	"net/url"
	"strconv"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/mapinput"
	"github.com/pakasa-io/qb/parser/querystring"
)

func TestParse(t *testing.T) {
	values := url.Values{
		"where[$or][0][status][$eq]": {"active"},
		"where[$or][1][status][$eq]": {"trial"},
		"where[age][$gte]":           {"21"},
		"sort":                       {"-created_at,name"},
		"limit":                      {"10"},
	}

	query, err := querystring.Parse(values, mapinput.WithValueDecoder(func(field string, op qb.Operator, value any) (any, error) {
		if field == "age" && op == qb.OpGte {
			return strconv.Atoi(value.(string))
		}
		return value, nil
	}))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.DollarDialect{})).Compile(query)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	wantSQL := `WHERE (("status" = $1 OR "status" = $2) AND "age" >= $3) ORDER BY "created_at" DESC, "name" ASC LIMIT 10`
	if statement.SQL != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, statement.SQL)
	}

	wantArgs := []any{"active", "trial", 21}
	if len(statement.Args) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(statement.Args))
	}

	for i := range wantArgs {
		if statement.Args[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], statement.Args[i])
		}
	}
}
