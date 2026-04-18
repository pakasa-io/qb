package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)

func main() {
	original := sqladapter.DefaultDialect()
	defer sqladapter.SetDefaultDialect(original)

	query, err := qb.New().
		SelectProjection(
			qb.F("users.name").Lower().As("normalized_name"),
			qb.CurrentTimestamp().As("compiled_at"),
		).
		Where(qb.F("users.status").Eq("active")).
		SortByExpr(qb.F("users.name").Lower(), qb.Asc).
		Query()
	if err != nil {
		panic(err)
	}

	printCompiled("process default", sqladapter.New(), query)

	if err := sqladapter.SetDefaultDialectByName("mysql"); err != nil {
		panic(err)
	}
	printCompiled("changed default", sqladapter.New(), query)

	printCompiled(
		"per-call override",
		sqladapter.New(sqladapter.WithDialect(sqladapter.SQLiteDialect{})),
		query,
	)
}

func printCompiled(label string, compiler sqladapter.Compiler, query qb.Query) {
	statement, err := compiler.Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(label + ":")
	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}
