package sql

import "github.com/pakasa-io/qb"

type MySQLDialect struct{}

func (MySQLDialect) spec() registeredDialect {
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
		"concat":            concatCompiler("mysql"),
		"substring":         substringCompiler("mysql"),
		"replace":           exactArity("replace", "REPLACE", 3),
		"coalesce":          variadic("coalesce", "COALESCE", 1),
		"nullif":            exactArity("nullif", "NULLIF", 2),
		"abs":               exactArity("abs", "ABS", 1),
		"ceil":              exactArity("ceil", "CEIL", 1),
		"floor":             exactArity("floor", "FLOOR", 1),
		"mod":               modCompiler(),
		"round":             arityRange("round", "ROUND", 1, 2),
		"round_double":      roundDoubleCompiler("mysql"),
		"left":              leftCompiler("mysql"),
		"right":             rightCompiler("mysql"),
		"date":              exactArity("date", "DATE", 1),
		"now":               nowCompiler("mysql"),
		"current_date":      keyword("CURRENT_DATE"),
		"localtime":         keyword("LOCALTIME"),
		"current_time":      keyword("CURRENT_TIME"),
		"localtimestamp":    keyword("LOCALTIMESTAMP"),
		"current_timestamp": keyword("CURRENT_TIMESTAMP"),
		"date_trunc":        unsupportedfunctionCompiler("mysql", "date_trunc"),
		"extract":           unsupportedfunctionCompiler("mysql", "extract"),
		"date_bin":          unsupportedfunctionCompiler("mysql", "date_bin"),
		"json_extract":      exactArity("json_extract", "JSON_EXTRACT", 2),
		"json_query":        exactArity("json_query", "JSON_EXTRACT", 2),
		"json_value":        exactArity("json_value", "JSON_VALUE", 2),
		"json_exists":       jsonExistsMySQL(),
		"json_array_length": arityRange("json_array_length", "JSON_LENGTH", 1, 2),
		"json_type":         jsonTypeMySQL(),
		"json_array":        variadic("json_array", "JSON_ARRAY", 0),
		"json_object":       jsonObjectSimple("JSON_OBJECT"),
	}

	operators := []qb.Operator{
		qb.OpEq, qb.OpNe, qb.OpGt, qb.OpGte, qb.OpLt, qb.OpLte,
		qb.OpIn, qb.OpNotIn, qb.OpLike, qb.OpRegexp,
		qb.OpContains, qb.OpPrefix, qb.OpSuffix, qb.OpIsNull, qb.OpNotNull,
	}

	return registeredDialect{
		name:        "mysql",
		quote:       "`",
		placeholder: func(int) string { return "?" },
		functions:   functions,
		cast:        castCompilerForDialect("mysql"),
		predicateCompilers: map[qb.Operator]predicateCompiler{
			qb.OpRegexp: regexLikeMySQL(),
		},
		capabilities: newCapabilities(functions, operators...),
	}
}

func (d MySQLDialect) Name() string { return d.spec().Name() }
func (d MySQLDialect) QuoteIdentifier(identifier string) string {
	return d.spec().QuoteIdentifier(identifier)
}
func (d MySQLDialect) Placeholder(index int) string { return d.spec().Placeholder(index) }
func (d MySQLDialect) CompileFunction(name string, args []string) (string, error) {
	return d.spec().CompileFunction(name, args)
}
func (d MySQLDialect) CompileCast(expr string, typeName string) (string, error) {
	return d.spec().CompileCast(expr, typeName)
}
func (d MySQLDialect) CompilePredicate(op qb.Operator, left string, right string) (string, bool, error) {
	return d.spec().CompilePredicate(op, left, right)
}
func (d MySQLDialect) Capabilities() qb.Capabilities { return d.spec().Capabilities() }
