package qb

// Direction describes ordering direction for a sort clause.
type Direction string

const (
	Asc  Direction = "asc"
	Desc Direction = "desc"
)

// Sort describes a field ordering.
type Sort struct {
	Expr      Scalar
	Direction Direction
}

// Cursor carries cursor-pagination metadata. Built-in adapters do not interpret
// it directly; query transformers are expected to translate it into filters and
// sorts before compilation or application.
type Cursor struct {
	Token  string
	Values map[string]any
}

// Clone returns a safe copy of the cursor.
func (c Cursor) Clone() Cursor {
	clone := Cursor{Token: c.Token}
	if len(c.Values) > 0 {
		clone.Values = make(map[string]any, len(c.Values))
		for key, value := range c.Values {
			clone.Values[key] = value
		}
	}
	return clone
}

// Query is the database-agnostic representation shared between parsers and
// adapters.
type Query struct {
	Selects  []Scalar
	Includes []string
	GroupBy  []Scalar
	Filter   Expr
	Sorts    []Sort
	Limit    *int
	Offset   *int
	Page     *int
	Size     *int
	Cursor   *Cursor
}

// QueryTransformer rewrites or validates a query.
type QueryTransformer func(Query) (Query, error)

// Clone returns a safe copy of the query.
func (q Query) Clone() Query {
	clone := Query{
		Selects:  cloneScalars(q.Selects),
		Includes: append([]string(nil), q.Includes...),
		GroupBy:  cloneScalars(q.GroupBy),
		Filter:   CloneExpr(q.Filter),
		Sorts:    cloneSorts(q.Sorts),
	}

	if q.Limit != nil {
		limit := *q.Limit
		clone.Limit = &limit
	}

	if q.Offset != nil {
		offset := *q.Offset
		clone.Offset = &offset
	}

	if q.Page != nil {
		page := *q.Page
		clone.Page = &page
	}

	if q.Size != nil {
		size := *q.Size
		clone.Size = &size
	}

	if q.Cursor != nil {
		cursor := q.Cursor.Clone()
		clone.Cursor = &cursor
	}

	return clone
}

// ResolvedPagination returns the effective limit/offset after considering both
// legacy limit/offset values and the newer page/size pagination model. Cursor
// pagination uses size for the page size and must be rewritten into filters and
// sorts by a query transformer.
func (q Query) ResolvedPagination() (limit *int, offset *int, err error) {
	if q.Page != nil && q.Cursor != nil {
		return nil, nil, NewError(
			ErrInvalidPagination("page and cursor cannot be combined"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Cursor != nil && (q.Limit != nil || q.Offset != nil) {
		return nil, nil, NewError(
			ErrInvalidPagination("cursor cannot be combined with limit/offset; use size"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Page != nil && (q.Limit != nil || q.Offset != nil) {
		return nil, nil, NewError(
			ErrInvalidPagination("page/size cannot be combined with limit/offset"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Size != nil && (q.Limit != nil || q.Offset != nil) {
		return nil, nil, NewError(
			ErrInvalidPagination("page/size cannot be combined with limit/offset"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Page != nil && q.Size == nil {
		return nil, nil, NewError(
			ErrInvalidPagination("page requires size"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Cursor != nil && q.Size == nil {
		return nil, nil, NewError(
			ErrInvalidPagination("cursor requires size"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Page != nil && *q.Page < 1 {
		return nil, nil, NewError(
			ErrInvalidPagination("page must be greater than or equal to 1"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Size != nil && *q.Size < 1 {
		return nil, nil, NewError(
			ErrInvalidPagination("size must be greater than or equal to 1"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Limit != nil && *q.Limit < 0 {
		return nil, nil, NewError(
			ErrInvalidPagination("limit cannot be negative"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Offset != nil && *q.Offset < 0 {
		return nil, nil, NewError(
			ErrInvalidPagination("offset cannot be negative"),
			WithCode(CodeInvalidQuery),
		)
	}

	if q.Page != nil && q.Size != nil {
		resolvedLimit := *q.Size
		resolvedOffset := (*q.Page - 1) * (*q.Size)
		return &resolvedLimit, &resolvedOffset, nil
	}

	if q.Size != nil {
		resolvedLimit := *q.Size
		return &resolvedLimit, nil, nil
	}

	return q.Limit, q.Offset, nil
}

// TransformQuery applies a sequence of query transformers.
func TransformQuery(query Query, transformers ...QueryTransformer) (Query, error) {
	clone := query.Clone()
	var err error

	for _, transformer := range transformers {
		if transformer == nil {
			continue
		}

		clone, err = transformer(clone)
		if err != nil {
			return Query{}, err
		}
	}

	return clone, nil
}

// ComposeTransformers collapses several transformers into one.
func ComposeTransformers(transformers ...QueryTransformer) QueryTransformer {
	return func(query Query) (Query, error) {
		return TransformQuery(query, transformers...)
	}
}

type invalidPagination string

func (e invalidPagination) Error() string {
	return string(e)
}

// ErrInvalidPagination creates a consistent pagination validation error.
func ErrInvalidPagination(message string) error {
	return invalidPagination(message)
}

func cloneScalars(values []Scalar) []Scalar {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]Scalar, len(values))
	for i, value := range values {
		cloned[i] = CloneScalar(value)
	}
	return cloned
}

func cloneSorts(values []Sort) []Sort {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]Sort, len(values))
	for i, value := range values {
		cloned[i] = Sort{
			Expr:      CloneScalar(value.Expr),
			Direction: value.Direction,
		}
	}
	return cloned
}
