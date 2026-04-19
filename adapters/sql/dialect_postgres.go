package sql

import (
	"fmt"

	"github.com/pakasa-io/qb"
)

type PostgresDialect struct{}

func (PostgresDialect) spec() registeredDialect {
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
		"concat":            concatCompiler("postgres"),
		"substring":         substringCompiler("postgres"),
		"replace":           exactArity("replace", "REPLACE", 3),
		"coalesce":          variadic("coalesce", "COALESCE", 1),
		"nullif":            exactArity("nullif", "NULLIF", 2),
		"abs":               exactArity("abs", "ABS", 1),
		"ceil":              exactArity("ceil", "CEIL", 1),
		"floor":             exactArity("floor", "FLOOR", 1),
		"mod":               modCompiler(),
		"round":             arityRange("round", "ROUND", 1, 2),
		"round_double":      roundDoubleCompiler("postgres"),
		"left":              leftCompiler("postgres"),
		"right":             rightCompiler("postgres"),
		"date":              exactArity("date", "DATE", 1),
		"now":               nowCompiler("postgres"),
		"current_date":      keyword("CURRENT_DATE"),
		"localtime":         keyword("LOCALTIME"),
		"current_time":      keyword("CURRENT_TIME"),
		"localtimestamp":    keyword("LOCALTIMESTAMP"),
		"current_timestamp": keyword("CURRENT_TIMESTAMP"),
		"date_trunc":        postgresOnly("date_trunc", "DATE_TRUNC", 2),
		"extract":           postgresOnly("extract", "DATE_PART", 2),
		"date_bin":          postgresOnly("date_bin", "DATE_BIN", 3),
		"json_extract":      exactArity("json_extract", "JSON_QUERY", 2),
		"json_query":        exactArity("json_query", "JSON_QUERY", 2),
		"json_value":        exactArity("json_value", "JSON_VALUE", 2),
		"json_exists":       exactArity("json_exists", "JSON_EXISTS", 2),
		"json_array_length": jsonArrayLengthPostgres(),
		"json_type":         jsonTypePostgres(),
		"json_array":        variadic("json_array", "JSON_ARRAY", 0),
		"json_object":       jsonObjectPostgres(),
	}

	operators := []qb.Operator{
		qb.OpEq, qb.OpNe, qb.OpGt, qb.OpGte, qb.OpLt, qb.OpLte,
		qb.OpIn, qb.OpNotIn, qb.OpLike, qb.OpILike, qb.OpRegexp,
		qb.OpContains, qb.OpPrefix, qb.OpSuffix, qb.OpIsNull, qb.OpNotNull,
	}

	return registeredDialect{
		name:        "postgres",
		quote:       `"`,
		placeholder: func(index int) string { return fmt.Sprintf("$%d", index) },
		functions:   functions,
		cast:        castCompilerForDialect("postgres"),
		predicateCompilers: map[qb.Operator]predicateCompiler{
			qb.OpILike:  regexOp("ILIKE"),
			qb.OpRegexp: regexOp("~"),
		},
		capabilities: newCapabilities(functions, operators...),
	}
}

func (d PostgresDialect) Name() string { return d.spec().Name() }
func (d PostgresDialect) QuoteIdentifier(identifier string) string {
	return d.spec().QuoteIdentifier(identifier)
}
func (d PostgresDialect) Placeholder(index int) string { return d.spec().Placeholder(index) }
func (d PostgresDialect) CompileFunction(name string, args []string) (string, error) {
	return d.spec().CompileFunction(name, args)
}
func (d PostgresDialect) CompileCast(expr string, typeName string) (string, error) {
	return d.spec().CompileCast(expr, typeName)
}
func (d PostgresDialect) CompilePredicate(op qb.Operator, left string, right string) (string, bool, error) {
	return d.spec().CompilePredicate(op, left, right)
}
func (d PostgresDialect) Capabilities() qb.Capabilities { return d.spec().Capabilities() }
