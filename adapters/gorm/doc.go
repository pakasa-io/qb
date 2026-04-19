// Package gorm applies qb.Query values to GORM query chains.
//
// The adapter preserves the core qb query model while translating filters,
// projections, grouping, sorting, pagination, and include hints into the
// corresponding GORM APIs. Query transformers can be attached to normalize,
// rewrite, or validate queries before they are applied.
package gorm
