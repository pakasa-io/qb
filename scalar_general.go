package qb

// Coalesce constructs a COALESCE(...) function call.
func Coalesce(args ...any) Call {
	return Func("coalesce", args...)
}

// NullIf constructs a NULLIF(...) function call.
func NullIf(left any, right any) Call {
	return Func("nullif", left, right)
}

func (r Ref) Coalesce(args ...any) Call     { return prependCallArg(r, Coalesce, args...) }
func (r Ref) NullIf(other any) Call         { return NullIf(r, other) }
func (l Literal) Coalesce(args ...any) Call { return prependCallArg(l, Coalesce, args...) }
func (l Literal) NullIf(other any) Call     { return NullIf(l, other) }
func (c Call) Coalesce(args ...any) Call    { return prependCallArg(c, Coalesce, args...) }
func (c Call) NullIf(other any) Call        { return NullIf(c, other) }
