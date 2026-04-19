// Package qs parses and emits the qb bracket-notation query-string transport.
//
// The package maps URL query parameters onto the same semantic qb.Query model
// used by the JSON, YAML, and normalized document codecs, and can encode a
// qb.Query back into canonical query-string form.
//
// Query strings are best suited to API-facing filters and pagination metadata.
// When exact type control matters, pair parsing with schema decoders because
// query-string encoding flattens leaf values to strings.
package qs
