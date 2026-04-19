package json_test

import (
	"fmt"

	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	jsoncodec "github.com/pakasa-io/qb/codecs/json"
)

func ExampleParse() {
	payload := []byte(`{
  "$select": ["users.id", "users.status"],
  "$where": {
    "users.status": "active"
  },
  "$sort": ["users.created_at desc"],
  "$size": 10
}`)

	query, err := jsoncodec.Parse(payload)
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
