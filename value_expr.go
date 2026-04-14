package qb

import (
	"reflect"
	"strings"
)

// Scalar is a scalar expression used in predicates, projections, grouping, and
// sorting.
type Scalar interface {
	scalarNode()
}

// Ref references a field or column path.
type Ref struct {
	Name string
}

// Literal wraps a literal scalar value.
type Literal struct {
	Value any
}

// Call is a generic function call expression.
type Call struct {
	Name string
	Args []Scalar
}

func (Ref) scalarNode()     {}
func (Literal) scalarNode() {}
func (Call) scalarNode()    {}

// Operand is the right-hand side of a predicate.
type Operand interface {
	operandNode()
}

// ScalarOperand wraps a scalar right-hand expression.
type ScalarOperand struct {
	Expr Scalar
}

// ListOperand wraps a list right-hand expression such as IN (...).
type ListOperand struct {
	Items []Scalar
}

func (ScalarOperand) operandNode() {}
func (ListOperand) operandNode()   {}

// F references a field in a scalar expression.
func F(name string) Ref {
	return Ref{Name: name}
}

// Field is a backward-compatible alias for F.
func Field(name string) Ref {
	return F(name)
}

// V wraps a literal value in a scalar expression.
func V(value any) Literal {
	return Literal{Value: cloneLiteralValue(value)}
}

// Lit is an alias for V.
func Lit(value any) Literal {
	return V(value)
}

// Func constructs a generic function call expression.
func Func(name string, args ...any) Call {
	call := Call{Name: strings.TrimSpace(name)}
	if len(args) == 0 {
		return call
	}

	call.Args = make([]Scalar, len(args))
	for i, arg := range args {
		call.Args[i] = scalarFromAny(arg)
	}

	return call
}

// Lower constructs a LOWER(...) function call.
func Lower(arg any) Call {
	return Func("lower", arg)
}

// Upper constructs an UPPER(...) function call.
func Upper(arg any) Call {
	return Func("upper", arg)
}

// Trim constructs a TRIM(...) function call.
func Trim(arg any) Call {
	return Func("trim", arg)
}

// LTrim constructs an LTRIM(...) function call.
func LTrim(arg any) Call {
	return Func("ltrim", arg)
}

// RTrim constructs an RTRIM(...) function call.
func RTrim(arg any) Call {
	return Func("rtrim", arg)
}

// Length constructs a LENGTH(...) function call.
func Length(arg any) Call {
	return Func("length", arg)
}

// Count constructs a COUNT(...) function call. With no args, it compiles to COUNT(*).
func Count(args ...any) Call {
	return Func("count", args...)
}

// Sum constructs a SUM(...) function call.
func Sum(arg any) Call {
	return Func("sum", arg)
}

// Avg constructs an AVG(...) function call.
func Avg(arg any) Call {
	return Func("avg", arg)
}

// Min constructs a MIN(...) function call.
func Min(arg any) Call {
	return Func("min", arg)
}

// Max constructs a MAX(...) function call.
func Max(arg any) Call {
	return Func("max", arg)
}

// Concat constructs a CONCAT(...) function call.
func Concat(args ...any) Call {
	return Func("concat", args...)
}

// Substring constructs a SUBSTRING(...) function call.
func Substring(arg any, start any, length ...any) Call {
	args := make([]any, 0, 2+len(length))
	args = append(args, arg, start)
	args = append(args, length...)
	return Func("substring", args...)
}

// Replace constructs a REPLACE(...) function call.
func Replace(arg any, old any, new any) Call {
	return Func("replace", arg, old, new)
}

// Coalesce constructs a COALESCE(...) function call.
func Coalesce(args ...any) Call {
	return Func("coalesce", args...)
}

// NullIf constructs a NULLIF(...) function call.
func NullIf(left any, right any) Call {
	return Func("nullif", left, right)
}

// Abs constructs an ABS(...) function call.
func Abs(arg any) Call {
	return Func("abs", arg)
}

// Ceil constructs a CEIL(...) function call.
func Ceil(arg any) Call {
	return Func("ceil", arg)
}

