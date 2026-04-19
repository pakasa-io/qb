# 02. Basic Codecs

Use codecs when the query comes from an external transport instead of native Go
builder code.

## Canonical Top-Level Keys

`qb` codecs accept the same semantic envelope across transports:

- `$select`
- `$include`
- `$where`
- `$group`
- `$sort`
- `$page`
- `$size`
- `$cursor`

## Parse A Normalized Map

```go
import "github.com/pakasa-io/qb/codecs"

query, err := codecs.Parse(map[string]any{
	"$select": []any{
		"users.id",
		"lower(users.name) as normalized_name",
	},
	"$where": map[string]any{
		"status": "active",
		"age":    map[string]any{"$gte": 18},
	},
	"$sort": []any{"created_at desc"},
	"$page": 1,
	"$size": 20,
})
```

## Parse JSON

```go
import jsoncodec "github.com/pakasa-io/qb/codecs/json"

payload := []byte(`{
  "$select": ["users.id", "lower(users.name) as normalized_name"],
  "$where": {
    "status": "active",
    "$expr": {
      "$eq": ["lower(@users.name)", "lower('john')"]
    }
  },
  "$sort": ["created_at desc"],
  "$page": 1,
  "$size": 20
}`)

query, err := jsoncodec.Parse(payload)
```

## Parse YAML

Quote DSL-bearing scalars in YAML so the parser sees them as strings:

```go
import yamlcodec "github.com/pakasa-io/qb/codecs/yaml"

payload := []byte(`
$select:
  - users.id
  - "lower(users.name) as normalized_name"
$where:
  status: active
  $expr:
    $eq:
      - "lower(@users.name)"
      - "lower('john')"
$sort:
  - "created_at desc"
$page: 1
$size: 20
`)

query, err := yamlcodec.Parse(payload)
```

## Parse Query Strings

Bracket notation maps directly to the same model:

```go
import (
	"net/url"

	querystring "github.com/pakasa-io/qb/codecs/qs"
)

values := url.Values{
	"$select[0]":            {"users.id"},
	"$select[1]":            {"lower(users.name) as normalized_name"},
	"$where[status]":        {"active"},
	"$where[$expr][$eq][0]": {"lower(@users.name)"},
	"$where[$expr][$eq][1]": {"lower('john')"},
	"$sort[0]":              {"created_at desc"},
	"$page":                 {"1"},
	"$size":                 {"20"},
}

query, err := querystring.Parse(values)
```

You can also parse a raw query string directly:

```go
query, err := querystring.ParseString(raw)
```

## Shorthand vs Canonical Arrays

- `$select`, `$group`, and `$sort` may use compact strings only for simple field references.
- Once aliases, functions, casts, or explicit directions appear, arrays are the canonical form.
- Use `$expr` when either side of a predicate is a computed expression.

## Matching Examples

- [examples/codecs/json-input](../../examples/codecs/json-input/main.go)
- [examples/codecs/yaml-input](../../examples/codecs/yaml-input/main.go)
- [examples/codecs/querystring](../../examples/codecs/querystring/main.go)
- [examples/codecs/querystring-advanced](../../examples/codecs/querystring-advanced/main.go)
- [examples/codecs/querystring-literals](../../examples/codecs/querystring-literals/main.go)
