package codec

import (
	"fmt"

	"github.com/pakasa-io/qb"
	"github.com/pakasa-io/qb/codecs"
	docmodel "github.com/pakasa-io/qb/codecs/internal/docmodel"
	"gopkg.in/yaml.v3"
)

// Parse decodes YAML input and parses it into a qb.Query using the shared codec document codecs.
func Parse(data []byte, opts ...codecs.Option) (qb.Query, error) {
	var payload any
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return qb.Query{}, qb.NewError(
			err,
			qb.WithStage(qb.StageParse),
			qb.WithCode(qb.CodeInvalidInput),
		)
	}

	object, ok := normalizeYAML(payload).(map[string]any)
	if !ok {
		return qb.Query{}, qb.NewError(
			fmt.Errorf("expected YAML document root to be an object"),
			qb.WithStage(qb.StageParse),
			qb.WithCode(qb.CodeInvalidInput),
		)
	}

	return docmodel.ParseDocument(object, opts...)
}

func normalizeYAML(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalizeYAML(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[fmt.Sprint(key)] = normalizeYAML(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = normalizeYAML(item)
		}
		return out
	default:
		return value
	}
}
