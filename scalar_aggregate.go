package qb

// Count constructs a COUNT(...) function call. With no args, it compiles to COUNT(*).
func Count(args ...any) Call {
	return Func("count", args...)
}

// Sum constructs a SUM(...) function call.
func Sum(arg any) Call {
	return Func("sum", arg)
}

// Avg constructs an AVG(...) function call.
func Avg(arg any) Call {
	return Func("avg", arg)
}

// Min constructs a MIN(...) function call.
func Min(arg any) Call {
	return Func("min", arg)
}

// Max constructs a MAX(...) function call.
func Max(arg any) Call {
	return Func("max", arg)
}

func (r Ref) Count() Call     { return Count(r) }
func (r Ref) Sum() Call       { return Sum(r) }
func (r Ref) Avg() Call       { return Avg(r) }
func (r Ref) Min() Call       { return Min(r) }
func (r Ref) Max() Call       { return Max(r) }
func (l Literal) Count() Call { return Count(l) }
func (l Literal) Sum() Call   { return Sum(l) }
func (l Literal) Avg() Call   { return Avg(l) }
func (l Literal) Min() Call   { return Min(l) }
func (l Literal) Max() Call   { return Max(l) }
func (c Call) Count() Call    { return Count(c) }
func (c Call) Sum() Call      { return Sum(c) }
func (c Call) Avg() Call      { return Avg(c) }
func (c Call) Min() Call      { return Min(c) }
func (c Call) Max() Call      { return Max(c) }
