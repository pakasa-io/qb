# Compact JSON DSL

This document defines the canonical JSON and map input format for `qb`.
It is the format implemented by `parser/mapinput`.

Status: implemented and canonical.

## Goals

- reduce verbosity for projections, grouping, sorting, and computed filters
- keep boolean query structure in JSON rather than inventing a full SQL-like language
- support functions, casts, aliases, and nested expressions
- keep the format transport-agnostic so query-string and JSON forms can mirror each other

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

```json
{
  "$select": "user.id,user.name",
  "$group": "user.id,user.status",
  "$sort": "-created_at,name"
}
```

Canonical expression-bearing form:

```json
{
  "$select": [
    "lower(user.name) as normalized_name",
    "round(user.age::decimal, 2) as rounded_age"
  ],
  "$group": [
    "user.joinedAt::date",
    "lower(user.name)"
  ],
  "$sort": [
    "user.joinedAt::date desc",
    "lower(user.name) asc"
  ]
}
```

This means the parser should reject comma-delimited single-string lists that
contain expression-bearing items.

## Canonical Query Example

```json
{
  "$select": [
    "user.id",
    "lower(user.name) as normalized_name",
    "round(user.age::decimal, 2) as rounded_age",
    "user.joinedAt::date as joined_on"
  ],
  "$include": ["company", "roles.permissions"],
  "$where": {
    "status": "active",
    "age": { "$gte": 18 },
    "$or": [
      { "role": "admin" },
      { "role": "owner" }
    ]
  },
  "$group": [
    "user.joinedAt::date",
    "lower(user.name)"
  ],
  "$sort": [
    "user.joinedAt::date desc",
    "lower(user.name) asc"
  ],
  "$page": 2,
  "$size": 20
}
```

## Scalar DSL

The compact format uses a small DSL for scalar expressions. The DSL is used in:

- `$select`
- `$group`
- `$sort`
- advanced `$where.$expr` comparisons
- computed-field keys inside `$where`, for example `lower(user.name)`

The DSL is not used for full boolean logic. Boolean structure stays in JSON via
`$and`, `$or`, and `$not`.

### Standalone Versus Mixed Expression Contexts

The DSL appears in two kinds of places:

- standalone expression contexts, where the whole string is a DSL expression
- mixed expression contexts, where JSON literals and DSL expressions can appear
  together

Standalone expression contexts:

- each item in `$select`
- each item in `$group`
- each item in expression-bearing `$sort` arrays
- computed expression keys inside `$where`, for example `lower(user.name)`

In standalone expression contexts, plain field references like `user.name` are
allowed.

Mixed expression contexts:

- operand arrays inside `$where.$expr`

In mixed expression contexts, the format is JSON-first:

- non-string JSON values stay JSON literals
- bare JSON strings like `"john"` are string literals
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

In mixed expression contexts, plain JSON strings are preferred for string
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

```json
{
  "$select": [
    "lower(user.name) as normalized_name",
    "user.joinedAt::date as joined_on"
  ]
}
```

Invalid:

