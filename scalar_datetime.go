package qb

// Date constructs a DATE(...) function call.
func Date(arg any) Call {
	return Func("date", arg)
}

// Now constructs a current-timestamp expression.
func Now() Call {
	return Func("now")
}

// CurrentDate constructs a CURRENT_DATE expression.
func CurrentDate() Call {
	return Func("current_date")
}

// LocalTime constructs a LOCALTIME expression.
func LocalTime() Call {
	return Func("localtime")
}

// CurrentTime constructs a CURRENT_TIME expression.
func CurrentTime() Call {
	return Func("current_time")
}

// LocalTimestamp constructs a LOCALTIMESTAMP expression.
func LocalTimestamp() Call {
	return Func("localtimestamp")
}

// CurrentTimestamp constructs a CURRENT_TIMESTAMP expression.
func CurrentTimestamp() Call {
	return Func("current_timestamp")
}

// DateTrunc constructs a date-truncation expression.
func DateTrunc(field any, source any) Call {
	return Func("date_trunc", field, source)
}

// Extract constructs a date-part extraction expression.
func Extract(field any, source any) Call {
	return Func("extract", field, source)
}

// DateBin constructs a DATE_BIN(...) expression.
func DateBin(stride any, source any, origin any) Call {
	return Func("date_bin", stride, source, origin)
}

func (r Ref) Date() Call               { return Date(r) }
func (r Ref) DateTrunc(field any) Call { return DateTrunc(field, r) }
func (r Ref) Extract(field any) Call   { return Extract(field, r) }
func (r Ref) DateBin(stride any, origin any) Call {
	return DateBin(stride, r, origin)
}
func (l Literal) Date() Call               { return Date(l) }
func (l Literal) DateTrunc(field any) Call { return DateTrunc(field, l) }
func (l Literal) Extract(field any) Call   { return Extract(field, l) }
func (l Literal) DateBin(stride any, origin any) Call {
	return DateBin(stride, l, origin)
}
func (c Call) Date() Call               { return Date(c) }
func (c Call) DateTrunc(field any) Call { return DateTrunc(field, c) }
func (c Call) Extract(field any) Call   { return Extract(field, c) }
func (c Call) DateBin(stride any, origin any) Call {
	return DateBin(stride, c, origin)
}
