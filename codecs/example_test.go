package codecs_test

import (
	"fmt"

	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	"github.com/pakasa-io/qb/codecs"
)

func ExampleParse() {
	query, err := codecs.Parse(map[string]any{
		"$select": "users.id,users.status",
		"$where": map[string]any{
			"users.status": "active",
		},
		"$sort": "-users.created_at",
		"$size": 10,
	})
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Println(statement.Args)

	// Output:
	// SELECT "users"."id", "users"."status" WHERE "users"."status" = $1 ORDER BY "users"."created_at" DESC LIMIT 10
	// [active]
}
