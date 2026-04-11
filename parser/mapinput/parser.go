package mapinput

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/pakasa-io/qb"
)

// ValueDecoder allows callers to coerce raw parser values into domain values.
type ValueDecoder func(field string, op qb.Operator, value any) (any, error)

// FilterFieldResolver canonicalizes and validates filter fields.
type FilterFieldResolver func(field string, op qb.Operator) (string, error)

// SortFieldResolver canonicalizes and validates sort fields.
type SortFieldResolver func(field string) (string, error)

type options struct {
	valueDecoder        ValueDecoder
	filterFieldResolver FilterFieldResolver
	sortFieldResolver   SortFieldResolver
}

// Option customizes parsing behavior.
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

// WithSortFieldResolver sets a hook for canonicalizing sort fields.
func WithSortFieldResolver(resolver SortFieldResolver) Option {
	return func(opts *options) {
		opts.sortFieldResolver = resolver
	}
}

// ParseJSON decodes JSON input and parses it into a query.
func ParseJSON(data []byte, opts ...Option) (qb.Query, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return qb.Query{}, err
	}

	return Parse(payload, opts...)
}

// Parse converts a normalized object into a query.
func Parse(input map[string]any, opts ...Option) (qb.Query, error) {
	config := options{}
	for _, opt := range opts {
		opt(&config)
	}

	var query qb.Query

	if where, ok := pickObject(input, "where", "filter"); ok {
		filter, err := parseExpr(where, config, "where")
		if err != nil {
			return qb.Query{}, err
		}
		query.Filter = filter
	}

	if rawSort, ok := pickValue(input, "sort"); ok {
		sorts, err := parseSorts(rawSort, config)
		if err != nil {
			return qb.Query{}, err
		}
		query.Sorts = sorts
	}

	if rawLimit, ok := pickValue(input, "limit"); ok {
		limit, err := parseInteger(rawLimit, "limit")
		if err != nil {
			return qb.Query{}, err
		}
		if limit < 0 {
			return qb.Query{}, fmt.Errorf("limit: cannot be negative")
		}
		query.Limit = &limit
	}

	if rawOffset, ok := pickValue(input, "offset"); ok {
		offset, err := parseInteger(rawOffset, "offset")
		if err != nil {
			return qb.Query{}, err
		}
		if offset < 0 {
			return qb.Query{}, fmt.Errorf("offset: cannot be negative")
		}
		query.Offset = &offset
	}

	return query, nil
}

func parseExpr(node any, opts options, path string) (qb.Expr, error) {
	object, ok := node.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected object", path)
	}

	keys := sortedKeys(object)
	exprs := make([]qb.Expr, 0, len(keys))

	for _, key := range keys {
		value := object[key]
		switch key {
		case "$and":
			expr, err := parseLogicalGroup(qb.And, value, opts, path+".$and")
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, expr)
		case "$or":
			expr, err := parseLogicalGroup(qb.Or, value, opts, path+".$or")
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, expr)
		case "$not":
			expr, err := parseNegation(value, opts, path+".$not")
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, expr)
		default:
			expr, err := parseField(key, value, opts, path+"."+key)
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, expr)
		}
	}

	return qb.And(exprs...), nil
}

