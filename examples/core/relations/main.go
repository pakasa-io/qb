package main

import (
	"fmt"
	"strings"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
	"github.com/pakasa-io/qb/schema"
)

func main() {
	orderSchema := schema.MustNew(
		schema.Define("status", schema.Storage("orders.status")),
		schema.Define("customer.email", schema.Storage("customers.email")),
		schema.Define("customer.company.name", schema.Storage("companies.name")),
		schema.Define("created_at", schema.Storage("orders.created_at"), schema.Sortable()),
	)

	query, err := qb.New().
		Where(qb.And(
			qb.Field("customer.email").Suffix("@example.com"),
			qb.Field("customer.company.name").Eq("Acme"),
		)).
		SortBy("created_at", qb.Desc).
		Query()
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(orderSchema.ToStorage),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	baseSQL := `
SELECT orders.*
FROM orders
JOIN customers ON customers.id = orders.customer_id
JOIN companies ON companies.id = customers.company_id
`

	fmt.Println(strings.TrimSpace(baseSQL))
	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}
