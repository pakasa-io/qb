package codec_test

import (
	"strings"
	"testing"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	yamlcodec "github.com/pakasa-io/qb/codecs/yamlcodec"
)

func TestMarshalParseRoundTrip(t *testing.T) {
	query, err := qb.New().
		SelectProjection(
			qb.F("users.name").Lower().As("normalized_name"),
			qb.Project(qb.F("users.age")),
		).
		Where(qb.F("users.status").Eq("active")).
		Where(qb.F("users.age").Gte(18)).
		GroupByExpr(qb.F("users.status")).
		SortByExpr(qb.F("users.name").Lower(), qb.Asc).
		Page(2).
		Size(10).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	payload, err := yamlcodec.Marshal(query)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(payload), "$select:") {
		t.Fatalf("unexpected YAML payload:\n%s", string(payload))
	}

	parsed, err := yamlcodec.Parse(payload)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	statement, err := sqladapter.New().Compile(parsed)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !strings.Contains(statement.SQL, `GROUP BY "users"."status"`) {
		t.Fatalf("unexpected SQL: %s", statement.SQL)
	}
}
