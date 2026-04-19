package gorm_test

import (
	"fmt"
	"sort"

	"github.com/pakasa-io/qb"
	gormadapter "github.com/pakasa-io/qb/adapters/gorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type exampleUser struct {
	ID        int
	Status    string
	CompanyID int
	Company   exampleCompany
}

type exampleCompany struct {
	ID int
}

func ExampleAdapter_Apply() {
	query, err := qb.New().
		Select("id", "status", "company_id").
		Include("Company").
		Where(qb.F("status").Eq("active")).
		Page(1).
		Size(5).
		Query()
	if err != nil {
		panic(err)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		panic(err)
	}

	tx, err := gormadapter.New().Apply(db.Model(&exampleUser{}), query)
	if err != nil {
		panic(err)
	}

	result := tx.Find(&[]exampleUser{})
	if result.Error != nil {
		panic(result.Error)
	}

	fmt.Println(result.Statement.SQL.String())
	fmt.Println(preloadNames(result.Statement.Preloads))

	// Output:
	// SELECT "id", "status", "company_id" FROM `example_users` WHERE "status" = ? LIMIT 5
	// [Company]
}

func preloadNames(preloads map[string][]interface{}) []string {
	names := make([]string, 0, len(preloads))
	for name := range preloads {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