// Floor constructs a FLOOR(...) function call.
func Floor(arg any) Call {
	return Func("floor", arg)
}

// Mod constructs a MOD(...) function call.
func Mod(left any, right any) Call {
	return Func("mod", left, right)
}

// Round constructs a ROUND(...) function call.
func Round(arg any, precision ...any) Call {
	args := make([]any, 0, 1+len(precision))
	args = append(args, arg)
	args = append(args, precision...)
	return Func("round", args...)
}

// Left constructs a LEFT(...) function call.
func Left(arg any, count any) Call {
	return Func("left", arg, count)
}

// Right constructs a RIGHT(...) function call.
func Right(arg any, count any) Call {
	return Func("right", arg, count)
}

// Date constructs a DATE(...) function call.
func Date(arg any) Call {
	return Func("date", arg)
}

// Now constructs a current-timestamp expression.
func Now() Call {
	return Func("now")
}

// CurrentDate constructs a CURRENT_DATE expression.
func CurrentDate() Call {
	return Func("current_date")
}

// LocalTime constructs a LOCALTIME expression.
func LocalTime() Call {
	return Func("localtime")
}

// CurrentTime constructs a CURRENT_TIME expression.
func CurrentTime() Call {
	return Func("current_time")
}

// LocalTimestamp constructs a LOCALTIMESTAMP expression.
func LocalTimestamp() Call {
	return Func("localtimestamp")
}

// CurrentTimestamp constructs a CURRENT_TIMESTAMP expression.
func CurrentTimestamp() Call {
	return Func("current_timestamp")
}

// DateTrunc constructs a date-truncation expression.
func DateTrunc(field any, source any) Call {
	return Func("date_trunc", field, source)
}

// Extract constructs a date-part extraction expression.
func Extract(field any, source any) Call {
	return Func("extract", field, source)
}

// DateBin constructs a DATE_BIN(...) expression.
func DateBin(stride any, source any, origin any) Call {
	return Func("date_bin", stride, source, origin)
}

// JsonExtract constructs a JSON extraction expression.
func JsonExtract(arg any, path any) Call {
	return Func("json_extract", arg, path)
}

// JsonQuery constructs a JSON-query expression.
func JsonQuery(arg any, path any) Call {
	return Func("json_query", arg, path)
}

// JsonValue constructs a scalar JSON value extraction expression.
func JsonValue(arg any, path any) Call {
	return Func("json_value", arg, path)
}

// JsonExists constructs a JSON path existence expression.
func JsonExists(arg any, path any) Call {
	return Func("json_exists", arg, path)
}

// JsonArrayLength constructs a JSON array-length expression.
func JsonArrayLength(arg any, path ...any) Call {
	args := make([]any, 0, 1+len(path))
	args = append(args, arg)
	args = append(args, path...)
	return Func("json_array_length", args...)
}

// JsonType constructs a JSON type expression.
func JsonType(arg any, path ...any) Call {
	args := make([]any, 0, 1+len(path))
	args = append(args, arg)
	args = append(args, path...)
	return Func("json_type", args...)
}

// JsonArray constructs a JSON array value.
func JsonArray(args ...any) Call {
	return Func("json_array", args...)
}

// JsonObject constructs a JSON object value from key/value pairs.
func JsonObject(args ...any) Call {
	return Func("json_object", args...)
}

func (r Ref) As(alias string) Projection     { return Project(r).As(alias) }
func (l Literal) As(alias string) Projection { return Project(l).As(alias) }
func (c Call) As(alias string) Projection    { return Project(c).As(alias) }

// AsScalar reports whether value is already a scalar expression.
func AsScalar(value any) (Scalar, bool) {
	switch typed := value.(type) {
	case Ref:
		return typed, true
	case Literal:
		return typed, true
	case Call:
		return typed, true
	default:
		return nil, false
	}
}

