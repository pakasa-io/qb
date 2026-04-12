package qb

import "strings"

// ValueExpr is a scalar expression used in predicates, select lists, and
// grouping expressions.
type ValueExpr interface {
	valueExprNode()
}

// Literal wraps a scalar literal value.
type Literal struct {
	Value any
}

func (Literal) valueExprNode() {}

// CallExpr represents a function call.
type CallExpr struct {
	Name string
	Args []ValueExpr
}

func (CallExpr) valueExprNode() {}

func (Ref) valueExprNode() {}

// Lit wraps a literal value so it can participate in expression trees.
func Lit(value any) Literal {
	return Literal{Value: value}
}

// Call constructs a generic function call expression.
func Call(name string, args ...any) CallExpr {
	call := CallExpr{Name: strings.TrimSpace(name)}
	if len(args) == 0 {
		return call
	}

	call.Args = make([]ValueExpr, len(args))
	for i, arg := range args {
		call.Args[i] = toValueExpr(arg)
	}

	return call
}

// Lower constructs a LOWER(...) function call.
func Lower(arg any) CallExpr {
	return Call("lower", arg)
}

// Upper constructs an UPPER(...) function call.
func Upper(arg any) CallExpr {
	return Call("upper", arg)
}

// Trim constructs a TRIM(...) function call.
func Trim(arg any) CallExpr {
	return Call("trim", arg)
}

// Length constructs a LENGTH(...) function call.
func Length(arg any) CallExpr {
	return Call("length", arg)
}

// CloneValueExpr returns a deep copy of a scalar expression.
func CloneValueExpr(expr ValueExpr) ValueExpr {
	switch typed := expr.(type) {
	case nil:
		return nil
	case Ref:
		return typed
	case Literal:
		return Literal{Value: clonePredicateValue(typed.Value)}
	case CallExpr:
		clone := CallExpr{
			Name: typed.Name,
			Args: make([]ValueExpr, len(typed.Args)),
		}
		for i, arg := range typed.Args {
			clone.Args[i] = CloneValueExpr(arg)
		}
		return clone
	default:
		return typed
	}
}

// WalkValueExpr traverses a scalar expression tree in pre-order.
func WalkValueExpr(expr ValueExpr, visit func(ValueExpr) error) error {
	if expr == nil || visit == nil {
		return nil
	}

	if err := visit(expr); err != nil {
		return err
	}

	switch typed := expr.(type) {
	case CallExpr:
		for _, arg := range typed.Args {
			if err := WalkValueExpr(arg, visit); err != nil {
				return err
			}
		}
	}

	return nil
}

