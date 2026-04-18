package jsoncodec

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"

	"github.com/pakasa-io/qb"
	"github.com/pakasa-io/qb/codecs"
	docmodel "github.com/pakasa-io/qb/codecs/internal/docmodel"
)

// Parse decodes a JSON codec document into a query.
func Parse(data []byte, opts ...codecs.Option) (qb.Query, error) {
	decoder := stdjson.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return qb.Query{}, qb.NewError(
			err,
			qb.WithStage(qb.StageParse),
			qb.WithCode(qb.CodeInvalidInput),
		)
	}

	return docmodel.ParseDocument(payload, opts...)
}

// Marshal lowers a query into ordered JSON bytes.
func Marshal(query qb.Query, opts ...codecs.Option) ([]byte, error) {
	document, err := docmodel.BuildDocument(query, docmodel.TransportJSON, opts...)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := writeJSONValue(&buf, document, 0); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func writeJSONValue(buf *bytes.Buffer, value any, indent int) error {
	switch typed := value.(type) {
	case docmodel.OrderedObject:
		buf.WriteByte('{')
		if len(typed) == 0 {
			buf.WriteByte('}')
			return nil
		}
		for i, member := range typed {
			if i == 0 {
				buf.WriteByte('\n')
			} else {
				buf.WriteString(",\n")
			}
			writeIndent(buf, indent+2)
			keyBytes, _ := stdjson.Marshal(member.Key)
			buf.Write(keyBytes)
			buf.WriteString(": ")
			if err := writeJSONValue(buf, member.Value, indent+2); err != nil {
				return err
			}
		}
		buf.WriteByte('\n')
		writeIndent(buf, indent)
		buf.WriteByte('}')
		return nil
	case []any:
		buf.WriteByte('[')
		if len(typed) == 0 {
			buf.WriteByte(']')
			return nil
		}
		for i, item := range typed {
			if i == 0 {
				buf.WriteByte('\n')
			} else {
				buf.WriteString(",\n")
			}
			writeIndent(buf, indent+2)
			if err := writeJSONValue(buf, item, indent+2); err != nil {
				return err
			}
		}
		buf.WriteByte('\n')
		writeIndent(buf, indent)
		buf.WriteByte(']')
		return nil
	default:
		raw, err := stdjson.Marshal(typed)
		if err != nil {
			return fmt.Errorf("marshal json scalar: %w", err)
		}
		buf.Write(raw)
		return nil
	}
}

func writeIndent(buf *bytes.Buffer, count int) {
	for range count / 2 {
		buf.WriteString("  ")
	}
}
