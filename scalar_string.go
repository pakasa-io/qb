package qb

// Lower constructs a LOWER(...) function call.
func Lower(arg any) Call {
	return Func("lower", arg)
}

// Upper constructs an UPPER(...) function call.
func Upper(arg any) Call {
	return Func("upper", arg)
}

// Trim constructs a TRIM(...) function call.
func Trim(arg any) Call {
	return Func("trim", arg)
}

// LTrim constructs an LTRIM(...) function call.
func LTrim(arg any) Call {
	return Func("ltrim", arg)
}

// RTrim constructs an RTRIM(...) function call.
func RTrim(arg any) Call {
	return Func("rtrim", arg)
}

// Length constructs a LENGTH(...) function call.
func Length(arg any) Call {
	return Func("length", arg)
}

// Concat constructs a CONCAT(...) function call.
func Concat(args ...any) Call {
	return Func("concat", args...)
}

// Substring constructs a SUBSTRING(...) function call.
func Substring(arg any, start any, length ...any) Call {
	args := make([]any, 0, 2+len(length))
	args = append(args, arg, start)
	args = append(args, length...)
	return Func("substring", args...)
}

// Replace constructs a REPLACE(...) function call.
func Replace(arg any, old any, new any) Call {
	return Func("replace", arg, old, new)
}

// Left constructs a LEFT(...) function call.
func Left(arg any, count any) Call {
	return Func("left", arg, count)
}

// Right constructs a RIGHT(...) function call.
func Right(arg any, count any) Call {
	return Func("right", arg, count)
}

func (r Ref) Lower() Call             { return Lower(r) }
func (r Ref) Upper() Call             { return Upper(r) }
func (r Ref) Trim() Call              { return Trim(r) }
func (r Ref) LTrim() Call             { return LTrim(r) }
func (r Ref) RTrim() Call             { return RTrim(r) }
func (r Ref) Length() Call            { return Length(r) }
func (r Ref) Concat(args ...any) Call { return prependCallArg(r, Concat, args...) }
func (r Ref) Substring(start any, length ...any) Call {
	return Substring(r, start, length...)
}
func (r Ref) Replace(old any, new any) Call { return Replace(r, old, new) }
func (r Ref) Left(count any) Call           { return Left(r, count) }
func (r Ref) Right(count any) Call          { return Right(r, count) }

func (l Literal) Lower() Call             { return Lower(l) }
func (l Literal) Upper() Call             { return Upper(l) }
func (l Literal) Trim() Call              { return Trim(l) }
func (l Literal) LTrim() Call             { return LTrim(l) }
func (l Literal) RTrim() Call             { return RTrim(l) }
func (l Literal) Length() Call            { return Length(l) }
func (l Literal) Concat(args ...any) Call { return prependCallArg(l, Concat, args...) }
func (l Literal) Substring(start any, length ...any) Call {
	return Substring(l, start, length...)
}
func (l Literal) Replace(old any, new any) Call { return Replace(l, old, new) }
func (l Literal) Left(count any) Call           { return Left(l, count) }
func (l Literal) Right(count any) Call          { return Right(l, count) }

func (c Call) Lower() Call             { return Lower(c) }
func (c Call) Upper() Call             { return Upper(c) }
func (c Call) Trim() Call              { return Trim(c) }
func (c Call) LTrim() Call             { return LTrim(c) }
func (c Call) RTrim() Call             { return RTrim(c) }
func (c Call) Length() Call            { return Length(c) }
func (c Call) Concat(args ...any) Call { return prependCallArg(c, Concat, args...) }
func (c Call) Substring(start any, length ...any) Call {
	return Substring(c, start, length...)
}
func (c Call) Replace(old any, new any) Call { return Replace(c, old, new) }
func (c Call) Left(count any) Call           { return Left(c, count) }
func (c Call) Right(count any) Call          { return Right(c, count) }
