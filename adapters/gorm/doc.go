// Package gorm applies qb.Query values to GORM query chains.
//
// The adapter preserves the core qb query model while translating filters,
// projections, grouping, sorting, pagination, and include hints into the
// corresponding GORM APIs.
//
// Use [Adapter.Apply] to mutate an existing `*gorm.DB` chain or [Adapter.Scope]
// to expose the same behavior as an idiomatic GORM scope. Query transformers
// can be attached to normalize, rewrite, or validate queries before they are
// applied.
package gorm
