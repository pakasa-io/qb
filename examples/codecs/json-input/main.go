package main

import (
	"fmt"

	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	jsoncodec "github.com/pakasa-io/qb/codecs/jsoncodec"
)

func main() {
	payload := []byte(`{
		"$select": [
			"lower(users.name) as normalized_name",
			"status",
			"role"
		],
		"$where": {
			"status": "active",
			"age": { "$gte": 18 },
			"$or": [
				{ "role": "admin" },
				{ "role": "owner" }
			],
			"$expr": {
				"$eq": ["lower(@users.name)", "lower('john')"]
			}
		},
		"$group": [
			"lower(users.name)",
			"status",
			"role"
		],
		"$sort": ["lower(users.name) asc", "created_at desc"],
		"$page": 3,
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
	fmt.Printf("%#v\n", statement.Args)
}
