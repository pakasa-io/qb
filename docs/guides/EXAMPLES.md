# Examples

This guide shows the current canonical `qb` model after the transport refactor:

For the staged guide set, start with [README.md](./README.md). For runnable
programs, see [examples/README.md](../../examples/README.md).

1. build a `qb.Query` directly with the fluent builder, or
2. parse the same semantic model from JSON, YAML, or query strings, then
3. normalize/rewrite it, and
4. hand it to `adapters/sql` or `adapters/gorm`.

## 1. Fluent Builder

```go
query, err := qb.New().
	SelectProjection(
		qb.F("users.id").As("id"),
		qb.F("users.name").Lower().As("normalized_name"),
	).
	Where(qb.And(
		qb.F("users.status").Eq("active"),
		qb.F("users.age").Cast("double").Gte(18),
	)).
	GroupByExpr(qb.F("users.name").Lower()).
	SortByExpr(qb.F("users.name").Lower(), qb.Asc).
	Page(2).
	Size(20).
	Query()
if err != nil {
	panic(err)
}
```

## 2. JSON / Map Input

`codecs/internal/docmodel` accepts normalized `$...` documents, while `codecs/json` parses
the same shape from JSON bytes.

```json
{
  "$select": [
    "users.id",
    "lower(users.name) as normalized_name",
    "round(users.age::decimal, 2) as rounded_age"
  ],
  "$include": ["company", "roles.permissions"],
  "$where": {
    "status": "active",
    "age": { "$gte": 18 },
    "$or": [
      { "role": "admin" },
      { "role": "owner" }
    ],
    "$expr": {
      "$eq": ["lower(@users.name)", "lower('john')"]
    }
  },
  "$group": [
    "lower(users.name)"
  ],
  "$sort": [
    "lower(users.name) asc"
  ],
  "$page": 2,
  "$size": 20
}
```

```go
query, err := jsoncodec.Parse(payload)
if err != nil {
	panic(err)
}
```

Rules:

- `$select`, `$group`, and `$sort` accept simple comma-delimited string
  shorthand only for plain field references.
- Once functions, casts, aliases, or explicit sort directions appear, arrays are
  the canonical form.
- `$where` stays JSON-first: plain values are literals, and `$expr` is the
  escape hatch for expression-vs-expression predicates.

## 3. YAML Input

`codecs/yaml` is semantically identical to the JSON/model layer. YAML is just
another serialization of the same model.

```yaml
$select:
  - users.id
  - "lower(users.name) as normalized_name"
  - "users.joinedAt::date as joined_on"
$where:
  status: active
  age:
    $gte: 18
  $expr:
    $eq:
      - "lower(@users.name)"
      - "lower('john')"
$group:
  - "users.joinedAt::date"
$sort:
  - "users.joinedAt::date desc"
$page: 2
$size: 20
```

Quote DSL-bearing scalars in YAML. That keeps parsing predictable and avoids
YAML implicit typing surprises.

## 4. Query-String Input

`codecs/qs` maps bracket-notation query params onto the same semantic
envelope.

```go
values := url.Values{
	"$select[0]":            {"users.id"},
	"$select[1]":            {"lower(users.name) as normalized_name"},
	"$where[status]":        {"active"},
	"$where[age][$gte]":     {"18"},
	"$where[$expr][$eq][0]": {"lower(@users.name)"},
	"$where[$expr][$eq][1]": {"lower('john')"},
	"$group[0]":             {"lower(users.name)"},
	"$sort[0]":              {"lower(users.name) asc"},
	"$page":                 {"2"},
	"$size":                 {"20"},
}

query, err := querystring.Parse(values)
if err != nil {
	panic(err)
}
```

Bracket notation is only a transport encoding. The resulting `qb.Query` is the
same as JSON or YAML would produce.

## 5. Nested Filters And Groups

Nested boolean logic is still JSON/YAML-native:

```go
query, err := codecs.Parse(map[string]any{
	"$where": map[string]any{
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
		"$not": map[string]any{
			"deleted_at": map[string]any{"$isnull": true},
		},
	},
})
```

