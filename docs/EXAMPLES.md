# Examples

This document shows how to use `qb` from basic fluent builder usage to more
advanced schema, relation, and pagination patterns.

For runnable versions of these patterns, see [`examples/README.md`](../examples/README.md), including [`10-functions`](../examples/10-functions).

The core idea stays the same in every example:

1. Build or parse a `qb.Query`.
2. Optionally normalize or rewrite it.
3. Hand it to an adapter such as `adapter/sql` or `adapter/gorm`.

## Imports Used In The Snippets

```go
import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/pakasa-io/qb"
	gormadapter "github.com/pakasa-io/qb/adapter/gorm"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"github.com/pakasa-io/qb/parser/mapinput"
	"github.com/pakasa-io/qb/parser/querystring"
	"github.com/pakasa-io/qb/schema"
	"gorm.io/gorm"
)
```

## 1. Basic Fluent Builder

Use the builder when the query is being assembled inside Go code.

```go
query, err := qb.New().
	Pick("id", "status").
	Where(qb.And(
		qb.Field("status").Eq("active"),
		qb.Field("age").Gte(18),
	)).
	SortBy("created_at", qb.Desc).
	Page(3).
	Size(20).
	Query()
if err != nil {
	panic(err)
}

statement, err := sqladapter.New().Compile(query)
if err != nil {
	panic(err)
}

fmt.Println(statement.SQL)
fmt.Println(statement.Args)
```

Expected SQL shape:

```sql
SELECT "id", "status" WHERE ("status" = ? AND "age" >= ?) ORDER BY "created_at" DESC LIMIT 20 OFFSET 40
```

## 2. Parse A JSON Structure

Use `parser/mapinput` when your API receives a JSON body or a decoded
`map[string]any`.

```go
payload := []byte(`{
  "pick": ["id", "status"],
  "where": {
    "status": "active",
    "age": { "$gte": 18 },
    "$or": [
      { "role": "admin" },
      { "role": "owner" }
    ]
  },
  "include": ["Customer"],
  "group_by": ["id", "status"],
  "sort": ["-created_at", "name"],
  "page": 3,
  "size": 20
}`)

query, err := mapinput.ParseJSON(payload)
if err != nil {
	panic(err)
}

statement, err := sqladapter.New().Compile(query)
if err != nil {
	panic(err)
}
```

Supported top-level constructs:

- `select` or `pick`
- `include`
- `where` or `filter`
- `sort`
- `group_by`
- `page`
- `size`
- `cursor`
- legacy `limit` and `offset`

Supported filter operators:

- `$eq`, `$ne`
- `$gt`, `$gte`, `$lt`, `$lte`
- `$in`, `$nin`
- `$like`, `$contains`, `$prefix`, `$suffix`
- `$isnull`, `$notnull`
- `$and`, `$or`, `$not`

## 3. Parse A Query String

Use `parser/querystring` when your API receives HTTP query parameters.

```go
values := url.Values{
	"pick":                        {"id,status"},
	"include":                     {"Customer"},
	"where[status][$eq]":      {"active"},
	"where[age][$gte]":        {"21"},
	"where[$or][0][role][$eq]": {"admin"},
	"where[$or][1][role][$eq]": {"owner"},
	"sort":                    {"-created_at,name"},
	"group_by":                {"id,status"},
	"page":                    {"2"},
	"size":                    {"10"},
}

query, err := querystring.Parse(values)
if err != nil {
	panic(err)
}

statement, err := sqladapter.New().Compile(query)
if err != nil {
	panic(err)
}
```

This is useful for REST APIs because it keeps the query syntax transport-agnostic
while still accepting conventional query-string input.

## 4. Nested Filters

Nested filters work the same way whether they come from JSON or the fluent API.

### JSON style

```go
input := map[string]any{
	"where": map[string]any{
		"$or": []any{
			map[string]any{
				"status": "active",
				"age":    map[string]any{"$gte": 18},
			},
			map[string]any{
				"status": "trial",
				"age":    map[string]any{"$gte": 21},
			},
		},
	},
}

query, err := mapinput.Parse(input)
```

### Fluent style

```go
query, err := qb.New().
	Where(qb.Or(
		qb.And(
			qb.Field("status").Eq("active"),
			qb.Field("age").Gte(18),
		),
		qb.And(
			qb.Field("status").Eq("trial"),
			qb.Field("age").Gte(21),
		),
	)).
	Query()
```

## 5. Grouped Filters And Negation

