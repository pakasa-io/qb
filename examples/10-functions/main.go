package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
)

func main() {
	query, err := qb.New().
		SelectExpr(
			qb.Lower(qb.Field("users.name")),
			qb.Field("users.age"),
		).
		GroupByExpr(qb.Lower(qb.Field("users.name"))).
		Where(qb.And(
			qb.Lower(qb.Field("users.name")).Eq("john"),
			qb.Field("users.name").Eq(qb.Lower("JOHN")),
		)).
		Query()
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.DollarDialect{})).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}
