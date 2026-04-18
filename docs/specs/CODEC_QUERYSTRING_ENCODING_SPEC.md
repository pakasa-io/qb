# Query-String Codec Encoding Spec

This document defines the planned query-string codec for `qb`.

Status: proposed, not implemented.

This document extends and constrains
[CODEC_MODEL_SPEC.md](./CODEC_MODEL_SPEC.md). The model spec is the semantic
source of truth.

## Scope

The planned query-string codec:

- encodes `qb.Query` into the canonical or compact query-string document
- decodes bracket-notation query strings into the canonical query-string document
- lifts the canonical query-string document into the shared semantic codec model
- lowers the semantic codec model into `qb.Query`

Planned package:

- `codecs/querystring`

## Relationship To The Model Spec

Query string is a transport-specific canonical projection of the shared semantic
codec model.

It does not share the exact same concrete canonical document as JSON or YAML.

Transport constraints:

- leaf values are strings
- null semantics may be normalized into unary null operators
- ordinary field and cursor values remain schema-decoded by default

These constraints are acceptable because the roundtrip contract is defined at
the semantic codec model layer, not at the concrete document-shape layer.

## Canonical Query-String Document

Canonical top-level keys, in order:

1. `$select`
2. `$include`
3. `$where`
4. `$group`
5. `$sort`
6. `$page`
7. `$size`
8. `$cursor`

Canonical example:

```txt
$select[0]=users.id
$select[1]=lower(users.name) as normalized_name
$include[0]=Company
$include[1]=Orders
$where[age][$gte]=18
$where[status]=active
$where[$expr][$eq][0]=lower(@users.name)
$where[$expr][$eq][1]=lower('john')
$group[0]=users.status
$sort[0]=lower(users.name) asc
$page=2
$size=20
```

## Canonical Transport Rules

### Bracket Notation

Objects are encoded with bracket notation:

```txt
$where[age][$gte]=18
```

### Indexed Arrays Only

Indexed arrays are canonical.

Examples:

```txt
$select[0]=users.id
$select[1]=lower(users.name) as normalized_name
```

```txt
$where[$or][0][role]=admin
$where[$or][1][role]=owner
```

Repeated keys are not canonical output.

### Pair Ordering

Deterministic pair ordering is required.

Rules:

- top-level keys follow this spec’s canonical order
- object members follow the model spec ordering rules
- arrays emit in ascending index order

Mixed `$where` example:

```txt
$where[age][$gte]=18
$where[status]=active
$where[lower(users.name)][$eq]=john
$where[$expr][$gte][0]=@users.score
$where[$expr][$gte][1]=90
$where[$or][0][role]=admin
$where[$or][1][role]=owner
```

## String-Only Leaf Values

All query-string leaves are strings.

Implications:

- `18` is a transport string
- `true` is a transport string
- `null` is a transport string
- `2026-04-14T15:04:05Z` is a transport string

Ordinary typed meaning is recovered through:

- schema decoding
- caller-provided value decoding
- explicit semantic rewrites described below

The query-string codec must not perform eager transport-level coercion.

## Scalar Encoding

Query string can encode semantic scalar nodes in two ways:

- DSL strings
- structured scalar subdocuments represented with bracket notation

### DSL Strings

DSL strings are the default form when no metadata would be lost.

Examples:

```txt
$select[0]=users.id
$sort[0]=lower(users.name) asc
$where[$expr][$eq][0]=lower(@users.name)
```

### Structured Scalar Subdocuments

Canonical query-string output must use structured scalar subdocuments when a
scalar contains typed literal metadata or other metadata that cannot be
losslessly represented as a plain DSL string.

Example:

```txt
$select[0][$expr][$call]=date_bin
$select[0][$expr][$args][0][$literal]=15m0s
$select[0][$expr][$args][0][$codec]=duration
$select[0][$expr][$args][1][$field]=events.created_at
$select[0][$expr][$args][2][$literal]=2026-04-15T00:00:00Z
$select[0][$expr][$args][2][$codec]=time
$select[0][$as]=bucket
```

Sort wrapper example:

```txt
$sort[0][$expr][$call]=lower
$sort[0][$expr][$args][0][$field]=users.name
$sort[0][$dir]=asc
```

This keeps query-string leaves string-only while still allowing metadata-rich
scalars to roundtrip through the semantic codec model.

## Compact Query-String Rules

Compact query-string output may apply only these shorthands:

- `$select=user.id,user.status` when every projection is a simple field reference
- `$group=users.status` when every group item is a simple field reference
- `$sort=-users.created_at,users.status` when every sort is a simple sortable field reference

