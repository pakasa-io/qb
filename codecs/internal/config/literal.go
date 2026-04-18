package codecconfig

import (
	"encoding"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// DefaultLiteralCodecMode controls how the built-in literal codec treats
// application-defined non-JSON-native values.
type DefaultLiteralCodecMode string

const (
	// LiteralCodecModeStrict preserves only lossless built-ins by default.
	LiteralCodecModeStrict DefaultLiteralCodecMode = "strict"
	// LiteralCodecModeReversibleText enables registered reversible text types.
	LiteralCodecModeReversibleText DefaultLiteralCodecMode = "reversible_text"
	// DefaultLiteralCodecModeEnv is the environment variable used to choose the process default.
	DefaultLiteralCodecModeEnv = "QB_DEFAULT_LITERAL_CODEC_MODE"
)

var (
	defaultLiteralCodecMode atomic.Value

	reversibleTextTypesMu sync.RWMutex
	reversibleTextByCodec = map[string]reversibleTextSpec{}
	reversibleTextByType  = map[reflect.Type]string{}
)

func init() {
	defaultLiteralCodecMode.Store(loadDefaultLiteralCodecModeFromEnv())
}

type reversibleTextSpec struct {
	codec       string
	pointerType reflect.Type
	valueType   reflect.Type
}

// DefaultLiteralCodec handles common literal types such as time, duration, and UUIDs.
type DefaultLiteralCodec struct {
	Mode DefaultLiteralCodecMode
}

// SetDefaultLiteralCodecMode changes the process-wide default mode used by zero-value DefaultLiteralCodec.
func SetDefaultLiteralCodecMode(mode DefaultLiteralCodecMode) error {
	mode, err := normalizeLiteralCodecMode(mode)
	if err != nil {
		return err
	}
	defaultLiteralCodecMode.Store(mode)
	return nil
}

// DefaultLiteralCodecModeValue returns the current process-wide default codec mode.
func DefaultLiteralCodecModeValue() DefaultLiteralCodecMode {
	value, _ := defaultLiteralCodecMode.Load().(DefaultLiteralCodecMode)
	if value == "" {
		return LiteralCodecModeStrict
	}
	return value
}

// ResetDefaultLiteralCodecModeFromEnv reloads the process-wide default mode from the environment.
func ResetDefaultLiteralCodecModeFromEnv() error {
	mode, err := parseLiteralCodecMode(os.Getenv(DefaultLiteralCodecModeEnv))
	if err != nil {
		return err
	}
	defaultLiteralCodecMode.Store(mode)
	return nil
}

// RegisterReversibleTextType registers a codec name for a type that roundtrips through
// encoding.TextMarshaler and encoding.TextUnmarshaler.
func RegisterReversibleTextType(codec string, prototype any) error {
	codec = strings.TrimSpace(codec)
	if codec == "" {
		return fmt.Errorf("codec/model: reversible text codec name cannot be empty")
	}
	if prototype == nil {
		return fmt.Errorf("codec/model: reversible text prototype cannot be nil")
	}

	ptrType := reflect.TypeOf(prototype)
	if ptrType.Kind() != reflect.Ptr || ptrType.Elem().Kind() == reflect.Invalid {
		return fmt.Errorf("codec/model: reversible text prototype must be a non-nil pointer")
	}

	textMarshalerType := reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	textUnmarshalerType := reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	if !ptrType.Implements(textMarshalerType) || !ptrType.Implements(textUnmarshalerType) {
		return fmt.Errorf("codec/model: prototype must implement both encoding.TextMarshaler and encoding.TextUnmarshaler")
	}

	spec := reversibleTextSpec{
		codec:       codec,
		pointerType: ptrType,
		valueType:   ptrType.Elem(),
	}

	reversibleTextTypesMu.Lock()
	defer reversibleTextTypesMu.Unlock()

	if existing, ok := reversibleTextByCodec[codec]; ok && existing.pointerType != ptrType {
		return fmt.Errorf("codec/model: reversible text codec %q already registered for %s", codec, existing.pointerType)
	}
	reversibleTextByCodec[codec] = spec
	reversibleTextByType[ptrType] = codec
	reversibleTextByType[ptrType.Elem()] = codec
	return nil
}

// FormatLiteral formats known non-JSON-native values into transport-safe literals.
func (c DefaultLiteralCodec) FormatLiteral(value any) (any, string, bool, error) {
	mode := c.effectiveMode()
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano), "time", true, nil
	case time.Duration:
		return typed.String(), "duration", true, nil
	case uuid.UUID:
		return typed.String(), "uuid", true, nil
	default:
		if mode == LiteralCodecModeReversibleText {
			literal, codec, handled, err := formatRegisteredReversibleText(value)
			if handled || err != nil {
				return literal, codec, handled, err
			}
		}
		return nil, "", false, nil
	}
}

