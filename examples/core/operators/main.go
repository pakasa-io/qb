package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
)

func main() {
	query, err := qb.New().
		Where(qb.And(
			qb.F("users.status").In("active", "trial"),
			qb.Or(
				qb.F("users.name").ILike("jo%"),
				qb.F("users.email").Suffix("@example.com"),
			),
			qb.F("users.bio").Contains("golang"),
			qb.F("users.role").NotIn("banned", "suspended"),
			qb.F("users.deleted_at").IsNull(),
			qb.Not(qb.F("users.email").Prefix("test+")),
		)).
		SortByExpr(qb.F("users.name").Lower(), qb.Asc).
		Page(1).
		Size(20).
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
