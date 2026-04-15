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

// GroupFieldResolver canonicalizes and validates grouping fields.
type GroupFieldResolver func(field string) (string, error)

// SortFieldResolver canonicalizes and validates sort fields.
type SortFieldResolver func(field string) (string, error)

type options struct {
	valueDecoder        ValueDecoder
	filterFieldResolver FilterFieldResolver
	groupFieldResolver  GroupFieldResolver
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

// WithGroupFieldResolver sets a hook for canonicalizing grouping fields.
func WithGroupFieldResolver(resolver GroupFieldResolver) Option {
	return func(opts *options) {
		opts.groupFieldResolver = resolver
	}
}

// WithSortFieldResolver sets a hook for canonicalizing sort fields.
func WithSortFieldResolver(resolver SortFieldResolver) Option {
	return func(opts *options) {
		opts.sortFieldResolver = resolver
	}
}

var allowedTopLevelKeys = map[string]struct{}{
	"$select":  {},
	"$include": {},
	"$where":   {},
	"$group":   {},
	"$sort":    {},
	"$page":    {},
	"$size":    {},
	"$cursor":  {},
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

// Parse converts a normalized object into a query using the compact `$...`
// envelope and scalar DSL.
func Parse(input map[string]any, opts ...Option) (qb.Query, error) {
	config := options{}
	for _, opt := range opts {
		opt(&config)
	}

	document := normalizeObject(input)
	if err := validateTopLevel(document); err != nil {
		return qb.Query{}, err
	}

	var query qb.Query

	if raw, ok := pickValue(document, "$select"); ok {
		projections, err := parseSelect(raw, config, "$select")
		if err != nil {
			return qb.Query{}, err
		}
		query.Projections = projections
	}

	if raw, ok := pickValue(document, "$include"); ok {
		includes, err := parseIncludes(raw, "$include")
		if err != nil {
			return qb.Query{}, err
		}
		query.Includes = includes
	}

	if raw, ok := pickValue(document, "$group"); ok {
		groupBy, err := parseGroup(raw, config, "$group")
		if err != nil {
			return qb.Query{}, err
		}
		query.GroupBy = groupBy
	}

	if raw, ok := pickValue(document, "$where"); ok {
		where, ok := raw.(map[string]any)
		if !ok {
			return qb.Query{}, parseError(
				fmt.Errorf("expected object"),
				qb.CodeInvalidInput,
				qb.WithPath("$where"),
			)
		}
		filter, err := parseWhere(where, config, "$where")
		if err != nil {
			return qb.Query{}, err
		}
		query.Filter = filter
	}

	if raw, ok := pickValue(document, "$sort"); ok {
		sorts, err := parseSorts(raw, config, "$sort")
		if err != nil {
			return qb.Query{}, err
		}
		query.Sorts = sorts
	}

	if raw, ok := pickValue(document, "$page"); ok {
		page, err := parseInteger(raw, "$page")
		if err != nil {
			return qb.Query{}, err
		}
		if page < 1 {
			return qb.Query{}, parseError(
				fmt.Errorf("page must be greater than or equal to 1"),
				qb.CodeInvalidValue,
				qb.WithPath("$page"),
			)
		}
		query.Page = &page
	}

	if raw, ok := pickValue(document, "$size"); ok {
		size, err := parseInteger(raw, "$size")
		if err != nil {
			return qb.Query{}, err
		}
		if size < 1 {
			return qb.Query{}, parseError(
				fmt.Errorf("size must be greater than or equal to 1"),
				qb.CodeInvalidValue,
				qb.WithPath("$size"),
			)
		}
		query.Size = &size
	}

	if raw, ok := pickValue(document, "$cursor"); ok {
		cursor, err := parseCursor(raw, "$cursor")
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

func validateTopLevel(input map[string]any) error {
	for _, key := range sortedKeys(input) {
		if _, ok := allowedTopLevelKeys[key]; ok {
			continue
		}
		return parseError(
			fmt.Errorf("unknown top-level key %q", key),
			qb.CodeInvalidInput,
			qb.WithPath(key),
		)
	}
	return nil
}

func parseSelect(node any, opts options, path string) ([]qb.Projection, error) {
	switch typed := node.(type) {
	case string:
		fields, err := parseSimpleFieldList(typed, path)
		if err != nil {
			return nil, err
		}
		out := make([]qb.Projection, len(fields))
		for i, field := range fields {
			out[i] = qb.Project(qb.F(field))
		}
		return out, nil
	case []any:
		out := make([]qb.Projection, 0, len(typed))
		for i, item := range typed {
			value, ok := item.(string)
			if !ok {
				return nil, parseError(
					fmt.Errorf("expected string, got %T", item),
					qb.CodeInvalidInput,
					qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
				)
			}
			expr, alias, err := parseStandaloneScalar(value, true)
			if err != nil {
				return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(fmt.Sprintf("%s[%d]", path, i)))
			}
			projection := qb.Project(expr)
			if alias != "" {
				projection = projection.As(alias)
			}
			out = append(out, projection)
		}
		return out, nil
	default:
		return nil, parseError(
			fmt.Errorf("expected string or list, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
}

func parseIncludes(node any, path string) ([]string, error) {
	return parseDelimitedStrings(node, path)
}

func parseGroup(node any, opts options, path string) ([]qb.Scalar, error) {
	switch typed := node.(type) {
	case string:
		fields, err := parseSimpleFieldList(typed, path)
		if err != nil {
			return nil, err
		}
		out := make([]qb.Scalar, len(fields))
		for i, field := range fields {
			out[i] = qb.F(field)
		}
		return resolveGroupScalars(out, opts, path)
	case []any:
		out := make([]qb.Scalar, 0, len(typed))
		for i, item := range typed {
			value, ok := item.(string)
			if !ok {
				return nil, parseError(
					fmt.Errorf("expected string, got %T", item),
					qb.CodeInvalidInput,
					qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
				)
			}
			expr, alias, err := parseStandaloneScalar(value, false)
			if err != nil {
				return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(fmt.Sprintf("%s[%d]", path, i)))
			}
			if alias != "" {
				return nil, parseError(
					fmt.Errorf("aliases are not allowed in $group"),
					qb.CodeInvalidInput,
					qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
				)
			}
			out = append(out, expr)
		}
		return resolveGroupScalars(out, opts, path)
	default:
		return nil, parseError(
			fmt.Errorf("expected string or list, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
}

func parseSorts(node any, opts options, path string) ([]qb.Sort, error) {
	switch typed := node.(type) {
	case string:
		items := splitCSV(typed)
		sorts := make([]qb.Sort, 0, len(items))
		for i, item := range items {
			if item == "" {
				continue
			}
			direction := qb.Asc
			field := item
			if strings.HasPrefix(field, "-") {
				direction = qb.Desc
				field = strings.TrimPrefix(field, "-")
			}
			if strings.HasPrefix(field, "+") {
				return nil, parseError(
					fmt.Errorf("simple $sort shorthand does not support +field"),
					qb.CodeInvalidInput,
					qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
				)
			}
			if !isSimpleFieldRef(field) {
				return nil, parseError(
					fmt.Errorf("expression-bearing sorts must use arrays"),
					qb.CodeInvalidInput,
					qb.WithPath(path),
				)
			}
			expr, err := resolveSortScalar(qb.F(field), opts, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			sorts = append(sorts, qb.Sort{Expr: expr, Direction: direction})
		}
		return sorts, nil
	case []any:
		sorts := make([]qb.Sort, 0, len(typed))
		for i, item := range typed {
			value, ok := item.(string)
			if !ok {
				return nil, parseError(
					fmt.Errorf("expected string, got %T", item),
					qb.CodeInvalidInput,
					qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
				)
			}
			expr, direction, err := parseSortItem(value)
			if err != nil {
				return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(fmt.Sprintf("%s[%d]", path, i)))
			}
			expr, err = resolveSortScalar(expr, opts, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			sorts = append(sorts, qb.Sort{Expr: expr, Direction: direction})
		}
		return sorts, nil
	default:
		return nil, parseError(
			fmt.Errorf("expected string or list, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
}

func parseWhere(node map[string]any, opts options, path string) (qb.Expr, error) {
	keys := sortedKeys(node)
	exprs := make([]qb.Expr, 0, len(keys))
	for _, key := range keys {
		value := node[key]
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
			child, ok := value.(map[string]any)
			if !ok {
				return nil, parseError(
					fmt.Errorf("expected object"),
					qb.CodeInvalidInput,
					qb.WithPath(path+".$not"),
				)
			}
			expr, err := parseWhere(child, opts, path+".$not")
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, qb.Not(expr))
		case "$expr":
			expr, err := parseExpressionPredicates(value, opts, path+".$expr")
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, expr)
		default:
			expr, err := parseComputedFilter(key, value, opts, path+"."+key)
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
		object, ok := item.(map[string]any)
		if !ok {
			return nil, parseError(
				fmt.Errorf("expected object, got %T", item),
				qb.CodeInvalidInput,
				qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
			)
		}
		expr, err := parseWhere(object, opts, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
	}

	return combine(exprs...), nil
}

func parseComputedFilter(key string, node any, opts options, path string) (qb.Expr, error) {
	left, alias, err := parseStandaloneScalar(key, false)
	if err != nil {
		return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(path))
	}
	if alias != "" {
		return nil, parseError(
			fmt.Errorf("aliases are not allowed inside $where"),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}

	switch typed := node.(type) {
	case map[string]any:
		keys := sortedKeys(typed)
		exprs := make([]qb.Expr, 0, len(keys))
		for _, key := range keys {
			op, err := parseOperatorName(key, path+"."+key)
			if err != nil {
				return nil, err
			}
			expr, err := parsePredicate(left, op, typed[key], opts, path+"."+key)
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, expr)
		}
		return qb.And(exprs...), nil
	case []any:
		return parsePredicate(left, qb.OpIn, typed, opts, path)
	default:
		return parsePredicate(left, qb.OpEq, typed, opts, path)
	}
}

func parsePredicate(left qb.Scalar, op qb.Operator, node any, opts options, path string) (qb.Expr, error) {
	resolvedLeft, err := resolveFilterScalar(left, op, opts, path)
	if err != nil {
		return nil, err
	}

	field := predicatePrimaryField(resolvedLeft)
	switch op {
	case qb.OpIsNull, qb.OpNotNull:
		if err := validateUnaryPredicateOperand(node, path); err != nil {
			return nil, err
		}
		return qb.Predicate{Left: resolvedLeft, Op: op}, nil
	case qb.OpIn, qb.OpNotIn:
		values, err := asList(node, path)
		if err != nil {
			return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(path))
		}
		items := make([]qb.Scalar, len(values))
		for i, value := range values {
			decoded, err := decodeValue(field, op, value, opts, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			items[i] = qb.V(decoded)
		}
		return qb.Predicate{
			Left:  resolvedLeft,
			Op:    op,
			Right: qb.ListOperand{Items: items},
		}, nil
	default:
		decoded, err := decodeValue(field, op, node, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Predicate{
			Left:  resolvedLeft,
			Op:    op,
			Right: qb.ScalarOperand{Expr: qb.V(decoded)},
		}, nil
	}
}

func parseExpressionPredicates(node any, opts options, path string) (qb.Expr, error) {
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
		op, err := parseOperatorName(key, path+"."+key)
		if err != nil {
			return nil, err
		}
		expr, err := parseExpressionPredicate(op, object[key], opts, path+"."+key)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
	}
	return qb.And(exprs...), nil
}

func parseExpressionPredicate(op qb.Operator, node any, opts options, path string) (qb.Expr, error) {
	if op == qb.OpIsNull || op == qb.OpNotNull {
		operand := node
		if values, ok := node.([]any); ok {
			if len(values) != 1 {
				return nil, parseError(
					fmt.Errorf("expected exactly one operand"),
					qb.CodeInvalidInput,
					qb.WithPath(path),
				)
			}
			operand = values[0]
		}

		left, err := parseMixedScalar(operand, "", op, opts, path)
		if err != nil {
			return nil, err
		}
		left, err = resolveFilterScalar(left, op, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.Predicate{Left: left, Op: op}, nil
	}

	values, err := asList(node, path)
	if err != nil {
		return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(path))
	}
	if len(values) == 0 {
		return nil, parseError(
			fmt.Errorf("expected at least one operand"),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}

	left, err := parseMixedScalar(values[0], "", op, opts, path+"[0]")
	if err != nil {
		return nil, err
	}
	left, err = resolveFilterScalar(left, op, opts, path+"[0]")
	if err != nil {
		return nil, err
	}

	field := predicatePrimaryField(left)
	switch op {
	case qb.OpIn, qb.OpNotIn:
		rhs := values[1:]
		if len(rhs) == 1 {
			if list, ok := rhs[0].([]any); ok {
				rhs = list
			}
		}
		if len(rhs) == 0 {
			return nil, parseError(
				fmt.Errorf("expected one or more right-hand operands"),
				qb.CodeInvalidInput,
				qb.WithPath(path),
			)
		}
		items := make([]qb.Scalar, len(rhs))
		for i, value := range rhs {
			item, err := parseMixedScalar(value, field, op, opts, fmt.Sprintf("%s[%d]", path, i+1))
			if err != nil {
				return nil, err
			}
			if item, err = resolveFilterScalar(item, op, opts, fmt.Sprintf("%s[%d]", path, i+1)); err != nil {
				return nil, err
			}
			items[i] = item
		}
		return qb.Predicate{
			Left:  left,
			Op:    op,
			Right: qb.ListOperand{Items: items},
		}, nil
	default:
		if len(values) != 2 {
			return nil, parseError(
				fmt.Errorf("expected exactly two operands"),
				qb.CodeInvalidInput,
				qb.WithPath(path),
			)
		}
		right, err := parseMixedScalar(values[1], field, op, opts, path+"[1]")
		if err != nil {
			return nil, err
		}
		if right, err = resolveFilterScalar(right, op, opts, path+"[1]"); err != nil {
			return nil, err
		}
		return qb.Predicate{
			Left:  left,
			Op:    op,
			Right: qb.ScalarOperand{Expr: right},
		}, nil
	}
}

func parseMixedScalar(node any, field string, op qb.Operator, opts options, path string) (qb.Scalar, error) {
	node = normalizeValue(node)
	if expr, ok := qb.AsScalar(node); ok {
		return qb.CloneScalar(expr), nil
	}

	if value, ok := node.(string); ok {
		trimmed := strings.TrimSpace(value)
		if isMixedDSLString(trimmed) {
			expr, alias, err := parseStandaloneScalar(trimmed, false)
			if err != nil {
				return nil, parseError(err, qb.CodeInvalidInput, qb.WithPath(path))
			}
			if alias != "" {
				return nil, parseError(
					fmt.Errorf("aliases are not allowed in mixed expression contexts"),
					qb.CodeInvalidInput,
					qb.WithPath(path),
				)
			}
			return expr, nil
		}
		decoded, err := decodeValue(field, op, value, opts, path)
		if err != nil {
			return nil, err
		}
		return qb.V(decoded), nil
	}

	decoded, err := decodeValue(field, op, node, opts, path)
	if err != nil {
		return nil, err
	}
	return qb.V(decoded), nil
}

func parseOperatorName(name, path string) (qb.Operator, error) {
	switch name {
	case "$eq":
		return qb.OpEq, nil
	case "$ne":
		return qb.OpNe, nil
	case "$gt":
		return qb.OpGt, nil
	case "$gte":
		return qb.OpGte, nil
	case "$lt":
		return qb.OpLt, nil
	case "$lte":
		return qb.OpLte, nil
	case "$in":
		return qb.OpIn, nil
	case "$nin":
		return qb.OpNotIn, nil
	case "$like":
		return qb.OpLike, nil
	case "$ilike":
		return qb.OpILike, nil
	case "$regexp":
		return qb.OpRegexp, nil
	case "$contains":
		return qb.OpContains, nil
	case "$prefix":
		return qb.OpPrefix, nil
	case "$suffix":
		return qb.OpSuffix, nil
	case "$isnull":
		return qb.OpIsNull, nil
	case "$notnull":
		return qb.OpNotNull, nil
	default:
		return "", parseError(
			fmt.Errorf("unsupported operator %q", name),
			qb.CodeUnsupportedOperator,
			qb.WithPath(path),
		)
	}
}

func resolveFilterScalar(expr qb.Scalar, op qb.Operator, opts options, path string) (qb.Scalar, error) {
	if opts.filterFieldResolver == nil {
		return expr, nil
	}
	return qb.RewriteScalar(expr, func(node qb.Scalar) (qb.Scalar, error) {
		ref, ok := node.(qb.Ref)
		if !ok {
			return node, nil
		}
		resolved, err := opts.filterFieldResolver(ref.Name, op)
		if err != nil {
			return nil, parseError(
				err,
				qb.CodeUnknownField,
				qb.WithPath(path),
				qb.WithField(ref.Name),
				qb.WithOperator(op),
			)
		}
		if strings.TrimSpace(resolved) == "" {
			return nil, parseError(
				fmt.Errorf("filter field resolver returned an empty field"),
				qb.CodeInvalidInput,
				qb.WithPath(path),
				qb.WithField(ref.Name),
				qb.WithOperator(op),
			)
		}
		return qb.F(resolved), nil
	})
}

func resolveGroupScalar(expr qb.Scalar, opts options, path string) (qb.Scalar, error) {
	if opts.groupFieldResolver == nil {
		return expr, nil
	}
	return qb.RewriteScalar(expr, func(node qb.Scalar) (qb.Scalar, error) {
		ref, ok := node.(qb.Ref)
		if !ok {
			return node, nil
		}
		resolved, err := opts.groupFieldResolver(ref.Name)
		if err != nil {
			return nil, parseError(
				err,
				qb.CodeUnknownField,
				qb.WithPath(path),
				qb.WithField(ref.Name),
			)
		}
		if strings.TrimSpace(resolved) == "" {
			return nil, parseError(
				fmt.Errorf("group field resolver returned an empty field"),
				qb.CodeInvalidInput,
				qb.WithPath(path),
				qb.WithField(ref.Name),
			)
		}
		return qb.F(resolved), nil
	})
}

func resolveGroupScalars(values []qb.Scalar, opts options, path string) ([]qb.Scalar, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]qb.Scalar, len(values))
	for i, expr := range values {
		resolved, err := resolveGroupScalar(expr, opts, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		out[i] = resolved
	}
	return out, nil
}

func resolveSortScalar(expr qb.Scalar, opts options, path string) (qb.Scalar, error) {
	if opts.sortFieldResolver == nil {
		return expr, nil
	}
	return qb.RewriteScalar(expr, func(node qb.Scalar) (qb.Scalar, error) {
		ref, ok := node.(qb.Ref)
		if !ok {
			return node, nil
		}
		resolved, err := opts.sortFieldResolver(ref.Name)
		if err != nil {
			return nil, parseError(
				err,
				qb.CodeUnknownField,
				qb.WithPath(path),
				qb.WithField(ref.Name),
			)
		}
		if strings.TrimSpace(resolved) == "" {
			return nil, parseError(
				fmt.Errorf("sort field resolver returned an empty field"),
				qb.CodeInvalidInput,
				qb.WithPath(path),
				qb.WithField(ref.Name),
			)
		}
		return qb.F(resolved), nil
	})
}

func resolveSortScalars(values []qb.Scalar, opts options, path string) ([]qb.Scalar, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]qb.Scalar, len(values))
	for i, expr := range values {
		resolved, err := resolveSortScalar(expr, opts, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		out[i] = resolved
	}
	return out, nil
}

func parseSimpleFieldList(value string, path string) ([]string, error) {
	items := splitCSV(value)
	out := make([]string, 0, len(items))
	for i, item := range items {
		if !isSimpleFieldRef(item) {
			return nil, parseError(
				fmt.Errorf("simple list shorthand only supports field references"),
				qb.CodeInvalidInput,
				qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
			)
		}
		out = append(out, item)
	}
	return out, nil
}

func parseDelimitedStrings(node any, path string) ([]string, error) {
	switch typed := node.(type) {
	case string:
		return splitCSV(typed), nil
	case []any:
		out := make([]string, 0, len(typed))
		for i, item := range typed {
			value, ok := item.(string)
			if !ok {
				return nil, parseError(
					fmt.Errorf("expected string, got %T", item),
					qb.CodeInvalidInput,
					qb.WithPath(fmt.Sprintf("%s[%d]", path, i)),
				)
			}
			out = append(out, splitCSV(value)...)
		}
		return out, nil
	default:
		return nil, parseError(
			fmt.Errorf("expected string or list, got %T", node),
			qb.CodeInvalidInput,
			qb.WithPath(path),
		)
	}
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
	case map[string]any:
		cursor := qb.Cursor{Values: make(map[string]any, len(typed))}
		for key, value := range typed {
			cursor.Values[key] = normalizeValue(value)
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

func predicatePrimaryField(expr qb.Scalar) string {
	field, ok := qb.SingleRef(expr)
	if !ok {
		return ""
	}
	return field
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func isSimpleFieldRef(value string) bool {
	if value == "" {
		return false
	}
	for _, part := range strings.Split(value, ".") {
		if part == "" {
			return false
		}
		for i, r := range part {
			if i == 0 {
				if !isIdentStart(r) {
					return false
				}
				continue
			}
			if !isIdentContinue(r) {
				return false
			}
		}
	}
	return true
}

func isMixedDSLString(value string) bool {
	return strings.Contains(value, "@") || strings.Contains(value, "(") || strings.Contains(value, "::") || strings.Contains(value, "'")
}

func validateUnaryPredicateOperand(value any, path string) error {
	switch typed := normalizeValue(value).(type) {
	case bool:
		if typed {
			return nil
		}
	case string:
		if strings.EqualFold(strings.TrimSpace(typed), "true") {
			return nil
		}
	}
	return parseError(
		fmt.Errorf("unary null operators require true as the operand"),
		qb.CodeInvalidInput,
		qb.WithPath(path),
	)
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
	value = normalizeValue(value)
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

func asList(value any, path string) ([]any, error) {
	switch typed := value.(type) {
	case []any:
		return typed, nil
	case string:
		items := splitCSV(typed)
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected list, got %T", value)
	}
}

func normalizeObject(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = normalizeValue(value)
	}
	return out
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		return normalizeJSONNumber(typed)
	case map[string]any:
		return normalizeObject(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = normalizeValue(item)
		}
		return out
	default:
		return value
	}
}

func normalizeJSONNumber(value json.Number) any {
	if integer, err := value.Int64(); err == nil {
		return integer
	}
	if floatValue, err := value.Float64(); err == nil {
		return floatValue
	}
	return value
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

func parseError(err error, code qb.ErrorCode, opts ...qb.ErrorOption) error {
	allOpts := make([]qb.ErrorOption, 0, len(opts)+2)
	allOpts = append(allOpts, qb.WithDefaultStage(qb.StageParse), qb.WithDefaultCode(code))
	allOpts = append(allOpts, opts...)
	return qb.WrapError(err, allOpts...)
}