// CloneScalar returns a deep copy of a scalar expression.
func CloneScalar(expr Scalar) Scalar {
	switch typed := expr.(type) {
	case nil:
		return nil
	case Ref:
		return typed
	case Literal:
		return Literal{Value: cloneLiteralValue(typed.Value)}
	case Call:
		clone := Call{
			Name: typed.Name,
			Args: make([]Scalar, len(typed.Args)),
		}
		for i, arg := range typed.Args {
			clone.Args[i] = CloneScalar(arg)
		}
		return clone
	default:
		return typed
	}
}

// WalkScalar traverses a scalar expression in pre-order.
func WalkScalar(expr Scalar, visit func(Scalar) error) error {
	if expr == nil || visit == nil {
		return nil
	}

	if err := visit(expr); err != nil {
		return err
	}

	switch typed := expr.(type) {
	case Call:
		for _, arg := range typed.Args {
			if err := WalkScalar(arg, visit); err != nil {
				return err
			}
		}
	}

	return nil
}

// RewriteScalar rewrites a scalar expression tree.
func RewriteScalar(expr Scalar, rewriter func(Scalar) (Scalar, error)) (Scalar, error) {
	if expr == nil {
		return nil, nil
	}
	if rewriter == nil {
		return CloneScalar(expr), nil
	}

	switch typed := expr.(type) {
	case Ref, Literal:
		return rewriter(CloneScalar(typed))
	case Call:
		rewritten := Call{
			Name: typed.Name,
			Args: make([]Scalar, len(typed.Args)),
		}
		for i, arg := range typed.Args {
			child, err := RewriteScalar(arg, rewriter)
			if err != nil {
				return nil, err
			}
			rewritten.Args[i] = child
		}
		return rewriter(rewritten)
	default:
		return rewriter(expr)
	}
}

// CloneOperand returns a deep copy of a predicate operand.
func CloneOperand(operand Operand) Operand {
	switch typed := operand.(type) {
	case nil:
		return nil
	case ScalarOperand:
		return ScalarOperand{Expr: CloneScalar(typed.Expr)}
	case ListOperand:
		clone := ListOperand{Items: make([]Scalar, len(typed.Items))}
		for i, item := range typed.Items {
			clone.Items[i] = CloneScalar(item)
		}
		return clone
	default:
		return typed
	}
}

// RewriteOperand rewrites a predicate operand.
func RewriteOperand(operand Operand, rewriter func(Scalar) (Scalar, error)) (Operand, error) {
	switch typed := operand.(type) {
	case nil:
		return nil, nil
	case ScalarOperand:
		expr, err := RewriteScalar(typed.Expr, rewriter)
		if err != nil {
			return nil, err
		}
		return ScalarOperand{Expr: expr}, nil
	case ListOperand:
		rewritten := ListOperand{Items: make([]Scalar, len(typed.Items))}
		for i, item := range typed.Items {
			expr, err := RewriteScalar(item, rewriter)
			if err != nil {
				return nil, err
			}
			rewritten.Items[i] = expr
		}
		return rewritten, nil
	default:
		return typed, nil
	}
}

// SingleRef returns the field name when the expression contains exactly one
// field reference.
func SingleRef(expr Scalar) (string, bool) {
	var (
		field string
		count int
	)

	err := WalkScalar(expr, func(node Scalar) error {
		ref, ok := node.(Ref)
		if !ok {
			return nil
		}
		field = ref.Name
		count++
		return nil
	})
	if err != nil || count != 1 {
		return "", false
	}

	return field, true
}

// CloneValue returns a safe copy of a literal-like value. Scalar inputs are
// preserved as scalar expressions.
func CloneValue(value any) any {
	return cloneLiteralValue(value)
}

