package codecs

import (
	"github.com/pakasa-io/qb"
	codecconfig "github.com/pakasa-io/qb/codecs/config"
	docmodel "github.com/pakasa-io/qb/codecs/internal/docmodel"
)

type ValueDecoder = codecconfig.ValueDecoder
type FilterFieldResolver = codecconfig.FilterFieldResolver
type GroupFieldResolver = codecconfig.GroupFieldResolver
type SortFieldResolver = codecconfig.SortFieldResolver
type LiteralCodec = codecconfig.LiteralCodec
type Config = codecconfig.Config
type Option = codecconfig.Option
type Mode = codecconfig.Mode
type DefaultLiteralCodec = codecconfig.DefaultLiteralCodec
type DefaultLiteralCodecMode = codecconfig.DefaultLiteralCodecMode

const (
	Canonical                      = codecconfig.Canonical
	Compact                        = codecconfig.Compact
	LiteralCodecModeStrict         = codecconfig.LiteralCodecModeStrict
	LiteralCodecModeReversibleText = codecconfig.LiteralCodecModeReversibleText
	DefaultLiteralCodecModeEnv     = codecconfig.DefaultLiteralCodecModeEnv
)

var (
	WithValueDecoder                    = codecconfig.WithValueDecoder
	WithFilterFieldResolver             = codecconfig.WithFilterFieldResolver
	WithGroupFieldResolver              = codecconfig.WithGroupFieldResolver
	WithSortFieldResolver               = codecconfig.WithSortFieldResolver
	WithLiteralCodec                    = codecconfig.WithLiteralCodec
	WithMode                            = codecconfig.WithMode
	ApplyOptions                        = codecconfig.ApplyOptions
	SetDefaultLiteralCodecMode          = codecconfig.SetDefaultLiteralCodecMode
	DefaultLiteralCodecModeValue        = codecconfig.DefaultLiteralCodecModeValue
	ResetDefaultLiteralCodecModeFromEnv = codecconfig.ResetDefaultLiteralCodecModeFromEnv
	RegisterReversibleTextType          = codecconfig.RegisterReversibleTextType
)

// Parse converts a normalized codec document into a query.
func Parse(input map[string]any, opts ...Option) (qb.Query, error) {
	return docmodel.ParseDocument(input, opts...)
}
