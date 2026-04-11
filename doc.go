// Package qb defines a database-agnostic query model for filtering, sorting,
// and pagination.
//
// The core package is intentionally small and dependency-free. It owns only the
// semantic query AST and a fluent builder. Input parsers and output adapters are
// expected to live in separate packages and depend on qb, but not on each other.
package qb
