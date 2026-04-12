package main

import (
	"fmt"
	"net/url"

	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/querystring"
)

func main() {
	values := url.Values{
		"pick":                     {"id,status"},
		"where[status][$eq]":       {"active"},
		"where[age][$gte]":         {"21"},
		"where[$or][0][role][$eq]": {"admin"},
		"where[$or][1][role][$eq]": {"owner"},
		"sort":                     {"-created_at,name"},
		"page":                     {"2"},
		"size":                     {"10"},
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
