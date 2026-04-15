# Compact Query-String DSL

This document defines the canonical query-string transport for the compact `qb`
DSL.

It is semantically identical to [JSON_DSL_SPEC.md](./JSON_DSL_SPEC.md) and
[YAML_DSL_SPEC.md](./YAML_DSL_SPEC.md). Query strings are just another
serialization of the same query model, not a separate language.

Status: implemented and canonical.

## Relationship To The JSON Spec

This query-string format keeps the same semantics as the JSON spec:

- same top-level keys
- same list encoding rules
- same scalar DSL
- same JSON-first mixed-expression behavior
- same predicate operators
- same pagination and cursor semantics

The only difference is transport encoding:

- objects become bracket-notation keys
- arrays become indexed bracket segments
- all incoming leaf values arrive as strings

If a query-string example and a JSON example appear to disagree, the JSON spec
is the semantic source of truth and the query-string example should be
corrected.

## Goals

- provide a transport-friendly representation of the same compact DSL
- preserve the same semantic model as JSON and YAML
- avoid query-string-only constructs that would create a separate language

## Transport Rules

### Bracket Notation

Objects are represented with bracket notation:

```txt
$where[age][$gte]=18
```

This maps to:

```json
{
  "$where": {
    "age": {
      "$gte": 18
    }
  }
}
```

### Canonical Array Encoding

Indexed arrays are canonical:

```txt
$select[0]=user.id
$select[1]=lower(user.name) as normalized_name
```

This maps to:

```json
{
  "$select": [
    "user.id",
    "lower(user.name) as normalized_name"
  ]
}
```

Repeated keys may be accepted as compatibility input, but they should not be
part of the canonical spec.

### URL Encoding

All examples in this document are shown in decoded form for readability.

Actual URLs must percent-encode reserved characters such as:

- spaces
- brackets
- quotes
- commas when appropriate for the surrounding transport

For example, this decoded query string:

```txt
$select[1]=lower(user.name) as normalized_name
```

would be transmitted more like:

