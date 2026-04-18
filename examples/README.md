# Runnable Examples

This directory contains small runnable programs that demonstrate `qb` from
basic usage to more advanced integration patterns.

Run any example with:

```bash
go run ./examples/core/basic-builder
```

Examples are grouped by concern:

- [`core/basic-builder`](./core/basic-builder): fluent builder, filtering, sorting, and `page`/`size`
- [`core/functions`](./core/functions): scalar expressions and SQL function helpers
- [`core/operators`](./core/operators): advanced predicate operators, null checks, negation, and grouped filters
- [`core/rewrite-pipeline`](./core/rewrite-pipeline): global query transformers for tenancy and soft deletes
- [`core/relations`](./core/relations): relation filtering while keeping joins outside the core
- [`core/cursor-token`](./core/cursor-token): opaque cursor token rewrites
- [`core/composite-cursor`](./core/composite-cursor): structured multi-column cursor pagination
- [`core/errors`](./core/errors): structured parse, normalize, and compile diagnostics
- [`core/scalar-toolbox`](./core/scalar-toolbox): broader scalar helper coverage
- [`core/preflight-pipeline`](./core/preflight-pipeline): explicit normalize, rewrite, storage-map, and capability-validation pipeline
- [`codecs/json-input`](./codecs/json-input): compact JSON envelope and scalar DSL parsing
- [`codecs/querystring`](./codecs/querystring): bracket-notation query-string parsing
- [`codecs/querystring-advanced`](./codecs/querystring-advanced): expression-bearing query-string lists, `$expr`, and grouped filters
- [`codecs/querystring-literals`](./codecs/querystring-literals): string-preserving query-string leaves with schema-driven decoding
- [`codecs/yaml-input`](./codecs/yaml-input): YAML transport over the same semantic model
- [`schema/storage-mapping`](./schema/storage-mapping): aliases, decoding, normalization, and storage projection
- [`schema/cursor-normalization`](./schema/cursor-normalization): structured cursor normalization through schema
- [`schema/gorm-public-api`](./schema/gorm-public-api): public API input normalized through schema and applied with GORM
- [`adapters/gorm-apply`](./adapters/gorm-apply): applying a query through GORM
- [`adapters/dialect-switching`](./adapters/dialect-switching): PostgreSQL, MySQL, and SQLite compilation of the same query
- [`adapters/default-dialect`](./adapters/default-dialect): process-wide default dialect changes and per-compiler overrides
- [`adapters/capabilities`](./adapters/capabilities): capability inspection and unsupported-feature behavior
- [`analytics/reporting`](./analytics/reporting): aggregate reporting queries with projections, aliases, grouping, and pagination
- [`analytics/json-analytics`](./analytics/json-analytics): reporting-style JSON input with date functions, JSON helpers, and aggregates
- [`analytics/date-json-toolbox`](./analytics/date-json-toolbox): PostgreSQL-first `date_bin`, `extract`, JSON constructors, and related helpers

The long-form narrative guide still lives in [`docs/guides/EXAMPLES.md`](../docs/guides/EXAMPLES.md).
