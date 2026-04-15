# Runnable Examples

This directory contains small runnable programs that demonstrate `qb` from
basic usage to more advanced integration patterns.

Run any example with:

```bash
go run ./examples/01-basic-builder
```

Examples are ordered so you can read them progressively:

- [`01-basic-builder`](./01-basic-builder): fluent builder, filtering, sorting, and `page`/`size`
- [`02-json-input`](./02-json-input): parse the compact `$...` JSON envelope and scalar DSL
- [`03-query-string`](./03-query-string): parse the same model from bracket-notation query strings
- [`04-schema-storage`](./04-schema-storage): aliases, decoding, normalization, and storage projection
- [`05-gorm-apply`](./05-gorm-apply): apply a query to GORM with `select` and `include`
- [`06-relations`](./06-relations): filter on related entities while keeping joins outside the core
- [`07-cursor-token`](./07-cursor-token): rewrite an opaque cursor token into deterministic filters
- [`08-composite-cursor`](./08-composite-cursor): use structured cursor values for stable multi-column pagination
- [`09-rewrite-pipeline`](./09-rewrite-pipeline): compose global query transformers for tenancy and soft deletes
- [`10-functions`](./10-functions): use `F`/`V`/`Func`-style scalar expressions and SQL functions
- [`11-yaml-input`](./11-yaml-input): parse the same query model from YAML
- [`12-schema-cursor`](./12-schema-cursor): normalize aliased structured cursors through schema and rewrite them into filters
- [`13-dialect-switching`](./13-dialect-switching): compile the same query for PostgreSQL, MySQL, and SQLite
- [`14-errors`](./14-errors): inspect structured parse, normalize, and compile diagnostics
- [`15-reporting`](./15-reporting): build aggregate reporting queries with projections, aliases, grouping, and pagination
- [`16-operators`](./16-operators): combine advanced predicate operators, null checks, negation, and grouped filters
- [`17-query-string-advanced`](./17-query-string-advanced): use expression-bearing query-string lists, `$expr`, grouping, includes, and pagination
- [`18-json-analytics`](./18-json-analytics): parse reporting-style JSON input with date functions, JSON helpers, aggregates, and grouping
- [`19-schema-gorm`](./19-schema-gorm): parse public API input, normalize through schema, project to storage, and apply it through GORM
- [`20-capabilities`](./20-capabilities): inspect dialect capability support and see unsupported features fail predictably

The long-form narrative guide still lives in [`docs/EXAMPLES.md`](../docs/EXAMPLES.md).
