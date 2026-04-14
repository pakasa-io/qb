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