Use explicit groups when precedence matters.

```go
query, err := qb.New().
	Where(qb.And(
		qb.Field("status").Eq("active"),
		qb.Or(
			qb.Field("role").Eq("admin"),
			qb.Field("role").Eq("owner"),
		),
		qb.Not(qb.Field("deleted_at").IsNull()),
	)).
	Query()
if err != nil {
	panic(err)
}
```

This produces a query equivalent to:

```text
status = active
AND (role = admin OR role = owner)
AND NOT deleted_at IS NULL
```

## 6. Schema-Driven Public API Fields

Use `schema` when you want a stable external query vocabulary that is different
from storage column names.

```go
userSchema := schema.MustNew(
	schema.Define(
		"status",
		schema.Storage("users.status"),
		schema.Aliases("state"),
		schema.Operators(qb.OpEq, qb.OpIn),
	),
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
	schema.Define(
		"created_at",
		schema.Storage("users.created_at"),
		schema.Aliases("createdAt"),
		schema.Sortable(),
		schema.DisableFiltering(),
	),
)

query, err := mapinput.Parse(
	map[string]any{
		"where": map[string]any{
			"state":  "active",
			"minAge": map[string]any{"$gte": "21"},
		},
		"sort": "-createdAt",
	},
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
```

Use:

- `Normalize` when you want canonical API-facing field names and decoded values
- `ToStorage` when you want adapter-facing storage identifiers too

## 7. Entity Relations

`qb` intentionally does not own joins as a first-class AST concept. Relation
filtering is expressed through schema/storage mapping and handled by the caller
or adapter-specific query setup.

### SQL relation example

```go
orderSchema := schema.MustNew(
	schema.Define("status", schema.Storage("orders.status")),
	schema.Define("customer.email", schema.Storage("customers.email")),
	schema.Define("customer.company.name", schema.Storage("companies.name")),
	schema.Define("created_at", schema.Storage("orders.created_at"), schema.Sortable()),
)

query, err := qb.New().
	Where(qb.And(
		qb.Field("customer.email").Suffix("@example.com"),
		qb.Field("customer.company.name").Eq("Acme"),
	)).
	SortBy("created_at", qb.Desc).
	Query()
if err != nil {
	panic(err)
}

statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(orderSchema.ToStorage),
).Compile(query)
if err != nil {
	panic(err)
}

baseSQL := `
SELECT orders.*
FROM orders
JOIN customers ON customers.id = orders.customer_id
JOIN companies ON companies.id = customers.company_id
`

finalSQL := baseSQL + " " + statement.SQL
fmt.Println(finalSQL, statement.Args)
```

### GORM relation example

The relation joins stay in GORM, while `qb` contributes the filter and sort.

```go
orderSchema := schema.MustNew(
	schema.Define("customer.email", schema.Storage("customers.email")),
	schema.Define("created_at", schema.Storage("orders.created_at"), schema.Sortable()),
)

query, err := qb.New().
	Where(qb.Field("customer.email").Suffix("@example.com")).
	SortBy("created_at", qb.Desc).
	Query()
if err != nil {
	panic(err)
}

tx := db.Model(&Order{}).
	Joins("Customer").
	Scopes(
		gormadapter.New(
			gormadapter.WithQueryTransformer(orderSchema.ToStorage),
		).Scope(query),
	)
```

This separation is deliberate: the query model stays storage-agnostic, while
join orchestration stays where actual relation loading belongs.

## 8. Offset Pagination With Page And Size

Offset pagination uses `page` and `size` as the primary inputs.

```go
query, err := qb.New().
	Where(qb.Field("status").Eq("active")).
	SortBy("created_at", qb.Desc).
	Page(3).
	Size(25).
	Query()
if err != nil {
	panic(err)
}
```

`query.ResolvedPagination()` turns that into `LIMIT 25 OFFSET 50` for
adapters.

`limit` and `offset` are still accepted for compatibility, but new APIs should
prefer `page` and `size`.

## 9. Cursor Pagination With A Token

Cursor pagination is first-class metadata in `qb.Query`, but it is intentionally
adapter-agnostic. A query transformer is expected to turn the cursor into
deterministic filters plus sorts before an adapter sees it.

For descending pagination by `created_at`:

```go
query, err := qb.New().
	SortBy("created_at", qb.Desc).
	Size(25).
	CursorToken("opaque-token").
	Query()
if err != nil {
	panic(err)
}

cursorTransformer := func(query qb.Query) (qb.Query, error) {
	if query.Cursor == nil {
		return query, nil
	}

	cursorTime := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	query.Filter = qb.Field("created_at").Lt(cursorTime)
	query.Cursor = nil
	return query, nil
}

statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(cursorTransformer),
).Compile(query)
```

