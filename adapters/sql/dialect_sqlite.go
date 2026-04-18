package adapter

import "github.com/pakasa-io/qb"

type SQLiteDialect struct{}

func (SQLiteDialect) spec() registeredDialect {
	functions := map[string]functionCompiler{
		"count":             zeroOrOne("count", "COUNT"),
		"sum":               exactArity("sum", "SUM", 1),
		"avg":               exactArity("avg", "AVG", 1),
		"min":               exactArity("min", "MIN", 1),
		"max":               exactArity("max", "MAX", 1),
		"lower":             exactArity("lower", "LOWER", 1),
		"upper":             exactArity("upper", "UPPER", 1),
		"trim":              exactArity("trim", "TRIM", 1),
		"ltrim":             exactArity("ltrim", "LTRIM", 1),
		"rtrim":             exactArity("rtrim", "RTRIM", 1),
		"length":            exactArity("length", "LENGTH", 1),
		"concat":            concatCompiler("sqlite"),
		"substring":         substringCompiler("sqlite"),
		"replace":           exactArity("replace", "REPLACE", 3),
		"coalesce":          variadic("coalesce", "COALESCE", 1),
		"nullif":            exactArity("nullif", "NULLIF", 2),
		"abs":               exactArity("abs", "ABS", 1),
		"ceil":              ceilCompiler("sqlite"),
		"floor":             floorCompiler("sqlite"),
		"mod":               modCompiler(),
		"round":             arityRange("round", "ROUND", 1, 2),
		"round_double":      roundDoubleCompiler("sqlite"),
		"left":              leftCompiler("sqlite"),
		"right":             rightCompiler("sqlite"),
		"date":              exactArity("date", "DATE", 1),
		"now":               nowCompiler("sqlite"),
		"current_date":      keyword("CURRENT_DATE"),
		"localtime":         localTimeCompiler("sqlite"),
		"current_time":      keyword("CURRENT_TIME"),
		"localtimestamp":    localTimestampCompiler("sqlite"),
		"current_timestamp": keyword("CURRENT_TIMESTAMP"),
		"date_trunc":        unsupportedfunctionCompiler("sqlite", "date_trunc"),
		"extract":           unsupportedfunctionCompiler("sqlite", "extract"),
		"date_bin":          unsupportedfunctionCompiler("sqlite", "date_bin"),
		"json_extract":      exactArity("json_extract", "json_extract", 2),
		"json_query":        exactArity("json_query", "json_extract", 2),
		"json_value":        exactArity("json_value", "json_extract", 2),
		"json_exists":       jsonExistsSQLite(),
		"json_array_length": arityRange("json_array_length", "json_array_length", 1, 2),
		"json_type":         arityRange("json_type", "json_type", 1, 2),
		"json_array":        variadic("json_array", "json_array", 0),
		"json_object":       jsonObjectSimple("json_object"),
	}

	operators := []qb.Operator{
		qb.OpEq, qb.OpNe, qb.OpGt, qb.OpGte, qb.OpLt, qb.OpLte,
		qb.OpIn, qb.OpNotIn, qb.OpLike,
		qb.OpContains, qb.OpPrefix, qb.OpSuffix, qb.OpIsNull, qb.OpNotNull,
	}

	return registeredDialect{
		name:               "sqlite",
		quote:              `"`,
		placeholder:        func(int) string { return "?" },
		functions:          functions,
		cast:               castCompilerForDialect("sqlite"),
		predicateCompilers: map[qb.Operator]predicateCompiler{},
		capabilities:       newCapabilities(functions, operators...),
	}
}

func (d SQLiteDialect) Name() string { return d.spec().Name() }
func (d SQLiteDialect) QuoteIdentifier(identifier string) string {
	return d.spec().QuoteIdentifier(identifier)
}
func (d SQLiteDialect) Placeholder(index int) string { return d.spec().Placeholder(index) }
func (d SQLiteDialect) CompileFunction(name string, args []string) (string, error) {
	return d.spec().CompileFunction(name, args)
}
func (d SQLiteDialect) CompileCast(expr string, typeName string) (string, error) {
	return d.spec().CompileCast(expr, typeName)
}
func (d SQLiteDialect) CompilePredicate(op qb.Operator, left string, right string) (string, bool, error) {
	return d.spec().CompilePredicate(op, left, right)
}
func (d SQLiteDialect) Capabilities() qb.Capabilities { return d.spec().Capabilities() }
