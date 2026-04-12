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
		schema.Define("status", schema.Storage("users.status"), schema.Aliases("state"), schema.Operators(qb.OpEq, qb.OpIn)),
		schema.Define("role", schema.Storage("users.role"), schema.Operators(qb.OpEq, qb.OpIn)),
		schema.Define(
			"age",
			schema.Storage("users.age"),
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
		schema.Define("created_at", schema.Storage("users.created_at"), schema.Aliases("createdAt"), schema.Sortable(), schema.DisableFiltering()),
	)

	payload := map[string]any{
		"pick": []any{"state", "role"},
		"where": map[string]any{
			"state":  "active",
			"minAge": map[string]any{"$gte": "21"},
			"$or": []any{
				map[string]any{"role": "admin"},
				map[string]any{"role": "owner"},
			},
		},
		"group_by": []any{"state", "role"},
		"sort":     []any{"-createdAt"},
		"page":     2,
		"size":     10,
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
		sqladapter.WithQueryTransformer(userSchema.ToStorage),
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
		gormadapter.WithQueryTransformer(userSchema.ToStorage),
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
	// SQL: SELECT "users"."status", "users"."role" WHERE (("users"."role" = ? OR "users"."role" = ?) AND "users"."age" >= ? AND "users"."status" = ?) GROUP BY "users"."status", "users"."role" ORDER BY "users"."created_at" DESC LIMIT 10 OFFSET 10
	// SQL args: [admin owner 21 active]
	// GORM: SELECT users.status,users.role FROM `example_users` WHERE (`users`.`role` = ? OR `users`.`role` = ?) AND `users`.`age` >= ? AND `users`.`status` = ? GROUP BY `users`.`status`,`users`.`role` ORDER BY `users`.`created_at` DESC LIMIT 10 OFFSET 10
	// GORM args: [admin owner 21 active]
}
