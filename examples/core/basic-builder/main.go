package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)

func main() {
	query, err := qb.New().
		Pick("id", "status").
		Where(qb.And(
			qb.Field("status").Eq("active"),
			qb.Field("age").Gte(18),
		)).
		SortBy("created_at", qb.Desc).
		Page(2).
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
