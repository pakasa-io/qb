# qb

`qb` is a database-agnostic query builder core for Go.

The package is built around one stable boundary: a semantic `qb.Query` AST.
Parsers compile external inputs into `qb.Query`, and adapters compile `qb.Query`
into storage-specific outputs. The core does not depend on parsers, adapters,
ORMs, or HTTP frameworks.

Core helpers now also include:

- `qb.RewriteQuery` for AST-level rewrites
- `qb.QueryTransformer` and `qb.ComposeTransformers` for shared pipelines
- structured `qb.Error` values for parse/normalize/rewrite/compile/apply failures
- adapter capability metadata for early validation

## Goals

- developer UX through a fluent builder and predictable input formats
- extensibility through a small semantic core
- testability through pure parsing/compilation steps
- low coupling by keeping the core free of transport and ORM concerns

## Package layout

- `qb`: core AST and fluent builder
- `adapter/gorm`: apply `qb.Query` to GORM chains without changing the core
- `schema`: optional field policy layer for aliases, operator allowlists, and decoding
- `parser/mapinput`: parse normalized maps or JSON documents
- `parser/querystring`: parse bracket-notation query strings
- `adapter/sql`: compile to parameterized SQL fragments with pluggable dialects

For runnable examples, see [examples/README.md](examples/README.md).
For the long-form narrative guide, see [docs/EXAMPLES.md](docs/EXAMPLES.md).

## Example

```go
query, err := qb.New().
    Pick("id", "status").
    Where(qb.And(
        qb.Field("status").Eq("active"),
        qb.Or(
            qb.Field("role").Eq("admin"),
            qb.Field("role").Eq("owner"),
        ),
    )).
    SortBy("created_at", qb.Desc).
    Page(2).
    Size(20).
    Query()
if err != nil {
    panic(err)
}

statement, err := sqladapter.New().Compile(query)
if err != nil {
    panic(err)
}
```

## Schema-driven usage

```go
userSchema := schema.MustNew(
    schema.Define("status", schema.Storage("users.status"), schema.Aliases("state")),
    schema.Define(
        "age",
        schema.Storage("users.age"),
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

query, err := mapinput.Parse(
    payload,
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

For GORM:

```go
tx := db.Model(&User{}).Scopes(
    gormadapter.New(
        gormadapter.WithQueryTransformer(userSchema.ToStorage),
    ).Scope(query),
)
```

If you only want canonical API-facing names and decoded values, use `userSchema.Normalize`.
If you want adapter-facing storage names as well, use `userSchema.ToStorage`.

For structured input:

```json
{
  "pick": ["id", "status"],
  "where": {
    "status": "active",
    "age": { "$gte": 18 },
    "$or": [
      { "role": "admin" },
      { "role": "owner" }
    ]
  },
  "group_by": ["id", "status"],
  "sort": ["-created_at", "name"],
  "page": 2,
  "size": 20
}
```

## Development

```bash
go test ./...
```

## Input spec

Structured input supports these top-level constructs:

- `select` or `pick`: comma-delimited string or array of projected fields
- `include`: comma-delimited string or array of eager-load/include paths
- `where` or `filter`: nested filter object
- `sort`: comma-delimited string or array such as `["-created_at", "name"]`
- `group_by`: comma-delimited string or array of grouping fields
- `page`: 1-based page number for offset pagination
- `size`: page size for both offset and cursor pagination
- `cursor`: opaque token string or object payload for cursor pagination
- `limit` and `offset`: accepted for backward compatibility, but `page` and `size` are preferred

Supported filter operators:

- `$eq`, `$ne`
- `$gt`, `$gte`, `$lt`, `$lte`
- `$in`, `$nin`
- `$like`, `$contains`, `$prefix`, `$suffix`
- `$isnull`, `$notnull`
- `$and`, `$or`, `$not`

Cursor notes:

- `cursor` is metadata in the core query model.
- built-in adapters do not interpret it directly
- use a `qb.QueryTransformer` to rewrite cursor metadata into filters and sorts before `adapter/sql` or `adapter/gorm`
- `cursor` requires `size`

## Design notes

- The core model intentionally omits joins, table metadata, and ORM concepts.
- `schema` separates public query field names from storage-facing identifiers.
- `Normalize` validates aliases, operator allowlists, and value decoding for both parsed and builder-created queries.
- `ToStorage` projects canonical fields into storage identifiers before adapter compilation.
- `qb.RewriteQuery` is the low-level AST transform primitive used by schema and can be reused for tenant filters or soft-delete policies.
- `parser/querystring` normalizes bracket-notation input and delegates to `parser/mapinput`.
- `select`, `include`, and `group_by` are first-class query metadata, but relation joins themselves still stay outside the core.
- `page` and `size` are the preferred pagination inputs; `size` is also used for cursor pagination.
- Adapters depend only on `qb.Query`, so a GORM adapter can be added without changing the core.
- `adapter/gorm` uses the same query-transform pattern as `adapter/sql`, so schema rules can be reused.
- Parser and adapter interfaces are not forced into the core; the query model itself is the extension point.
- `schema` is optional and lives outside the core so policy rules stay composable instead of hard-coded.