Use `$expr` when either side is a computed expression:

```go
query, err := codecs.Parse(map[string]any{
	"$where": map[string]any{
		"$expr": map[string]any{
			"$gte": []any{"round(@users.age::decimal, 2)", 18},
			"$eq":  []any{"lower(@users.name)", "lower('john')"},
		},
	},
})
```

## 6. Schema Normalization And Storage Mapping

`schema` remains optional. Use it when you need:

- API-facing aliases like `createdAt`
- per-field operator allowlists
- value decoding/coercion
- storage-facing names like `users.created_at`

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
)

query, err := codecs.Parse(
	map[string]any{
		"$select": "state",
		"$where": map[string]any{
			"state":  "active",
			"minAge": map[string]any{"$gte": "21"},
		},
	},
	codecs.WithFilterFieldResolver(userSchema.ResolveFilterField),
	codecs.WithGroupFieldResolver(userSchema.ResolveGroupField),
	codecs.WithSortFieldResolver(userSchema.ResolveSortField),
	codecs.WithValueDecoder(userSchema.DecodeValue),
)
if err != nil {
	panic(err)
}

normalized, err := userSchema.Normalize(query)
projected, err := userSchema.ToStorage(query)
```

Structured cursor values flow through the same schema layer, so aliases like
`createdAt` can be normalized and then projected to storage-facing names before
your cursor rewrite transformer runs.

## 7. Relations

Relation joins still stay outside the core. `qb` handles filter, projection,
grouping, and sort semantics; the calling layer owns the join strategy.

Typical pattern:

- API input uses fields like `company.name` or `orders.total`
- schema maps them to storage-facing paths
- the SQL or GORM layer adds the actual joins

This keeps the query model portable instead of baking ORM or SQL join semantics
into the core AST.

## 8. Offset And Cursor Pagination

Offset pagination:

```go
query, err := qb.New().
	SortBy("created_at", qb.Desc).
	Page(3).
	Size(25).
	Query()
```

Cursor pagination in the core is metadata plus a rewrite step:

```go
query, err := codecs.Parse(map[string]any{
	"$cursor": map[string]any{
		"created_at": "2026-04-11T12:00:00Z",
		"id":         981,
	},
	"$size": 25,
})
```

Transformers are expected to turn `$cursor` into concrete filters and sorts
before compilation.

## 9. Functions, Casts, And Aliases

Scalar expressions are first-class everywhere:

```go
query, err := qb.New().
	SelectProjection(
		qb.F("users.name").Lower().As("normalized_name"),
		qb.Round(qb.F("users.age").Cast("decimal"), 2).As("rounded_age"),
		qb.RoundDouble(qb.F("users.score").Cast("double"), 2).As("rounded_score"),
		qb.F("users.joined_at").Cast("date").As("joined_on"),
	).
	Where(qb.And(
		qb.Func("substring", qb.F("users.name"), 1, 4).Eq("john"),
		qb.F("users.name").Eq(qb.Lower("JOHN")),
	)).
	GroupByExpr(
		qb.F("users.name").Lower(),
		qb.F("users.joined_at").Cast("date"),
	).
	SortByExpr(qb.F("users.name").Lower(), qb.Asc).
	Query()
```

The parsers can express the same model with the DSL:

- `lower(users.name) as normalized_name`
- `round(users.age::decimal, 2)`
- `round_double(users.score::double, 2)`
- `users.joined_at::date as joined_on`

## 10. Error Handling

Parser, schema, and adapter failures return structured `qb.Error` values.

```go
_, err := codecs.Parse(map[string]any{
	"$size": "not-a-number",
})

var diagnostic *qb.Error
if errors.As(err, &diagnostic) {
	fmt.Println(diagnostic.Stage) // parse
	fmt.Println(diagnostic.Code)  // invalid_value
	fmt.Println(diagnostic.Path)  // $size
}
```

That gives you stable diagnostics for API responses and tests without depending
on string matching.
