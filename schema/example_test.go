package schema_test

import (
	"fmt"
	"strconv"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	"github.com/pakasa-io/qb/codecs"
	"github.com/pakasa-io/qb/schema"
)

func ExampleSchema_ToStorage() {
	userSchema := schema.MustNew(
		schema.Define(
			"status",
			schema.Storage("users.status"),
			schema.Aliases("state"),
			schema.Operators(qb.OpEq),
		),
		schema.Define(
			"age",
			schema.Storage("users.age"),
			schema.Aliases("minAge"),
			schema.Operators(qb.OpGte),
			schema.Decode(func(_ qb.Operator, value any) (any, error) {
				switch typed := value.(type) {
				case string:
					return strconv.Atoi(typed)
				default:
					return value, nil
				}
			}),
		),
		schema.Define(
			"created_at",
			schema.Storage("users.created_at"),
			schema.Aliases("createdAt"),
			schema.Sortable(),
		),
	)

	query, err := codecs.Parse(
		map[string]any{
			"$where": map[string]any{
				"state":  "active",
				"minAge": map[string]any{"$gte": "21"},
			},
			"$sort": "-createdAt",
		},
		codecs.WithFilterFieldResolver(userSchema.ResolveFilterField),
		codecs.WithSortFieldResolver(userSchema.ResolveSortField),
		codecs.WithValueDecoder(userSchema.DecodeValue),
	)
	if err != nil {
		panic(err)
	}

	projected, err := userSchema.ToStorage(query)
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New().Compile(projected)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Println(statement.Args)

	// Output:
	// WHERE ("users"."age" >= $1 AND "users"."status" = $2) ORDER BY "users"."created_at" DESC
	// [21 active]
}
