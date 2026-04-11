package qb

import "fmt"

// Builder incrementally constructs a Query.
type Builder struct {
	query Query
	err   error
}

// New creates a new query builder.
func New() Builder {
	return Builder{}
}

// Where adds an expression to the query. Multiple calls are combined with AND.
func (b Builder) Where(expr Expr) Builder {
	if b.err != nil || expr == nil {
		return b
	}

	if b.query.Filter == nil {
		b.query.Filter = expr
		return b
	}

	b.query.Filter = And(b.query.Filter, expr)
	return b
}

// SortBy appends a sort clause.
func (b Builder) SortBy(field string, direction Direction) Builder {
	if b.err != nil {
		return b
	}

	if field == "" {
		b.err = fmt.Errorf("qb: sort field cannot be empty")
		return b
	}

	if direction == "" {
		direction = Asc
	}

	if direction != Asc && direction != Desc {
		b.err = fmt.Errorf("qb: unsupported sort direction %q", direction)
		return b
	}

	b.query.Sorts = append(append([]Sort(nil), b.query.Sorts...), Sort{
		Field:     field,
		Direction: direction,
	})
	return b
}

// Limit sets the maximum number of rows to return.
func (b Builder) Limit(limit int) Builder {
	if b.err != nil {
		return b
	}

	if limit < 0 {
		b.err = fmt.Errorf("qb: limit cannot be negative")
		return b
	}

	b.query.Limit = intPtr(limit)
	return b
}

// Offset sets the row offset.
func (b Builder) Offset(offset int) Builder {
	if b.err != nil {
		return b
	}

	if offset < 0 {
		b.err = fmt.Errorf("qb: offset cannot be negative")
		return b
	}

	b.query.Offset = intPtr(offset)
	return b
}

// Query returns the built query.
func (b Builder) Query() (Query, error) {
	if b.err != nil {
		return Query{}, b.err
	}

	return b.query.Clone(), nil
}

func intPtr(v int) *int {
	return &v
}
