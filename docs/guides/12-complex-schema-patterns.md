# 12. Complex Schema Patterns

Use `schema` when the public query model should be stable even while storage
names, decode rules, and API constraints evolve.

This is a deeper companion to [04. Schema, Aliases, And Storage Mapping](./04-medium-schema-mapping.md).

## Build A Strict Public Query Surface

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
	schema.Define("company.id", schema.Storage("users.company_id")),
)
```

This gives you a public API where:

- `state` maps to canonical field `status`
- `createdAt` can sort but cannot be filtered
- `minAge` accepts only the operators you allow
- `company.id` can stay API-friendly while still mapping to storage

## Parse Transport Input Through Schema Resolvers

```go
query, err := codecs.Parse(
	payload,
	codecs.WithFilterFieldResolver(userSchema.ResolveFilterField),
	codecs.WithGroupFieldResolver(userSchema.ResolveGroupField),
	codecs.WithSortFieldResolver(userSchema.ResolveSortField),
	codecs.WithValueDecoder(userSchema.DecodeValue),
)
```

Use the resolver that matches the context:

- `ResolveFilterField` enforces filterability and operator allowlists
- `ResolveSortField` enforces sortability
- `ResolveGroupField` canonicalizes names for grouping without requiring sortability
- `ResolveCursorField` canonicalizes structured cursor keys

## Preserve String-Looking Query-String Values

Query strings are a common place to over-coerce values. This pattern keeps
string-looking identifiers intact while decoding only the fields that need it.

```go
userSchema := schema.MustNew(
	schema.Define("zip", schema.Storage("users.zip")),
	schema.Define("status", schema.Storage("users.status")),
	schema.Define("external_id", schema.Storage("users.external_id")),
	schema.Define(
		"age",
		schema.Storage("users.age"),
		schema.Decode(func(_ qb.Operator, value any) (any, error) {
			switch typed := value.(type) {
			case string:
				return strconv.Atoi(typed)
			default:
				return value, nil
			}
		}),
	),
)

values := url.Values{
	"$where[zip]":             {"02110"},
	"$where[status]":          {"false"},
	"$where[age][$gte]":       {"21"},
	"$where[$expr][$eq][0]":   {"@external_id"},
	"$where[$expr][$eq][1]":   {"0012"},
	"$where[$expr][$notnull]": {"@external_id"},
}

query, err := querystring.Parse(
	values,
	codecs.WithFilterFieldResolver(userSchema.ResolveFilterField),
	codecs.WithValueDecoder(userSchema.DecodeValue),
)
```

With this shape:

- `age` becomes an integer because the schema decoder says so
- `zip` stays `"02110"`
- `status` stays `"false"` instead of becoming a boolean
- `$expr` literals such as `"0012"` stay string literals

## Rewrite Functions And Casts Through The Schema

Schema projection rewrites references inside function expressions, casts,
grouping, and sorting, not just simple field predicates.

```go
metricSchema := schema.MustNew(
	schema.Define("name", schema.Storage("users.name"), schema.Sortable()),
	schema.Define("age", schema.Storage("users.age"), schema.Sortable()),
)

query, err := qb.New().
	SelectProjection(
		qb.F("name").Lower().As("normalized_name"),
		qb.Round(qb.F("age").Cast("decimal"), 2).As("rounded_age"),
	).
	GroupByExpr(
		qb.F("name").Lower(),
		qb.F("age").Cast("decimal"),
	).
	SortByExpr(qb.F("age").Cast("double"), qb.Desc).
	Where(qb.And(
		qb.F("name").Lower().Eq("john"),
		qb.F("age").Cast("decimal").Gte(18),
	)).
	Query()

projected, err := metricSchema.ToStorage(query)
```

After projection, the same expression tree uses `users.name` and `users.age`
inside every call and cast.

## Canonical Queries vs Storage Queries

Use `Normalize` when you need a canonical API-facing query:

```go
normalized, err := userSchema.Normalize(query)
```

Use `ToStorage` when the adapter should see storage names:

```go
projected, err := userSchema.ToStorage(query)
```

`ToStorage` already normalizes internally, so use one or the other by default,
not both.

## Cursor-Heavy Schema Flows

Structured cursors go through the same alias and decoder machinery:

```go
orderSchema := schema.MustNew(
	schema.Define("created_at", schema.Storage("orders.created_at"), schema.Aliases("createdAt"), schema.Sortable()),
	schema.Define(
		"id",
		schema.Storage("orders.id"),
		schema.Sortable(),
		schema.Decode(func(_ qb.Operator, value any) (any, error) {
			switch typed := value.(type) {
			case string:
				return strconv.Atoi(typed)
			default:
				return value, nil
			}
		}),
	),
)

query, err := codecs.Parse(
	map[string]any{
		"$sort": []any{"createdAt desc", "id desc"},
		"$cursor": map[string]any{
			"createdAt": "2026-04-11T12:00:00Z",
			"id":        "981",
		},
		"$size": 25,
	},
	codecs.WithSortFieldResolver(orderSchema.ResolveSortField),
	codecs.WithValueDecoder(orderSchema.DecodeValue),
)

projected, err := orderSchema.ToStorage(query)
```

That flow:

- normalizes `createdAt` to canonical `created_at`
- decodes `"981"` into `981`
- projects cursor keys to `orders.created_at` and `orders.id`

## Failure Modes Worth Documenting

- duplicate aliases are rejected when the schema is built
- filtering fails on `DisableFiltering()` fields
- unsupported operators fail during normalization
- duplicate structured cursor keys after alias normalization fail
- duplicate storage cursor keys after projection fail

## Matching References

- [examples/schema/storage-mapping](../../examples/schema/storage-mapping/main.go)
- [examples/schema/cursor-normalization](../../examples/schema/cursor-normalization/main.go)
- [examples/schema/gorm-public-api](../../examples/schema/gorm-public-api/main.go)
- [examples/codecs/querystring-literals](../../examples/codecs/querystring-literals/main.go)
- [schema tests](../../schema/schema_test.go)
- [schema internal tests](../../schema/schema_internal_test.go)
