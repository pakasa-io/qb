package main

import (
	"fmt"
	"sort"

	"github.com/pakasa-io/qb"
	gormadapter "github.com/pakasa-io/qb/adapters/gorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type user struct {
	ID        int
	Status    string
	Role      string
	CompanyID int
	Company   company
	Orders    []order
}

type company struct {
	ID   int
	Name string
}

type order struct {
	ID     int
	UserID int
}

func main() {
	query, err := qb.New().
		Select("id", "status", "company_id").
		Include("Company", "Orders").
		Where(qb.Field("status").Eq("active")).
		SortBy("created_at", qb.Desc).
		Page(1).
		Size(10).
		Query()
	if err != nil {
		panic(err)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		panic(err)
	}

	tx, err := gormadapter.New().Apply(db.Model(&user{}), query)
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
