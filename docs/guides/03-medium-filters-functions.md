# 03. Filters, Functions, And Aggregates

The same scalar expression model is used in projections, predicates, grouping,
sorting, and function arguments.

## Grouped Filters And Operators

```go
query, err := qb.New().
	Where(qb.And(
		qb.F("users.status").In("active", "trial"),
		qb.Or(
			qb.F("users.name").ILike("jo%"),
			qb.F("users.email").Suffix("@example.com"),
		),
		qb.F("users.bio").Contains("golang"),
		qb.F("users.role").NotIn("banned", "suspended"),
		qb.F("users.deleted_at").IsNull(),
		qb.Not(qb.F("users.email").Prefix("test+")),
	)).
	Query()
```

## Use Functions Anywhere Scalars Are Allowed

```go
query, err := qb.New().
	SelectProjection(
		qb.F("users.name").Lower().As("normalized_name"),
		qb.F("users.name").Substring(1, 4).As("short_name"),
		qb.F("users.first_name").Concat(" ", qb.F("users.last_name")).As("full_name"),
		qb.Round(qb.F("users.amount").Cast("decimal"), 2).As("rounded_amount"),
		qb.F("users.profile").JsonValue("$.nickname").As("nickname"),
	).
	Where(qb.And(
		qb.F("users.name").Lower().Eq("john"),
		qb.Func("substring", qb.F("users.name"), 1, 4).Eq("john"),
	)).
	GroupByExpr(qb.F("users.name").Lower()).
	SortByExpr(qb.F("users.name").Lower(), qb.Asc).
	Query()
```

## Aggregate Queries

```go
day := qb.F("orders.created_at").DateTrunc("day")

query, err := qb.New().
	SelectProjection(
		day.As("day"),
		qb.Count().As("order_count"),
		qb.F("orders.total").Sum().As("gross_total"),
		qb.F("orders.total").Avg().As("avg_total"),
	).
	Where(qb.And(
		qb.F("orders.status").Eq("paid"),
		qb.F("orders.created_at").Gte("2026-04-01T00:00:00Z"),
	)).
	GroupByExpr(day).
	SortByExpr(day, qb.Desc).
	Query()
```

## Useful Helper Families

- string helpers: `Lower`, `Upper`, `Trim`, `Concat`, `Substring`, `Replace`
- math helpers: `Abs`, `Ceil`, `Floor`, `Mod`, `Round`, `RoundDouble`
- aggregate helpers: `Count`, `Sum`, `Avg`, `Min`, `Max`
- date/time helpers: `Date`, `DateTrunc`, `Extract`, `DateBin`, `CurrentTimestamp`
- JSON helpers: `JsonExtract`, `JsonQuery`, `JsonValue`, `JsonExists`, `JsonArrayLength`

## Portability Notes

- `Round(..., scale)` is the portable choice for `decimal` or `numeric` values.
- `RoundDouble(..., scale)` is the PostgreSQL-safe helper for double precision.
- Some helpers and operators are dialect-specific; unsupported ones return structured `qb.Error` values.

## Matching Examples

- [examples/core/operators](../../examples/core/operators/main.go)
- [examples/core/functions](../../examples/core/functions/main.go)
- [examples/core/scalar-toolbox](../../examples/core/scalar-toolbox/main.go)
- [examples/analytics/reporting](../../examples/analytics/reporting/main.go)
