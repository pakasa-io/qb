// Package schema defines field policy, aliasing, decoding, and storage mapping
// helpers for qb queries.
//
// Use schema when your public API should expose stable field names while the
// underlying adapters operate on different storage-facing identifiers. Schema
// values can normalize filters, sorts, groups, projections, and structured
// cursors, and can project the same canonical query into storage-facing form.
//
// Schema is optional. It is most useful when public field aliases, operator
// allowlists, decoding rules, or relation-like API paths should be kept out of
// the core query model.
package schema
