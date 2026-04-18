package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
)

func main() {
	query, err := qb.New().
		SelectProjection(
			qb.Round(qb.F("orders.total").Cast("decimal"), 2).As("rounded_total"),
			qb.RoundDouble(qb.F("orders.score").Cast("double"), 2).As("rounded_score"),
			qb.F("orders.customer_name").Lower().As("normalized_customer"),
		).
		Where(qb.F("orders.status").Eq("paid")).
		SortByExpr(qb.F("orders.customer_name").Lower(), qb.Asc).
		Query()
	if err != nil {
		panic(err)
	}

	dialects := []struct {
		name    string
		dialect sqladapter.Dialect
	}{
		{name: "postgres", dialect: sqladapter.PostgresDialect{}},
		{name: "mysql", dialect: sqladapter.MySQLDialect{}},
		{name: "sqlite", dialect: sqladapter.SQLiteDialect{}},
	}

	for _, item := range dialects {
		statement, err := sqladapter.New(sqladapter.WithDialect(item.dialect)).Compile(query)
		if err != nil {
			panic(err)
		}

		fmt.Println(item.name + ":")
		fmt.Println(statement.SQL)
		fmt.Printf("%#v\n", statement.Args)
	}
}
