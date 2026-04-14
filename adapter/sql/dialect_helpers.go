package sqladapter

import (
	"fmt"
	"strings"

	"github.com/pakasa-io/qb"
)

func keyword(sql string) FunctionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 0 {
			return "", fmt.Errorf("keyword expression expects no arguments")
		}
		return sql, nil
	}
}

func zeroOrOne(name, sql string) FunctionCompiler {
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

func exactArity(name, sql string, arity int) FunctionCompiler {
	return func(args []string) (string, error) {
		if len(args) != arity {
			return "", fmt.Errorf("function %q expects exactly %d argument(s)", name, arity)
		}
		return sql + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func arityRange(name, sql string, min, max int) FunctionCompiler {
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

func variadic(name, sql string, min int) FunctionCompiler {
	return func(args []string) (string, error) {
		if len(args) < min {
			return "", fmt.Errorf("function %q expects at least %d argument(s)", name, min)
		}
		return sql + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func unsupportedFunctionCompiler(dialect, name string) FunctionCompiler {
	return func(args []string) (string, error) {
		return "", qb.UnsupportedFunction("", dialect, name)
	}
}

func concatCompiler(dialect string) FunctionCompiler {
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

func substringCompiler(dialect string) FunctionCompiler {
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

func ceilCompiler(dialect string) FunctionCompiler {
	if dialect == "sqlite" {
		return unsupportedFunctionCompiler(dialect, "ceil")
	}
	return exactArity("ceil", "CEIL", 1)
}

func floorCompiler(dialect string) FunctionCompiler {
	if dialect == "sqlite" {
		return unsupportedFunctionCompiler(dialect, "floor")
	}
	return exactArity("floor", "FLOOR", 1)
}

func modCompiler() FunctionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "mod")
		}
		return "(" + args[0] + " % " + args[1] + ")", nil
	}
}

func leftCompiler(dialect string) FunctionCompiler {
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

func rightCompiler(dialect string) FunctionCompiler {
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

func nowCompiler(dialect string) FunctionCompiler {
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

func localTimeCompiler(dialect string) FunctionCompiler {
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

func localTimestampCompiler(dialect string) FunctionCompiler {
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

func postgresOnly(name, sql string, arity int) FunctionCompiler {
	return func(args []string) (string, error) {
		if len(args) != arity {
			return "", fmt.Errorf("function %q expects exactly %d argument(s)", name, arity)
		}
		return sql + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func jsonArrayLengthPostgres() FunctionCompiler {
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

func jsonTypePostgres() FunctionCompiler {
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

func jsonObjectPostgres() FunctionCompiler {
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

func jsonExistsMySQL() FunctionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "json_exists")
		}
		return "JSON_CONTAINS_PATH(" + args[0] + ", 'one', " + args[1] + ")", nil
	}
}

func jsonExistsSQLite() FunctionCompiler {
	return func(args []string) (string, error) {
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", "json_exists")
		}
		return "(json_type(" + args[0] + ", " + args[1] + ") IS NOT NULL)", nil
	}
}

func jsonTypeMySQL() FunctionCompiler {
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

func jsonObjectSimple(prefix string) FunctionCompiler {
	return func(args []string) (string, error) {
		if len(args)%2 != 0 {
			return "", fmt.Errorf("function %q expects key/value pairs", "json_object")
		}
		return prefix + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func regexOp(sql string) PredicateCompiler {
	return func(left string, right string) (string, error) {
		return left + " " + sql + " " + right, nil
	}
}

func regexLikeMySQL() PredicateCompiler {
	return func(left string, right string) (string, error) {
		return "REGEXP_LIKE(" + left + ", " + right + ")", nil
	}
}
