package main

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
	"github.com/pakasa-io/qb/codecs"
	"github.com/pakasa-io/qb/codecs/querystring"
	"github.com/pakasa-io/qb/schema"
)

func main() {
	userSchema := schema.MustNew(
		schema.Define("zip", schema.Storage("users.zip")),
		schema.Define("status", schema.Storage("users.status")),
		schema.Define("external_id", schema.Storage("users.external_id")),
		schema.Define(
			"age",
			schema.Storage("users.age"),
			schema.Decode(func(_ qb.Operator, value any) (any, error) {
				switch typed := value.(type) {
				case string:
					return strconv.Atoi(typed)
				default:
					return value, nil
				}
			}),
		),
	)

	values := url.Values{
		"$where[zip]":             {"02110"},
		"$where[status]":          {"false"},
		"$where[age][$gte]":       {"21"},
		"$where[$expr][$eq][0]":   {"@external_id"},
		"$where[$expr][$eq][1]":   {"0012"},
		"$where[$expr][$notnull]": {"@external_id"},
		"$page":                   {"1"},
		"$size":                   {"5"},
	}

	query, err := querystring.Parse(
		values,
		codecs.WithFilterFieldResolver(userSchema.ResolveFilterField),
		codecs.WithValueDecoder(userSchema.DecodeValue),
	)
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(userSchema.ToStorage),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	for i, arg := range statement.Args {
		fmt.Printf("arg[%d] = %v (%T)\n", i, arg, arg)
	}
}
