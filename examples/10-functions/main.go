package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
)

func main() {
	query, err := qb.New().
		SelectExpr(
			qb.Lower(qb.F("users.name")),
			qb.F("users.age"),
		).
		GroupByExpr(qb.Lower(qb.F("users.name"))).
		SortByExpr(qb.Lower(qb.F("users.name")), qb.Asc).
		Where(qb.And(
			qb.Lower(qb.F("users.name")).Eq("john"),
			qb.F("users.name").Eq(qb.Lower("JOHN")),
		)).
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
