# Guide Path

These guides cover the main `qb` workflows from first query to schema-driven,
dialect-aware, rewrite-heavy pipelines. Start at the top and stop when the
examples match the complexity you need.

## Basic

- [01. Basic Builder](./01-basic-builder.md): build a `qb.Query` directly and compile it to SQL.
- [02. Basic Codecs](./02-basic-codecs.md): parse the same query shape from maps, JSON, YAML, and query strings.

## Medium

- [03. Filters, Functions, And Aggregates](./03-medium-filters-functions.md): grouped predicates, scalar helpers, and reporting-style projections.
- [04. Schema, Aliases, And Storage Mapping](./04-medium-schema-mapping.md): public API fields, decoders, allowlists, and storage projection.

## Advanced

- [05. Pagination And Cursors](./05-advanced-pagination-cursors.md): page/size, opaque cursor tokens, and composite cursors.
- [06. SQL, GORM, And Dialects](./06-advanced-adapters-dialects.md): compile once, switch dialects, and apply queries to GORM chains.
- [07. Rewrites, Preflight, And Diagnostics](./07-advanced-rewrites-preflight.md): transformer pipelines, capability validation, and structured errors.

## Complex

- [08. Relations And Analytics](./08-complex-relations-analytics.md): relation-aware filtering patterns, aggregate reports, and JSON/date-heavy queries.
- [09. Codec Output And Literal Round-Trips](./09-complex-codec-output.md): marshal queries back to transports, compact output, and typed literal codecs.

## Deep Dives

- [10. Custom Literal Codecs](./10-custom-literal-codecs.md): implement your own literal codec, delegate to the defaults, and understand transport limits.
- [11. Transformer Patterns](./11-transformer-patterns.md): compose rewrite pipelines, choose ordering, and build reusable query policies.
- [12. Complex Schema Patterns](./12-complex-schema-patterns.md): strict public APIs, relation paths, string-preserving query strings, and cursor-heavy schema flows.

## Coverage

These guides cover the bulk of the public surface:

- fluent query construction
- select/include/where/group/sort/page/size
- scalar helpers for string, math, aggregate, date, and JSON work
- JSON, YAML, query-string, and map-based parsing
- schema normalization, alias resolution, value decoding, and storage mapping
- SQL compilation, GORM application, dialect selection, and capability checks
- rewrite pipelines, cursor pagination, structured errors, and transport encoding
- custom literal codecs, advanced transformer composition, and complex schema policies

## Reference Material

- [Runnable examples](../../examples/README.md)
- [Narrative examples](./EXAMPLES.md)
- [JSON DSL spec](../specs/JSON_DSL_SPEC.md)
- [YAML DSL spec](../specs/YAML_DSL_SPEC.md)
- [Query-string DSL spec](../specs/QUERYSTRING_DSL_SPEC.md)
