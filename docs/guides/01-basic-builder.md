# 01. Basic Builder

Use the fluent builder when your Go code owns the query shape.

## Imports

```go
import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)
```

## Build A Query

```go
query, err := qb.New().
	Pick("id", "status").
	Where(qb.And(
		qb.Field("status").Eq("active"),
		qb.Field("age").Gte(18),
	)).
	SortBy("created_at", qb.Desc).
	Page(2).
	Size(20).
	Query()
if err != nil {
	panic(err)
}
```

Multiple `Where(...)` calls are merged with `AND`, so this is equivalent:

```go
query, err := qb.New().
	Where(qb.F("status").Eq("active")).
	Where(qb.F("age").Gte(18)).
	Query()
```

## Compile To SQL

```go
statement, err := sqladapter.New().Compile(query)
if err != nil {
	panic(err)
}

fmt.Println(statement.SQL)
fmt.Println(statement.Args)
```

## Select Expressions Instead Of Plain Fields

```go
query, err := qb.New().
	SelectProjection(
		qb.F("users.id").As("id"),
		qb.F("users.name").Lower().As("normalized_name"),
	).
	GroupByExpr(qb.F("users.name").Lower()).
	SortByExpr(qb.F("users.name").Lower(), qb.Asc).
	Query()
```

## Legacy Limit And Offset

`Page` and `Size` are the preferred pagination API, but legacy `Limit` and
`Offset` still exist:

```go
query, err := qb.New().
	SortBy("created_at", qb.Desc).
	Limit(50).
	Offset(100).
	Query()
```

## Rules To Remember

- `Pick(...)` is an alias for `Select(...)`.
- `Page(...)` requires `Size(...)`.
- `CursorToken(...)` and `CursorValues(...)` also require `Size(...)`.
- Builder validation happens at `Query()`, not when you chain each call.

## Matching Examples

- [examples/core/basic-builder](../../examples/core/basic-builder/main.go)
- [examples/core/functions](../../examples/core/functions/main.go)
