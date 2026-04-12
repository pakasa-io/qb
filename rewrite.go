package qb

// ExprRewriter rewrites an expression tree node.
type ExprRewriter func(Expr) (Expr, error)

// RewriteExpr returns a rewritten copy of the expression tree.
func RewriteExpr(expr Expr, rewriter ExprRewriter) (Expr, error) {
	if expr == nil {
		return nil, nil
	}
	if rewriter == nil {
		return CloneExpr(expr), nil
	}

	switch typed := expr.(type) {
	case Predicate:
		return rewriter(typed)
	case Group:
		rewritten := Group{
			Kind:  typed.Kind,
			Terms: make([]Expr, len(typed.Terms)),
		}
		for i, term := range typed.Terms {
			child, err := RewriteExpr(term, rewriter)
			if err != nil {
				return nil, err
			}
			rewritten.Terms[i] = child
		}
		return rewriter(rewritten)
	case Negation:
		child, err := RewriteExpr(typed.Expr, rewriter)
		if err != nil {
			return nil, err
		}
		return rewriter(Negation{Expr: child})
	default:
		return rewriter(expr)
	}
}

// RewriteQuery rewrites the filter tree of a query while preserving other
// fields.
func RewriteQuery(query Query, rewriter ExprRewriter) (Query, error) {
	clone := query.Clone()
	if clone.Filter == nil {
		return clone, nil
	}

	filter, err := RewriteExpr(clone.Filter, rewriter)
	if err != nil {
		return Query{}, err
	}
	clone.Filter = filter
	return clone, nil
}
