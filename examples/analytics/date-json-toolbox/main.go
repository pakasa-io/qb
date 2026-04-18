package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
)

func main() {
	timeBucket := qb.DateBin("15 minutes", qb.F("events.created_at"), "2001-01-01T00:00:00Z")
	country := qb.F("events.payload").JsonValue("$.country")

	query, err := qb.New().
		SelectProjection(
			qb.F("events.payload").JsonExtract("$.customer").As("customer_json"),
			qb.F("events.payload").JsonQuery("$.items").As("items_json"),
			country.As("country"),
			qb.F("events.payload").JsonExists("$.customer.id").As("has_customer"),
			qb.F("events.payload").JsonType("$.items").As("items_type"),
			qb.F("events.payload").JsonArrayLength("$.items").As("item_count"),
			qb.JsonArray("web", "mobile", qb.F("events.source")).As("channels"),
			qb.JsonObject("status", qb.F("events.status"), "country", qb.F("events.country")).As("metadata"),
			qb.F("events.created_at").Extract("year").As("event_year"),
			timeBucket.As("time_bucket"),
			qb.CurrentDate().As("report_date"),
			qb.CurrentTime().As("report_time"),
			qb.CurrentTimestamp().As("report_timestamp"),
		).
		Where(qb.And(
			qb.F("events.payload").JsonExists("$.customer.id").Eq(true),
			qb.F("events.created_at").Gte("2026-04-01T00:00:00Z"),
		)).
		GroupByExpr(
			country,
			timeBucket,
			qb.F("events.payload").JsonExtract("$.customer"),
			qb.F("events.payload").JsonQuery("$.items"),
			qb.F("events.payload").JsonExists("$.customer.id"),
			qb.F("events.payload").JsonType("$.items"),
			qb.F("events.payload").JsonArrayLength("$.items"),
		).
		SortByExpr(timeBucket, qb.Desc).
		Query()
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(sqladapter.WithDialect(sqladapter.PostgresDialect{})).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}
