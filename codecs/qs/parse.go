package qs

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/pakasa-io/qb"
	"github.com/pakasa-io/qb/codecs"
	docmodel "github.com/pakasa-io/qb/codecs/internal/docmodel"
)

// Parse converts bracket-notation query-string values into a qb.Query.
func Parse(values url.Values, opts ...codecs.Option) (qb.Query, error) {
	document := map[string]any{}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		path, err := tokenize(key)
		if err != nil {
			return qb.Query{}, qb.NewError(
				err,
				qb.WithDefaultStage(qb.StageParse),
				qb.WithDefaultCode(qb.CodeInvalidInput),
				qb.WithPath(key),
			)
		}

		raw := values[key]
		if len(raw) == 0 {
			continue
		}

		var value any
		if len(raw) == 1 {
			value = raw[0]
		} else {
			items := make([]any, len(raw))
			for i, item := range raw {
				items[i] = item
			}
			value = items
		}

		inserted, err := insert(document, path, value)
		if err != nil {
			return qb.Query{}, qb.NewError(
				err,
				qb.WithDefaultStage(qb.StageParse),
				qb.WithDefaultCode(qb.CodeInvalidInput),
				qb.WithPath(key),
			)
		}

		root, ok := inserted.(map[string]any)
		if !ok {
			return qb.Query{}, qb.NewError(
				fmt.Errorf("invalid root structure"),
				qb.WithStage(qb.StageParse),
				qb.WithCode(qb.CodeInvalidInput),
				qb.WithPath(key),
			)
		}
		document = root
	}

	return docmodel.ParseDocument(document, opts...)
}

// ParseString parses a raw query-string into a qb.Query.
func ParseString(raw string, opts ...codecs.Option) (qb.Query, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return qb.Query{}, qb.NewError(
			err,
			qb.WithStage(qb.StageParse),
			qb.WithCode(qb.CodeInvalidInput),
		)
	}
	return Parse(values, opts...)
}

func tokenize(key string) ([]string, error) {
	if key == "" {
		return nil, fmt.Errorf("empty key")
	}

	tokens := make([]string, 0, 4)
	var current strings.Builder

	for i := 0; i < len(key); i++ {
		switch key[i] {
		case '[':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}

			end := strings.IndexByte(key[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("missing closing bracket")
			}

			token := key[i+1 : i+end]
			if token == "" {
				return nil, fmt.Errorf("empty bracket segment")
			}

			tokens = append(tokens, token)
			i += end
		case ']':
			return nil, fmt.Errorf("unexpected closing bracket")
		default:
			current.WriteByte(key[i])
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty key")
	}

	return tokens, nil
}

func insert(current any, path []string, value any) (any, error) {
	if len(path) == 0 {
		return value, nil
	}

	token := path[0]
	if token == "" {
		return nil, fmt.Errorf("empty path segment")
	}

	if index, ok := parseIndex(token); ok {
		var list []any
		switch typed := current.(type) {
		case nil:
			list = []any{}
		case []any:
			list = append([]any(nil), typed...)
		default:
			return nil, fmt.Errorf("expected list, got %T", current)
		}

		for len(list) <= index {
			list = append(list, nil)
		}

		child, err := insert(list[index], path[1:], value)
		if err != nil {
			return nil, err
		}
		list[index] = child
		return list, nil
	}

	var object map[string]any
	switch typed := current.(type) {
	case nil:
		object = map[string]any{}
	case map[string]any:
		object = typed
	default:
		return nil, fmt.Errorf("expected object, got %T", current)
	}

	if len(path) == 1 {
		if existing, ok := object[token]; ok {
			switch typed := existing.(type) {
			case []any:
				object[token] = append(typed, value)
			default:
				object[token] = []any{typed, value}
			}
		} else {
			object[token] = value
		}
		return object, nil
	}

	child, err := insert(object[token], path[1:], value)
	if err != nil {
		return nil, err
	}

	object[token] = child
	return object, nil
}

func parseIndex(value string) (int, bool) {
	index, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}

	return index, true
}
