package main

import (
	"fmt"
	"time"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)

func main() {
	query, err := qb.New().
		SortBy("created_at", qb.Desc).
		SortBy("id", qb.Desc).
		Size(25).
		CursorValues(map[string]any{
			"created_at": time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
			"id":         int64(981),
		}).
		Query()
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(rewriteCompositeCursor),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}

func rewriteCompositeCursor(query qb.Query) (qb.Query, error) {
	if query.Cursor == nil {
		return query, nil
	}

	createdAt := query.Cursor.Values["created_at"]
	id := query.Cursor.Values["id"]
	query.Filter = qb.Or(
		qb.Field("created_at").Lt(createdAt),
		qb.And(
			qb.Field("created_at").Eq(createdAt),
			qb.Field("id").Lt(id),
		),
	)
	query.Cursor = nil
	return query, nil
}