Interpretation:

- the parser or caller stores the raw cursor metadata in `query.Cursor`
- a transformer decodes the token and rewrites the query
- adapters only see plain filters, sorts, and the resolved `size`

## 10. Composite Cursor Pagination

When the sort key is not unique, use a tie-breaker. A common pattern is
`created_at DESC, id DESC`.

```go
type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        int64     `json:"id"`
}

query, err := qb.New().
	SortBy("created_at", qb.Desc).
	SortBy("id", qb.Desc).
	Size(25).
	CursorValues(map[string]any{
		"created_at": time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
		"id":         int64(981),
	}).
	Query()
if err != nil {
	panic(err)
}

cursorTransformer := func(query qb.Query) (qb.Query, error) {
	if query.Cursor == nil {
		return query, nil
	}

	createdAt := query.Cursor.Values["created_at"]
	id := query.Cursor.Values["id"]
	query.Filter = qb.Or(
		qb.Field("created_at").Lt(createdAt),
		qb.And(
			qb.Field("created_at").Eq(createdAt),
			qb.Field("id").Lt(id),
		),
	)
	query.Cursor = nil
	return query, nil
}
```

That gives stable page boundaries even when multiple rows share the same
timestamp.

### Encoding and decoding a cursor token

```go
func encodeCursor(cursor Cursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(token string) (Cursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, err
	}

	var cursor Cursor
	err = json.Unmarshal(data, &cursor)
	return cursor, err
}
```

## 11. Rewrite Pipelines For Multi-Tenancy Or Soft Deletes

Use `qb.RewriteQuery` or `qb.ComposeTransformers` when you need global policies.

```go
tenantFilter := func(tenantID int64) qb.QueryTransformer {
	return func(query qb.Query) (qb.Query, error) {
		if query.Filter == nil {
			query.Filter = qb.Field("tenant_id").Eq(tenantID)
			return query, nil
		}

		query.Filter = qb.And(
			qb.Field("tenant_id").Eq(tenantID),
			query.Filter,
		)
		return query, nil
	}
}

softDeleteFilter := func(query qb.Query) (qb.Query, error) {
	if query.Filter == nil {
		query.Filter = qb.Field("deleted_at").IsNull()
		return query, nil
	}

	query.Filter = qb.And(
		qb.Field("deleted_at").IsNull(),
		query.Filter,
	)
	return query, nil
}

pipeline := qb.ComposeTransformers(
	tenantFilter(42),
	softDeleteFilter,
)

statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(pipeline),
).Compile(query)
```

If you want to rewrite specific nodes rather than whole queries, use
`qb.RewriteQuery`.

## 12. Error Handling

Errors are structured and include stage metadata.

```go
_, err := mapinput.Parse(map[string]any{
	"limit": "not-a-number",
})
if err != nil {
	var diagnostic *qb.Error
	if errors.As(err, &diagnostic) {
		fmt.Println(diagnostic.Stage) // parse
		fmt.Println(diagnostic.Code)  // invalid_value
		fmt.Println(diagnostic.Path)  // limit
	}
}
```

Useful stages:

- `parse`
- `normalize`
- `rewrite`
- `compile`
- `apply`

## 13. Choosing The Right Entry Point

Use this as the default decision guide:

- Fluent builder: internal Go code and service-layer query assembly.
- `parser/mapinput`: JSON body or already-decoded generic map payloads.
- `parser/querystring`: REST-style query parameters.
- `schema.Normalize`: keep public field names and decoded values.
- `schema.ToStorage`: compile or apply with storage-facing identifiers.
- `adapter/sql`: raw SQL fragments for hand-built SQL execution.
- `adapter/gorm`: apply the query to an existing GORM chain.

## 14. Current Boundary And Future Expansion

The current library handles:

- filtering
- grouping and negation
- field projection through `select` or `pick`
- include/preload hints through `include`
- sorting
- `group_by`
- offset pagination through `page` and `size`
- cursor metadata that can be rewritten into filter-and-sort patterns
- schema-based aliasing, validation, and storage projection

The current library does not model these as first-class AST nodes yet:

- joins
- aggregations

That boundary is usually the right tradeoff early on because it keeps the core
small and lets parsers, schemas, and adapters evolve independently.
