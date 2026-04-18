# Compact YAML DSL

This document defines the canonical YAML representation for the compact `qb` DSL.
It is semantically identical to [JSON_DSL_SPEC.md](./JSON_DSL_SPEC.md).

YAML is just another serialization of the same query model, not a different
query language.

Status: implemented and canonical.

## Relationship To The JSON Spec

This YAML format keeps the same semantics as the JSON spec:

- same top-level keys
- same list encoding rules
- same scalar DSL
- same JSON-first mixed-expression behavior
- same predicate operators
- same pagination and cursor semantics

If a YAML example and a JSON example appear to disagree, the JSON spec is the
semantic source of truth and the YAML example should be corrected.

## Goals

- provide a human-friendly configuration format for the same query model
- preserve transport-agnostic semantics across JSON, YAML, and query strings
- avoid YAML-only features that would create a separate language surface

## YAML-Specific Rules

Use a strict YAML subset:

- mappings
- sequences
- scalars

Do not use:

- anchors
- aliases
- merge keys
- custom tags
- multiline folded or literal block scalars for DSL expressions

### Quoting Rules

YAML is more permissive than JSON, so quoting needs to be tighter.

Quote all scalar DSL expressions:

- `"lower(user.name) as normalized_name"`
- `"round(user.age::decimal, 2) as rounded_age"`
- `"user.joinedAt::date"`
- `"lower(user.name) asc"`
- `"@user.name"`

Quote all comma-delimited shorthand list strings:

- `"$select: \"user.id,user.name\""`
- `"$group: \"user.id,user.status\""`
- `"$sort: \"-created_at,name\""`

Quote ambiguous scalar strings that YAML might otherwise reinterpret:

- dates or timestamps like `"2026-04-11"` or `"2026-04-11T12:00:00Z"`
- strings that look like booleans or nulls such as `"true"` or `"null"`
- strings containing `#` or leading `@`

Prefer double quotes for DSL strings so embedded single-quoted literals stay
readable:

```yaml
$where:
  $expr:
    $eq:
      - "@user.name"
      - "lower('john')"
```

## Canonical Top-Level Keys

The canonical query envelope uses `$`-prefixed keys:

- `$select`
- `$include`
- `$where`
- `$group`
- `$sort`
- `$page`
- `$size`
- `$cursor`

Aliases like `$pick` or `$filter` are not part of the implemented spec.

## List Encoding Rules

String shorthand is allowed only for simple field lists and simple `-field`
sorts.

Use arrays as the canonical form for anything involving:

- functions
- casts
- aliases
- explicit direction suffixes like `asc` or `desc`

Allowed shorthand:

```yaml
$select: "user.id,user.name"
$group: "user.id,user.status"
$sort: "-created_at,name"
```

Canonical expression-bearing form:

```yaml
$select:
  - "lower(user.name) as normalized_name"
  - "round(user.age::decimal, 2) as rounded_age"

$group:
  - "user.joinedAt::date"
  - "lower(user.name)"

$sort:
  - "user.joinedAt::date desc"
  - "lower(user.name) asc"
```

This means the parser should reject comma-delimited single-string lists that
contain expression-bearing items.

## Canonical Query Example

```yaml
$select:
  - user.id
  - "lower(user.name) as normalized_name"
  - "round(user.age::decimal, 2) as rounded_age"
  - "user.joinedAt::date as joined_on"

$include:
  - company
  - roles.permissions

$where:
  status: active
  age:
    $gte: 18
  $or:
    - role: admin
    - role: owner

$group:
  - "user.joinedAt::date"
  - "lower(user.name)"

$sort:
  - "user.joinedAt::date desc"
  - "lower(user.name) asc"

$page: 2
$size: 20
```

## Scalar DSL

The compact format uses a small DSL for scalar expressions. The DSL is used in:

- `$select`
- `$group`
- `$sort`
- advanced `$where.$expr` comparisons
- computed-field keys inside `$where`, for example `lower(user.name)`

The DSL is not used for full boolean logic. Boolean structure stays in YAML via
`$and`, `$or`, and `$not`.

### Standalone Versus Mixed Expression Contexts

The DSL appears in two kinds of places:

- standalone expression contexts, where the whole string is a DSL expression
- mixed expression contexts, where native YAML literals and DSL expressions can
  appear together

Standalone expression contexts:

- each item in `$select`
- each item in `$group`
- each item in expression-bearing `$sort` arrays
- computed expression keys inside `$where`, for example `lower(user.name)`

In standalone expression contexts, plain field references like `user.name` are
allowed.

Mixed expression contexts:

- operand arrays inside `$where.$expr`

