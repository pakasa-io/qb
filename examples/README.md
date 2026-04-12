# Runnable Examples

This directory contains small runnable programs that demonstrate `qb` from
basic usage to more advanced integration patterns.

Run any example with:

```bash
go run ./examples/01-basic-builder
```

Examples are ordered so you can read them progressively:

- [`01-basic-builder`](./01-basic-builder): fluent builder, filtering, sorting, `pick`, and `page`/`size`
- [`02-json-input`](./02-json-input): parse a JSON payload with nested filters and `group_by`
- [`03-query-string`](./03-query-string): parse bracket-notation query-string input
- [`04-schema-storage`](./04-schema-storage): aliases, decoding, normalization, and storage projection
- [`05-gorm-apply`](./05-gorm-apply): apply a query to GORM with `select` and `include`
- [`06-relations`](./06-relations): filter on related entities while keeping joins outside the core
- [`07-cursor-token`](./07-cursor-token): rewrite an opaque cursor token into deterministic filters
- [`08-composite-cursor`](./08-composite-cursor): use structured cursor values for stable multi-column pagination
- [`09-rewrite-pipeline`](./09-rewrite-pipeline): compose global query transformers for tenancy and soft deletes

The long-form narrative guide still lives in [`docs/EXAMPLES.md`](../docs/EXAMPLES.md).
