package qb

func compareScalar(left Scalar, op Operator, value any) Expr {
	return Predicate{
		Left:  CloneScalar(left),
		Op:    op,
		Right: ScalarOperand{Expr: scalarFromAny(value)},
	}
}

func compareList(left Scalar, op Operator, values ...any) Expr {
	return Predicate{
		Left:  CloneScalar(left),
		Op:    op,
		Right: ListOperand{Items: flattenScalars(values)},
	}
}

// ILike builds a case-insensitive pattern-match predicate.
func ILike(left any, pattern any) Expr {
	return compareScalar(scalarFromAny(left), OpILike, pattern)
}

// Regexp builds a regular-expression predicate.
func Regexp(left any, pattern any) Expr {
	return compareScalar(scalarFromAny(left), OpRegexp, pattern)
}

func (r Ref) Eq(value any) Expr        { return compareScalar(r, OpEq, value) }
func (r Ref) Ne(value any) Expr        { return compareScalar(r, OpNe, value) }
func (r Ref) Gt(value any) Expr        { return compareScalar(r, OpGt, value) }
func (r Ref) Gte(value any) Expr       { return compareScalar(r, OpGte, value) }
func (r Ref) Lt(value any) Expr        { return compareScalar(r, OpLt, value) }
func (r Ref) Lte(value any) Expr       { return compareScalar(r, OpLte, value) }
func (r Ref) Like(value any) Expr      { return compareScalar(r, OpLike, value) }
func (r Ref) ILike(value any) Expr     { return compareScalar(r, OpILike, value) }
func (r Ref) Regexp(value any) Expr    { return compareScalar(r, OpRegexp, value) }
func (r Ref) Contains(value any) Expr  { return compareScalar(r, OpContains, value) }
func (r Ref) Prefix(value any) Expr    { return compareScalar(r, OpPrefix, value) }
func (r Ref) Suffix(value any) Expr    { return compareScalar(r, OpSuffix, value) }
func (r Ref) In(values ...any) Expr    { return compareList(r, OpIn, values...) }
func (r Ref) NotIn(values ...any) Expr { return compareList(r, OpNotIn, values...) }
func (r Ref) IsNull() Expr             { return Predicate{Left: CloneScalar(r), Op: OpIsNull} }
func (r Ref) NotNull() Expr            { return Predicate{Left: CloneScalar(r), Op: OpNotNull} }

func (c Call) Eq(value any) Expr        { return compareScalar(c, OpEq, value) }
func (c Call) Ne(value any) Expr        { return compareScalar(c, OpNe, value) }
func (c Call) Gt(value any) Expr        { return compareScalar(c, OpGt, value) }
func (c Call) Gte(value any) Expr       { return compareScalar(c, OpGte, value) }
func (c Call) Lt(value any) Expr        { return compareScalar(c, OpLt, value) }
func (c Call) Lte(value any) Expr       { return compareScalar(c, OpLte, value) }
func (c Call) Like(value any) Expr      { return compareScalar(c, OpLike, value) }
func (c Call) ILike(value any) Expr     { return compareScalar(c, OpILike, value) }
func (c Call) Regexp(value any) Expr    { return compareScalar(c, OpRegexp, value) }
func (c Call) Contains(value any) Expr  { return compareScalar(c, OpContains, value) }
func (c Call) Prefix(value any) Expr    { return compareScalar(c, OpPrefix, value) }
func (c Call) Suffix(value any) Expr    { return compareScalar(c, OpSuffix, value) }
func (c Call) In(values ...any) Expr    { return compareList(c, OpIn, values...) }
func (c Call) NotIn(values ...any) Expr { return compareList(c, OpNotIn, values...) }
func (c Call) IsNull() Expr             { return Predicate{Left: CloneScalar(c), Op: OpIsNull} }
func (c Call) NotNull() Expr            { return Predicate{Left: CloneScalar(c), Op: OpNotNull} }
