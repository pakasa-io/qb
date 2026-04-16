package main

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/pakasa-io/qb"
	gormadapter "github.com/pakasa-io/qb/adapter/gorm"
	"github.com/pakasa-io/qb/codec/model"
	"github.com/pakasa-io/qb/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type user struct {
	ID        int
	Status    string
	Age       int
	CreatedAt string
	CompanyID int
	Company   company
}

type company struct {
	ID   int
	Name string
}

func main() {
	userSchema := schema.MustNew(
		schema.Define("id", schema.Storage("users.id"), schema.Sortable()),
		schema.Define("status", schema.Storage("users.status"), schema.Aliases("state"), schema.Sortable()),
		schema.Define(
			"age",
			schema.Storage("users.age"),
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
		schema.Define("created_at", schema.Storage("users.created_at"), schema.Aliases("createdAt"), schema.Sortable()),
		schema.Define("company.id", schema.Storage("users.company_id")),
	)

	payload := map[string]any{
		"$select": []any{
			"id",
			"lower(status) as normalized_status",
			"createdAt::date as joined_on",
		},
		"$include": []any{"Company"},
		"$where": map[string]any{
			"state": "active",
			"age":   map[string]any{"$gte": "21"},
		},
		"$sort": []any{"createdAt desc"},
		"$page": 1,
		"$size": 10,
	}

	query, err := model.Parse(
		payload,
		model.WithFilterFieldResolver(userSchema.ResolveFilterField),
		model.WithGroupFieldResolver(userSchema.ResolveGroupField),
		model.WithSortFieldResolver(userSchema.ResolveSortField),
		model.WithValueDecoder(userSchema.DecodeValue),
	)
	if err != nil {
		panic(err)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		panic(err)
	}

	tx, err := gormadapter.New(
		gormadapter.WithQueryTransformer(userSchema.Normalize),
		gormadapter.WithQueryTransformer(userSchema.ToStorage),
	).Apply(db.Model(&user{}), query)
	if err != nil {
		panic(err)
	}

	result := tx.Find(&[]user{})
	if result.Error != nil {
		panic(result.Error)
	}

	fmt.Println(result.Statement.SQL.String())
	fmt.Println(preloadNames(result.Statement.Preloads))
}

func preloadNames(preloads map[string][]interface{}) []string {
	names := make([]string, 0, len(preloads))
	for name := range preloads {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