func (r Ref) Lower() Call             { return Lower(r) }
func (r Ref) Upper() Call             { return Upper(r) }
func (r Ref) Trim() Call              { return Trim(r) }
func (r Ref) LTrim() Call             { return LTrim(r) }
func (r Ref) RTrim() Call             { return RTrim(r) }
func (r Ref) Length() Call            { return Length(r) }
func (r Ref) Count() Call             { return Count(r) }
func (r Ref) Sum() Call               { return Sum(r) }
func (r Ref) Avg() Call               { return Avg(r) }
func (r Ref) Min() Call               { return Min(r) }
func (r Ref) Max() Call               { return Max(r) }
func (r Ref) Concat(args ...any) Call { return prependCallArg(r, Concat, args...) }
func (r Ref) Substring(start any, length ...any) Call {
	return Substring(r, start, length...)
}
func (r Ref) Replace(old any, new any) Call { return Replace(r, old, new) }
func (r Ref) Coalesce(args ...any) Call     { return prependCallArg(r, Coalesce, args...) }
func (r Ref) NullIf(other any) Call         { return NullIf(r, other) }
func (r Ref) Abs() Call                     { return Abs(r) }
func (r Ref) Ceil() Call                    { return Ceil(r) }
func (r Ref) Floor() Call                   { return Floor(r) }
func (r Ref) Mod(other any) Call            { return Mod(r, other) }
func (r Ref) Round(precision ...any) Call   { return Round(r, precision...) }
func (r Ref) Left(count any) Call           { return Left(r, count) }
func (r Ref) Right(count any) Call          { return Right(r, count) }
func (r Ref) Date() Call                    { return Date(r) }
func (r Ref) DateTrunc(field any) Call      { return DateTrunc(field, r) }
func (r Ref) Extract(field any) Call        { return Extract(field, r) }
func (r Ref) DateBin(stride any, origin any) Call {
	return DateBin(stride, r, origin)
}
func (r Ref) JsonExtract(path any) Call { return JsonExtract(r, path) }
func (r Ref) JsonQuery(path any) Call   { return JsonQuery(r, path) }
func (r Ref) JsonValue(path any) Call   { return JsonValue(r, path) }
func (r Ref) JsonExists(path any) Call  { return JsonExists(r, path) }
func (r Ref) JsonArrayLength(path ...any) Call {
	return JsonArrayLength(r, path...)
}
func (r Ref) JsonType(path ...any) Call   { return JsonType(r, path...) }
func (l Literal) Lower() Call             { return Lower(l) }
func (l Literal) Upper() Call             { return Upper(l) }
func (l Literal) Trim() Call              { return Trim(l) }
func (l Literal) LTrim() Call             { return LTrim(l) }
func (l Literal) RTrim() Call             { return RTrim(l) }
func (l Literal) Length() Call            { return Length(l) }
func (l Literal) Count() Call             { return Count(l) }
func (l Literal) Sum() Call               { return Sum(l) }
func (l Literal) Avg() Call               { return Avg(l) }
func (l Literal) Min() Call               { return Min(l) }
func (l Literal) Max() Call               { return Max(l) }
func (l Literal) Concat(args ...any) Call { return prependCallArg(l, Concat, args...) }
func (l Literal) Substring(start any, length ...any) Call {
	return Substring(l, start, length...)
}
func (l Literal) Replace(old any, new any) Call { return Replace(l, old, new) }
func (l Literal) Coalesce(args ...any) Call     { return prependCallArg(l, Coalesce, args...) }
func (l Literal) NullIf(other any) Call         { return NullIf(l, other) }
func (l Literal) Abs() Call                     { return Abs(l) }
func (l Literal) Ceil() Call                    { return Ceil(l) }
func (l Literal) Floor() Call                   { return Floor(l) }
func (l Literal) Mod(other any) Call            { return Mod(l, other) }
func (l Literal) Round(precision ...any) Call   { return Round(l, precision...) }
func (l Literal) Left(count any) Call           { return Left(l, count) }
func (l Literal) Right(count any) Call          { return Right(l, count) }
func (l Literal) Date() Call                    { return Date(l) }
func (l Literal) DateTrunc(field any) Call      { return DateTrunc(field, l) }
func (l Literal) Extract(field any) Call        { return Extract(field, l) }
func (l Literal) DateBin(stride any, origin any) Call {
	return DateBin(stride, l, origin)
}
func (l Literal) JsonExtract(path any) Call { return JsonExtract(l, path) }
func (l Literal) JsonQuery(path any) Call   { return JsonQuery(l, path) }
func (l Literal) JsonValue(path any) Call   { return JsonValue(l, path) }
func (l Literal) JsonExists(path any) Call  { return JsonExists(l, path) }
func (l Literal) JsonArrayLength(path ...any) Call {
	return JsonArrayLength(l, path...)
}
func (l Literal) JsonType(path ...any) Call { return JsonType(l, path...) }
func (c Call) Lower() Call                  { return Lower(c) }
func (c Call) Upper() Call                  { return Upper(c) }
func (c Call) Trim() Call                   { return Trim(c) }
func (c Call) LTrim() Call                  { return LTrim(c) }
func (c Call) RTrim() Call                  { return RTrim(c) }
func (c Call) Length() Call                 { return Length(c) }
func (c Call) Count() Call                  { return Count(c) }
func (c Call) Sum() Call                    { return Sum(c) }
func (c Call) Avg() Call                    { return Avg(c) }
func (c Call) Min() Call                    { return Min(c) }
func (c Call) Max() Call                    { return Max(c) }
func (c Call) Concat(args ...any) Call      { return prependCallArg(c, Concat, args...) }
func (c Call) Substring(start any, length ...any) Call {
	return Substring(c, start, length...)
}
func (c Call) Replace(old any, new any) Call { return Replace(c, old, new) }
func (c Call) Coalesce(args ...any) Call     { return prependCallArg(c, Coalesce, args...) }
func (c Call) NullIf(other any) Call         { return NullIf(c, other) }
func (c Call) Abs() Call                     { return Abs(c) }
func (c Call) Ceil() Call                    { return Ceil(c) }
func (c Call) Floor() Call                   { return Floor(c) }
func (c Call) Mod(other any) Call            { return Mod(c, other) }
func (c Call) Round(precision ...any) Call   { return Round(c, precision...) }
func (c Call) Left(count any) Call           { return Left(c, count) }
func (c Call) Right(count any) Call          { return Right(c, count) }
func (c Call) Date() Call                    { return Date(c) }
func (c Call) DateTrunc(field any) Call      { return DateTrunc(field, c) }
func (c Call) Extract(field any) Call        { return Extract(field, c) }
func (c Call) DateBin(stride any, origin any) Call {
	return DateBin(stride, c, origin)
}
func (c Call) JsonExtract(path any) Call { return JsonExtract(c, path) }
func (c Call) JsonQuery(path any) Call   { return JsonQuery(c, path) }
func (c Call) JsonValue(path any) Call   { return JsonValue(c, path) }
func (c Call) JsonExists(path any) Call  { return JsonExists(c, path) }
func (c Call) JsonArrayLength(path ...any) Call {
	return JsonArrayLength(c, path...)
}
func (c Call) JsonType(path ...any) Call { return JsonType(c, path...) }

