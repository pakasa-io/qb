package codecconfig

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

// Config carries codec parsing and encoding behavior.
type Config struct {
	ValueDecoder        ValueDecoder
	FilterFieldResolver FilterFieldResolver
	GroupFieldResolver  GroupFieldResolver
	SortFieldResolver   SortFieldResolver
	LiteralCodec        LiteralCodec
	Mode                Mode
}

// Option customizes parsing or encoding behavior.
type Option func(*Config)

// WithValueDecoder sets the value coercion hook used for predicate values.
func WithValueDecoder(decoder ValueDecoder) Option {
	return func(opts *Config) {
		opts.ValueDecoder = decoder
	}
}

// WithFilterFieldResolver sets a hook for canonicalizing filter fields.
func WithFilterFieldResolver(resolver FilterFieldResolver) Option {
	return func(opts *Config) {
		opts.FilterFieldResolver = resolver
	}
}

// WithGroupFieldResolver sets a hook for canonicalizing grouping fields.
func WithGroupFieldResolver(resolver GroupFieldResolver) Option {
	return func(opts *Config) {
		opts.GroupFieldResolver = resolver
	}
}

// WithSortFieldResolver sets a hook for canonicalizing sort fields.
func WithSortFieldResolver(resolver SortFieldResolver) Option {
	return func(opts *Config) {
		opts.SortFieldResolver = resolver
	}
}

// WithLiteralCodec sets the codec used for non-JSON-native literal values.
func WithLiteralCodec(codec LiteralCodec) Option {
	return func(opts *Config) {
		opts.LiteralCodec = codec
	}
}

// WithMode controls whether codecs emit canonical or compact output.
func WithMode(mode Mode) Option {
	return func(opts *Config) {
		if mode == "" {
			mode = Canonical
		}
		opts.Mode = mode
	}
}

// ApplyOptions resolves the final codec config from the provided options.
func ApplyOptions(opts ...Option) Config {
	config := Config{
		LiteralCodec: DefaultLiteralCodec{},
		Mode:         Canonical,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	if config.LiteralCodec == nil {
		config.LiteralCodec = DefaultLiteralCodec{}
	}
	if config.Mode == "" {
		config.Mode = Canonical
	}
	return config
}
