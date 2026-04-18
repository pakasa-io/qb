package main

import (
	"fmt"

	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
	yamlcodec "github.com/pakasa-io/qb/codecs/yamlcodec"
)

func main() {
	payload := []byte(`
$select:
  - users.id
  - "lower(users.name) as normalized_name"
$where:
  status: active
  age:
    $gte: 18
  $expr:
    $eq:
      - "lower(@users.name)"
      - "lower('john')"
$group:
  - "lower(users.name)"
$sort:
  - "lower(users.name) asc"
$page: 2
$size: 10
`)

	query, err := yamlcodec.Parse(payload)
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