In mixed expression contexts, the format is YAML-first in the same way the JSON
spec is JSON-first:

- non-string YAML values stay native literals
- bare YAML strings like `john` are string literals
- strings with DSL markers such as `@`, `(`, or `::` are parsed as DSL expressions
- field references must be explicit with `@`, for example `@user.name`
- functions or casts that reference fields must use explicit refs inside the
  DSL, for example `lower(@user.name)` or `round(@user.age::decimal, 2)`

### Supported Expression Forms

- standalone field reference: `user.name`
- explicit field reference: `@user.name`
- function call: `lower(user.name)`
- nested function call: `round(avg(user.score), 2)`
- cast: `user.age::double`
- cast after a function call: `round(user.age::decimal, 2)::decimal`
- alias: `lower(user.name) as normalized_name`
- string literal: `'john'`
- numeric literal: `2`, `3.14`
- boolean literal: `true`, `false`
- null literal: `null`
- parenthesized expression: `(user.age::double)`

In mixed expression contexts, plain YAML strings are preferred for string
literals. SQL-style quoted string literals are primarily needed inside DSL
strings like `lower('john')`.

### Grammar

```txt
expr         := alias_expr
alias_expr   := cast_expr [AS ident]
cast_expr    := primary {"::" type_name}
primary      := ref | literal | call | "(" expr_no_alias ")"
call         := ident "(" [arg {"," arg}] ")"
arg          := expr_no_alias
expr_no_alias:= cast_expr
ref          := ["@"] ident {"." ident}
literal      := quoted_string | number | true | false | null
```

`as` is case-insensitive.

`@` is required for field references in mixed expression contexts like
`$where.$expr`.

### Alias Rules

- aliases are allowed only in `$select`
- aliases are not allowed in `$group`
- aliases are not allowed in `$sort`
- aliases are not allowed inside `$where`

Examples:

```yaml
$select:
  - "lower(user.name) as normalized_name"
  - "user.joinedAt::date as joined_on"
```

Invalid:

```yaml
$group:
  - "lower(user.name) as normalized_name"
```

## Cast Types

Casts use canonical logical types, not backend-specific SQL type names.

Recommended baseline:

- `string`
- `int`
- `bigint`
- `double`
- `decimal`
- `bool`
- `date`
- `timestamp`
- `json`

Adapters map these names to dialect-specific SQL.

Examples:

```yaml
$select:
  - "user.age::double"
  - "user.joinedAt::date as joined_on"
```

## `$select`

`$select` accepts:

- a comma-delimited string of simple field references
- an array of scalar DSL expressions

Simple:

```yaml
$select: "user.id,user.name"
```

Canonical:

```yaml
$select:
  - user.id
  - "lower(user.name) as normalized_name"
  - "round(user.age::decimal, 2) as rounded_age"
```

Rejected:

```yaml
$select: "lower(user.name) as normalized_name, user.age"
```

Reason: expression-bearing selections must use arrays.

## `$include`

`$include` accepts a string or an array of relation paths.

```yaml
$include:
  - company
  - orders.items
```

## `$group`

`$group` accepts:

- a comma-delimited string of simple field references
- an array of scalar DSL expressions

```yaml
$group:
  - "user.joinedAt::date"
  - "lower(user.name)"
```

## `$sort`

`$sort` accepts:

- a comma-delimited string of simple field sorts such as `created_at` or `-created_at`
- an array for expression-bearing sorts or explicit direction suffixes

Supported sort item forms:

- simple string shorthand items: `created_at`, `-created_at`
- array-only expression items: `lower(user.name) asc`, `user.joinedAt::date desc`

Simple:

```yaml
$sort: "-created_at,name"
```

Canonical:

```yaml
$sort:
  - "user.joinedAt::date desc"
  - "lower(user.name) asc"
```

Recommended rule: sort by expressions directly, not by projection aliases.

Rejected:

```yaml
$sort: "lower(user.name) asc,user.joinedAt::date desc"
```

Reason: expression-bearing sorts must use arrays.

## `$where`

`$where` keeps boolean structure in YAML.

### Shorthand Field Filters

Field-key shorthand remains the primary filter format:

```yaml
$where:
  status: active
  age:
    $gte: 18
  role:
    - admin
    - owner
```

This means:

- scalar value => equality
- array value => `$in`
- operator object => explicit operator predicates

### Computed-Field Filters

Computed expressions may be used as field keys:

```yaml
$where:
  lower(user.name):
    $eq: john
```

The left-hand key is parsed as a scalar DSL expression. The right-hand side
keeps operator mapping semantics.

### Advanced Expression Comparisons

When both sides need to be expressions, use `$expr`.

