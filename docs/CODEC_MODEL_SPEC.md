# Codec Model Spec

This document defines the planned codec architecture for `qb`.

Status: proposed, not implemented.

This spec assumes a hard refactor from `parser/*` to `codec/*` with no concern
for backward compatibility.

## Goals

- make input and output codecs symmetric
- keep `qb.Query` as the semantic source of truth
- define one shared semantic codec model
- let each transport define its own canonical document shape
- preserve deterministic output ordering
- support compact human-readable output without weakening canonical output
- keep reverse schema mapping explicit rather than automatic

## Planned Package Layout

Planned package layout after the hard refactor:

- `codec/model`
  - lowers `qb.Query` to and from the shared semantic codec model
  - owns semantic lowering, lifting, and validation rules
- `codec/json`
  - projects the semantic codec model into the canonical JSON document
  - serializes and deserializes JSON
- `codec/yaml`
  - projects the semantic codec model into the canonical YAML document
  - serializes and deserializes YAML
- `codec/querystring`
  - projects the semantic codec model into the canonical query-string document
  - serializes and deserializes bracket-notation query strings
- `codec/dsl`
  - optional shared scalar-expression formatter and parser used by transport codecs

Whether `codec/dsl` is public or internal is out of scope for this spec. The
behavior it must implement is normative.

## Semantic Source

The semantic source for encoding is `qb.Query`.

Primary codec APIs operate on `qb.Query`.

Convenience helpers may accept `qb.Builder`, but they must first resolve the
builder into `qb.Query` by calling `builder.Query()`.

`qb.Builder` is not itself a transport model.

## Shared Codec Pipeline

All codecs follow the same logical pipeline:

1. `qb.Query`
2. optional caller-owned query transforms
3. lowering into the shared semantic codec model
4. projection into a transport-specific canonical document
5. serialization into bytes or `url.Values`

Decoding follows the reverse direction:

1. transport bytes or query-string values
2. transport-specific canonical document
3. shared semantic codec model
4. `qb.Query`
5. optional caller-owned query transforms such as schema normalization or storage mapping

This means:

- the shared layer is semantic
- JSON, YAML, and query string do not have to share one concrete canonical document
- transport-specific rewrites are allowed as long as they roundtrip to the same
  semantic codec model

## Shared Semantic Codec Model

The shared semantic codec model is not tied to a specific concrete Go map or
JSON object shape. It captures the semantic content of a query:

- projections
- includes
- filter tree
- grouping expressions
- sort expressions and directions
- page/size pagination
- cursor token or structured cursor values
- scalar expressions
- literal nodes, optionally carrying codec metadata

### Scalar Nodes

The semantic scalar model supports at least these node kinds:

- field reference
- function call
- cast
- literal

Projection and sort wrappers are separate semantic nodes:

- projection = scalar + optional alias
- sort = scalar + direction

### Literal Nodes

The semantic literal node carries:

- literal value
- optional codec identity
- whether the literal must stay a literal even if its string form resembles DSL

Examples of semantic literal nodes:

- plain string literal `"john"`
- plain numeric literal `18`
- plain null literal `nil`
- typed literal `"2026-04-15T00:00:00Z"` with codec `time`
- typed literal `"15m0s"` with codec `duration`
- forced literal `"lower('john')"` with no codec

The semantic model does not require every transport to render literals the same
way. It requires them to preserve the same semantic node.

## Transport-Specific Canonical Documents

Each transport defines its own canonical document projected from the shared
semantic codec model.

Examples:

- JSON canonical document
- YAML canonical document
- query-string canonical document

Those canonical documents may differ in shape where transport constraints force
it.

Examples of allowed divergence:

- query string may canonically normalize null equality into unary null operators
- query string may keep ordinary leaf values string-only
- JSON and YAML may preserve direct numeric and null literals more naturally

What must stay stable is the shared semantic codec model, not the concrete
transport document shape.

## Encoding Modes

Codecs must support two output modes:

- `Canonical`
- `Compact`

Default mode is `Canonical`.

### Canonical Mode

Canonical mode is deterministic and safest for roundtrips.

Rules:

- transport-specific canonical documents are used
- shorthands are minimized
- scalar expressions may fall back to structured scalar forms when needed for
  lossless roundtrips
- filter lowering is conservative

### Compact Mode

Compact mode is presentation-oriented.

Rules:

- simple list shorthands may be used where the target transport allows them
- scalar expressions should prefer DSL-string rendering when it is lossless
- typed literals may use inline interpolation tokens when safe
- structured scalar fallback remains available when compact rendering would be ambiguous

## Scalar Projection Rules

Transport codecs may project semantic scalar nodes in two ways:

- scalar DSL strings
- structured scalar objects

### Scalar DSL Strings

Scalar DSL strings are the default human-readable form.

Standalone expression contexts:

- projection items
- group items
- sort items
- computed filter keys

Mixed expression contexts:

- expression-predicate operands such as `$where.$expr`

Ref formatting rules:

- standalone contexts emit field refs without `@`
- mixed expression contexts emit field refs with `@`

