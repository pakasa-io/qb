# 07. Rewrites, Preflight, And Diagnostics

Transformers are the main extension point for policy, defaulting, cursor
rewrites, storage projection, and capability validation.

## Compose A Rewrite Pipeline

```go
pipeline := qb.ComposeTransformers(
	tenantFilter(42),
	softDeleteFilter,
	enforceMaxSize(100),
	defaultSort("created_at", qb.Desc),
	userSchema.ToStorage,
	sqladapter.PostgresDialect{}.Capabilities().Validator(qb.StageCompile),
)

prepared, err := qb.TransformQuery(query, pipeline)
if err != nil {
	panic(err)
}
```

The same pipeline can be attached directly to a compiler or adapter:

```go
statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(pipeline),
).Compile(query)
```

`ToStorage` already normalizes the query. Add a separate `Normalize` step only
when a custom transformer needs to inspect canonical API-facing field names
before storage projection.

## Common Transformer Patterns

Inject tenant or soft-delete filters:

```go
func tenantFilter(tenantID int64) qb.QueryTransformer {
	return func(query qb.Query) (qb.Query, error) {
		query.Filter = qb.And(qb.F("tenant_id").Eq(tenantID), query.Filter)
		return query, nil
	}
}
```

Cap page size:

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

## Rewrite The Filter Tree Directly

Use `RewriteQuery` when you only want to transform predicates:

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

## Inspect Structured Errors

Every major stage can return a `*qb.Error`:

```go
var diagnostic *qb.Error
if errors.As(err, &diagnostic) {
	fmt.Println("stage:", diagnostic.Stage)
	fmt.Println("code:", diagnostic.Code)
	fmt.Println("path:", diagnostic.Path)
	fmt.Println("field:", diagnostic.Field)
	fmt.Println("operator:", diagnostic.Operator)
	fmt.Println("function:", diagnostic.Function)
}
```

## Capability Checks

You can inspect a dialect up front:

```go
capabilities := sqladapter.SQLiteDialect{}.Capabilities()

fmt.Println(capabilities.SupportsOperator(qb.OpILike))
fmt.Println(capabilities.SupportsFunction("date_trunc"))
```

Or fail early with a validator transformer:

```go
validator := sqladapter.SQLiteDialect{}.Capabilities().Validator(qb.StageCompile)
prepared, err := qb.TransformQuery(query, validator)
```

## Matching Examples

- [examples/core/rewrite-pipeline](../../examples/core/rewrite-pipeline/main.go)
- [examples/core/preflight-pipeline](../../examples/core/preflight-pipeline/main.go)
- [examples/core/errors](../../examples/core/errors/main.go)
- [examples/adapters/capabilities](../../examples/adapters/capabilities/main.go)

For a deeper transformer catalog and ordering guidance, see
[11. Transformer Patterns](./11-transformer-patterns.md).
