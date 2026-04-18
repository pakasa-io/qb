package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)

func main() {
	query, err := qb.New().
		Where(qb.Field("status").Eq("active")).
		SortBy("created_at", qb.Desc).
		Size(20).
		Query()
	if err != nil {
		panic(err)
	}

	pipeline := qb.ComposeTransformers(
		tenantFilter(42),
		softDeleteFilter,
	)

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(pipeline),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}

func tenantFilter(tenantID int64) qb.QueryTransformer {
	return func(query qb.Query) (qb.Query, error) {
		if query.Filter == nil {
			query.Filter = qb.Field("tenant_id").Eq(tenantID)
			return query, nil
		}

		query.Filter = qb.And(
			qb.Field("tenant_id").Eq(tenantID),
			query.Filter,
		)
		return query, nil
	}
}

func softDeleteFilter(query qb.Query) (qb.Query, error) {
	if query.Filter == nil {
		query.Filter = qb.Field("deleted_at").IsNull()
		return query, nil
	}

	query.Filter = qb.And(
		qb.Field("deleted_at").IsNull(),
		query.Filter,
	)
	return query, nil
}