// RewriteValueExpr rewrites a scalar expression tree.
func RewriteValueExpr(expr ValueExpr, rewriter func(ValueExpr) (ValueExpr, error)) (ValueExpr, error) {
	if expr == nil {
		return nil, nil
	}
	if rewriter == nil {
		return CloneValueExpr(expr), nil
	}

	switch typed := expr.(type) {
	case Ref, Literal:
		return rewriter(CloneValueExpr(typed))
	case CallExpr:
		rewritten := CallExpr{
			Name: typed.Name,
			Args: make([]ValueExpr, len(typed.Args)),
		}
		for i, arg := range typed.Args {
			child, err := RewriteValueExpr(arg, rewriter)
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

// SingleRef returns the field name when the expression contains exactly one
// field reference.
func SingleRef(expr ValueExpr) (string, bool) {
	var (
		field string
		count int
	)

	err := WalkValueExpr(expr, func(node ValueExpr) error {
		ref, ok := node.(Ref)
		if !ok {
			return nil
		}
		field = string(ref)
		count++
		return nil
	})
	if err != nil || count != 1 {
		return "", false
	}

	return field, true
}

// Lower wraps the reference in a LOWER(...) call.
func (r Ref) Lower() CallExpr {
	return Lower(r)
}

// Upper wraps the reference in an UPPER(...) call.
func (r Ref) Upper() CallExpr {
	return Upper(r)
}

// Trim wraps the reference in a TRIM(...) call.
func (r Ref) Trim() CallExpr {
	return Trim(r)
}

// Length wraps the reference in a LENGTH(...) call.
func (r Ref) Length() CallExpr {
	return Length(r)
}

// Lower wraps the function result in a LOWER(...) call.
func (c CallExpr) Lower() CallExpr {
	return Lower(c)
}

// Upper wraps the function result in an UPPER(...) call.
func (c CallExpr) Upper() CallExpr {
	return Upper(c)
}

// Trim wraps the function result in a TRIM(...) call.
func (c CallExpr) Trim() CallExpr {
	return Trim(c)
}

// Length wraps the function result in a LENGTH(...) call.
func (c CallExpr) Length() CallExpr {
	return Length(c)
}

// Eq compares the function result with a value.
func (c CallExpr) Eq(value any) Expr {
	return predicateWithLeftExpr(c, OpEq, value)
}

// Ne compares the function result with a value.
func (c CallExpr) Ne(value any) Expr {
	return predicateWithLeftExpr(c, OpNe, value)
}

// Gt compares the function result with a value.
func (c CallExpr) Gt(value any) Expr {
	return predicateWithLeftExpr(c, OpGt, value)
}

// Gte compares the function result with a value.
func (c CallExpr) Gte(value any) Expr {
	return predicateWithLeftExpr(c, OpGte, value)
}

// Lt compares the function result with a value.
func (c CallExpr) Lt(value any) Expr {
	return predicateWithLeftExpr(c, OpLt, value)
}

// Lte compares the function result with a value.
func (c CallExpr) Lte(value any) Expr {
	return predicateWithLeftExpr(c, OpLte, value)
}

// In compares the function result against a list.
func (c CallExpr) In(values ...any) Expr {
	return predicateWithLeftExpr(c, OpIn, flattenValues(values))
}

// NotIn compares the function result against a negated list.
func (c CallExpr) NotIn(values ...any) Expr {
	return predicateWithLeftExpr(c, OpNotIn, flattenValues(values))
}

// Like applies a LIKE predicate to the function result.
func (c CallExpr) Like(value any) Expr {
	return predicateWithLeftExpr(c, OpLike, value)
}

// Contains applies a contains predicate to the function result.
func (c CallExpr) Contains(value any) Expr {
	return predicateWithLeftExpr(c, OpContains, value)
}

// Prefix applies a prefix predicate to the function result.
func (c CallExpr) Prefix(value any) Expr {
	return predicateWithLeftExpr(c, OpPrefix, value)
}

// Suffix applies a suffix predicate to the function result.
func (c CallExpr) Suffix(value any) Expr {
	return predicateWithLeftExpr(c, OpSuffix, value)
}

// IsNull checks whether the function result is NULL.
func (c CallExpr) IsNull() Expr {
	return predicateWithLeftExpr(c, OpIsNull, nil)
}

// NotNull checks whether the function result is not NULL.
func (c CallExpr) NotNull() Expr {
	return predicateWithLeftExpr(c, OpNotNull, nil)
}

func asValueExpr(value any) (ValueExpr, bool) {
	switch typed := value.(type) {
	case Ref:
		return typed, true
	case Literal:
		return typed, true
	case CallExpr:
		return typed, true
	default:
		return nil, false
	}
}

// AsValueExpr reports whether the value is a scalar expression node.
func AsValueExpr(value any) (ValueExpr, bool) {
	return asValueExpr(value)
}

func toValueExpr(value any) ValueExpr {
	if expr, ok := asValueExpr(value); ok {
		return CloneValueExpr(expr)
	}
	return Literal{Value: clonePredicateValue(value)}
}

func clonePredicateValue(value any) any {
	if expr, ok := asValueExpr(value); ok {
		return CloneValueExpr(expr)
	}

	if values, ok := anySlice(value); ok {
		cloned := make([]any, len(values))
		for i, item := range values {
			cloned[i] = clonePredicateValue(item)
		}
		return cloned
	}

	return value
}

// CloneValue returns a safe copy of a predicate value, including nested
// expression nodes.
func CloneValue(value any) any {
	return clonePredicateValue(value)
}

func predicateWithLeftExpr(left ValueExpr, op Operator, value any) Expr {
	return Predicate{
		Left:  CloneValueExpr(left),
		Op:    op,
		Value: clonePredicateValue(value),
	}
}

func cloneValueExprSlice(values []ValueExpr) []ValueExpr {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]ValueExpr, len(values))
	for i, value := range values {
		cloned[i] = CloneValueExpr(value)
	}
	return cloned
}