func parseLogicalGroup(combine func(...qb.Expr) qb.Expr, node any, opts options, path string) (qb.Expr, error) {
	items, err := asList(node, path)
	if err != nil {
		return nil, err
	}

	exprs := make([]qb.Expr, 0, len(items))
	for i, item := range items {
		expr, err := parseExpr(item, opts, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
	}

	return combine(exprs...), nil
}

func parseNegation(node any, opts options, path string) (qb.Expr, error) {
	expr, err := parseExpr(node, opts, path)
	if err != nil {
		return nil, err
	}

	return qb.Not(expr), nil
}

func parseField(field string, node any, opts options, path string) (qb.Expr, error) {
	switch typed := node.(type) {
	case map[string]any:
		keys := sortedKeys(typed)
		exprs := make([]qb.Expr, 0, len(keys))

		for _, key := range keys {
			expr, err := parseOperator(field, key, typed[key], opts, path+"."+key)
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, expr)
		}

		return qb.And(exprs...), nil
	case []any:
		resolvedField, err := resolveFilterField(field, qb.OpIn, opts, path)
		if err != nil {
			return nil, err
		}

		values, err := decodeList(resolvedField, qb.OpIn, typed, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).In(values...), nil
	default:
		resolvedField, err := resolveFilterField(field, qb.OpEq, opts, path)
		if err != nil {
			return nil, err
		}

		value, err := decodeValue(resolvedField, qb.OpEq, typed, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Eq(value), nil
	}
}

func parseOperator(field, operator string, node any, opts options, path string) (qb.Expr, error) {
	switch operator {
	case "$eq":
		resolvedField, err := resolveFilterField(field, qb.OpEq, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpEq, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Eq(value), nil
	case "$ne":
		resolvedField, err := resolveFilterField(field, qb.OpNe, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpNe, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Ne(value), nil
	case "$gt":
		resolvedField, err := resolveFilterField(field, qb.OpGt, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpGt, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Gt(value), nil
	case "$gte":
		resolvedField, err := resolveFilterField(field, qb.OpGte, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpGte, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Gte(value), nil
	case "$lt":
		resolvedField, err := resolveFilterField(field, qb.OpLt, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpLt, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Lt(value), nil
	case "$lte":
		resolvedField, err := resolveFilterField(field, qb.OpLte, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpLte, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Lte(value), nil
	case "$in":
		resolvedField, err := resolveFilterField(field, qb.OpIn, opts, path)
		if err != nil {
			return nil, err
		}
		values, err := decodeListFromNode(resolvedField, qb.OpIn, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).In(values...), nil
	case "$nin":
		resolvedField, err := resolveFilterField(field, qb.OpNotIn, opts, path)
		if err != nil {
			return nil, err
		}
		values, err := decodeListFromNode(resolvedField, qb.OpNotIn, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).NotIn(values...), nil
	case "$like":
		resolvedField, err := resolveFilterField(field, qb.OpLike, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpLike, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Like(value), nil
	case "$contains":
		resolvedField, err := resolveFilterField(field, qb.OpContains, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpContains, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Contains(value), nil
	case "$prefix":
		resolvedField, err := resolveFilterField(field, qb.OpPrefix, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpPrefix, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Prefix(value), nil
	case "$suffix":
		resolvedField, err := resolveFilterField(field, qb.OpSuffix, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpSuffix, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Suffix(value), nil
	case "$isnull":
		resolvedField, err := resolveFilterField(field, qb.OpIsNull, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).IsNull(), nil
	case "$notnull":
		resolvedField, err := resolveFilterField(field, qb.OpNotNull, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).NotNull(), nil
	default:
		return nil, fmt.Errorf("%s: unsupported operator %q", path, operator)
	}
}

func parseSorts(node any, opts options) ([]qb.Sort, error) {
	rawItems, err := asStringList(node, "sort")
	if err != nil {
		return nil, err
	}

	sorts := make([]qb.Sort, 0, len(rawItems))
	for _, item := range rawItems {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		direction := qb.Asc
		field := item
		if strings.HasPrefix(item, "-") {
			direction = qb.Desc
			field = strings.TrimPrefix(item, "-")
		} else if strings.HasPrefix(item, "+") {
			field = strings.TrimPrefix(item, "+")
		}

		if field == "" {
			return nil, fmt.Errorf("sort: empty sort field")
		}

		resolvedField, err := resolveSortField(field, opts)
		if err != nil {
			return nil, err
		}

		sorts = append(sorts, qb.Sort{
			Field:     resolvedField,
			Direction: direction,
		})
	}

	return sorts, nil
}

func parseInteger(value any, path string) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case float32:
		if math.Trunc(float64(typed)) != float64(typed) {
			return 0, fmt.Errorf("%s: expected whole number, got %v", path, typed)
		}
		return int(typed), nil
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("%s: expected whole number, got %v", path, typed)
		}
		return int(typed), nil
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return int(n), nil
		}
		f, err := typed.Float64()
		if err != nil {
			return 0, fmt.Errorf("%s: %w", path, err)
		}
		if math.Trunc(f) != f {
			return 0, fmt.Errorf("%s: expected whole number, got %v", path, typed)
		}
		return int(f), nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("%s: %w", path, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%s: expected integer, got %T", path, value)
	}
}

func decodeValue(field string, op qb.Operator, value any, opts options, path string) (any, error) {
	value = normalizeJSONNumber(value)
	if opts.valueDecoder == nil {
		return value, nil
	}

	decoded, err := opts.valueDecoder(field, op, value)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return decoded, nil
}

func resolveFilterField(field string, op qb.Operator, opts options, path string) (string, error) {
	if opts.filterFieldResolver == nil {
		return field, nil
	}

	resolvedField, err := opts.filterFieldResolver(field, op)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if resolvedField == "" {
		return "", fmt.Errorf("%s: filter field resolver returned an empty field", path)
	}
	return resolvedField, nil
}

func resolveSortField(field string, opts options) (string, error) {
	if opts.sortFieldResolver == nil {
		return field, nil
	}

	resolvedField, err := opts.sortFieldResolver(field)
	if err != nil {
		return "", fmt.Errorf("sort: %w", err)
	}
	if resolvedField == "" {
		return "", fmt.Errorf("sort: sort field resolver returned an empty field")
	}
	return resolvedField, nil
}

func decodeListFromNode(field string, op qb.Operator, node any, opts options, path string) ([]any, error) {
	values, err := asList(node, path)
	if err != nil {
		return nil, err
	}
	return decodeList(field, op, values, opts, path)
}

func decodeList(field string, op qb.Operator, values []any, opts options, path string) ([]any, error) {
	decoded := make([]any, 0, len(values))
	for i, value := range values {
		item, err := decodeValue(field, op, value, opts, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		decoded = append(decoded, item)
	}
	return decoded, nil
}

func asList(value any, path string) ([]any, error) {
	switch typed := value.(type) {
	case []any:
		return typed, nil
	case string:
		items := strings.Split(typed, ",")
		list := make([]any, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			list = append(list, item)
		}
		return list, nil
	default:
		return nil, fmt.Errorf("%s: expected list, got %T", path, value)
	}
}

func asStringList(value any, path string) ([]string, error) {
	switch typed := value.(type) {
	case string:
		items := strings.Split(typed, ",")
		out := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			out = append(out, item)
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(typed))
		for i, item := range typed {
			switch value := item.(type) {
			case string:
				for _, part := range strings.Split(value, ",") {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					out = append(out, part)
				}
			default:
				return nil, fmt.Errorf("%s[%d]: expected string, got %T", path, i, item)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s: expected string or list, got %T", path, value)
	}
}

func normalizeJSONNumber(value any) any {
	number, ok := value.(json.Number)
	if !ok {
		return value
	}

	if integer, err := number.Int64(); err == nil {
		return integer
	}

	if floatValue, err := number.Float64(); err == nil {
		return floatValue
	}

	return value
}

func pickObject(input map[string]any, keys ...string) (map[string]any, bool) {
	for _, key := range keys {
		value, ok := input[key]
		if !ok || value == nil {
			continue
		}
		object, ok := value.(map[string]any)
		if ok {
			return object, true
		}
	}
	return nil, false
}

func pickValue(input map[string]any, key string) (any, bool) {
	value, ok := input[key]
	if !ok || value == nil {
		return nil, false
	}
	return value, true
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
