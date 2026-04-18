# JSON Codec Encoding Spec

This document defines the planned JSON codec for `qb`.

Status: proposed, not implemented.

This document extends and constrains
[CODEC_MODEL_SPEC.md](./CODEC_MODEL_SPEC.md). The model spec is the semantic
source of truth.

## Scope

The planned JSON codec:

- encodes `qb.Query` into the canonical or compact JSON document
- decodes JSON into the canonical JSON document
- lifts the canonical JSON document into the shared semantic codec model
- lowers the semantic codec model into `qb.Query`

Planned package:

- `codecs/jsoncodec`

## Relationship To The Model Spec

JSON serialization follows the shared semantic codec model and projects it into
the JSON transport’s canonical document.

JSON is not required to share the exact same concrete document shape as other
transports, but it must preserve the same semantic codec model.

## Canonical JSON Document

Canonical JSON uses these top-level keys, in this order when present:

1. `$select`
2. `$include`
3. `$where`
4. `$group`
5. `$sort`
6. `$page`
7. `$size`
8. `$cursor`

Canonical example:

```json
{
  "$select": [
    "users.id",
    "lower(users.name) as normalized_name"
  ],
  "$include": [
    "Company",
    "Orders"
  ],
  "$where": {
    "age": { "$gte": 18 },
    "status": "active",
    "$expr": {
      "$eq": ["lower(@users.name)", "lower('john')"]
    }
  },
  "$group": [
    "users.status"
  ],
  "$sort": [
    "lower(users.name) asc"
  ],
  "$page": 2,
  "$size": 20
}
```

## Scalar Encoding

JSON can encode semantic scalar nodes in two ways:

- DSL strings
- structured scalar objects

### DSL Strings

DSL strings are the default form when no metadata would be lost.

Examples:

```json
{
  "$select": [
    "users.id",
    "users.created_at::date as joined_on"
  ]
}
```

```json
{
  "$sort": [
    "users.created_at desc",
    "lower(users.name) asc"
  ]
}
```

### Structured Scalar Objects

Canonical JSON must use structured scalar objects when a scalar contains typed
literal metadata or other metadata that cannot be losslessly represented as a
plain DSL string.

Canonical scalar object shapes:

- field ref:
  - `{ "$field": "users.name" }`
- function call:
  - `{ "$call": "lower", "$args": [<scalar>] }`
- cast:
  - `{ "$cast": "date", "$expr": <scalar> }`
- literal:
  - `{ "$literal": "john" }`
- typed literal:
  - `{ "$literal": "2026-04-15T00:00:00Z", "$codec": "time" }`

Projection wrapper:

```json
{
  "$select": [
    {
      "$expr": {
        "$call": "date_bin",
        "$args": [
          { "$literal": "15m0s", "$codec": "duration" },
          { "$field": "events.created_at" },
          { "$literal": "2026-04-15T00:00:00Z", "$codec": "time" }
        ]
      },
      "$as": "bucket"
    }
  ]
}
```

Sort wrapper:

```json
{
  "$sort": [
    {
      "$expr": {
        "$call": "lower",
        "$args": [{ "$field": "users.name" }]
      },
      "$dir": "asc"
    }
  ]
}
```

`$group` items and `$where.$expr` operands may use scalar objects directly.

## Compact JSON Rules

Compact JSON may apply only these shorthands:

- `$select` may collapse to a comma-delimited string when every item is a simple field reference
- `$group` may collapse to a comma-delimited string when every item is a simple field reference
- `$sort` may collapse to a comma-delimited string when every item is a simple sortable field reference

Compact mode may also use inline typed-literal interpolation tokens inside DSL
strings when the literal body is inline-safe:

- `!#:time:2026-04-15T00:00:00Z`
- `!#:duration:15m0s`
- `!#::lower('john')`

Example:

```json
{
  "$select": [
    "date_bin(!#:duration:15m0s, events.created_at, !#:time:2026-04-15T00:00:00Z) as bucket"
  ]
}
```

If a typed literal is not inline-safe, compact mode must still use the
structured scalar form.

## Filter Encoding

JSON filter encoding follows the conservative lowering rules from the model
spec.

Examples:

Simple equality:

```json
{
  "$where": {
    "status": "active"
  }
}
```

Explicit operator:

```json
{
  "$where": {
    "age": { "$gte": 18 }
  }
}
```

Computed left-hand expression:

```json
{
  "$where": {
    "lower(users.name)": {
      "$eq": "john"
    }
  }
}
```

Expression fallback:

```json
{
  "$where": {
    "$expr": {
      "$eq": ["lower(@users.name)", "lower('JOHN')"]
    }
  }
}
```

Typed literal inside an expression fallback:

```json
{
  "$where": {
    "$expr": {
      "$gte": [
        { "$field": "events.created_at" },
        { "$literal": "2026-04-15T00:00:00Z", "$codec": "time" }
      ]
    }
  }
}
```

## Literal Handling

### Direct Literals

JSON-native literals may be emitted directly:

- string
- number
- boolean
- `null`

### Forced-Literal Strings

Use `$literal` in mixed-expression contexts when a string would otherwise be
interpreted as DSL:

```json
{
  "$where": {
    "$expr": {
      "$eq": ["@users.note", { "$literal": "@admin" }]
    }
  }
}
```

### Typed Literals

Use `$literal` with `$codec` for semantic typed literals:

```json
{
  "$where": {
    "created_at": {
      "$gte": {
        "$literal": "2026-04-14T15:04:05Z",
        "$codec": "time"
      }
    }
  }
}
```

Cursor example:

```json
{
  "$cursor": {
    "createdAt": {
      "$literal": "2026-04-11T12:00:00Z",
      "$codec": "time"
    },
    "id": 981
  },
  "$size": 25
}
```

## Null Rules

JSON can preserve null semantics directly.

Examples:

```json
{
  "$where": {
    "deleted_at": null
  }
}
```

```json
{
  "$where": {
    "deleted_at": { "$ne": null }
  }
}
```

```json
{
  "$where": {
    "deleted_at": { "$isnull": true }
  }
}
```

## Deterministic JSON Emission

Deterministic JSON emission is required.

Requirements:

- top-level key order must match this spec
- `$where` member order must match the model spec
- arrays preserve semantic order
- emitters must not rely on unordered Go map iteration
