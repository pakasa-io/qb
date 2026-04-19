# 11. Transformer Patterns

`qb.QueryTransformer` is the main extension point for policy, rewrites, and
adapter-specific preparation.

This is a deeper companion to [07. Rewrites, Preflight, And Diagnostics](./07-advanced-rewrites-preflight.md).

## Core Shape

```go
type QueryTransformer func(qb.Query) (qb.Query, error)
```

Transformers can:

- add or rewrite filters
- clamp or default pagination
- inject sorts
- rewrite cursor metadata into concrete predicates
- normalize or project schema fields
- reject unsupported query shapes before compile or apply

`qb.TransformQuery(...)` clones the input query before applying the sequence, so
the original query is preserved.

## Compose A Storage-Oriented Pipeline

This pattern works well when the final adapter should only see storage-facing
field names:

```go
pipeline := qb.ComposeTransformers(
	enforceMaxSize(100),
	defaultSort("created_at", qb.Desc),
	orderSchema.ToStorage,
	rewriteCursor,
	sqladapter.PostgresDialect{}.Capabilities().Validator(qb.StageCompile),
)

statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(pipeline),
).Compile(query)
```

In this ordering, `rewriteCursor` should expect storage-facing keys such as
`orders.created_at`.

## Compose An API-Oriented Pipeline

If custom business logic wants canonical API field names instead, normalize
first and inspect that canonical shape separately:

```go
normalized, err := qb.TransformQuery(
	query,
	userSchema.Normalize,
	requireAllowedExportFields,
)
```

Then keep the adapter-facing path storage-oriented:

```go
statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(userSchema.ToStorage),
).Compile(query)
```

Use this split shape only when the intermediate canonical query is actually
useful. Do not feed `normalized` back into `ToStorage` by default, because
`ToStorage` already normalizes internally.

## Common Reusable Transformers

Default a sort when none is provided:

```go
func defaultSort(field string, direction qb.Direction) qb.QueryTransformer {
	return func(query qb.Query) (qb.Query, error) {
		if len(query.Sorts) == 0 {
			query.Sorts = []qb.Sort{{Expr: qb.F(field), Direction: direction}}
		}
		return query, nil
	}
}
```

Clamp page size:

```go
func enforceMaxSize(max int) qb.QueryTransformer {
	return func(query qb.Query) (qb.Query, error) {
		if query.Size != nil && *query.Size > max {
			size := max
			query.Size = &size
		}
		return query, nil
	}
}
```

Inject tenancy or soft-delete filters:

```go
func tenantFilter(tenantID int64) qb.QueryTransformer {
	return func(query qb.Query) (qb.Query, error) {
		query.Filter = qb.And(qb.F("tenant_id").Eq(tenantID), query.Filter)
		return query, nil
	}
}
```

## Cursor Rewrites

Cursor pagination is metadata until your transformer turns it into predicates
and sorts:

```go
func rewriteCursor(query qb.Query) (qb.Query, error) {
	if query.Cursor == nil {
		return query, nil
	}

	createdAt := query.Cursor.Values["orders.created_at"]
	id := query.Cursor.Values["orders.id"]
	query.Filter = qb.Or(
		qb.F("orders.created_at").Lt(createdAt),
		qb.And(
			qb.F("orders.created_at").Eq(createdAt),
			qb.F("orders.id").Lt(id),
		),
	)
	query.Cursor = nil
	return query, nil
}
```

Always clear `query.Cursor` after consuming it so the adapter does not see
stale cursor metadata.

## Filter-Only Rewrites

If you only need to transform the filter tree, use `RewriteQuery`:

```go
rewritten, err := qb.RewriteQuery(query, func(expr qb.Expr) (qb.Expr, error) {
	predicate, ok := expr.(qb.Predicate)
	if !ok {
		return expr, nil
	}

	ref, ok := predicate.Left.(qb.Ref)
	if ok && ref.Name == "state" {
		predicate.Left = qb.F("status")
	}
	return predicate, nil
})
```

This is useful for small filter-only migrations without rewriting projections,
groups, sorts, or pagination.

## Validation-Only Transformers

Not every transformer rewrites. Some only reject:

```go
validator := sqladapter.SQLiteDialect{}.Capabilities().Validator(qb.StageCompile)

prepared, err := qb.TransformQuery(query, validator)
```

You can also write your own:

```go
func requireSort(query qb.Query) (qb.Query, error) {
	if len(query.Sorts) == 0 {
		return qb.Query{}, qb.NewError(
			fmt.Errorf("sorting is required"),
			qb.WithStage(qb.StageRewrite),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
	return query, nil
}
```

## Ordering Rules

- run policy/defaulting before adapter-specific validation so the validator sees the final shape
- run `ToStorage` before custom logic that expects storage names
- run custom canonical-field logic before `ToStorage`
- run cursor rewrites before compile/apply completes, and clear the cursor afterwards

## Where To Attach Transformers

Preflight in your own code:

```go
prepared, err := qb.TransformQuery(query, pipeline)
```

Attach to SQL compilation:

```go
statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(pipeline),
).Compile(query)
```

Attach to GORM application:

```go
tx, err := gormadapter.New(
	gormadapter.WithQueryTransformer(pipeline),
).Apply(db, query)
```

## Matching References

- [examples/core/rewrite-pipeline](../../examples/core/rewrite-pipeline/main.go)
- [examples/core/preflight-pipeline](../../examples/core/preflight-pipeline/main.go)
- [examples/core/cursor-token](../../examples/core/cursor-token/main.go)
- [examples/core/composite-cursor](../../examples/core/composite-cursor/main.go)
- [examples/adapters/capabilities](../../examples/adapters/capabilities/main.go)
- [transform tests](../../transform_test.go)
