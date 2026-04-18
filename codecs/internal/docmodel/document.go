package docmodel

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pakasa-io/qb"
	codecconfig "github.com/pakasa-io/qb/codecs/config"
	"github.com/pakasa-io/qb/codecs/internal/dsl"
)

// Transport identifies the target codec transport.
type Transport string

const (
	TransportJSON        Transport = "json"
	TransportYAML        Transport = "yaml"
	TransportQueryString Transport = "querystring"
)

// Member preserves deterministic object ordering for transport serializers.
type Member struct {
	Key   string
	Value any
}

// OrderedObject preserves member order.
type OrderedObject []Member

// BuildDocument lowers a query into a transport-specific canonical or compact document.
func BuildDocument(query qb.Query, transport Transport, opts ...codecconfig.Option) (OrderedObject, error) {
	config := codecconfig.ApplyOptions(opts...)

	page, size, err := canonicalPageSize(query)
	if err != nil {
		return nil, err
	}

	document := OrderedObject{}

	if len(query.Projections) > 0 {
		value, err := encodeProjections(query.Projections, transport, config)
		if err != nil {
			return nil, err
		}
		document = append(document, Member{Key: "$select", Value: value})
	}

	if len(query.Includes) > 0 {
		document = append(document, Member{Key: "$include", Value: stringsToAny(query.Includes)})
	}

	if query.Filter != nil {
		where, err := encodeWhere(query.Filter, transport, config)
		if err != nil {
			return nil, err
		}
		document = append(document, Member{Key: "$where", Value: where})
	}

	if len(query.GroupBy) > 0 {
		value, err := encodeGroups(query.GroupBy, transport, config)
		if err != nil {
			return nil, err
		}
		document = append(document, Member{Key: "$group", Value: value})
	}

	if len(query.Sorts) > 0 {
		value, err := encodeSorts(query.Sorts, transport, config)
		if err != nil {
			return nil, err
		}
		document = append(document, Member{Key: "$sort", Value: value})
	}

	if page != nil {
		value, err := encodeTransportLeaf(*page, transport, config, false)
		if err != nil {
			return nil, err
		}
		document = append(document, Member{Key: "$page", Value: value})
	}
	if size != nil {
		value, err := encodeTransportLeaf(*size, transport, config, false)
		if err != nil {
			return nil, err
		}
		document = append(document, Member{Key: "$size", Value: value})
	}
	if query.Cursor != nil {
		value, err := encodeCursor(*query.Cursor, transport, config)
		if err != nil {
			return nil, err
		}
		document = append(document, Member{Key: "$cursor", Value: value})
	}

	return document, nil
}

