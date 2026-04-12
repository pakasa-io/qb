// Package qb defines a database-agnostic query model for filtering, projection,
// grouping, and pagination.
//
// The core package is intentionally small and dependency-free. It owns the
// semantic query AST, a fluent builder, rewrite helpers, transformer pipelines,
// structured errors, and adapter capability metadata. Input parsers and output
// adapters are expected to live in separate packages and depend on qb, but not
// on each other.
package qb