func (r Ref) Eq(value any) Expr        { return compareScalar(r, OpEq, value) }
func (r Ref) Ne(value any) Expr        { return compareScalar(r, OpNe, value) }
func (r Ref) Gt(value any) Expr        { return compareScalar(r, OpGt, value) }
func (r Ref) Gte(value any) Expr       { return compareScalar(r, OpGte, value) }
func (r Ref) Lt(value any) Expr        { return compareScalar(r, OpLt, value) }
func (r Ref) Lte(value any) Expr       { return compareScalar(r, OpLte, value) }
func (r Ref) Like(value any) Expr      { return compareScalar(r, OpLike, value) }
func (r Ref) ILike(value any) Expr     { return compareScalar(r, OpILike, value) }
func (r Ref) Regexp(value any) Expr    { return compareScalar(r, OpRegexp, value) }
func (r Ref) Contains(value any) Expr  { return compareScalar(r, OpContains, value) }
func (r Ref) Prefix(value any) Expr    { return compareScalar(r, OpPrefix, value) }
func (r Ref) Suffix(value any) Expr    { return compareScalar(r, OpSuffix, value) }
func (r Ref) In(values ...any) Expr    { return compareList(r, OpIn, values...) }
func (r Ref) NotIn(values ...any) Expr { return compareList(r, OpNotIn, values...) }
func (r Ref) IsNull() Expr             { return Predicate{Left: CloneScalar(r), Op: OpIsNull} }
func (r Ref) NotNull() Expr            { return Predicate{Left: CloneScalar(r), Op: OpNotNull} }

