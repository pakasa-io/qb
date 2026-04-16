package model

import "github.com/pakasa-io/qb"

// ValueDecoder allows callers to coerce raw parser values into domain values.
type ValueDecoder func(field string, op qb.Operator, value any) (any, error)

// FilterFieldResolver canonicalizes and validates filter fields.
type FilterFieldResolver func(field string, op qb.Operator) (string, error)

// GroupFieldResolver canonicalizes and validates grouping fields.
type GroupFieldResolver func(field string) (string, error)

// SortFieldResolver canonicalizes and validates sort fields.
type SortFieldResolver func(field string) (string, error)

// LiteralCodec formats and parses non-JSON-native literal values.
type LiteralCodec interface {
	FormatLiteral(value any) (literal any, codec string, handled bool, err error)
	ParseLiteral(codec string, literal any) (value any, handled bool, err error)
}

// Mode controls whether codecs emit canonical or compact transport documents.
type Mode string

const (
	Canonical Mode = "canonical"
	Compact   Mode = "compact"
)

type options struct {
	valueDecoder        ValueDecoder
	filterFieldResolver FilterFieldResolver
	groupFieldResolver  GroupFieldResolver
	sortFieldResolver   SortFieldResolver
	literalCodec        LiteralCodec
	mode                Mode
}

// Option customizes parsing or encoding behavior.
type Option func(*options)

// WithValueDecoder sets the value coercion hook used for predicate values.
func WithValueDecoder(decoder ValueDecoder) Option {
	return func(opts *options) {
		opts.valueDecoder = decoder
	}
}

// WithFilterFieldResolver sets a hook for canonicalizing filter fields.
func WithFilterFieldResolver(resolver FilterFieldResolver) Option {
	return func(opts *options) {
		opts.filterFieldResolver = resolver
	}
}

// WithGroupFieldResolver sets a hook for canonicalizing grouping fields.
func WithGroupFieldResolver(resolver GroupFieldResolver) Option {
	return func(opts *options) {
		opts.groupFieldResolver = resolver
	}
}

// WithSortFieldResolver sets a hook for canonicalizing sort fields.
func WithSortFieldResolver(resolver SortFieldResolver) Option {
	return func(opts *options) {
		opts.sortFieldResolver = resolver
	}
}

// WithLiteralCodec sets the codec used for non-JSON-native literal values.
func WithLiteralCodec(codec LiteralCodec) Option {
	return func(opts *options) {
		opts.literalCodec = codec
	}
}

// WithMode controls whether codecs emit canonical or compact output.
func WithMode(mode Mode) Option {
	return func(opts *options) {
		if mode == "" {
			mode = Canonical
		}
		opts.mode = mode
	}
}

func buildOptions(opts ...Option) options {
	config := options{
		literalCodec: DefaultLiteralCodec{},
		mode:         Canonical,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	if config.literalCodec == nil {
		config.literalCodec = DefaultLiteralCodec{}
	}
	if config.mode == "" {
		config.mode = Canonical
	}
	return config
}
