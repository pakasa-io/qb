# 04. Schema, Aliases, And Storage Mapping

Use `schema` when your public API should not expose storage column names
directly.

## Define Canonical Fields

```go
userSchema := schema.MustNew(
	schema.Define("status", schema.Storage("users.status"), schema.Aliases("state"), schema.Sortable()),
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
	schema.Define("created_at", schema.Storage("users.created_at"), schema.Aliases("createdAt"), schema.Sortable()),
)
```

## Parse Public Input Through The Schema

```go
query, err := codecs.Parse(
	map[string]any{
		"$select": "state",
		"$where": map[string]any{
			"state":  "active",
			"minAge": map[string]any{"$gte": "21"},
		},
		"$sort": "-createdAt",
	},
	codecs.WithFilterFieldResolver(userSchema.ResolveFilterField),
	codecs.WithGroupFieldResolver(userSchema.ResolveGroupField),
	codecs.WithSortFieldResolver(userSchema.ResolveSortField),
	codecs.WithValueDecoder(userSchema.DecodeValue),
)
```

## Normalize vs Project To Storage

Use `Normalize` when you want canonical API-facing names and decoded values:

```go
normalized, err := userSchema.Normalize(query)
```

Use `ToStorage` when adapters should see storage-facing names:

```go
projected, err := userSchema.ToStorage(query)
```

`ToStorage` already calls `Normalize`, so this is the normal adapter-facing path:

```go
statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(userSchema.ToStorage),
).Compile(query)
```

Run `Normalize` separately only when you actually need the intermediate,
canonical API-facing query before storage projection.

## Relation Paths

The schema layer can map nested API fields without teaching the core how to
join tables:

```go
orderSchema := schema.MustNew(
	schema.Define("customer.email", schema.Storage("customers.email")),
	schema.Define("customer.company.name", schema.Storage("companies.name")),
	schema.Define("created_at", schema.Storage("orders.created_at"), schema.Sortable()),
)
```

## What Schema Gives You

- aliases like `createdAt` or `state`
- per-field operator allowlists
- sort allowlists
- field-specific decoding and coercion
- projection from canonical names to storage names

Structured cursor values flow through the same resolver and decoder pipeline.

For stricter public API patterns and more complex schema behavior, see
[12. Complex Schema Patterns](./12-complex-schema-patterns.md).

## Matching Examples

- [examples/schema/storage-mapping](../../examples/schema/storage-mapping/main.go)
- [examples/schema/cursor-normalization](../../examples/schema/cursor-normalization/main.go)
- [examples/core/relations](../../examples/core/relations/main.go)
