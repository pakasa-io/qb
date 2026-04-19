package main

import (
	"fmt"
	"net/url"

	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	querystring "github.com/pakasa-io/qb/codecs/qs"
)

func main() {
	values := url.Values{
		"$select[0]":            {"users.id"},
		"$select[1]":            {"lower(users.name) as normalized_name"},
		"$where[status]":        {"active"},
		"$where[age][$gte]":     {"21"},
		"$where[$or][0][role]":  {"admin"},
		"$where[$or][1][role]":  {"owner"},
		"$where[$expr][$eq][0]": {"lower(@users.name)"},
		"$where[$expr][$eq][1]": {"lower('john')"},
		"$sort[0]":              {"lower(users.name) asc"},
		"$sort[1]":              {"created_at desc"},
		"$page":                 {"2"},
		"$size":                 {"10"},
	}

	query, err := querystring.Parse(values)
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.DollarDialect{})).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}
