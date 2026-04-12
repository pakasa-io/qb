package main

import (
	"fmt"

	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/mapinput"
)

func main() {
	payload := []byte(`{
		"pick": ["status", "role"],
		"where": {
			"status": "active",
			"age": { "$gte": 18 },
			"$or": [
				{ "role": "admin" },
				{ "role": "owner" }
			]
		},
		"group_by": ["status", "role"],
		"sort": ["-created_at"],
		"page": 3,
		"size": 10
	}`)

	query, err := mapinput.ParseJSON(payload)
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
