package qb

// Abs constructs an ABS(...) function call.
func Abs(arg any) Call {
	return Func("abs", arg)
}

// Ceil constructs a CEIL(...) function call.
func Ceil(arg any) Call {
	return Func("ceil", arg)
}

// Floor constructs a FLOOR(...) function call.
func Floor(arg any) Call {
	return Func("floor", arg)
}

// Mod constructs a MOD(...) function call.
func Mod(left any, right any) Call {
	return Func("mod", left, right)
}

// Round constructs a ROUND(...) function call.
func Round(arg any, precision ...any) Call {
	args := make([]any, 0, 1+len(precision))
	args = append(args, arg)
	args = append(args, precision...)
	return Func("round", args...)
}

// RoundDouble constructs a PostgreSQL-safe ROUND helper for double-precision
// values with a scale. On PostgreSQL it rounds via NUMERIC and casts back to
// DOUBLE PRECISION; on other dialects it compiles to ROUND(...).
func RoundDouble(arg any, precision any) Call {
	return Func("round_double", arg, precision)
}

func (r Ref) Abs() Call                   { return Abs(r) }
func (r Ref) Ceil() Call                  { return Ceil(r) }
func (r Ref) Floor() Call                 { return Floor(r) }
func (r Ref) Mod(other any) Call          { return Mod(r, other) }
func (r Ref) Round(precision ...any) Call { return Round(r, precision...) }
func (r Ref) RoundDouble(precision any) Call {
	return RoundDouble(r, precision)
}
func (l Literal) Abs() Call          { return Abs(l) }
func (l Literal) Ceil() Call         { return Ceil(l) }
func (l Literal) Floor() Call        { return Floor(l) }
func (l Literal) Mod(other any) Call { return Mod(l, other) }
func (l Literal) RoundDouble(precision any) Call {
	return RoundDouble(l, precision)
}
func (l Literal) Round(precision ...any) Call {
	return Round(l, precision...)
}
func (c Call) Abs() Call          { return Abs(c) }
func (c Call) Ceil() Call         { return Ceil(c) }
func (c Call) Floor() Call        { return Floor(c) }
func (c Call) Mod(other any) Call { return Mod(c, other) }
func (c Call) RoundDouble(precision any) Call {
	return RoundDouble(c, precision)
}
func (c Call) Round(precision ...any) Call {
	return Round(c, precision...)
}
func (c Cast) RoundDouble(precision any) Call {
	return RoundDouble(c, precision)
}
