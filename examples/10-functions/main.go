package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
)

func main() {
	query, err := qb.New().
		SelectExpr(
			qb.F("users.name").Lower(),
			qb.F("users.name").Substring(1, 4),
			qb.F("users.first_name").Concat(" ", qb.F("users.last_name")),
			qb.F("users.nickname").Coalesce(qb.F("users.name")),
			qb.Round(qb.F("users.amount").Cast("decimal"), 2),
			qb.RoundDouble(qb.F("users.score").Cast("double"), 2),
			qb.F("users.created_at").DateTrunc("day"),
			qb.F("users.profile").JsonValue("$.nickname"),
			qb.Now(),
			qb.F("users.age"),
		).
		GroupByExpr(qb.F("users.name").Lower()).
		SortByExpr(qb.F("users.name").Lower(), qb.Asc).
		Where(qb.And(
			qb.F("users.name").Lower().Eq("john"),
			qb.F("users.name").Substring(1, 4).Eq("john"),
			qb.F("users.name").Eq(qb.V("JOHN").Lower()),
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