// ParseLiteral parses known codec-tagged literal values back into Go values.
func (c DefaultLiteralCodec) ParseLiteral(codec string, literal any) (any, bool, error) {
	mode := c.effectiveMode()
	text := fmt.Sprint(literal)
	switch codec {
	case "":
		return nil, false, nil
	case "time":
		value, err := time.Parse(time.RFC3339Nano, text)
		return value, true, err
	case "duration":
		value, err := time.ParseDuration(text)
		return value, true, err
	case "uuid":
		value, err := uuid.Parse(text)
		return value, true, err
	default:
		if mode == LiteralCodecModeReversibleText {
			value, handled, err := parseRegisteredReversibleText(codec, text)
			if handled || err != nil {
				return value, handled, err
			}
		}
		return nil, false, nil
	}
}

func (c DefaultLiteralCodec) effectiveMode() DefaultLiteralCodecMode {
	if c.Mode == "" {
		return DefaultLiteralCodecModeValue()
	}
	mode, err := normalizeLiteralCodecMode(c.Mode)
	if err != nil {
		return LiteralCodecModeStrict
	}
	return mode
}

func loadDefaultLiteralCodecModeFromEnv() DefaultLiteralCodecMode {
	mode, err := parseLiteralCodecMode(os.Getenv(DefaultLiteralCodecModeEnv))
	if err != nil {
		return LiteralCodecModeStrict
	}
	return mode
}

func parseLiteralCodecMode(raw string) (DefaultLiteralCodecMode, error) {
	if strings.TrimSpace(raw) == "" {
		return LiteralCodecModeStrict, nil
	}
	return normalizeLiteralCodecMode(DefaultLiteralCodecMode(raw))
}

func normalizeLiteralCodecMode(mode DefaultLiteralCodecMode) (DefaultLiteralCodecMode, error) {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case "", string(LiteralCodecModeStrict):
		return LiteralCodecModeStrict, nil
	case "reversible_text", "reversible-text":
		return LiteralCodecModeReversibleText, nil
	default:
		return "", fmt.Errorf("codec/model: unsupported literal codec mode %q", mode)
	}
}

func formatRegisteredReversibleText(value any) (any, string, bool, error) {
	reversibleTextTypesMu.RLock()
	codec, spec, ok := lookupReversibleTextSpec(value)
	reversibleTextTypesMu.RUnlock()
	if !ok {
		return nil, "", false, nil
	}

	marshaler, err := makeTextMarshaler(value, spec)
	if err != nil {
		return nil, "", true, err
	}
	text, err := marshaler.MarshalText()
	if err != nil {
		return nil, "", true, err
	}
	return string(text), codec, true, nil
}

func parseRegisteredReversibleText(codec string, literal string) (any, bool, error) {
	reversibleTextTypesMu.RLock()
	spec, ok := reversibleTextByCodec[codec]
	reversibleTextTypesMu.RUnlock()
	if !ok {
		return nil, false, nil
	}

	instance := reflect.New(spec.valueType)
	unmarshaler := instance.Interface().(encoding.TextUnmarshaler)
	if err := unmarshaler.UnmarshalText([]byte(literal)); err != nil {
		return nil, true, err
	}
	return instance.Elem().Interface(), true, nil
}

func lookupReversibleTextSpec(value any) (string, reversibleTextSpec, bool) {
	valueType := reflect.TypeOf(value)
	if valueType == nil {
		return "", reversibleTextSpec{}, false
	}
	if codec, ok := reversibleTextByType[valueType]; ok {
		return codec, reversibleTextByCodec[codec], true
	}
	if valueType.Kind() != reflect.Ptr {
		if codec, ok := reversibleTextByType[reflect.PointerTo(valueType)]; ok {
			return codec, reversibleTextByCodec[codec], true
		}
	}
	return "", reversibleTextSpec{}, false
}

func makeTextMarshaler(value any, spec reversibleTextSpec) (encoding.TextMarshaler, error) {
	if marshaler, ok := value.(encoding.TextMarshaler); ok {
		return marshaler, nil
	}

	valueType := reflect.TypeOf(value)
	if valueType == nil {
		return nil, fmt.Errorf("codec/model: nil reversible text value")
	}
	if valueType.Kind() == reflect.Ptr {
		return nil, fmt.Errorf("codec/model: pointer value %s does not implement encoding.TextMarshaler", valueType)
	}

	ptr := reflect.New(valueType)
	ptr.Elem().Set(reflect.ValueOf(value))
	marshaler, ok := ptr.Interface().(encoding.TextMarshaler)
	if !ok {
		return nil, fmt.Errorf("codec/model: type %s does not implement encoding.TextMarshaler", spec.pointerType)
	}
	return marshaler, nil
}
