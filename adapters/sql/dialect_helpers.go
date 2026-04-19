package sql

import (
	"fmt"
	"strings"

	"github.com/pakasa-io/qb"
)

func keyword(sql string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 0 {
			return "", fmt.Errorf("keyword expression expects no arguments")
		}
		return sql, nil
	}
}

func zeroOrOne(name, sql string) functionCompiler {
	return func(args []string) (string, error) {
		switch len(args) {
		case 0:
			return sql + "(*)", nil
		case 1:
			return sql + "(" + args[0] + ")", nil
		default:
			return "", fmt.Errorf("function %q expects zero or one argument", name)
		}
	}
}

func exactArity(name, sql string, arity int) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != arity {
			return "", fmt.Errorf("function %q expects exactly %d argument(s)", name, arity)
		}
		return sql + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func arityRange(name, sql string, min, max int) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) < min || len(args) > max {
			if min == max {
				return "", fmt.Errorf("function %q expects exactly %d argument(s)", name, min)
			}
			return "", fmt.Errorf("function %q expects between %d and %d argument(s)", name, min, max)
		}
		return sql + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func variadic(name, sql string, min int) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) < min {
			return "", fmt.Errorf("function %q expects at least %d argument(s)", name, min)
		}
		return sql + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func unsupportedfunctionCompiler(dialect, name string) functionCompiler {
	return func(args []string) (string, error) {
		return "", qb.UnsupportedFunction("", dialect, name)
	}
}

func concatCompiler(dialect string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) == 0 {
			return "", fmt.Errorf("function %q expects at least one argument", "concat")
		}
		if dialect == "mysql" {
			return "CONCAT(" + strings.Join(args, ", ") + ")", nil
		}
		return "(" + strings.Join(args, " || ") + ")", nil
	}
}

func substringCompiler(dialect string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 && len(args) != 3 {
			return "", fmt.Errorf("function %q expects two or three arguments", "substring")
		}
		name := "SUBSTRING"
		if dialect == "sqlite" {
			name = "SUBSTR"
		}
		return name + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func ceilCompiler(dialect string) functionCompiler {
	if dialect == "sqlite" {
		return unsupportedfunctionCompiler(dialect, "ceil")
	}
	return exactArity("ceil", "CEIL", 1)
}

func floorCompiler(dialect string) functionCompiler {
	if dialect == "sqlite" {
		return unsupportedfunctionCompiler(dialect, "floor")
	}
	return exactArity("floor", "FLOOR", 1)
}

func modCompiler() functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "mod")
		}
		return "(" + args[0] + " % " + args[1] + ")", nil
	}
}

func roundDoubleCompiler(dialect string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "round_double")
		}
		switch dialect {
		case "postgres":
			return "CAST(ROUND(CAST(" + args[0] + " AS NUMERIC), " + args[1] + ") AS DOUBLE PRECISION)", nil
		default:
			return "ROUND(" + strings.Join(args, ", ") + ")", nil
		}
	}
}

func leftCompiler(dialect string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "left")
		}
		if dialect == "sqlite" {
			return "SUBSTR(" + args[0] + ", 1, " + args[1] + ")", nil
		}
		return "LEFT(" + strings.Join(args, ", ") + ")", nil
	}
}

func rightCompiler(dialect string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "right")
		}
		if dialect == "sqlite" {
			return "SUBSTR(" + args[0] + ", -" + args[1] + ")", nil
		}
		return "RIGHT(" + strings.Join(args, ", ") + ")", nil
	}
}

func nowCompiler(dialect string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", "now")
		}
		if dialect == "sqlite" {
			return "CURRENT_TIMESTAMP", nil
		}
		return "NOW()", nil
	}
}

func localTimeCompiler(dialect string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", "localtime")
		}
		if dialect == "sqlite" {
			return "TIME('now', 'localtime')", nil
		}
		return "LOCALTIME", nil
	}
}

func localTimestampCompiler(dialect string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", "localtimestamp")
		}
		if dialect == "sqlite" {
			return "DATETIME('now', 'localtime')", nil
		}
		return "LOCALTIMESTAMP", nil
	}
}

func postgresOnly(name, sql string, arity int) functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != arity {
			return "", fmt.Errorf("function %q expects exactly %d argument(s)", name, arity)
		}
		return sql + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func jsonArrayLengthPostgres() functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 1 && len(args) != 2 {
			return "", fmt.Errorf("function %q expects one or two arguments", "json_array_length")
		}
		if len(args) == 1 {
			return "JSON_ARRAY_LENGTH(" + args[0] + ")", nil
		}
		return "JSON_ARRAY_LENGTH(JSON_QUERY(" + args[0] + ", " + args[1] + "))", nil
	}
}

