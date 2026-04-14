package qb

// JsonExtract constructs a JSON extraction expression.
func JsonExtract(arg any, path any) Call {
	return Func("json_extract", arg, path)
}

// JsonQuery constructs a JSON-query expression.
func JsonQuery(arg any, path any) Call {
	return Func("json_query", arg, path)
}

// JsonValue constructs a scalar JSON value extraction expression.
func JsonValue(arg any, path any) Call {
	return Func("json_value", arg, path)
}

// JsonExists constructs a JSON path existence expression.
func JsonExists(arg any, path any) Call {
	return Func("json_exists", arg, path)
}

// JsonArrayLength constructs a JSON array-length expression.
func JsonArrayLength(arg any, path ...any) Call {
	args := make([]any, 0, 1+len(path))
	args = append(args, arg)
	args = append(args, path...)
	return Func("json_array_length", args...)
}

// JsonType constructs a JSON type expression.
func JsonType(arg any, path ...any) Call {
	args := make([]any, 0, 1+len(path))
	args = append(args, arg)
	args = append(args, path...)
	return Func("json_type", args...)
}

// JsonArray constructs a JSON array value.
func JsonArray(args ...any) Call {
	return Func("json_array", args...)
}

// JsonObject constructs a JSON object value from key/value pairs.
func JsonObject(args ...any) Call {
	return Func("json_object", args...)
}

func (r Ref) JsonExtract(path any) Call { return JsonExtract(r, path) }
func (r Ref) JsonQuery(path any) Call   { return JsonQuery(r, path) }
func (r Ref) JsonValue(path any) Call   { return JsonValue(r, path) }
func (r Ref) JsonExists(path any) Call  { return JsonExists(r, path) }
func (r Ref) JsonArrayLength(path ...any) Call {
	return JsonArrayLength(r, path...)
}
func (r Ref) JsonType(path ...any) Call     { return JsonType(r, path...) }
func (l Literal) JsonExtract(path any) Call { return JsonExtract(l, path) }
func (l Literal) JsonQuery(path any) Call   { return JsonQuery(l, path) }
func (l Literal) JsonValue(path any) Call   { return JsonValue(l, path) }
func (l Literal) JsonExists(path any) Call  { return JsonExists(l, path) }
func (l Literal) JsonArrayLength(path ...any) Call {
	return JsonArrayLength(l, path...)
}
func (l Literal) JsonType(path ...any) Call { return JsonType(l, path...) }
func (c Call) JsonExtract(path any) Call    { return JsonExtract(c, path) }
func (c Call) JsonQuery(path any) Call      { return JsonQuery(c, path) }
func (c Call) JsonValue(path any) Call      { return JsonValue(c, path) }
func (c Call) JsonExists(path any) Call     { return JsonExists(c, path) }
func (c Call) JsonArrayLength(path ...any) Call {
	return JsonArrayLength(c, path...)
}
func (c Call) JsonType(path ...any) Call { return JsonType(c, path...) }
