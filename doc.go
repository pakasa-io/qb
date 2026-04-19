// Package qb defines a database-agnostic query model for filtering, projection,
// grouping, sorting, and pagination.
//
// The package centers on the [Query] AST and a fluent [Builder] for constructing
// it directly in Go code. The same model can also be produced by transport
// codecs and then normalized, rewritten, validated, compiled, or applied by
// other packages in the module.
//
// # Workflow
//
// A typical workflow is:
//
//  1. build a [Query] with [New] and the fluent builder, or parse one from a
//     transport-specific package
//  2. apply schema normalization or custom rewrites with [TransformQuery]
//  3. hand the final query to an adapter package such as `adapters/sql` or
//     `adapters/gorm`
//
// # What Lives In The Core Package
//
// The core package is intentionally dependency-free. It owns:
//
//   - the semantic query AST
//   - scalar expressions and predicate operators
//   - projections, grouping, sorting, and pagination metadata
//   - rewrite helpers and transformer pipelines
//   - structured errors and adapter capability metadata
//
// Input parsers and backend adapters live in separate packages so that the core
// query model stays independent from HTTP frameworks, ORMs, SQL drivers, and
// transport-specific parsing concerns.
package qb
