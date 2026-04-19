package sql_test

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)

func ExampleCompiler_Compile() {
	query, err := qb.New().
		Select("users.id", "users.status").
		Where(qb.F("users.status").Eq("active")).
		SortBy("users.created_at", qb.Desc).
		Size(10).
		Query()
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithDialect(sqladapter.PostgresDialect{}),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Println(statement.Args)

	// Output:
	// SELECT "users"."id", "users"."status" WHERE "users"."status" = $1 ORDER BY "users"."created_at" DESC LIMIT 10
	// [active]
}
