package qs_test

import (
	"fmt"
	"net/url"

	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	querystring "github.com/pakasa-io/qb/codecs/qs"
)

func ExampleParse() {
	values := url.Values{
		"$select[0]":           {"users.id"},
		"$select[1]":           {"users.status"},
		"$where[users.status]": {"active"},
		"$sort[0]":             {"users.created_at desc"},
		"$size":                {"10"},
	}

	query, err := querystring.Parse(values)
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
