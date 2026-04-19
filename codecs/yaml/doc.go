// Package yaml parses and emits the qb YAML transport.
//
// It turns YAML documents into qb.Query values and marshals qb.Query values
// back into ordered YAML using the shared codec model and options from the
// parent codecs package.
//
// The YAML transport is semantically equivalent to the JSON transport and is
// often useful for configuration-driven queries or human-edited documents.
package yaml