```yaml
$where:
  $expr:
    $eq:
      - "@user.name"
      - "lower('john')"
    $gte:
      - "round(@user.age::decimal, 2)"
      - 18
```

Inside `$where.$expr`:

- bare YAML strings like `john` are string literals
- strings with DSL markers such as `@`, `(`, or `::` are parsed as DSL expressions
- explicit field references must use `@`, for example `"@user.name"`
- function or cast expressions that reference fields must use explicit refs, for
  example `"lower(@user.name)"` or `"round(@user.age::decimal, 2)"`
- DSL-quoted string literals are only needed inside DSL strings, for example
  `"lower('john')"`
- non-string YAML values like `18`, `true`, and `null` remain native literals

### Logical Operators

- `$and`
- `$or`
- `$not`

Example:

```yaml
$where:
  $or:
    - status: active
    - $expr:
        $eq:
          - "lower(@user.name)"
          - john
```

### Supported Predicate Operators

- `$eq`, `$ne`
- `$gt`, `$gte`, `$lt`, `$lte`
- `$in`, `$nin`
- `$like`, `$ilike`
- `$regexp`
- `$contains`, `$prefix`, `$suffix`
- `$isnull`, `$notnull`

## `$page` and `$size`

Offset pagination remains page-based:

```yaml
$page: 3
$size: 25
```

Rules:

- `$page` is 1-based
- `$size` is required when `$page` is present

## `$cursor`

Cursor pagination stays metadata-driven.

Opaque token:

```yaml
$cursor: "eyJjcmVhdGVkX2F0IjoiMjAyNi0wNC0xMVQxMjowMDowMFoiLCJpZCI6OTgxfQ=="
$size: 25
```

Structured cursor:

```yaml
$cursor:
  created_at: "2026-04-11T12:00:00Z"
  id: 981
$size: 25
```

Built-in adapters should still treat `$cursor` as metadata and rely on a query
transformer to rewrite it into filters and sorts.

When structured cursor payloads are passed through `schema.Normalize(...)` or
`schema.ToStorage(...)`, cursor keys follow the same alias normalization and
storage projection rules as the rest of the query.

## Disambiguation Rules

These rules keep the format parseable.

### Rule 1: Boolean logic stays in YAML

Allowed:

```yaml
$where:
  $or:
    - status: active
    - status: trial
```

Not allowed:

```yaml
$where: "status = 'active' or status = 'trial'"
```

### Rule 2: Aliases are select-only

Allowed:

```yaml
$select:
  - "lower(user.name) as normalized_name"
```

Not allowed:

```yaml
$group:
  - "lower(user.name) as normalized_name"
```

### Rule 3: `$where.$expr` strings are JSON-first by intent

Allowed:

```yaml
$where:
  $expr:
    $eq:
      - "lower(@user.name)"
      - john
```

Not allowed:

```yaml
$where:
  $expr:
    $eq:
      - "lower(user.name)"
      - john
```

Reason: mixed expression contexts require explicit `@` markers for field
references so bare YAML strings can remain string literals.

### Rule 4: Use canonical cast names

Allowed:

```yaml
$select:
  - "user.age::double"
```

Not allowed:

```yaml
$select:
  - "user.age::float8"
```

Reason: backend-specific type names should stay in adapters, not the public API.

## Rejected Forms

These forms should not be part of the compact spec.

### Full SQL snippets

Reject:

```yaml
$select:
  - "LOWER(user.name) AS normalized_name, user.age"
```

Reason: this collapses the format into raw SQL, weakens portability, and
violates the array-only rule for expression-bearing selection lists.

### Function objects mixed with boolean objects

Reject:

```yaml
$where:
  lower(user.name):
    $eq: john
  $or: "status = 'active'"
```

Reason: boolean structure should remain YAML-native.

### Alias-based sorting

Reject:

```yaml
$select:
  - "lower(user.name) as normalized_name"
$sort:
  - "normalized_name asc"
```

Reason: sorting by aliases is not portable enough to be part of the canonical API.

Use:

```yaml
$select:
  - "lower(user.name) as normalized_name"
$sort:
  - "lower(user.name) asc"
```

## Query-String Follow-On

This document is YAML-first only as a serialization concern. Query-string
transport should still map to the same semantics defined in
[JSON_DSL_SPEC.md](./JSON_DSL_SPEC.md), for example:

```txt
$select[0]=lower(user.name) as normalized_name
$select[1]=user.joinedAt::date as joined_on
$where[status]=active
$where[age][$gte]=18
$sort[0]=lower(user.name) asc
$page=2
$size=20
```

The query-string shape should be defined as a transport mapping of the same
semantic model, not as a separate language.
