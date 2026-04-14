package qb

// WalkQueryScalars traverses every scalar expression reachable from the query.
func WalkQueryScalars(query Query, visit func(Scalar) error) error {
	if visit == nil {
		return nil
	}

	for _, projection := range query.Projections {
		if err := WalkScalar(projection.Expr, visit); err != nil {
			return err
		}
	}

	for _, expr := range query.GroupBy {
		if err := WalkScalar(expr, visit); err != nil {
			return err
		}
	}

	for _, sort := range query.Sorts {
		if err := WalkScalar(sort.Expr, visit); err != nil {
			return err
		}
	}

	return Walk(query.Filter, func(expr Expr) error {
		predicate, ok := expr.(Predicate)
		if !ok {
			return nil
		}

		if err := WalkScalar(predicate.Left, visit); err != nil {
			return err
		}

		switch typed := predicate.Right.(type) {
		case ScalarOperand:
			return WalkScalar(typed.Expr, visit)
		case ListOperand:
			for _, item := range typed.Items {
				if err := WalkScalar(item, visit); err != nil {
					return err
				}
			}
		}

		return nil
	})
}
