package main

import (
	"fmt"

	sqladapter "github.com/pakasa-io/qb/adapters/sql"
	jsoncodec "github.com/pakasa-io/qb/codecs/jsoncodec"
)

func main() {
	payload := []byte(`{
  "$select": [
    "date_trunc('day', events.created_at) as day",
    "json_value(events.payload, '$.country') as country",
    "count() as event_count",
    "sum(events.revenue::decimal) as gross_revenue"
  ],
  "$where": {
    "events.status": "processed",
    "$expr": {
      "$gte": ["json_array_length(@events.items)", 2]
    }
  },
  "$group": [
    "date_trunc('day', events.created_at)",
    "json_value(events.payload, '$.country')"
  ],
  "$sort": [
    "date_trunc('day', events.created_at) desc",
    "json_value(events.payload, '$.country') asc"
  ],
  "$page": 1,
  "$size": 50
}`)

	query, err := jsoncodec.Parse(payload)
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
