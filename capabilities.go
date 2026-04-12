package qb

import "fmt"

// Capabilities describes which query features an adapter can compile or apply.
type Capabilities struct {
	Operators       map[Operator]struct{}
	SupportsSelect  bool
	SupportsInclude bool
	SupportsGroupBy bool
	SupportsSort    bool
	SupportsLimit   bool
	SupportsOffset  bool
	SupportsPage    bool
	SupportsSize    bool
	SupportsCursor  bool
}

// SupportsOperator reports whether the operator is supported.
func (c Capabilities) SupportsOperator(op Operator) bool {
	_, ok := c.Operators[op]
	return ok
}

// Validate checks whether the query fits within the declared capabilities.
func (c Capabilities) Validate(stage ErrorStage, query Query) error {
	if !c.SupportsSelect && len(query.Selects) > 0 {
		return NewError(
			fmt.Errorf("select is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsInclude && len(query.Includes) > 0 {
		return NewError(
			fmt.Errorf("include is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsGroupBy && len(query.GroupBy) > 0 {
		return NewError(
			fmt.Errorf("group_by is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsSort && len(query.Sorts) > 0 {
		return NewError(
			fmt.Errorf("sorting is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsLimit && query.Limit != nil {
		return NewError(
			fmt.Errorf("limit is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsOffset && query.Offset != nil {
		return NewError(
			fmt.Errorf("offset is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsPage && query.Page != nil {
		return NewError(
			fmt.Errorf("page pagination is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsSize && query.Size != nil {
		return NewError(
			fmt.Errorf("size pagination is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsCursor && query.Cursor != nil {
		return NewError(
			fmt.Errorf("cursor pagination is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	limit, offset, err := query.ResolvedPagination()
	if err != nil {
		return WrapError(err, WithDefaultStage(stage))
	}
	if !c.SupportsLimit && limit != nil {
		return NewError(
			fmt.Errorf("limit is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	if !c.SupportsOffset && offset != nil {
		return NewError(
			fmt.Errorf("offset is not supported"),
			WithStage(stage),
			WithCode(CodeUnsupportedFeature),
		)
	}

	return Walk(query.Filter, func(expr Expr) error {
		predicate, ok := expr.(Predicate)
		if !ok {
			return nil
		}

		if c.SupportsOperator(predicate.Op) {
			return nil
		}

		return NewError(
			fmt.Errorf("operator %q is not supported", predicate.Op),
			WithStage(stage),
			WithCode(CodeUnsupportedOperator),
			WithField(predicate.Field),
			WithOperator(predicate.Op),
		)
	})
}

// Validator returns a query transformer that enforces the capabilities.
func (c Capabilities) Validator(stage ErrorStage) QueryTransformer {
	return func(query Query) (Query, error) {
		if err := c.Validate(stage, query); err != nil {
			return Query{}, err
		}
		return query.Clone(), nil
	}
}
