package mapinput

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pakasa-io/qb"
)

func parseProjectionList(node any, path string) ([]qb.Projection, error) {
	switch typed := node.(type) {
	case string:
		names, err := parseNames(typed, path)
		if err != nil {
			return nil, err
		}
		return namesToProjections(names), nil
	case []string:
		out := make([]qb.Projection, 0, len(typed))
		for i, item := range typed {
			names, err := parseNames(item, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			out = append(out, namesToProjections(names)...)
		}
		return out, nil
	case []any:
		out := make([]qb.Projection, 0, len(typed))
		for i, item := range typed {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if value, ok := item.(string); ok {
				names, err := parseNames(value, itemPath)
				if err != nil {
					return nil, err
				}
				out = append(out, namesToProjections(names)...)
				continue
			}

			projection, err := parseProjection(item, itemPath)
			if err != nil {
				return nil, err
			}
			out = append(out, projection)
		}
		return out, nil
	case map[string]any:
		projection, err := parseProjection(typed, path)
		if err != nil {
			return nil, err
		}
		return []qb.Projection{projection}, nil
	default:
		return nil, parseError(
			fmt.Errorf("expected string, list, or object, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
}

func parseGroupByList(node any, path string) ([]qb.Scalar, error) {
	switch typed := node.(type) {
	case string:
		names, err := parseNames(typed, path)
		if err != nil {
			return nil, err
		}
		return namesToRefs(names), nil
	case []string:
		out := make([]qb.Scalar, 0, len(typed))
		for i, item := range typed {
			names, err := parseNames(item, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			out = append(out, namesToRefs(names)...)
		}
		return out, nil
	case []any:
		out := make([]qb.Scalar, 0, len(typed))
		for i, item := range typed {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if value, ok := item.(string); ok {
				names, err := parseNames(value, itemPath)
				if err != nil {
					return nil, err
				}
				out = append(out, namesToRefs(names)...)
				continue
			}

			expr, err := parseProjectionScalar(item, itemPath)
			if err != nil {
				return nil, err
			}
			out = append(out, expr)
		}
		return out, nil
	case map[string]any:
		expr, err := parseProjectionScalar(typed, path)
		if err != nil {
			return nil, err
		}
		return []qb.Scalar{expr}, nil
	default:
		return nil, parseError(
			fmt.Errorf("expected string, list, or object, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
}

func parseProjection(node any, path string) (qb.Projection, error) {
	switch typed := node.(type) {
	case string:
		expr, err := parseProjectionScalar(typed, path)
		if err != nil {
			return qb.Projection{}, err
		}
		return qb.Project(expr), nil
	case map[string]any:
		alias, err := parseProjectionAlias(typed, path)
		if err != nil {
			return qb.Projection{}, err
		}

		exprNode, exprPath, err := projectionExprNode(typed, path)
		if err != nil {
			return qb.Projection{}, err
		}

		expr, err := parseProjectionScalar(exprNode, exprPath)
		if err != nil {
			return qb.Projection{}, err
		}

		projection := qb.Project(expr)
		if alias != "" {
			projection = projection.As(alias)
		}
		return projection, nil
	default:
		return qb.Projection{}, parseError(
			fmt.Errorf("expected string or object, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
}

func parseProjectionAlias(node map[string]any, path string) (string, error) {
	keys := []string{"$as", "alias"}
	var (
		alias string
		found string
	)

	for _, key := range keys {
		value, ok := node[key]
		if !ok || value == nil {
			continue
		}
		if found != "" {
			return "", parseError(
				fmt.Errorf("only one of $as or alias may be provided"),
				qb.CodeInvalidInput,
				qb.WithPath(path),
			)
		}
		name, ok := value.(string)
		if !ok {
			return "", parseError(
				fmt.Errorf("expected alias to be a string, got %T", value),
				qb.CodeInvalidInput,
				qb.WithPath(path+"."+key),
			)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return "", parseError(
				fmt.Errorf("projection alias cannot be empty"),
				qb.CodeInvalidInput,
				qb.WithPath(path+"."+key),
			)
		}
		alias = name
		found = key
	}

	return alias, nil
}

func projectionExprNode(node map[string]any, path string) (any, string, error) {
	if expr, ok := node["$expr"]; ok {
		for key := range node {
			if key == "$expr" || key == "$as" || key == "alias" {
				continue
			}
			return nil, "", parseError(
				fmt.Errorf("projection object cannot mix $expr with %q", key),
				qb.CodeInvalidInput,
				qb.WithPath(path+"."+key),
			)
		}
		return expr, path + ".$expr", nil
	}

	expr := map[string]any{}
	for key, value := range node {
		if key == "$as" || key == "alias" {
			continue
		}
		expr[key] = value
	}
	if len(expr) == 0 {
		return nil, "", parseError(
			fmt.Errorf("projection object requires an expression"),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
	return expr, path, nil
}

func parseProjectionScalar(node any, path string) (qb.Scalar, error) {
	switch typed := node.(type) {
	case string:
		name := strings.TrimSpace(typed)
		if name == "" {
			return nil, parseError(
				fmt.Errorf("field reference cannot be empty"),
				qb.CodeInvalidInput,
				qb.WithPath(path),
			)
		}
		return qb.F(name), nil
	default:
		return parseStructuredScalar(node, path)
	}
}

func parseStructuredScalar(node any, path string) (qb.Scalar, error) {
	if expr, ok := qb.AsScalar(node); ok {
		return qb.CloneScalar(expr), nil
	}

	object, ok := node.(map[string]any)
	if !ok {
		return nil, parseError(
			fmt.Errorf("expected expression object, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}

	if wrapped, ok := object["$expr"]; ok {
		for key := range object {
			if key == "$expr" {
				continue
			}
			return nil, parseError(
				fmt.Errorf("expression object cannot mix $expr with %q", key),
				qb.CodeInvalidInput,
				qb.WithPath(path+"."+key),
			)
		}
		return parseProjectionScalar(wrapped, path+".$expr")
	}

	if rawField, ok := object["$field"]; ok {
		if err := expectOnlyKeys(object, "$field"); err != nil {
			return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(path))
		}
		field, ok := rawField.(string)
		if !ok {
			return nil, parseError(
				fmt.Errorf("expected field reference to be a string, got %T", rawField),
				qb.CodeInvalidInput,
				qb.WithPath(path+".$field"),
			)
		}
		field = strings.TrimSpace(field)
		if field == "" {
			return nil, parseError(
				fmt.Errorf("field reference cannot be empty"),
				qb.CodeInvalidInput,
				qb.WithPath(path+".$field"),
			)
		}
		return qb.F(field), nil
	}

	if rawCall, ok := object["$call"]; ok {
		if err := expectOnlyKeys(object, "$call", "args"); err != nil {
			return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(path))
		}
		name, ok := rawCall.(string)
		if !ok {
			return nil, parseError(
				fmt.Errorf("expected function name to be a string, got %T", rawCall),
				qb.CodeInvalidInput,
				qb.WithPath(path+".$call"),
			)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, parseError(
				fmt.Errorf("function name cannot be empty"),
				qb.CodeInvalidInput,
				qb.WithPath(path+".$call"),
			)
		}

		args, err := parseCallArgs(object["args"], path+".args")
		if err != nil {
			return nil, err
		}
		return qb.Call{Name: name, Args: args}, nil
	}

	if rawValue, ok := object["$value"]; ok {
		if err := expectOnlyKeys(object, "$value"); err != nil {
			return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(path))
		}
		return qb.V(normalizeJSONLiteral(rawValue)), nil
	}

	return nil, parseError(
		fmt.Errorf("expected one of $field, $call, or $value"),
		qb.CodeInvalidInput,
		qb.WithPath(path),
	)
}

func parseCallArgs(node any, path string) ([]qb.Scalar, error) {
	if node == nil {
		return nil, nil
	}

	switch typed := node.(type) {
	case []any:
		args := make([]qb.Scalar, len(typed))
		for i, item := range typed {
			arg, err := parseCallArg(item, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			args[i] = arg
		}
		return args, nil
	case []string:
		args := make([]qb.Scalar, len(typed))
		for i, item := range typed {
			args[i] = qb.V(item)
		}
		return args, nil
	default:
		return nil, parseError(
			fmt.Errorf("expected args to be a list, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
}

func parseCallArg(node any, path string) (qb.Scalar, error) {
	if expr, ok := qb.AsScalar(node); ok {
		return qb.CloneScalar(expr), nil
	}

	if object, ok := node.(map[string]any); ok {
		return parseStructuredScalar(object, path)
	}

	return qb.V(normalizeJSONLiteral(node)), nil
}

func normalizeJSONLiteral(value any) any {
	switch typed := value.(type) {
	case json.Number:
		return normalizeJSONNumber(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = normalizeJSONLiteral(item)
		}
		return out
	case []string:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = item
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalizeJSONLiteral(item)
		}
		return out
	default:
		return value
	}
}

func expectOnlyKeys(values map[string]any, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}

	for key := range values {
		if _, ok := allowedSet[key]; ok {
			continue
		}
		return fmt.Errorf("unexpected key %q", key)
	}

	return nil
}

func namesToRefs(values []string) []qb.Scalar {
	if len(values) == 0 {
		return nil
	}

	out := make([]qb.Scalar, len(values))
	for i, value := range values {
		out[i] = qb.F(value)
	}

	return out
}

func namesToProjections(values []string) []qb.Projection {
	if len(values) == 0 {
		return nil
	}

	out := make([]qb.Projection, len(values))
	for i, value := range values {
		out[i] = qb.Project(qb.F(value))
	}

	return out
}
