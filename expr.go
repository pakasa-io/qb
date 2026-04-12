package qb

import "reflect"

// Operator describes a comparison operation.
type Operator string

const (
	OpEq       Operator = "eq"
	OpNe       Operator = "ne"
	OpGt       Operator = "gt"
	OpGte      Operator = "gte"
	OpLt       Operator = "lt"
	OpLte      Operator = "lte"
	OpIn       Operator = "in"
	OpNotIn    Operator = "not_in"
	OpLike     Operator = "like"
	OpContains Operator = "contains"
	OpPrefix   Operator = "prefix"
	OpSuffix   Operator = "suffix"
	OpIsNull   Operator = "is_null"
	OpNotNull  Operator = "not_null"
)

// GroupKind describes how a group combines child expressions.
type GroupKind string

const (
	AndGroup GroupKind = "and"
	OrGroup  GroupKind = "or"
)

// Expr is a node in the query AST.
type Expr interface {
	exprNode()
}

// Predicate matches a left-hand expression using an operator and optional
// value.
type Predicate struct {
	Field string
	Left  ValueExpr
	Op    Operator
	Value any
}

func (Predicate) exprNode() {}

// Group combines a set of expressions with a logical operator.
type Group struct {
	Kind  GroupKind
	Terms []Expr
}

func (Group) exprNode() {}

// Negation negates its child expression.
type Negation struct {
	Expr Expr
}

func (Negation) exprNode() {}

// Ref is a field reference used by the fluent helpers.
type Ref string

// Field references a field in a predicate.
func Field(name string) Ref {
	return Ref(name)
}

func (r Ref) Eq(value any) Expr {
	return Predicate{Field: string(r), Op: OpEq, Value: clonePredicateValue(value)}
}

func (r Ref) Ne(value any) Expr {
	return Predicate{Field: string(r), Op: OpNe, Value: clonePredicateValue(value)}
}

func (r Ref) Gt(value any) Expr {
	return Predicate{Field: string(r), Op: OpGt, Value: clonePredicateValue(value)}
}

func (r Ref) Gte(value any) Expr {
	return Predicate{Field: string(r), Op: OpGte, Value: clonePredicateValue(value)}
}

func (r Ref) Lt(value any) Expr {
	return Predicate{Field: string(r), Op: OpLt, Value: clonePredicateValue(value)}
}

func (r Ref) Lte(value any) Expr {
	return Predicate{Field: string(r), Op: OpLte, Value: clonePredicateValue(value)}
}

func (r Ref) In(values ...any) Expr {
	return Predicate{Field: string(r), Op: OpIn, Value: clonePredicateValue(flattenValues(values))}
}

func (r Ref) NotIn(values ...any) Expr {
	return Predicate{Field: string(r), Op: OpNotIn, Value: clonePredicateValue(flattenValues(values))}
}

func (r Ref) Like(value any) Expr {
	return Predicate{Field: string(r), Op: OpLike, Value: clonePredicateValue(value)}
}

func (r Ref) Contains(value any) Expr {
	return Predicate{Field: string(r), Op: OpContains, Value: clonePredicateValue(value)}
}

func (r Ref) Prefix(value any) Expr {
	return Predicate{Field: string(r), Op: OpPrefix, Value: clonePredicateValue(value)}
}

func (r Ref) Suffix(value any) Expr {
	return Predicate{Field: string(r), Op: OpSuffix, Value: clonePredicateValue(value)}
}

func (r Ref) IsNull() Expr {
	return Predicate{Field: string(r), Op: OpIsNull}
}

func (r Ref) NotNull() Expr {
	return Predicate{Field: string(r), Op: OpNotNull}
}

// And combines expressions with logical AND and flattens nested AND groups.
func And(exprs ...Expr) Expr {
	return group(AndGroup, exprs...)
}

// Or combines expressions with logical OR and flattens nested OR groups.
func Or(exprs ...Expr) Expr {
	return group(OrGroup, exprs...)
}

// Not negates an expression.
func Not(expr Expr) Expr {
	if expr == nil {
		return nil
	}

	return Negation{Expr: expr}
}

// Walk traverses the query AST in pre-order.
func Walk(expr Expr, visit func(Expr) error) error {
	if expr == nil || visit == nil {
		return nil
	}

	if err := visit(expr); err != nil {
		return err
	}

	switch typed := expr.(type) {
	case Predicate:
		if typed.Left != nil {
			if err := WalkValueExpr(typed.Left, func(ValueExpr) error { return nil }); err != nil {
				return err
			}
		}
		if valueExpr, ok := asValueExpr(typed.Value); ok {
			if err := WalkValueExpr(valueExpr, func(ValueExpr) error { return nil }); err != nil {
				return err
			}
		}
		return nil
	case Group:
		for _, term := range typed.Terms {
			if err := Walk(term, visit); err != nil {
				return err
			}
		}
		return nil
	case Negation:
		return Walk(typed.Expr, visit)
	default:
		return nil
	}
}

// CloneExpr returns a deep copy of an expression tree.
func CloneExpr(expr Expr) Expr {
	switch typed := expr.(type) {
	case nil:
		return nil
	case Predicate:
		typed.Left = CloneValueExpr(typed.Left)
		typed.Value = clonePredicateValue(typed.Value)
		return typed
	case Group:
		clone := Group{
			Kind:  typed.Kind,
			Terms: make([]Expr, len(typed.Terms)),
		}
		for i, term := range typed.Terms {
			clone.Terms[i] = CloneExpr(term)
		}
		return clone
	case Negation:
		return Negation{Expr: CloneExpr(typed.Expr)}
	default:
		return typed
	}
}

func group(kind GroupKind, exprs ...Expr) Expr {
	terms := make([]Expr, 0, len(exprs))

	for _, expr := range exprs {
		if expr == nil {
			continue
		}

		if nested, ok := expr.(Group); ok && nested.Kind == kind {
			terms = append(terms, nested.Terms...)
			continue
		}

		terms = append(terms, expr)
	}

	switch len(terms) {
	case 0:
		return nil
	case 1:
		return terms[0]
	default:
		return Group{Kind: kind, Terms: terms}
	}
}

func flattenValues(values []any) []any {
	if len(values) == 1 {
		if flattened, ok := anySlice(values[0]); ok {
			return flattened
		}
	}

	return append([]any(nil), values...)
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
