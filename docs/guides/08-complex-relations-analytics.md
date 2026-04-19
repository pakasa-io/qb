# 08. Relations And Analytics

The core stays intentionally small, but it still supports relation-oriented
API fields and analytics-style expression trees.

## Relation Filtering Without Core Joins

Let the schema map relation paths to storage names, then add joins outside the
core query:

```go
orderSchema := schema.MustNew(
	schema.Define("status", schema.Storage("orders.status")),
	schema.Define("customer.email", schema.Storage("customers.email")),
	schema.Define("customer.company.name", schema.Storage("companies.name")),
	schema.Define("created_at", schema.Storage("orders.created_at"), schema.Sortable()),
)

query, err := qb.New().
	Where(qb.And(
		qb.F("customer.email").Suffix("@example.com"),
		qb.F("customer.company.name").Eq("Acme"),
	)).
	SortBy("created_at", qb.Desc).
	Query()

statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(orderSchema.ToStorage),
).Compile(query)
```

Wrap the compiled fragment with your own joins:

```sql
SELECT orders.*
FROM orders
JOIN customers ON customers.id = orders.customer_id
JOIN companies ON companies.id = customers.company_id
WHERE ...
```

## Reporting Queries

```go
day := qb.F("orders.created_at").DateTrunc("day")

query, err := qb.New().
	SelectProjection(
		day.As("day"),
		qb.Count().As("order_count"),
		qb.F("orders.total").Sum().As("gross_total"),
		qb.F("orders.total").Avg().As("avg_total"),
	).
	Where(qb.F("orders.status").Eq("paid")).
	GroupByExpr(day).
	SortByExpr(day, qb.Desc).
	Page(1).
	Size(30).
	Query()
```

## JSON And Date Heavy Analytics

PostgreSQL-first helpers let you keep complex expressions inside the query AST:

```go
timeBucket := qb.DateBin("15 minutes", qb.F("events.created_at"), "2001-01-01T00:00:00Z")
country := qb.F("events.payload").JsonValue("$.country")
metadata := qb.JsonObject("status", qb.F("events.status"), "country", qb.F("events.country"))
itemCount := qb.F("events.payload").JsonArrayLength("$.items")
eventYear := qb.F("events.created_at").Extract("year")

query, err := qb.New().
	SelectProjection(
		country.As("country"),
		timeBucket.As("time_bucket"),
		itemCount.As("item_count"),
		metadata.As("metadata"),
		eventYear.As("event_year"),
	).
	Where(qb.And(
		qb.F("events.payload").JsonExists("$.customer.id").Eq(true),
		qb.F("events.created_at").Gte("2026-04-01T00:00:00Z"),
	)).
	GroupByExpr(
		country,
		timeBucket,
		itemCount,
		metadata,
		eventYear,
	).
	SortByExpr(timeBucket, qb.Desc).
	Query()
```

## Good Patterns For Complex Queries

- keep the external payload small and push policy into transformers
- reuse computed scalars like `day` or `timeBucket` so `SELECT`, `GROUP BY`, and `ORDER BY` stay aligned
- validate against dialect capabilities before the final compile step

## Matching Examples

- [examples/core/relations](../../examples/core/relations/main.go)
- [examples/analytics/reporting](../../examples/analytics/reporting/main.go)
- [examples/analytics/json-analytics](../../examples/analytics/json-analytics/main.go)
- [examples/analytics/date-json-toolbox](../../examples/analytics/date-json-toolbox/main.go)
