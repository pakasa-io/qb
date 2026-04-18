package yamlcodec

import (
	"bytes"
	"fmt"

	"github.com/pakasa-io/qb"
	"github.com/pakasa-io/qb/codecs"
	docmodel "github.com/pakasa-io/qb/codecs/internal/docmodel"
	"gopkg.in/yaml.v3"
)

// Marshal lowers a query into ordered YAML bytes.
func Marshal(query qb.Query, opts ...codecs.Option) ([]byte, error) {
	document, err := docmodel.BuildDocument(query, docmodel.TransportYAML, opts...)
	if err != nil {
		return nil, err
	}

	root, err := buildYAMLNode(document)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildYAMLNode(value any) (*yaml.Node, error) {
	switch typed := value.(type) {
	case docmodel.OrderedObject:
		node := &yaml.Node{Kind: yaml.MappingNode}
		for _, member := range typed {
			key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: member.Key}
			child, err := buildYAMLNode(member.Value)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, key, child)
		}
		return node, nil
	case []any:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range typed {
			child, err := buildYAMLNode(item)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
		return node, nil
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: typed}, nil
	case bool:
		if typed {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}, nil
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(typed)}, nil
	default:
		return nil, fmt.Errorf("unsupported yaml value %T", value)
	}
}