func jsonTypePostgres() functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 1 && len(args) != 2 {
			return "", fmt.Errorf("function %q expects one or two arguments", "json_type")
		}
		if len(args) == 1 {
			return "JSON_TYPEOF(" + args[0] + ")", nil
		}
		return "JSON_TYPEOF(JSON_QUERY(" + args[0] + ", " + args[1] + "))", nil
	}
}

func jsonObjectPostgres() functionCompiler {
	return func(args []string) (string, error) {
		if len(args)%2 != 0 {
			return "", fmt.Errorf("function %q expects key/value pairs", "json_object")
		}
		if len(args) == 0 {
			return "JSON_OBJECT()", nil
		}
		pairs := make([]string, 0, len(args)/2)
		for i := 0; i < len(args); i += 2 {
			pairs = append(pairs, args[i]+" VALUE "+args[i+1])
		}
		return "JSON_OBJECT(" + strings.Join(pairs, ", ") + ")", nil
	}
}

func jsonExistsMySQL() functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "json_exists")
		}
		return "JSON_CONTAINS_PATH(" + args[0] + ", 'one', " + args[1] + ")", nil
	}
}

func jsonExistsSQLite() functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "json_exists")
		}
		return "(json_type(" + args[0] + ", " + args[1] + ") IS NOT NULL)", nil
	}
}

func jsonTypeMySQL() functionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 1 && len(args) != 2 {
			return "", fmt.Errorf("function %q expects one or two arguments", "json_type")
		}
		if len(args) == 1 {
			return "JSON_TYPE(" + args[0] + ")", nil
		}
		return "JSON_TYPE(JSON_EXTRACT(" + args[0] + ", " + args[1] + "))", nil
	}
}

func jsonObjectSimple(prefix string) functionCompiler {
	return func(args []string) (string, error) {
		if len(args)%2 != 0 {
			return "", fmt.Errorf("function %q expects key/value pairs", "json_object")
		}
		return prefix + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func regexOp(sql string) predicateCompiler {
	return func(left string, right string) (string, error) {
		return left + " " + sql + " " + right, nil
	}
}

func regexLikeMySQL() predicateCompiler {
	return func(left string, right string) (string, error) {
		return "REGEXP_LIKE(" + left + ", " + right + ")", nil
	}
}

func castCompilerForDialect(dialect string) castCompiler {
	return func(expr string, typeName string) (string, error) {
		typeName = strings.ToLower(strings.TrimSpace(typeName))
		if expr == "" {
			return "", fmt.Errorf("cast expression cannot be empty")
		}

		switch dialect {
		case "postgres":
			switch typeName {
			case "string":
				return "CAST(" + expr + " AS TEXT)", nil
			case "int":
				return "CAST(" + expr + " AS INTEGER)", nil
			case "bigint":
				return "CAST(" + expr + " AS BIGINT)", nil
			case "double":
				return "CAST(" + expr + " AS DOUBLE PRECISION)", nil
			case "decimal":
				return "CAST(" + expr + " AS NUMERIC)", nil
			case "bool":
				return "CAST(" + expr + " AS BOOLEAN)", nil
			case "date":
				return "CAST(" + expr + " AS DATE)", nil
			case "timestamp":
				return "CAST(" + expr + " AS TIMESTAMP)", nil
			case "json":
				return "CAST(" + expr + " AS JSONB)", nil
			}
		case "mysql":
			switch typeName {
			case "string":
				return "CAST(" + expr + " AS CHAR)", nil
			case "int":
				return "CAST(" + expr + " AS SIGNED)", nil
			case "bigint":
				return "CAST(" + expr + " AS SIGNED)", nil
			case "double":
				return "CAST(" + expr + " AS DOUBLE)", nil
			case "decimal":
				return "CAST(" + expr + " AS DECIMAL(65,30))", nil
			case "bool":
				return "CAST(" + expr + " AS SIGNED)", nil
			case "date":
				return "CAST(" + expr + " AS DATE)", nil
			case "timestamp":
				return "CAST(" + expr + " AS DATETIME)", nil
			case "json":
				return "CAST(" + expr + " AS JSON)", nil
			}
		case "sqlite":
			switch typeName {
			case "string":
				return "CAST(" + expr + " AS TEXT)", nil
			case "int", "bigint":
				return "CAST(" + expr + " AS INTEGER)", nil
			case "double":
				return "CAST(" + expr + " AS REAL)", nil
			case "decimal":
				return "CAST(" + expr + " AS NUMERIC)", nil
			case "bool":
				return "CAST(" + expr + " AS INTEGER)", nil
			case "date":
				return "DATE(" + expr + ")", nil
			case "timestamp":
				return "DATETIME(" + expr + ")", nil
			case "json":
				return "CAST(" + expr + " AS TEXT)", nil
			}
		}

		return "", fmt.Errorf("unsupported cast type %q for dialect %q", typeName, dialect)
	}
}
