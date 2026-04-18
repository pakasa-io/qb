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
	orderSchema := schema.MustNew(
		schema.Define("created_at", schema.Storage("orders.created_at"), schema.Aliases("createdAt"), schema.Sortable()),
		schema.Define(
			"id",
			schema.Storage("orders.id"),
			schema.Sortable(),
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

	query, err := codecs.Parse(
		map[string]any{
			"$sort": []any{"createdAt desc", "id desc"},
			"$cursor": map[string]any{
				"createdAt": "2026-04-11T12:00:00Z",
				"id":        "981",
			},
			"$size": 25,
		},
		codecs.WithSortFieldResolver(orderSchema.ResolveSortField),
		codecs.WithValueDecoder(orderSchema.DecodeValue),
	)
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(orderSchema.ToStorage),
		sqladapter.WithQueryTransformer(rewriteCursor),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}

func rewriteCursor(query qb.Query) (qb.Query, error) {
	if query.Cursor == nil {
		return query, nil
	}

	createdAt := query.Cursor.Values["orders.created_at"]
	id := query.Cursor.Values["orders.id"]
	query.Filter = qb.Or(
		qb.Field("orders.created_at").Lt(createdAt),
		qb.And(
			qb.Field("orders.created_at").Eq(createdAt),
			qb.Field("orders.id").Lt(id),
		),
	)
	query.Cursor = nil
	return query, nil
}
