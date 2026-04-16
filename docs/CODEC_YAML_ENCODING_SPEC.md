# YAML Codec Encoding Spec

This document defines the planned YAML codec for `qb`.

Status: proposed, not implemented.

This document extends and constrains
[CODEC_MODEL_SPEC.md](./CODEC_MODEL_SPEC.md). The model spec is the semantic
source of truth.

## Scope

The planned YAML codec:

- encodes `qb.Query` into the canonical or compact YAML document
- decodes YAML into the canonical YAML document
- lifts the canonical YAML document into the shared semantic codec model
- lowers the semantic codec model into `qb.Query`

Planned package:

- `codec/yaml`

YAML is not a separate query language. It is a transport-specific projection of
the same shared semantic codec model used by JSON and query string.

## Relationship To The Model Spec

YAML serialization follows the shared semantic codec model and projects it into
the YAML transport’s canonical document.

## Canonical YAML Document

Canonical YAML uses these top-level keys, in this order when present:

1. `$select`
2. `$include`
3. `$where`
4. `$group`
5. `$sort`
6. `$page`
7. `$size`
8. `$cursor`

Canonical example:

```yaml
$select:
  - users.id
  - "lower(users.name) as normalized_name"
$include:
  - Company
  - Orders
$where:
  age:
    $gte: 18
  status: active
  $expr:
    $eq:
      - "lower(@users.name)"
      - "lower('john')"
$group:
  - users.status
$sort:
  - "lower(users.name) asc"
$page: 2
$size: 20
```

## Scalar Encoding

YAML can encode semantic scalar nodes in two ways:

- DSL strings
- structured scalar objects

### DSL Strings

DSL strings are the default form when no metadata would be lost.

### Structured Scalar Objects

Canonical YAML must use structured scalar objects when a scalar contains typed
literal metadata or other metadata that cannot be losslessly represented as a
plain DSL string.

Canonical scalar object shapes:

- field ref:
  - `$field: users.name`
- function call:
  - `$call: lower`
  - `$args: [...]`
- cast:
  - `$cast: date`
  - `$expr: <scalar>`
- literal:
  - `$literal: john`
- typed literal:
  - `$literal: "2026-04-15T00:00:00Z"`
  - `$codec: time`

Projection wrapper:

```yaml
$select:
  - $expr:
      $call: date_bin
      $args:
        - $literal: 15m0s
          $codec: duration
        - $field: events.created_at
        - $literal: "2026-04-15T00:00:00Z"
          $codec: time
    $as: bucket
```

Sort wrapper:

```yaml
$sort:
  - $expr:
      $call: lower
      $args:
        - $field: users.name
    $dir: asc
```

## Compact YAML Rules

Compact YAML may apply only these shorthands:

- `$select` may collapse to a comma-delimited string when every item is a simple field reference
- `$group` may collapse to a comma-delimited string when every item is a simple field reference
- `$sort` may collapse to a comma-delimited string when every item is a simple sortable field reference

Compact mode may also use inline typed-literal interpolation tokens inside DSL
strings when the literal body is inline-safe:

- `!#:time:2026-04-15T00:00:00Z`
- `!#:duration:15m0s`
- `!#::lower('john')`

Example:

```yaml
$select:
  - "date_bin(!#:duration:15m0s, events.created_at, !#:time:2026-04-15T00:00:00Z) as bucket"
```

If a typed literal is not inline-safe, compact mode must still use the
structured scalar form.

## YAML-Specific Quoting Rules

To keep YAML output predictable, codecs should quote DSL-bearing strings.

Recommended quoting rules:

- quote all expression-bearing `$select` items
- quote all expression-bearing `$group` items
- quote all `$sort` items that contain spaces or explicit directions
- quote all strings containing `@`, `(`, `)`, `::`, commas, or single quotes
- quote all typed-literal interpolation tokens
- quote ambiguous strings such as `true`, `false`, `null`, timestamps, and numeric-looking IDs when they are meant to stay strings

## Literal Handling

Forced-literal example:

```yaml
$where:
  $expr:
    $eq:
      - "@users.note"
      - $literal: "@admin"
```

Typed literal example:

```yaml
$where:
  created_at:
    $gte:
      $literal: "2026-04-14T15:04:05Z"
      $codec: time
```

Structured cursor example:

```yaml
$cursor:
  createdAt:
    $literal: "2026-04-11T12:00:00Z"
    $codec: time
  id: 981
$size: 25
```

## Filter Encoding

YAML filter encoding follows the same conservative lowering rules as JSON.

Examples:

```yaml
$where:
  status: active
```

```yaml
$where:
  age:
    $gte: 18
```

```yaml
$where:
  "lower(users.name)":
    $eq: john
```

```yaml
$where:
  $expr:
    $eq:
      - "lower(@users.name)"
      - "lower('JOHN')"
```

## Null Rules

YAML can preserve null semantics directly.

Examples:

```yaml
$where:
  deleted_at: null
```

```yaml
$where:
  deleted_at:
    $ne: null
```

```yaml
$where:
  deleted_at:
    $isnull: true
```

## Deterministic YAML Emission

Deterministic YAML emission is required.

Requirements:

- top-level key order must match this spec
- `$where` member order must match the model spec
- arrays preserve semantic order
- emitters must not rely on unordered Go map iteration

## YAML Safety Rules

The YAML codec should emit and accept only a safe YAML subset:

- mappings
- sequences
- scalars

Avoid or reject:

- anchors
- aliases
- merge keys
- custom tags
- transport-specific YAML extensions