func (c Call) Eq(value any) Expr        { return compareScalar(c, OpEq, value) }
func (c Call) Ne(value any) Expr        { return compareScalar(c, OpNe, value) }
func (c Call) Gt(value any) Expr        { return compareScalar(c, OpGt, value) }
func (c Call) Gte(value any) Expr       { return compareScalar(c, OpGte, value) }
func (c Call) Lt(value any) Expr        { return compareScalar(c, OpLt, value) }
func (c Call) Lte(value any) Expr       { return compareScalar(c, OpLte, value) }
func (c Call) Like(value any) Expr      { return compareScalar(c, OpLike, value) }
func (c Call) ILike(value any) Expr     { return compareScalar(c, OpILike, value) }
func (c Call) Regexp(value any) Expr    { return compareScalar(c, OpRegexp, value) }
func (c Call) Contains(value any) Expr  { return compareScalar(c, OpContains, value) }
func (c Call) Prefix(value any) Expr    { return compareScalar(c, OpPrefix, value) }
func (c Call) Suffix(value any) Expr    { return compareScalar(c, OpSuffix, value) }
func (c Call) In(values ...any) Expr    { return compareList(c, OpIn, values...) }
func (c Call) NotIn(values ...any) Expr { return compareList(c, OpNotIn, values...) }
func (c Call) IsNull() Expr             { return Predicate{Left: CloneScalar(c), Op: OpIsNull} }
func (c Call) NotNull() Expr            { return Predicate{Left: CloneScalar(c), Op: OpNotNull} }

func compareScalar(left Scalar, op Operator, value any) Expr {
	return Predicate{
		Left:  CloneScalar(left),
		Op:    op,
		Right: ScalarOperand{Expr: scalarFromAny(value)},
	}
}

func compareList(left Scalar, op Operator, values ...any) Expr {
	return Predicate{
		Left:  CloneScalar(left),
		Op:    op,
		Right: ListOperand{Items: flattenScalars(values)},
	}
}

// ILike builds a case-insensitive pattern-match predicate.
func ILike(left any, pattern any) Expr {
	return compareScalar(scalarFromAny(left), OpILike, pattern)
}

// Regexp builds a regular-expression predicate.
func Regexp(left any, pattern any) Expr {
	return compareScalar(scalarFromAny(left), OpRegexp, pattern)
}

func prependCallArg(first any, builder func(...any) Call, args ...any) Call {
	values := make([]any, 0, 1+len(args))
	values = append(values, first)
	values = append(values, args...)
	return builder(values...)
}

func scalarFromAny(value any) Scalar {
	if expr, ok := AsScalar(value); ok {
		return CloneScalar(expr)
	}
	return Literal{Value: cloneLiteralValue(value)}
}

func flattenScalars(values []any) []Scalar {
	if len(values) == 1 {
		if flattened, ok := anySlice(values[0]); ok {
			out := make([]Scalar, len(flattened))
			for i, item := range flattened {
				out[i] = scalarFromAny(item)
			}
			return out
		}
	}

	out := make([]Scalar, len(values))
	for i, value := range values {
		out[i] = scalarFromAny(value)
	}
	return out
}

func cloneLiteralValue(value any) any {
	if expr, ok := AsScalar(value); ok {
		return CloneScalar(expr)
	}

	if values, ok := anySlice(value); ok {
		cloned := make([]any, len(values))
		for i, item := range values {
			cloned[i] = cloneLiteralValue(item)
		}
		return cloned
	}

	return value
}

func anySlice(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}

	switch typed := value.(type) {
	case []any:
		return append([]any(nil), typed...), true
	case []string:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = item
		}
		return out, true
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}

	if rv.Type().Elem().Kind() == reflect.Uint8 {
		return nil, false
	}

	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = rv.Index(i).Interface()
	}

	return out, true
}
