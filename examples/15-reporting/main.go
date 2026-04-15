package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
)

func main() {
	day := qb.F("orders.created_at").DateTrunc("day")

	query, err := qb.New().
		SelectProjection(
			day.As("day"),
			qb.Count().As("order_count"),
			qb.F("orders.total").Sum().As("gross_total"),
			qb.F("orders.total").Avg().As("avg_total"),
		).
		Where(qb.And(
			qb.F("orders.status").Eq("paid"),
			qb.F("orders.created_at").Gte("2026-04-01T00:00:00Z"),
		)).
		GroupByExpr(day).
		SortByExpr(day, qb.Desc).
		Page(1).
		Size(30).
		Query()
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