Examples:

- `users.name`
- `lower(users.name)`
- `lower(@users.name)`
- `users.created_at::date`

### Structured Scalar Objects

Structured scalar objects are the lossless fallback form.

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

Wrapper shapes:

- projection item:
  - `{ "$expr": <scalar>, "$as": "alias" }`
- sort item:
  - `{ "$expr": <scalar>, "$dir": "asc" | "desc" }`

Structured scalar forms must be used in canonical mode when the scalar contains
metadata that cannot be losslessly preserved in plain DSL strings.

Examples:

- typed literal arguments inside function calls
- forced-literal strings that would otherwise be interpreted as DSL
- any future scalar metadata not representable in plain DSL text

## Typed Literal Interpolation Token

Compact mode may inline literal metadata into DSL strings with this token form:

- `!#:<codec>:<literal>`
- `!#::<literal>`

Rules:

- the codec segment may be empty
- `!#::<literal>` means “literal, no codec”
- `!#:<codec>:<literal>` means “literal with explicit codec identity”

Examples:

- `!#:time:2026-04-15T00:00:00Z`
- `!#:duration:15m0s`
- `!#::@admin`
- `!#::lower('john')`

This token is compact sugar over the semantic literal node. It is not the
authoritative semantic representation.

## Inline-Safe Rule

Compact typed-literal interpolation may be used only when the literal body is
inline-safe for the surrounding scalar DSL context.

A literal body is inline-safe only when all of the following are true:

- it has no leading or trailing whitespace
- it contains no comma `,`
- it contains no closing parenthesis `)`
- it contains no carriage return or newline
- rendering it inline would not make the surrounding DSL ambiguous

If a literal is not inline-safe, the transport codec must fall back to the
structured scalar form.

## Literal Handling

### Direct Literals

When the transport supports them naturally and no metadata is needed, literal
values may be emitted directly.

Examples:

- JSON number `18`
- JSON boolean `true`
- JSON null `null`
- query-string leaf `active`

### Forced-Literal Wrapper

When a string must remain a literal even though it looks like DSL, codecs use a
literal node with no codec identity.

Examples:

- literal string `@admin`
- literal string `lower('john')`

### Typed Literal Wrapper

When a literal carries codec metadata, the semantic node must preserve that
metadata even if the transport renders it differently.

Examples:

- `time`
- `date`
- `uuid`
- `duration`

## Literal Codec Hook

Codecs must support an explicit literal codec hook, for example:

```go
codec.WithLiteralCodec(...)
```

The literal codec hook is responsible for:

- formatting non-JSON-native Go values for transport output
- parsing typed literal wrappers or typed interpolation tokens back into Go values
  when the transport supports that roundtrip

Default built-in literal codecs should cover common cases:

- `time`
  - RFC3339 or RFC3339Nano timestamps
- `date`
  - `YYYY-MM-DD`
- `uuid`
  - canonical lowercase UUID string form
- `duration`
  - Go duration string form

If no literal codec can format a non-JSON-native value, encoding must fail with
a codec error.

## Conservative Filter Lowering

Filter lowering is conservative.

The encoder must prefer the simplest representation that is unambiguous and
deterministic.

Examples:

- simple field equality may lower to field shorthand
- non-equality operators lower to explicit operator objects
- computed left-hand expressions may lower to computed keys
- complex expression-vs-expression predicates fall back to expression form

### Null Semantics

The semantic model distinguishes:

- `field = null`
- `field <> null`
- `field IS NULL`
- `field IS NOT NULL`

Transport-specific canonical documents may project these differently as long as
they roundtrip to the same semantic model.

Examples:

- JSON and YAML may preserve `eq null` as direct `null`
- query string may canonically rewrite `eq null` to an `isnull` operator

## Pagination And Cursor

Canonical public pagination uses:

- page
- size
- cursor

Raw `limit` and `offset` are not part of canonical transport output.

If a query contains raw `Limit` or `Offset` without a lossless page/size
mapping, encoding must fail.

Cursor semantics:

- token cursor
- structured cursor values

No automatic reverse storage-to-public mapping is performed. If callers want
public field names, they must transform the query first, for example with a
future `schema.ToPublic(query)` transformer.

## Deterministic Ordering

Deterministic ordering is part of the spec.

Rules:

- top-level transport keys follow the canonical order defined by that transport
- projections, includes, groups, sorts, `$and`, and `$or` preserve source order
- structured cursor object keys are emitted in sorted lexical order
- `$where` members are emitted in this exact order:
  1. simple field keys, sorted lexically
  2. computed-expression keys, sorted lexically
  3. `$expr`
  4. `$not`
  5. `$or`
  6. `$and`

There is no competing “reserved keys first” rule. The list above is the
authoritative canonical ordering.

## Schema Mapping

No automatic reverse schema mapping occurs during encoding.

If callers want transport output expressed in public API field names, they must
apply an explicit query transform before encoding.

This spec expects a future optional complement to `schema.ToStorage`:

```go
publicQuery, err := userSchema.ToPublic(query)
```

That transformer is not part of the current implementation.
