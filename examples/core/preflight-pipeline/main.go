package main

import (
	"fmt"
	"strconv"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	"github.com/pakasa-io/qb/codecs"
	"github.com/pakasa-io/qb/schema"
)

func main() {
	userSchema := schema.MustNew(
		schema.Define("status", schema.Storage("users.status"), schema.Aliases("state"), schema.Sortable()),
		schema.Define(
			"age",
			schema.Storage("users.age"),
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
			"$select": []any{
				"lower(state) as normalized_status",
				"createdAt::date as joined_on",
			},
			"$where": map[string]any{
				"state": "active",
				"age":   map[string]any{"$gte": "21"},
			},
			"$size": 250,
		},
		codecs.WithFilterFieldResolver(userSchema.ResolveFilterField),
		codecs.WithGroupFieldResolver(userSchema.ResolveGroupField),
		codecs.WithSortFieldResolver(userSchema.ResolveSortField),
		codecs.WithValueDecoder(userSchema.DecodeValue),
	)
	if err != nil {
		panic(err)
	}

	pipeline := qb.ComposeTransformers(
		enforceMaxSize(100),
		defaultSort("created_at", qb.Desc),
		userSchema.Normalize,
		userSchema.ToStorage,
		sqladapter.PostgresDialect{}.Capabilities().Validator(qb.StageCompile),
	)

	prepared, err := qb.TransformQuery(query, pipeline)
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.PostgresDialect{})).Compile(prepared)
	if err != nil {
		panic(err)
	}

	limit, offset, err := prepared.ResolvedPagination()
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
	fmt.Println("resolved limit:", *limit)
	if offset != nil {
		fmt.Println("resolved offset:", *offset)
	}
}

func enforceMaxSize(max int) qb.QueryTransformer {
	return func(query qb.Query) (qb.Query, error) {
		if query.Size != nil && *query.Size > max {
			size := max
			query.Size = &size
		}
		return query, nil
	}
}

func defaultSort(field string, direction qb.Direction) qb.QueryTransformer {
	return func(query qb.Query) (qb.Query, error) {
		if len(query.Sorts) == 0 {
			query.Sorts = []qb.Sort{{Expr: qb.F(field), Direction: direction}}
		}
		return query, nil
	}
}
