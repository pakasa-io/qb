package main

import (
	"errors"
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/codec/model"
	"github.com/pakasa-io/qb/schema"
)

func main() {
	_, err := model.Parse(map[string]any{
		"$where": map[string]any{
			"deleted_at": map[string]any{"$isnull": false},
		},
	})
	printDiagnostic("parse", err)

	userSchema := schema.MustNew(
		schema.Define("status", schema.Operators(qb.OpEq)),
	)

	query, err := qb.New().
		Where(qb.F("status").Gt("active")).
		Query()
	if err != nil {
		panic(err)
	}

	_, err = userSchema.Normalize(query)
	printDiagnostic("normalize", err)

	unsupported, err := qb.New().
		Where(qb.F("users.name").Regexp("jo.*")).
		Query()
	if err != nil {
		panic(err)
	}

	_, err = sqladapter.New(sqladapter.WithDialect(sqladapter.SQLiteDialect{})).Compile(unsupported)
	printDiagnostic("compile", err)
}

func printDiagnostic(label string, err error) {
	var diagnostic *qb.Error
	if !errors.As(err, &diagnostic) {
		fmt.Println(label+": unexpected error:", err)
		return
	}

	fmt.Println(label + ":")
	fmt.Println("  stage:", diagnostic.Stage)
	fmt.Println("  code:", diagnostic.Code)
	fmt.Println("  path:", diagnostic.Path)
	fmt.Println("  field:", diagnostic.Field)
	fmt.Println("  operator:", diagnostic.Operator)
	fmt.Println("  function:", diagnostic.Function)
}
