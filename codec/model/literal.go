package model

import (
	"encoding"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DefaultLiteralCodec handles common literal types such as time, duration, and UUIDs.
type DefaultLiteralCodec struct{}

// FormatLiteral formats known non-JSON-native values into transport-safe literals.
func (DefaultLiteralCodec) FormatLiteral(value any) (any, string, bool, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano), "time", true, nil
	case time.Duration:
		return typed.String(), "duration", true, nil
	case uuid.UUID:
		return typed.String(), "uuid", true, nil
	case encoding.TextMarshaler:
		text, err := typed.MarshalText()
		if err != nil {
			return nil, "", true, err
		}
		return string(text), "", true, nil
	default:
		return nil, "", false, nil
	}
}

// ParseLiteral parses known codec-tagged literal values back into Go values.
func (DefaultLiteralCodec) ParseLiteral(codec string, literal any) (any, bool, error) {
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
		return nil, false, nil
	}
}
