package qb_test

import (
	"fmt"
	"strconv"

	"github.com/pakasa-io/qb"
	gormadapter "github.com/pakasa-io/qb/adapter/gorm"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/mapinput"
	"github.com/pakasa-io/qb/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type exampleUser struct {
	ID        uint
	Status    string
	Role      string
	Age       int
	CreatedAt int64
}

func Example() {
	userSchema := schema.MustNew(
		schema.Define("status", schema.Aliases("state"), schema.Operators(qb.OpEq, qb.OpIn)),
		schema.Define("role", schema.Operators(qb.OpEq, qb.OpIn)),
		schema.Define(
			"age",
			schema.Aliases("minAge"),
			schema.Operators(qb.OpEq, qb.OpGte, qb.OpLte),
			schema.Decode(func(_ qb.Operator, value any) (any, error) {
				switch typed := value.(type) {
				case string:
					return strconv.Atoi(typed)
				default:
					return value, nil
				}
			}),
		),
		schema.Define("created_at", schema.Aliases("createdAt"), schema.Sortable(), schema.DisableFiltering()),
	)

	payload := map[string]any{
		"where": map[string]any{
			"state":  "active",
			"minAge": map[string]any{"$gte": "21"},
			"$or": []any{
				map[string]any{"role": "admin"},
				map[string]any{"role": "owner"},
			},
		},
		"sort":  []any{"-createdAt"},
		"limit": 10,
	}

	query, err := mapinput.Parse(
		payload,
		mapinput.WithFilterFieldResolver(userSchema.ResolveFilterField),
		mapinput.WithSortFieldResolver(userSchema.ResolveSortField),
		mapinput.WithValueDecoder(userSchema.DecodeValue),
	)
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(userSchema.Normalize),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println("SQL:", statement.SQL)
	fmt.Println("SQL args:", statement.Args)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		panic(err)
	}

	tx, err := gormadapter.New(
		gormadapter.WithQueryTransformer(userSchema.Normalize),
	).Apply(db.Model(&exampleUser{}), query)
	if err != nil {
		panic(err)
	}

	result := tx.Find(&[]exampleUser{})
	if result.Error != nil {
		panic(result.Error)
	}

	fmt.Println("GORM:", result.Statement.SQL.String())
	fmt.Println("GORM args:", result.Statement.Vars)

	// Output:
	// SQL: WHERE (("role" = ? OR "role" = ?) AND "age" >= ? AND "status" = ?) ORDER BY "created_at" DESC LIMIT 10
	// SQL args: [admin owner 21 active]
	// GORM: SELECT * FROM `example_users` WHERE (`role` = ? OR `role` = ?) AND `age` >= ? AND `status` = ? ORDER BY `created_at` DESC LIMIT 10
	// GORM args: [admin owner 21 active]
}