func canonicalPageSize(query qb.Query) (*int, *int, error) {
	if query.Page != nil || query.Size != nil || query.Cursor != nil {
		if _, _, err := query.ResolvedPagination(); err != nil {
			return nil, nil, err
		}
		return query.Page, query.Size, nil
	}

	if query.Limit == nil && query.Offset == nil {
		return nil, nil, nil
	}
	if query.Limit == nil {
		return nil, nil, qb.NewError(
			fmt.Errorf("limit is required when offset is present"),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
	limit := *query.Limit
	if limit <= 0 {
		return nil, nil, qb.NewError(
			fmt.Errorf("limit must be greater than zero"),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
	offset := 0
	if query.Offset != nil {
		offset = *query.Offset
	}
	if offset%limit != 0 {
		return nil, nil, qb.NewError(
			fmt.Errorf("limit/offset cannot be losslessly mapped to page/size"),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
	page := (offset / limit) + 1
	size := limit
	return &page, &size, nil
}

func encodeProjections(values []qb.Projection, transport Transport, config codecconfig.Config) (any, error) {
	if config.Mode == codecconfig.Compact && projectionsAreSimple(values) {
		fields := make([]string, len(values))
		for i, projection := range values {
			fields[i] = projection.Expr.(qb.Ref).Name
		}
		return strings.Join(fields, ","), nil
	}

	out := make([]any, len(values))
	for i, projection := range values {
		item, err := encodeProjection(projection, transport, config)
		if err != nil {
			return nil, err
		}
		out[i] = item
	}
	return out, nil
}

func encodeProjection(value qb.Projection, transport Transport, config codecconfig.Config) (any, error) {
	text, err := dsl.FormatProjection(value, formatOptions(config, dsl.StandaloneContext))
	if err == nil {
		return text, nil
	}
	if err != dsl.ErrStructuredRequired {
		return nil, err
	}

	expr, err := encodeStructuredScalar(value.Expr, transport, config)
	if err != nil {
		return nil, err
	}
	object := OrderedObject{{Key: "$expr", Value: expr}}
	if value.Alias != "" {
		object = append(object, Member{Key: "$as", Value: value.Alias})
	}
	return object, nil
}

func encodeGroups(values []qb.Scalar, transport Transport, config codecconfig.Config) (any, error) {
	if config.Mode == codecconfig.Compact && scalarsAreSimpleFields(values) {
		fields := make([]string, len(values))
		for i, expr := range values {
			fields[i] = expr.(qb.Ref).Name
		}
		return strings.Join(fields, ","), nil
	}

	out := make([]any, len(values))
	for i, expr := range values {
		item, err := encodeScalar(expr, transport, config, dsl.StandaloneContext)
		if err != nil {
			return nil, err
		}
		out[i] = item
	}
	return out, nil
}

func encodeSorts(values []qb.Sort, transport Transport, config codecconfig.Config) (any, error) {
	if config.Mode == codecconfig.Compact && sortsAreSimple(values) {
		items := make([]string, len(values))
		for i, sortValue := range values {
			field := sortValue.Expr.(qb.Ref).Name
			if sortValue.Direction == qb.Desc {
				items[i] = "-" + field
			} else {
				items[i] = field
			}
		}
		return strings.Join(items, ","), nil
	}

	out := make([]any, len(values))
	for i, sortValue := range values {
		text, err := dsl.FormatSort(sortValue, formatOptions(config, dsl.StandaloneContext))
		if err == nil {
			out[i] = text
			continue
		}
		if err != dsl.ErrStructuredRequired {
			return nil, err
		}
		expr, err := encodeStructuredScalar(sortValue.Expr, transport, config)
		if err != nil {
			return nil, err
		}
		out[i] = OrderedObject{
			{Key: "$expr", Value: expr},
			{Key: "$dir", Value: string(defaultDirection(sortValue.Direction))},
		}
	}
	return out, nil
}

func encodeCursor(cursor qb.Cursor, transport Transport, config codecconfig.Config) (any, error) {
	if cursor.Token != "" {
		return encodeTransportLeaf(cursor.Token, transport, config, false)
	}
	keys := make([]string, 0, len(cursor.Values))
	for key := range cursor.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	object := OrderedObject{}
	for _, key := range keys {
		value, err := encodeTransportLeaf(cursor.Values[key], transport, config, true)
		if err != nil {
			return nil, err
		}
		object = append(object, Member{
			Key:   key,
			Value: value,
		})
	}
	return object, nil
}

func encodeWhere(expr qb.Expr, transport Transport, config codecconfig.Config) (OrderedObject, error) {
	raw, err := encodeWhereRaw(expr, transport, config)
	if err != nil {
		return nil, err
	}
	return orderWhereMap(raw), nil
}

func encodeWhereRaw(expr qb.Expr, transport Transport, config codecconfig.Config) (map[string]any, error) {
	switch typed := expr.(type) {
	case nil:
		return map[string]any{}, nil
	case qb.Predicate:
		return encodePredicateWhere(typed, transport, config)
	case qb.Negation:
		child, err := encodeWhereRaw(typed.Expr, transport, config)
		if err != nil {
			return nil, err
		}
		return map[string]any{"$not": child}, nil
	case qb.Group:
		if typed.Kind == qb.OrGroup {
			items := make([]any, len(typed.Terms))
			for i, term := range typed.Terms {
				child, err := encodeWhereRaw(term, transport, config)
				if err != nil {
					return nil, err
				}
				items[i] = orderWhereMap(child)
			}
			return map[string]any{"$or": items}, nil
		}
		return encodeAndWhere(typed.Terms, transport, config)
	default:
		return nil, fmt.Errorf("unsupported expr %T", expr)
	}
}

func encodeAndWhere(terms []qb.Expr, transport Transport, config codecconfig.Config) (map[string]any, error) {
	merged := map[string]any{}
	children := make([]map[string]any, 0, len(terms))
	for _, term := range terms {
		child, err := encodeWhereRaw(term, transport, config)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
		if !mergeWhereMap(merged, child) {
			items := make([]any, len(children))
			for i, item := range children {
				items[i] = orderWhereMap(item)
			}
			return map[string]any{"$and": items}, nil
		}
	}
	return merged, nil
}

func encodePredicateWhere(predicate qb.Predicate, transport Transport, config codecconfig.Config) (map[string]any, error) {
	if key, value, ok, err := encodeFieldPredicate(predicate, transport, config); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return map[string]any{key: value}, nil
	}
	exprValue, err := encodeExpressionPredicate(predicate, transport, config)
	if err != nil {
		return nil, err
	}
	return map[string]any{"$expr": exprValue}, nil
}

func encodeFieldPredicate(predicate qb.Predicate, transport Transport, config codecconfig.Config) (string, any, bool, error) {
	key, ok, err := encodeFilterKey(predicate.Left, config)
	if err != nil || !ok {
		return "", nil, ok, err
	}

	switch right := predicate.Right.(type) {
	case nil:
		if predicate.Op == qb.OpIsNull || predicate.Op == qb.OpNotNull {
			return key, map[string]any{operatorToken(predicate.Op, transport, true): true}, true, nil
		}
		return "", nil, false, nil
	case qb.ScalarOperand:
		literal, ok := right.Expr.(qb.Literal)
		if !ok {
			return "", nil, false, nil
		}
		if transport == TransportQueryString && predicate.Op == qb.OpEq && literal.Value == nil {
			return key, map[string]any{"$isnull": true}, true, nil
		}
		if transport == TransportQueryString && predicate.Op == qb.OpNe && literal.Value == nil {
			return key, map[string]any{"$notnull": true}, true, nil
		}
		value, err := encodeTransportLeaf(literal.Value, transport, config, true)
		if err != nil {
			return "", nil, true, err
		}
		switch predicate.Op {
		case qb.OpEq:
			return key, value, true, nil
		default:
			return key, map[string]any{operatorToken(predicate.Op, transport, false): value}, true, nil
		}
	case qb.ListOperand:
		items := make([]any, len(right.Items))
		for i, item := range right.Items {
			literal, ok := item.(qb.Literal)
			if !ok {
				return "", nil, false, nil
			}
			value, err := encodeTransportLeaf(literal.Value, transport, config, true)
			if err != nil {
				return "", nil, true, err
			}
			items[i] = value
		}
		return key, map[string]any{operatorToken(predicate.Op, transport, false): items}, true, nil
	default:
		return "", nil, false, nil
	}
}

func encodeExpressionPredicate(predicate qb.Predicate, transport Transport, config codecconfig.Config) (map[string]any, error) {
	op := operatorToken(predicate.Op, transport, true)
	switch right := predicate.Right.(type) {
	case nil:
		left, err := encodeScalar(predicate.Left, transport, config, dsl.MixedContext)
		if err != nil {
			return nil, err
		}
		return map[string]any{op: left}, nil
	case qb.ScalarOperand:
		items := make([]any, 2)
		left, err := encodeScalar(predicate.Left, transport, config, dsl.MixedContext)
		if err != nil {
			return nil, err
		}
		rightValue, err := encodeScalar(right.Expr, transport, config, dsl.MixedContext)
		if err != nil {
			return nil, err
		}
		if transport == TransportQueryString && predicate.Op == qb.OpEq {
			if literal, ok := right.Expr.(qb.Literal); ok && literal.Value == nil {
				return map[string]any{"$isnull": left}, nil
			}
		}
		if transport == TransportQueryString && predicate.Op == qb.OpNe {
			if literal, ok := right.Expr.(qb.Literal); ok && literal.Value == nil {
				return map[string]any{"$notnull": left}, nil
			}
		}
		items[0] = left
		items[1] = rightValue
		return map[string]any{op: items}, nil
	case qb.ListOperand:
		items := make([]any, 1, len(right.Items)+1)
		left, err := encodeScalar(predicate.Left, transport, config, dsl.MixedContext)
		if err != nil {
			return nil, err
		}
		items[0] = left
		for _, item := range right.Items {
			value, err := encodeScalar(item, transport, config, dsl.MixedContext)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		return map[string]any{op: items}, nil
	default:
		return nil, fmt.Errorf("unsupported operand %T", predicate.Right)
	}
}

func encodeFilterKey(expr qb.Scalar, config codecconfig.Config) (string, bool, error) {
	text, err := dsl.FormatScalar(expr, formatOptions(config, dsl.StandaloneContext))
	if err == nil {
		return text, true, nil
	}
	if err == dsl.ErrStructuredRequired {
		return "", false, nil
	}
	return "", false, err
}

func encodeScalar(expr qb.Scalar, transport Transport, config codecconfig.Config, ctx dsl.Context) (any, error) {
	text, err := dsl.FormatScalar(expr, formatOptions(config, ctx))
	if err == nil {
		return text, nil
	}
	if err != dsl.ErrStructuredRequired {
		return nil, err
	}
	return encodeStructuredScalar(expr, transport, config)
}

func encodeStructuredScalar(expr qb.Scalar, transport Transport, config codecconfig.Config) (any, error) {
	switch typed := expr.(type) {
	case qb.Ref:
		return OrderedObject{{Key: "$field", Value: typed.Name}}, nil
	case qb.Literal:
		literal, codec, err := encodeLiteralWrapper(typed.Value, transport, config)
		if err != nil {
			return nil, err
		}
		object := OrderedObject{{Key: "$literal", Value: literal}}
		if codec != "" {
			object = append(object, Member{Key: "$codec", Value: codec})
		}
		return object, nil
	case qb.Call:
		args := make([]any, len(typed.Args))
		for i, arg := range typed.Args {
			value, err := encodeStructuredScalar(arg, transport, config)
			if err != nil {
				return nil, err
			}
			args[i] = value
		}
		object := OrderedObject{{Key: "$call", Value: typed.Name}}
		if len(args) > 0 {
			object = append(object, Member{Key: "$args", Value: args})
		}
		return object, nil
	case qb.Cast:
		expr, err := encodeStructuredScalar(typed.Expr, transport, config)
		if err != nil {
			return nil, err
		}
		return OrderedObject{
			{Key: "$cast", Value: typed.Type},
			{Key: "$expr", Value: expr},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported scalar %T", expr)
	}
}

func encodeLiteralWrapper(value any, transport Transport, config codecconfig.Config) (any, string, error) {
	if config.LiteralCodec != nil {
		literal, codec, handled, err := config.LiteralCodec.FormatLiteral(value)
		if err != nil {
			return nil, "", err
		}
		if handled {
			if transport == TransportQueryString {
				return fmt.Sprint(literal), codec, nil
			}
			return literal, codec, nil
		}
	}
	if transport == TransportQueryString {
		return fmt.Sprint(value), "", nil
	}
	return value, "", nil
}

func encodeTransportLeaf(value any, transport Transport, config codecconfig.Config, preferWrapper bool) (any, error) {
	if transport == TransportQueryString {
		literal, _, err := encodeLiteralWrapper(value, transport, config)
		if err != nil {
			return nil, err
		}
		return fmt.Sprint(literal), nil
	}
	literal, codec, err := encodeLiteralWrapper(value, transport, config)
	if err != nil {
		return nil, err
	}
	if codec != "" && preferWrapper {
		return OrderedObject{
			{Key: "$literal", Value: literal},
			{Key: "$codec", Value: codec},
		}, nil
	}
	return literal, nil
}

func formatOptions(config codecconfig.Config, ctx dsl.Context) dsl.FormatOptions {
	mode := dsl.CanonicalMode
	if config.Mode == codecconfig.Compact {
		mode = dsl.CompactMode
	}
	return dsl.FormatOptions{
		Context: ctx,
		Mode:    mode,
		LiteralFormatter: func(value any) (string, string, bool, error) {
			if config.LiteralCodec == nil {
				return "", "", false, nil
			}
			literal, codec, handled, err := config.LiteralCodec.FormatLiteral(value)
			if err != nil || !handled {
				return "", "", handled, err
			}
			return fmt.Sprint(literal), codec, true, nil
		},
	}
}

func mergeWhereMap(dst map[string]any, src map[string]any) bool {
	for key, value := range src {
		existing, ok := dst[key]
		if !ok {
			dst[key] = value
			continue
		}
		merged, ok := mergeWhereValue(key, existing, value)
		if !ok {
			return false
		}
		dst[key] = merged
	}
	return true
}

func mergeWhereValue(key string, existing any, incoming any) (any, bool) {
	if key == "$expr" {
		left, ok1 := existing.(map[string]any)
		right, ok2 := incoming.(map[string]any)
		if !ok1 || !ok2 {
			return nil, false
		}
		merged := map[string]any{}
		for op, value := range left {
			merged[op] = value
		}
		for op, value := range right {
			if _, exists := merged[op]; exists {
				return nil, false
			}
			merged[op] = value
		}
		return merged, true
	}
	if strings.HasPrefix(key, "$") {
		return nil, false
	}
	left := normalizePredicateValue(existing)
	right := normalizePredicateValue(incoming)
	merged := map[string]any{}
	for op, value := range left {
		merged[op] = value
	}
	for op, value := range right {
		if _, exists := merged[op]; exists {
			return nil, false
		}
		merged[op] = value
	}
	if eq, ok := merged["$eq"]; ok && len(merged) == 1 {
		return eq, true
	}
	return merged, true
}

func normalizePredicateValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{"$eq": value}
}

func orderWhereMap(values map[string]any) OrderedObject {
	simple := make([]string, 0)
	computed := make([]string, 0)
	for key := range values {
		if strings.HasPrefix(key, "$") {
			continue
		}
		if isSimpleFieldRef(key) {
			simple = append(simple, key)
		} else {
			computed = append(computed, key)
		}
	}
	sort.Strings(simple)
	sort.Strings(computed)

	object := OrderedObject{}
	for _, key := range simple {
		object = append(object, Member{Key: key, Value: orderValue(values[key], false)})
	}
	for _, key := range computed {
		object = append(object, Member{Key: key, Value: orderValue(values[key], false)})
	}
	for _, key := range []string{"$expr", "$not", "$or", "$and"} {
		if value, ok := values[key]; ok {
			object = append(object, Member{Key: key, Value: orderValue(value, key == "$or" || key == "$and" || key == "$not")})
		}
	}
	return object
}

func orderValue(value any, whereChild bool) any {
	switch typed := value.(type) {
	case OrderedObject:
		return typed
	case map[string]any:
		if whereChild {
			return orderWhereMap(typed)
		}
		return orderGenericMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			if whereObject, ok := item.(map[string]any); ok {
				out[i] = orderWhereMap(whereObject)
				continue
			}
			out[i] = orderValue(item, false)
		}
		return out
	default:
		return value
	}
}

func orderGenericMap(values map[string]any) OrderedObject {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	object := OrderedObject{}
	for _, key := range keys {
		object = append(object, Member{Key: key, Value: orderValue(values[key], false)})
	}
	return object
}

func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func projectionsAreSimple(values []qb.Projection) bool {
	for _, value := range values {
		if value.Alias != "" {
			return false
		}
		ref, ok := value.Expr.(qb.Ref)
		if !ok || !isSimpleFieldRef(ref.Name) {
			return false
		}
	}
	return len(values) > 0
}

func scalarsAreSimpleFields(values []qb.Scalar) bool {
	for _, value := range values {
		ref, ok := value.(qb.Ref)
		if !ok || !isSimpleFieldRef(ref.Name) {
			return false
		}
	}
	return len(values) > 0
}

func sortsAreSimple(values []qb.Sort) bool {
	for _, value := range values {
		ref, ok := value.Expr.(qb.Ref)
		if !ok || !isSimpleFieldRef(ref.Name) {
			return false
		}
		if defaultDirection(value.Direction) != qb.Asc && defaultDirection(value.Direction) != qb.Desc {
			return false
		}
	}
	return len(values) > 0
}

func defaultDirection(direction qb.Direction) qb.Direction {
	if direction == "" {
		return qb.Asc
	}
	return direction
}

func operatorToken(op qb.Operator, transport Transport, allowNullRewrite bool) string {
	switch op {
	case qb.OpEq:
		return "$eq"
	case qb.OpNe:
		return "$ne"
	case qb.OpGt:
		return "$gt"
	case qb.OpGte:
		return "$gte"
	case qb.OpLt:
		return "$lt"
	case qb.OpLte:
		return "$lte"
	case qb.OpIn:
		return "$in"
	case qb.OpNotIn:
		return "$nin"
	case qb.OpLike:
		return "$like"
	case qb.OpILike:
		return "$ilike"
	case qb.OpRegexp:
		return "$regexp"
	case qb.OpContains:
		return "$contains"
	case qb.OpPrefix:
		return "$prefix"
	case qb.OpSuffix:
		return "$suffix"
	case qb.OpIsNull:
		return "$isnull"
	case qb.OpNotNull:
		return "$notnull"
	default:
		return "$" + string(op)
	}
}
