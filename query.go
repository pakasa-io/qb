package qb

// Direction describes ordering direction for a sort clause.
type Direction string

const (
	Asc  Direction = "asc"
	Desc Direction = "desc"
)

// Sort describes a field ordering.
type Sort struct {
	Field     string
	Direction Direction
}

// Query is the database-agnostic representation shared between parsers and
// adapters.
type Query struct {
	Filter Expr
	Sorts  []Sort
	Limit  *int
	Offset *int
}

// Clone returns a safe copy of the query.
func (q Query) Clone() Query {
	clone := Query{
		Filter: CloneExpr(q.Filter),
		Sorts:  append([]Sort(nil), q.Sorts...),
	}

	if q.Limit != nil {
		limit := *q.Limit
		clone.Limit = &limit
	}

	if q.Offset != nil {
		offset := *q.Offset
		clone.Offset = &offset
	}

	return clone
}
