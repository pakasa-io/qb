# qb

`qb` is a database-agnostic query builder core for Go.

The package centers on a semantic `qb.Query` AST. Codecs turn external
payloads into `qb.Query`, and adapters turn `qb.Query` into backend-specific
output. The core stays independent from HTTP, JSON, YAML, SQL drivers, and
ORMs.

## Install

```bash
go get github.com/pakasa-io/qb@latest
```

`qb` targets Go `1.24+`.

## Package Layout

- `qb`: query AST, fluent builder, scalar expressions, projections, rewrites
- `codecs`: generic document parsing facade plus the stable codec options and literal codec controls
- `codecs/json`: JSON parse and ordered JSON emission
- `codecs/yaml`: YAML parse and ordered YAML emission
- `codecs/qs`: bracket-notation query-string parse and canonical encoding
- `schema`: optional aliasing, field policy, value decoding, and storage mapping
- `adapters/sql`: compile to parameterized SQL with PostgreSQL, MySQL, and SQLite dialects
- `adapters/gorm`: apply the same query AST to GORM chains

See:

- [docs/guides/README.md](docs/guides/README.md) for the staged guide path from basic to complex usage
- [examples/README.md](examples/README.md) for runnable examples
- [docs/guides/EXAMPLES.md](docs/guides/EXAMPLES.md) for a narrative guide
- [docs/specs/JSON_DSL_SPEC.md](docs/specs/JSON_DSL_SPEC.md) for the canonical JSON spec
- [docs/specs/YAML_DSL_SPEC.md](docs/specs/YAML_DSL_SPEC.md) for the semantically identical YAML spec
- [docs/specs/QUERYSTRING_DSL_SPEC.md](docs/specs/QUERYSTRING_DSL_SPEC.md) for the semantically identical query-string spec
- [docs/specs/CODEC_MODEL_SPEC.md](docs/specs/CODEC_MODEL_SPEC.md) for the planned shared codec model
- [docs/specs/CODEC_JSON_ENCODING_SPEC.md](docs/specs/CODEC_JSON_ENCODING_SPEC.md) for the planned JSON output codec
- [docs/specs/CODEC_YAML_ENCODING_SPEC.md](docs/specs/CODEC_YAML_ENCODING_SPEC.md) for the planned YAML output codec
- [docs/specs/CODEC_QUERYSTRING_ENCODING_SPEC.md](docs/specs/CODEC_QUERYSTRING_ENCODING_SPEC.md) for the planned query-string output codec
- [CONTRIBUTING.md](CONTRIBUTING.md) for contribution expectations
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for community standards
- [SECURITY.md](SECURITY.md) for vulnerability reporting guidance
- [RELEASING.md](RELEASING.md) for the public Go module release checklist

`adapters/sql` defaults to PostgreSQL v17+ syntax. You can change the process-wide
default with `sqladapter.SetDefaultDialect(...)` or override it per compiler
with `sqladapter.WithDialect(...)`.

## Core Example

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
	SortByExpr(qb.F("users.name").Lower(), qb.Asc).
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

## Compact JSON / Map Input

```go
payload := map[string]any{
	"$select": []any{
		"users.id",
		"lower(users.name) as normalized_name",
		"round(users.age::decimal, 2) as rounded_age",
	},
	"$where": map[string]any{
		"status": "active",
		"age":    map[string]any{"$gte": 18},
		"$expr": map[string]any{
			"$eq": []any{"lower(@users.name)", "lower('john')"},
		},
	},
	"$group": []any{"lower(users.name)"},
	"$sort":  []any{"lower(users.name) asc"},
	"$page":  2,
	"$size":  20,
}

query, err := codecs.Parse(payload)
```

Canonical top-level keys are:

- `$select`
- `$include`
- `$where`
- `$group`
- `$sort`
- `$page`
- `$size`
- `$cursor`

String shorthand is only for simple field lists and simple `-field` sorts. Once
functions, casts, aliases, or explicit directions appear, arrays are the
canonical form.

## Query-String Input

```go
values := url.Values{
	"$select[0]":            {"users.id"},
	"$select[1]":            {"lower(users.name) as normalized_name"},
	"$where[status]":        {"active"},
	"$where[age][$gte]":     {"18"},
	"$where[$expr][$eq][0]": {"lower(@users.name)"},
	"$where[$expr][$eq][1]": {"lower('john')"},
	"$sort[0]":              {"lower(users.name) asc"},
	"$page":                 {"2"},
	"$size":                 {"20"},
}

query, err := querystring.Parse(values)
```

## YAML Input

```go
payload := []byte(`
$select:
  - users.id
  - "lower(users.name) as normalized_name"
$where:
  status: active
  age:
    $gte: 18
$sort:
  - "lower(users.name) asc"
$page: 2
$size: 20
`)

query, err := yamlcodec.Parse(payload)
```

## Schema-Driven Usage

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

Use `userSchema.Normalize` when you only want canonical API-facing field names
and decoded values. Use `userSchema.ToStorage` when adapters should see
storage-facing names like `users.created_at`. Structured cursor payloads are
normalized and storage-projected through the same schema layer.

## Function Expressions

All scalar contexts share one expression model: projections, predicates,
grouping, sorting, and function arguments.

```go
query, err := qb.New().
	SelectProjection(
		qb.F("users.name").Lower().As("normalized_name"),
		qb.Round(qb.F("users.age").Cast("decimal"), 2).As("rounded_age"),
		qb.RoundDouble(qb.F("users.score").Cast("double"), 2).As("rounded_score"),
	).
	Where(qb.And(
		qb.F("users.name").Lower().Eq("john"),
		qb.Func("substring", qb.F("users.name"), 1, 4).Eq("john"),
	)).
	GroupByExpr(qb.F("users.name").Lower()).
	SortByExpr(qb.F("users.name").Lower(), qb.Asc).
	Query()
```

`qb` includes helpers for common string, numeric, aggregate, date/time, JSON,
and PostgreSQL-first functions. Use `Round(..., scale)` with `decimal`/`numeric`
inputs for portable scaled rounding. Use `RoundDouble(..., scale)` when you
explicitly want a PostgreSQL-safe double-precision rounding helper. Unsupported
helpers fail with structured `unsupported_function` errors for dialects that
cannot render them cleanly.

## Development

```bash
go test ./...
```

## License

Released under the [MIT License](LICENSE).
