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
		return qb.Query{}, parseError(err, qb.CodeInvalidInput)
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

	if selectKey, rawSelect, ok, err := pickExclusiveValue(input, "select", "pick"); err != nil {
		return qb.Query{}, parseError(err, qb.CodeInvalidInput)
	} else if ok {
		selects, err := parseNames(rawSelect, selectKey)
		if err != nil {
			return qb.Query{}, err
		}
		query.Projections = namesToProjections(selects)
	}

	if rawInclude, ok := pickValue(input, "include"); ok {
		includes, err := parseNames(rawInclude, "include")
		if err != nil {
			return qb.Query{}, err
		}
		query.Includes = includes
	}

	if rawGroupBy, ok := pickValue(input, "group_by"); ok {
		groupBy, err := parseNames(rawGroupBy, "group_by")
		if err != nil {
			return qb.Query{}, err
		}
		query.GroupBy = namesToRefs(groupBy)
	}

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
			return qb.Query{}, parseError(
				fmt.Errorf("limit cannot be negative"),
				qb.CodeInvalidValue,
				qb.WithPath("limit"),
			)
		}
		query.Limit = &limit
	}

	if rawOffset, ok := pickValue(input, "offset"); ok {
		offset, err := parseInteger(rawOffset, "offset")
		if err != nil {
			return qb.Query{}, err
		}
		if offset < 0 {
			return qb.Query{}, parseError(
				fmt.Errorf("offset cannot be negative"),
				qb.CodeInvalidValue,
				qb.WithPath("offset"),
			)
		}
		query.Offset = &offset
	}

	if rawPage, ok := pickValue(input, "page"); ok {
		page, err := parseInteger(rawPage, "page")
		if err != nil {
			return qb.Query{}, err
		}
		if page < 1 {
			return qb.Query{}, parseError(
				fmt.Errorf("page must be greater than or equal to 1"),
				qb.CodeInvalidValue,
				qb.WithPath("page"),
			)
		}
		query.Page = &page
	}

	if rawSize, ok := pickValue(input, "size"); ok {
		size, err := parseInteger(rawSize, "size")
		if err != nil {
			return qb.Query{}, err
		}
		if size < 1 {
			return qb.Query{}, parseError(
				fmt.Errorf("size must be greater than or equal to 1"),
				qb.CodeInvalidValue,
				qb.WithPath("size"),
			)
		}
		query.Size = &size
	}

	if rawCursor, ok := pickValue(input, "cursor"); ok {
		cursor, err := parseCursor(rawCursor, "cursor")
		if err != nil {
			return qb.Query{}, err
		}
		query.Cursor = cursor
	}

	if _, _, err := query.ResolvedPagination(); err != nil {
		return qb.Query{}, parseError(err, qb.CodeInvalidQuery)
	}

	return query, nil
}

func parseExpr(node any, opts options, path string) (qb.Expr, error) {
	object, ok := node.(map[string]any)
	if !ok {
		return nil, parseError(
			fmt.Errorf("expected object"),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
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
		return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(path))
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
	case "$ilike":
		resolvedField, err := resolveFilterField(field, qb.OpILike, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpILike, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).ILike(value), nil
	case "$regexp":
		resolvedField, err := resolveFilterField(field, qb.OpRegexp, opts, path)
		if err != nil {
			return nil, err
		}
		value, err := decodeValue(resolvedField, qb.OpRegexp, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Field(resolvedField).Regexp(value), nil
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
		return nil, parseError(
			fmt.Errorf("unsupported operator %q", operator),
			qb.CodeUnsupportedOperator,
			qb.WithPath(path),
			qb.WithField(field),
		)
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
			return nil, parseError(
				fmt.Errorf("empty sort field"),
				qb.CodeInvalidInput,
				qb.WithPath("sort"),
			)
		}

		resolvedField, err := resolveSortField(field, opts)
		if err != nil {
			return nil, err
		}

		sorts = append(sorts, qb.Sort{
			Expr:      qb.F(resolvedField),
			Direction: direction,
		})
	}

	return sorts, nil
}

func parseNames(node any, path string) ([]string, error) {
	items, err := asStringList(node, path)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		names = append(names, item)
	}

	return names, nil
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