```txt
%24select%5B1%5D=lower%28user.name%29%20as%20normalized_name
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

Use indexed arrays as the canonical form for anything involving:

- functions
- casts
- aliases
- explicit direction suffixes like `asc` or `desc`

Allowed shorthand:

```txt
$select=user.id,user.name
$group=user.id,user.status
$sort=-created_at,name
```

Canonical expression-bearing form:

```txt
$select[0]=lower(user.name) as normalized_name
$select[1]=round(user.age::decimal, 2) as rounded_age
$group[0]=user.joinedAt::date
$group[1]=lower(user.name)
$sort[0]=user.joinedAt::date desc
$sort[1]=lower(user.name) asc
```

This means the parser should reject comma-delimited single-string lists that
contain expression-bearing items.

## Canonical Query Example

```txt
$select[0]=user.id
$select[1]=lower(user.name) as normalized_name
$select[2]=round(user.age::decimal, 2) as rounded_age
$select[3]=user.joinedAt::date as joined_on
$include[0]=company
$include[1]=roles.permissions
$where[status]=active
$where[age][$gte]=18
$where[$or][0][role]=admin
$where[$or][1][role]=owner
$group[0]=user.joinedAt::date
$group[1]=lower(user.name)
$sort[0]=user.joinedAt::date desc
$sort[1]=lower(user.name) asc
$page=2
$size=20
```

## Scalar DSL

The compact format uses the same scalar DSL defined in
[JSON_DSL_SPEC.md](./JSON_DSL_SPEC.md).

The DSL is used in:

- `$select`
- `$group`
- `$sort`
- advanced `$where.$expr` comparisons
- computed-field keys inside `$where`, for example `lower(user.name)`

The DSL is not used for full boolean logic. Boolean structure stays in bracket
notation via `$and`, `$or`, and `$not`.

### Standalone Versus Mixed Expression Contexts

The DSL appears in two kinds of places:

- standalone expression contexts, where the whole value is a DSL expression
- mixed expression contexts, where transport strings represent either literals
  or DSL expressions depending on content

Standalone expression contexts:

- each item in `$select`
- each item in `$group`
- each item in expression-bearing `$sort` arrays
- computed expression keys inside `$where`, for example `lower(user.name)`

In standalone expression contexts, plain field references like `user.name` are
allowed.

Mixed expression contexts:

- operand arrays inside `$where[$expr]`

In mixed expression contexts, the format follows the same JSON-first intent as
the JSON spec:

- strings without DSL markers are string literals
- strings with DSL markers such as `@`, `(`, or `::` are parsed as DSL expressions
- field references must be explicit with `@`, for example `@user.name`
- functions or casts that reference fields must use explicit refs inside the
  DSL, for example `lower(@user.name)` or `round(@user.age::decimal, 2)`

Examples:

```txt
$where[$expr][$eq][0]=@user.name
$where[$expr][$eq][1]=john
```

```txt
$where[$expr][$eq][0]=lower(@user.name)
$where[$expr][$eq][1]=lower('john')
```

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

In mixed expression contexts, transport strings like `john` are preferred for
plain string literals. SQL-style quoted string literals are primarily needed
inside DSL strings like `lower('john')`.

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
`$where[$expr]`.

### Alias Rules

- aliases are allowed only in `$select`
- aliases are not allowed in `$group`
- aliases are not allowed in `$sort`
- aliases are not allowed inside `$where`

Examples:

```txt
$select[0]=lower(user.name) as normalized_name
$select[1]=user.joinedAt::date as joined_on
```

Invalid:

```txt
$group[0]=lower(user.name) as normalized_name
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

```txt
$select[0]=user.age::double
$select[1]=user.joinedAt::date as joined_on
```

## `$select`

`$select` accepts:

- a comma-delimited string of simple field references
- an indexed array of scalar DSL expressions

Simple:

```txt
$select=user.id,user.name
```

Canonical:

```txt
$select[0]=user.id
$select[1]=lower(user.name) as normalized_name
$select[2]=round(user.age::decimal, 2) as rounded_age
```

Rejected:

```txt
$select=lower(user.name) as normalized_name,user.age
```

Reason: expression-bearing selections must use arrays.

## `$include`

`$include` accepts a comma-delimited string or an indexed array of relation
paths.

```txt
$include[0]=company
$include[1]=orders.items
```

## `$group`

`$group` accepts:

- a comma-delimited string of simple field references
- an indexed array of scalar DSL expressions

```txt
$group[0]=user.joinedAt::date
$group[1]=lower(user.name)
```

## `$sort`

`$sort` accepts:

- a comma-delimited string of simple field sorts such as `created_at` or `-created_at`
- an indexed array for expression-bearing sorts or explicit direction suffixes

Supported sort item forms:

- simple string shorthand items: `created_at`, `-created_at`
- array-only expression items: `lower(user.name) asc`, `user.joinedAt::date desc`

Simple:

```txt
$sort=-created_at,name
```

Canonical:

```txt
$sort[0]=user.joinedAt::date desc
$sort[1]=lower(user.name) asc
```

Recommended rule: sort by expressions directly, not by projection aliases.

Rejected:

```txt
$sort=lower(user.name) asc,user.joinedAt::date desc
```

Reason: expression-bearing sorts must use arrays.

## `$where`

`$where` keeps boolean structure in bracket notation.

### Shorthand Field Filters

Field-key shorthand remains the primary filter format:

```txt
$where[status]=active
$where[age][$gte]=18
$where[role][0]=admin
$where[role][1]=owner
```

This means:

- scalar value => equality
- indexed array value => `$in`
- operator object => explicit operator predicates

### Computed-Field Filters

Computed expressions may be used as field keys:

```txt
$where[lower(user.name)][$eq]=john
```

The left-hand key is parsed as a scalar DSL expression. The right-hand side
keeps operator transport semantics.

### Advanced Expression Comparisons

When both sides need to be expressions, use `$expr`.

```txt
$where[$expr][$eq][0]=@user.name
$where[$expr][$eq][1]=lower('john')
$where[$expr][$gte][0]=round(@user.age::decimal, 2)
$where[$expr][$gte][1]=18
```

Inside `$where[$expr]`:

- strings without DSL markers are string literals
- strings with DSL markers such as `@`, `(`, or `::` are parsed as DSL expressions
- explicit field references must use `@`, for example `@user.name`
- function or cast expressions that reference fields must use explicit refs, for
  example `lower(@user.name)` or `round(@user.age::decimal, 2)`
- DSL-quoted string literals are only needed inside DSL strings, for example
  `lower('john')`

### Logical Operators

- `$and`
- `$or`
- `$not`

Example:

```txt
$where[$or][0][status]=active
$where[$or][1][$expr][$eq][0]=lower(@user.name)
$where[$or][1][$expr][$eq][1]=john
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

```txt
$page=3
$size=25
```

Rules:

- `$page` is 1-based
- `$size` is required when `$page` is present

## `$cursor`

Cursor pagination stays metadata-driven.

Opaque token:

```txt
$cursor=eyJjcmVhdGVkX2F0IjoiMjAyNi0wNC0xMVQxMjowMDowMFoiLCJpZCI6OTgxfQ==
$size=25
```

Structured cursor:

```txt
$cursor[created_at]=2026-04-11T12:00:00Z
$cursor[id]=981
$size=25
```

Built-in adapters should still treat `$cursor` as metadata and rely on a query
transformer to rewrite it into filters and sorts.

When structured cursor payloads are passed through `schema.Normalize(...)` or
`schema.ToStorage(...)`, cursor keys follow the same alias normalization and
storage projection rules as the rest of the query.

## Disambiguation Rules

These rules keep the format parseable.

### Rule 1: Boolean logic stays in bracket notation

Allowed:

```txt
$where[$or][0][status]=active
$where[$or][1][status]=trial
```

Not allowed:

```txt
$where=status = 'active' or status = 'trial'
```

### Rule 2: Aliases are select-only

Allowed:

```txt
$select[0]=lower(user.name) as normalized_name
```

Not allowed:

```txt
$group[0]=lower(user.name) as normalized_name
```

### Rule 3: `$where[$expr]` strings are JSON-first by intent

Allowed:

```txt
$where[$expr][$eq][0]=lower(@user.name)
$where[$expr][$eq][1]=john
```

Not allowed:

```txt
$where[$expr][$eq][0]=lower(user.name)
$where[$expr][$eq][1]=john
```

Reason: mixed expression contexts require explicit `@` markers for field
references so bare transport strings can remain string literals.

### Rule 4: Use canonical cast names

Allowed:

```txt
$select[0]=user.age::double
```

Not allowed:

```txt
$select[0]=user.age::float8
```

Reason: backend-specific type names should stay in adapters, not the public API.

## Rejected Forms

These forms should not be part of the compact spec.

### Full SQL snippets

Reject:

```txt
$select[0]=LOWER(user.name) AS normalized_name, user.age
```

Reason: this collapses the format into raw SQL, weakens portability, and
violates the array-only rule for expression-bearing selection lists.

### Function objects mixed with boolean objects

Reject:

```txt
$where[lower(user.name)][$eq]=john
$where[$or]=status = 'active'
```

Reason: boolean structure should remain transport-native through bracket
notation.

### Alias-based sorting

Reject:

```txt
$select[0]=lower(user.name) as normalized_name
$sort[0]=normalized_name asc
```

Reason: sorting by aliases is not portable enough to be part of the canonical API.
