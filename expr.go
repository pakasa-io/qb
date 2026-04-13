package qb

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

// Expr is a node in the logical query AST.
type Expr interface {
	exprNode()
}

// Predicate matches a scalar expression using an operator and optional
// operand.
type Predicate struct {
	Left  Scalar
	Op    Operator
	Right Operand
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

// Walk traverses the logical expression tree in pre-order.
func Walk(expr Expr, visit func(Expr) error) error {
	if expr == nil || visit == nil {
		return nil
	}

	if err := visit(expr); err != nil {
		return err
	}

	switch typed := expr.(type) {
	case Predicate:
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
		return Predicate{
			Left:  CloneScalar(typed.Left),
			Op:    typed.Op,
			Right: CloneOperand(typed.Right),
		}
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
