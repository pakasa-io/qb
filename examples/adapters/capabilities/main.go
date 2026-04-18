package main

import (
	"errors"
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)

func main() {
	dialects := []sqladapter.Dialect{
		sqladapter.PostgresDialect{},
		sqladapter.MySQLDialect{},
		sqladapter.SQLiteDialect{},
	}

	for _, dialect := range dialects {
		capabilities := dialect.Capabilities()
		fmt.Println(dialect.Name())
		fmt.Println("  ilike:", capabilities.SupportsOperator(qb.OpILike))
		fmt.Println("  regexp:", capabilities.SupportsOperator(qb.OpRegexp))
		fmt.Println("  date_trunc:", capabilities.SupportsFunction("date_trunc"))
		fmt.Println("  json_value:", capabilities.SupportsFunction("json_value"))
	}

	query, err := qb.New().
		SelectProjection(qb.F("events.created_at").DateTrunc("day").As("day")).
		Where(qb.F("users.name").ILike("jo%")).
		Query()
	if err != nil {
		panic(err)
	}

	_, err = sqladapter.New(sqladapter.WithDialect(sqladapter.SQLiteDialect{})).Compile(query)
	if err == nil {
		panic("expected unsupported feature error")
	}

	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		panic(err)
	}

	fmt.Println("sqlite compile error:")
	fmt.Println("  stage:", diagnostic.Stage)
	fmt.Println("  code:", diagnostic.Code)
	fmt.Println("  operator:", diagnostic.Operator)
	fmt.Println("  function:", diagnostic.Function)
}
