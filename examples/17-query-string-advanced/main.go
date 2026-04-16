package main

import (
	"fmt"
	"net/url"

	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/codec/querystring"
)

func main() {
	values := url.Values{
		"$select[0]":             {"users.id"},
		"$select[1]":             {"lower(users.name) as normalized_name"},
		"$select[2]":             {"json_value(users.profile, '$.timezone') as timezone"},
		"$include[0]":            {"Company"},
		"$include[1]":            {"Orders"},
		"$where[status][$in][0]": {"active"},
		"$where[status][$in][1]": {"trial"},
		"$where[$or][0][role]":   {"admin"},
		"$where[$or][1][role]":   {"owner"},
		"$where[$expr][$gte][0]": {"round(@users.score::decimal, 2)"},
		"$where[$expr][$gte][1]": {"4.5"},
		"$where[$expr][$isnull]": {"@users.deleted_at"},
		"$group[0]":              {"date(users.created_at)"},
		"$group[1]":              {"json_value(users.profile, '$.timezone')"},
		"$sort[0]":               {"date(users.created_at) desc"},
		"$sort[1]":               {"lower(users.name) asc"},
		"$page":                  {"3"},
		"$size":                  {"25"},
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
	fmt.Printf("%#v\n", statement.Args)
}