func parseCursor(node any, path string) (*qb.Cursor, error) {
	switch typed := node.(type) {
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return nil, parseError(
				fmt.Errorf("cursor token cannot be empty"),
				qb.CodeInvalidInput,
				qb.WithPath(path),
			)
		}
		return &qb.Cursor{Token: typed}, nil
	case json.Number:
		return &qb.Cursor{Token: typed.String()}, nil
	case bool:
		return &qb.Cursor{Token: strconv.FormatBool(typed)}, nil
	case int:
		return &qb.Cursor{Token: strconv.Itoa(typed)}, nil
	case int64:
		return &qb.Cursor{Token: strconv.FormatInt(typed, 10)}, nil
	case float64:
		return &qb.Cursor{Token: strconv.FormatFloat(typed, 'f', -1, 64)}, nil
	case map[string]any:
		cursor := qb.Cursor{Values: make(map[string]any, len(typed))}
		for key, value := range typed {
			cursor.Values[key] = normalizeJSONNumber(value)
		}
		return &cursor, nil
	default:
		return nil, parseError(
			fmt.Errorf("expected string or object cursor, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
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
			return 0, parseError(
				fmt.Errorf("expected whole number, got %v", typed),
				qb.CodeInvalidValue,
				qb.WithPath(path),
			)
		}
		return int(typed), nil
	case float64:
		if math.Trunc(typed) != typed {
			return 0, parseError(
				fmt.Errorf("expected whole number, got %v", typed),
				qb.CodeInvalidValue,
				qb.WithPath(path),
			)
		}
		return int(typed), nil
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return int(n), nil
		}
		f, err := typed.Float64()
		if err != nil {
			return 0, parseError(err, qb.CodeInvalidValue, qb.WithPath(path))
		}
		if math.Trunc(f) != f {
			return 0, parseError(
				fmt.Errorf("expected whole number, got %v", typed),
				qb.CodeInvalidValue,
				qb.WithPath(path),
			)
		}
		return int(f), nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, parseError(err, qb.CodeInvalidValue, qb.WithPath(path))
		}
		return n, nil
	default:
		return 0, parseError(
			fmt.Errorf("expected integer, got %T", value),
			qb.CodeInvalidValue,
			qb.WithPath(path),
		)
	}
}

func decodeValue(field string, op qb.Operator, value any, opts options, path string) (any, error) {
	value = normalizeJSONNumber(value)
	if opts.valueDecoder == nil {
		return value, nil
	}

	decoded, err := opts.valueDecoder(field, op, value)
	if err != nil {
		return nil, parseError(
			err,
			qb.CodeInvalidValue,
			qb.WithPath(path),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}
	return decoded, nil
}

func resolveFilterField(field string, op qb.Operator, opts options, path string) (string, error) {
	if opts.filterFieldResolver == nil {
		return field, nil
	}

	resolvedField, err := opts.filterFieldResolver(field, op)
	if err != nil {
		return "", parseError(
			err,
			qb.CodeUnknownField,
			qb.WithPath(path),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}
	if resolvedField == "" {
		return "", parseError(
			fmt.Errorf("filter field resolver returned an empty field"),
			qb.CodeInvalidInput,
			qb.WithPath(path),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}
	return resolvedField, nil
}

func resolveSortField(field string, opts options) (string, error) {
	if opts.sortFieldResolver == nil {
		return field, nil
	}

	resolvedField, err := opts.sortFieldResolver(field)
	if err != nil {
		return "", parseError(
			err,
			qb.CodeUnknownField,
			qb.WithPath("sort"),
			qb.WithField(field),
		)
	}
	if resolvedField == "" {
		return "", parseError(
			fmt.Errorf("sort field resolver returned an empty field"),
			qb.CodeInvalidInput,
			qb.WithPath("sort"),
			qb.WithField(field),
		)
	}
	return resolvedField, nil
}

func decodeListFromNode(field string, op qb.Operator, node any, opts options, path string) ([]any, error) {
	values, err := asList(node, path)
	if err != nil {
		return nil, parseError(
			err,
			qb.CodeInvalidInput,
			qb.WithPath(path),
			qb.WithField(field),
			qb.WithOperator(op),
		)
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
		return nil, fmt.Errorf("expected list, got %T", value)
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
				return nil, parseError(
					fmt.Errorf("expected string, got %T", item),
					qb.CodeInvalidInput,
					qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
				)
			}
		}
		return out, nil
	default:
		return nil, parseError(
			fmt.Errorf("expected string or list, got %T", value),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
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

func pickExclusiveValue(input map[string]any, keys ...string) (string, any, bool, error) {
	var (
		foundKey string
		found    any
		ok       bool
	)

	for _, key := range keys {
		value, exists := pickValue(input, key)
		if !exists {
			continue
		}
		if ok {
			return "", nil, false, fmt.Errorf("only one of %s may be provided", strings.Join(keys, ", "))
		}
		foundKey = key
		found = value
		ok = true
	}

	return foundKey, found, ok, nil
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parseError(err error, code qb.ErrorCode, opts ...qb.ErrorOption) error {
	allOpts := make([]qb.ErrorOption, 0, len(opts)+2)
	allOpts = append(allOpts, qb.WithDefaultStage(qb.StageParse), qb.WithDefaultCode(code))
	allOpts = append(allOpts, opts...)
	return qb.WrapError(err, allOpts...)
}