`$include` remains indexed-array output in both modes.

Compact mode may also use inline typed-literal interpolation tokens inside DSL
strings when the literal body is inline-safe:

- `!#:time:2026-04-15T00:00:00Z`
- `!#:duration:15m0s`
- `!#::lower('john')`

Example:

```txt
$select[0]=date_bin(!#:duration:15m0s, events.created_at, !#:time:2026-04-15T00:00:00Z) as bucket
```

If a typed literal is not inline-safe, compact mode must still use the
structured scalar subdocument form.

## Filter Encoding

Query-string filter lowering follows the shared semantic model but may use
transport-specific canonical rewrites.

### Simple Equality

```go
qb.F("status").Eq("active")
```

becomes:

```txt
$where[status]=active
```

### Explicit Operators

```go
qb.F("age").Gte(18)
```

becomes:

```txt
$where[age][$gte]=18
```

### Computed Left-Hand Expression

```go
qb.F("users.name").Lower().Eq("john")
```

becomes:

```txt
$where[lower(users.name)][$eq]=john
```

### Expression Fallback

```go
qb.F("users.name").Lower().Eq(qb.V("JOHN").Lower())
```

becomes:

```txt
$where[$expr][$eq][0]=lower(@users.name)
$where[$expr][$eq][1]=lower('JOHN')
```

Typed literal fallback inside `$expr`:

```txt
$where[$expr][$gte][0][$field]=events.created_at
$where[$expr][$gte][1][$literal]=2026-04-15T00:00:00Z
$where[$expr][$gte][1][$codec]=time
```

## `$literal` And `$codec`

### Ordinary Leaves

For ordinary field predicates and structured cursor values, canonical query
string output keeps literal values as plain transport strings by default. Typed
recovery there is schema-driven.

### Structured Scalar Subdocuments

Inside structured scalar subdocuments, `$literal` and `$codec` may appear so
that typed literal metadata can roundtrip through the semantic codec model.

Example:

```txt
$where[$expr][$eq][0][$field]=users.note
$where[$expr][$eq][1][$literal]=@admin
```

Example with codec:

```txt
$select[0][$expr][$args][0][$literal]=15m0s
$select[0][$expr][$args][0][$codec]=duration
```

### Compact Forced-Literal Strings

Compact mode may use `!#::...` to force literal treatment inside DSL strings:

```txt
$where[$expr][$eq][0]=@users.note
$where[$expr][$eq][1]=!#::lower('john')
```

## Null Rules

Because query-string leaves are string-only, canonical query-string output
normalizes semantic null comparisons into unary null operators.

Canonical rewrites:

- `qb.F("deleted_at").Eq(nil)` -> `$where[deleted_at][$isnull]=true`
- `qb.F("deleted_at").Ne(nil)` -> `$where[deleted_at][$notnull]=true`
- `qb.F("deleted_at").IsNull()` -> `$where[deleted_at][$isnull]=true`
- `qb.F("deleted_at").NotNull()` -> `$where[deleted_at][$notnull]=true`

For expression predicates:

- `Eq(nil)` -> `$where[$expr][$isnull]=<left expr or scalar subdocument>`
- `Ne(nil)` -> `$where[$expr][$notnull]=<left expr or scalar subdocument>`

Examples:

```txt
$where[deleted_at][$isnull]=true
```

```txt
$where[$expr][$isnull]=lower(@users.deleted_at)
```

This normalization is part of the canonical query-string document and is
acceptable because the semantic roundtrip target is the shared codec model.

## Cursor Encoding

Canonical cursor shapes:

- token cursor: `$cursor=opaque-token`
- structured cursor: bracket-notation object

Examples:

```txt
$cursor=opaque-token
$size=25
```

```txt
$cursor[createdAt]=2026-04-11T12:00:00Z
$cursor[id]=981
$size=25
```

Ordinary structured cursor values are emitted as plain strings. Recovering their
types is caller-owned through schema or other value decoding.

## Pagination Rules

Canonical public pagination uses:

- `$page`
- `$size`
- `$cursor`

Raw `limit` and `offset` are not part of canonical query-string output.

If a query contains raw `Limit` or `Offset` without a lossless page/size
mapping, encoding must fail.

## URL Encoding

All examples in this document are shown in decoded form for readability.

Actual URLs must percent-encode reserved characters such as:

- spaces
- brackets
- quotes
- commas when required by the surrounding transport

For example:

```txt
$select[1]=lower(users.name) as normalized_name
```

would be transmitted more like:

```txt
%24select%5B1%5D=lower%28users.name%29%20as%20normalized_name
```
