# qb

`qb` is a database-agnostic query builder core for Go.

The package is built around one stable boundary: a semantic `qb.Query` AST.
Parsers compile external inputs into `qb.Query`, and adapters compile `qb.Query`
into storage-specific outputs. The core does not depend on parsers, adapters,
ORMs, or HTTP frameworks.

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

## Example

```go
query, err := qb.New().
    Where(qb.And(
        qb.Field("status").Eq("active"),
        qb.Or(
            qb.Field("role").Eq("admin"),
            qb.Field("role").Eq("owner"),
        ),
    )).
    SortBy("created_at", qb.Desc).
    Limit(20).
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
    schema.Define("status", schema.Aliases("state")),
    schema.Define(
        "age",
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
    schema.Define("created_at", schema.Aliases("createdAt"), schema.Sortable()),
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
    sqladapter.WithQueryTransformer(userSchema.Normalize),
).Compile(query)
if err != nil {
    panic(err)
}
```

For GORM:

```go
tx := db.Model(&User{}).Scopes(
    gormadapter.New(
        gormadapter.WithQueryTransformer(userSchema.Normalize),
    ).Scope(query),
)
```

For structured input:

```json
{
  "where": {
    "status": "active",
    "age": { "$gte": 18 },
    "$or": [
      { "role": "admin" },
      { "role": "owner" }
    ]
  },
  "sort": ["-created_at", "name"],
  "limit": 20,
  "offset": 40
}
```

## Development

```bash
go test ./...
```

## Input spec

Structured input uses four top-level constructs:

- `where` or `filter`: nested filter object
- `sort`: comma-delimited string or array such as `["-created_at", "name"]`
- `limit`: non-negative integer
- `offset`: non-negative integer

Supported filter operators:

- `$eq`, `$ne`
- `$gt`, `$gte`, `$lt`, `$lte`
- `$in`, `$nin`
- `$like`, `$contains`, `$prefix`, `$suffix`
- `$isnull`, `$notnull`
- `$and`, `$or`, `$not`

## Design notes

- The core model intentionally omits joins, table metadata, and ORM concepts.
- `parser/querystring` normalizes bracket-notation input and delegates to `parser/mapinput`.
- Adapters depend only on `qb.Query`, so a GORM adapter can be added without changing the core.
- `adapter/gorm` uses the same query-transform pattern as `adapter/sql`, so schema rules can be reused.
- Parser and adapter interfaces are not forced into the core; the query model itself is the extension point.
- `schema` is optional and lives outside the core so policy rules stay composable instead of hard-coded.