```json
{
  "$group": ["lower(user.name) as normalized_name"]
}
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

```json
{
  "$select": [
    "user.age::double",
    "user.joinedAt::date as joined_on"
  ]
}
```

## `$select`

`$select` accepts:

- a comma-delimited string of simple field references
- an array of scalar DSL expressions

Simple:

```json
{
  "$select": "user.id,user.name"
}
```

Canonical:

```json
{
  "$select": [
    "user.id",
    "lower(user.name) as normalized_name",
    "round(user.age::decimal, 2) as rounded_age"
  ]
}
```

Rejected:

```json
{
  "$select": "lower(user.name) as normalized_name, user.age"
}
```

Reason: expression-bearing selections must use arrays.

## `$include`

`$include` accepts a string or an array of relation paths.

```json
{
  "$include": ["company", "orders.items"]
}
```

## `$group`

`$group` accepts:

- a comma-delimited string of simple field references
- an array of scalar DSL expressions

```json
{
  "$group": [
    "user.joinedAt::date",
    "lower(user.name)"
  ]
}
```

## `$sort`

`$sort` accepts:

- a comma-delimited string of simple field sorts such as `created_at` or `-created_at`
- an array for expression-bearing sorts or explicit direction suffixes

Supported sort item forms:

- simple string shorthand items: `created_at`, `-created_at`
- array-only expression items: `lower(user.name) asc`, `user.joinedAt::date desc`

Simple:

```json
{
  "$sort": "-created_at,name"
}
```

Canonical:

```json
{
  "$sort": [
    "user.joinedAt::date desc",
    "lower(user.name) asc"
  ]
}
```

Recommended rule: sort by expressions directly, not by projection aliases.

Rejected:

```json
{
  "$sort": "lower(user.name) asc,user.joinedAt::date desc"
}
```

Reason: expression-bearing sorts must use arrays.

## `$where`

`$where` keeps boolean structure in JSON.

### Shorthand Field Filters

Field-key shorthand remains the primary filter format:

```json
{
  "$where": {
    "status": "active",
    "age": { "$gte": 18 },
    "role": ["admin", "owner"]
  }
}
```

This means:

- scalar value => equality
- array value => `$in`
- operator object => explicit operator predicates

### Computed-Field Filters

Computed expressions may be used as field keys:

```json
{
  "$where": {
    "lower(user.name)": { "$eq": "john" }
  }
}
```

The left-hand key is parsed as a scalar DSL expression. The right-hand side
keeps operator JSON semantics.

### Advanced Expression Comparisons

When both sides need to be expressions, use `$expr`.

```json
{
  "$where": {
    "$expr": {
      "$eq": ["@user.name", "lower('john')"],
      "$gte": ["round(@user.age::decimal, 2)", 18]
    }
  }
}
```

Inside `$where.$expr`:

- bare JSON strings like `"john"` are string literals
- strings with DSL markers such as `@`, `(`, or `::` are parsed as DSL expressions
- explicit field references must use `@`, for example `"@user.name"`
- function or cast expressions that reference fields must use explicit refs, for
  example `"lower(@user.name)"` or `"round(@user.age::decimal, 2)"`
- DSL-quoted string literals are only needed inside DSL strings, for example
  `"lower('john')"`
- non-string JSON values like `18`, `true`, and `null` remain literals

### Logical Operators

- `$and`
- `$or`
- `$not`

Example:

```json
{
  "$where": {
    "$or": [
      { "status": "active" },
      {
        "$expr": {
          "$eq": ["lower(@user.name)", "john"]
        }
      }
    ]
  }
}
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

```json
{
  "$page": 3,
  "$size": 25
}
```

Rules:

- `$page` is 1-based
- `$size` is required when `$page` is present

## `$cursor`

Cursor pagination stays metadata-driven.

Opaque token:

```json
{
  "$cursor": "eyJjcmVhdGVkX2F0IjoiMjAyNi0wNC0xMVQxMjowMDowMFoiLCJpZCI6OTgxfQ==",
  "$size": 25
}
```

Structured cursor:

```json
{
  "$cursor": {
    "created_at": "2026-04-11T12:00:00Z",
    "id": 981
  },
  "$size": 25
}
```

Built-in adapters should still treat `$cursor` as metadata and rely on a query
transformer to rewrite it into filters and sorts.

When structured cursor payloads are passed through `schema.Normalize(...)` or
`schema.ToStorage(...)`, cursor keys follow the same alias normalization and
storage projection rules as the rest of the query.

## Disambiguation Rules

These rules keep the format parseable.

### Rule 1: Boolean logic stays in JSON

Allowed:

```json
{
  "$where": {
    "$or": [
      { "status": "active" },
      { "status": "trial" }
    ]
  }
}
```

Not allowed:

```json
{
  "$where": "status = 'active' or status = 'trial'"
}
```

### Rule 2: Aliases are select-only

Allowed:

```json
{
  "$select": ["lower(user.name) as normalized_name"]
}
```

Not allowed:

```json
{
  "$group": ["lower(user.name) as normalized_name"]
}
```

### Rule 3: `$where.$expr` strings are DSL expressions

Allowed:

```json
{
  "$where": {
    "$expr": {
      "$eq": ["lower(@user.name)", "john"]
    }
  }
}
```

Not allowed:

```json
{
  "$where": {
    "$expr": {
      "$eq": ["lower(user.name)", "john"]
    }
  }
}
```

Reason: mixed expression contexts require explicit `@` markers for field
references so bare JSON strings can remain JSON string literals.

### Rule 4: Use canonical cast names

Allowed:

```json
{
  "$select": ["user.age::double"]
}
```

Not allowed:

```json
{
  "$select": ["user.age::float8"]
}
```

Reason: backend-specific type names should stay in adapters, not the public API.

## Rejected Forms

These forms should not be part of the compact spec.

### Full SQL snippets

Reject:

```json
{
  "$select": ["LOWER(user.name) AS normalized_name, user.age"]
}
```

Reason: this collapses the format into raw SQL, weakens portability, and
violates the array-only rule for expression-bearing selection lists.

### Function objects mixed with boolean objects

Reject:

```json
{
  "$where": {
    "lower(user.name)": { "$eq": "john" },
    "$or": "status = 'active'"
  }
}
```

Reason: boolean structure should remain JSON-native.

### Alias-based sorting

Reject:

```json
{
  "$select": ["lower(user.name) as normalized_name"],
  "$sort": ["normalized_name asc"]
}
```

Reason: sorting by aliases is not portable enough to be part of the canonical API.

Use:

```json
{
  "$select": ["lower(user.name) as normalized_name"],
  "$sort": ["lower(user.name) asc"]
}
```

## Query-String Follow-On

This document is JSON-first. A query-string representation should mirror the
same semantics through bracket notation, for example:

```txt
$select[0]=lower(user.name) as normalized_name
$select[1]=user.joinedAt::date as joined_on
$where[status]=active
$where[age][$gte]=18
$sort[0]=lower(user.name) asc
$page=2
$size=20
```

The query-string shape should be defined as a transport mapping of this spec,
not as a separate semantic model.
