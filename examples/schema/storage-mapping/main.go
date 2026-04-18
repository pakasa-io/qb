package main

import (
	"fmt"
	"strconv"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
	"github.com/pakasa-io/qb/codecs"
	"github.com/pakasa-io/qb/schema"
)

func main() {
	userSchema := schema.MustNew(
		schema.Define("status", schema.Storage("users.status"), schema.Aliases("state"), schema.Operators(qb.OpEq, qb.OpIn)),
		schema.Define("role", schema.Storage("users.role"), schema.Operators(qb.OpEq, qb.OpIn)),
		schema.Define(
			"age",
			schema.Storage("users.age"),
			schema.Aliases("minAge"),
			schema.Operators(qb.OpEq, qb.OpGte, qb.OpLte),
			schema.Decode(func(_ qb.Operator, value any) (any, error) {
				switch typed := value.(type) {
				case string:
					return strconv.Atoi(typed)
				default:
					return value, nil
				}
			}),
		),
		schema.Define("created_at", schema.Storage("users.created_at"), schema.Aliases("createdAt"), schema.Sortable()),
	)

	query, err := codecs.Parse(
		map[string]any{
			"$select": "state,role",
			"$where": map[string]any{
				"state":  "active",
				"minAge": map[string]any{"$gte": "21"},
			},
			"$group": "state,role",
			"$sort":  "-createdAt",
			"$page":  2,
			"$size":  10,
		},
		codecs.WithFilterFieldResolver(userSchema.ResolveFilterField),
		codecs.WithGroupFieldResolver(userSchema.ResolveGroupField),
		codecs.WithSortFieldResolver(userSchema.ResolveSortField),
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
	fmt.Printf("%#v\n", statement.Args)
}
