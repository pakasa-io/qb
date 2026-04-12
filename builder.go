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

// Select appends projected fields.
func (b Builder) Select(fields ...string) Builder {
	appended, err := appendFields(b.query.Selects, "select", fields...)
	if err != nil {
		b.err = err
		return b
	}

	b.query.Selects = appended
	return b
}

// Pick is an alias for Select.
func (b Builder) Pick(fields ...string) Builder {
	return b.Select(fields...)
}

// Include appends eager-load/include hints.
func (b Builder) Include(paths ...string) Builder {
	appended, err := appendFields(b.query.Includes, "include", paths...)
	if err != nil {
		b.err = err
		return b
	}

	b.query.Includes = appended
	return b
}

// GroupBy appends grouping fields.
func (b Builder) GroupBy(fields ...string) Builder {
	appended, err := appendFields(b.query.GroupBy, "group_by", fields...)
	if err != nil {
		b.err = err
		return b
	}

	b.query.GroupBy = appended
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

// Page sets the offset-based page number. Page numbering starts at 1.
func (b Builder) Page(page int) Builder {
	if b.err != nil {
		return b
	}

	if page < 1 {
		b.err = fmt.Errorf("qb: page must be greater than or equal to 1")
		return b
	}

	b.query.Page = intPtr(page)
	return b
}

// Size sets the requested page size. It is used by both page-based and cursor
// pagination.
func (b Builder) Size(size int) Builder {
	if b.err != nil {
		return b
	}

	if size < 1 {
		b.err = fmt.Errorf("qb: size must be greater than or equal to 1")
		return b
	}

	b.query.Size = intPtr(size)
	return b
}

// Limit sets the maximum number of rows to return. Prefer Size for new code.
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

// Offset sets the row offset. Prefer Page and Size for new code.
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

// CursorToken sets an opaque cursor token. Cursor pagination requires Size.
func (b Builder) CursorToken(token string) Builder {
	if b.err != nil {
		return b
	}

	if token == "" {
		b.err = fmt.Errorf("qb: cursor token cannot be empty")
		return b
	}

	cursor := Cursor{Token: token}
	b.query.Cursor = &cursor
	return b
}

// CursorValues sets structured cursor values. Cursor pagination requires Size.
func (b Builder) CursorValues(values map[string]any) Builder {
	if b.err != nil {
		return b
	}

	if len(values) == 0 {
		b.err = fmt.Errorf("qb: cursor values cannot be empty")
		return b
	}

	cursor := Cursor{
		Values: make(map[string]any, len(values)),
	}
	for key, value := range values {
		cursor.Values[key] = value
	}
	b.query.Cursor = &cursor
	return b
}

// Query returns the built query.
func (b Builder) Query() (Query, error) {
	if b.err != nil {
		return Query{}, b.err
	}

	if _, _, err := b.query.ResolvedPagination(); err != nil {
		return Query{}, err
	}

	return b.query.Clone(), nil
}

func intPtr(v int) *int {
	return &v
}

func appendFields(target []string, label string, fields ...string) ([]string, error) {
	appended := append([]string(nil), target...)
	for _, field := range fields {
		if field == "" {
			return nil, fmt.Errorf("qb: %s field cannot be empty", label)
		}
		appended = append(appended, field)
	}

	return appended, nil
}
