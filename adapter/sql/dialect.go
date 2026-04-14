package sqladapter

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/pakasa-io/qb"
)

// Dialect controls identifier quoting, placeholders, and function rendering.
type Dialect interface {
	Name() string
	QuoteIdentifier(string) string
	Placeholder(int) string
	CompileFunction(name string, args []string) (string, error)
}

type dialectHolder struct {
	dialect Dialect
}

var defaultDialect atomic.Value

func init() {
	defaultDialect.Store(dialectHolder{dialect: PostgresDialect{}})
}

// DefaultDialect returns the current process-wide default SQL dialect.
func DefaultDialect() Dialect {
	if holder, ok := defaultDialect.Load().(dialectHolder); ok && holder.dialect != nil {
		return holder.dialect
	}
	return PostgresDialect{}
}

// SetDefaultDialect changes the process-wide default SQL dialect.
func SetDefaultDialect(dialect Dialect) {
	if dialect == nil {
		return
	}
	defaultDialect.Store(dialectHolder{dialect: dialect})
}

// SetDefaultDialectByName changes the process-wide default SQL dialect using a
// known dialect name such as "postgres", "mysql", or "sqlite".
func SetDefaultDialectByName(name string) error {
	dialect, err := LookupDialect(name)
	if err != nil {
		return err
	}

	SetDefaultDialect(dialect)
	return nil
}

// LookupDialect resolves a known dialect name.
func LookupDialect(name string) (Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "postgres", "postgresql", "pg":
		return PostgresDialect{}, nil
	case "mysql":
		return MySQLDialect{}, nil
	case "sqlite", "sqlite3":
		return SQLiteDialect{}, nil
	default:
		return nil, fmt.Errorf("unsupported sql dialect %q", name)
	}
}

// MustDialect resolves a known dialect name or panics.
func MustDialect(name string) Dialect {
	dialect, err := LookupDialect(name)
	if err != nil {
		panic(err)
	}
	return dialect
}

// PostgresDialect targets PostgreSQL v17+.
type PostgresDialect struct{}

func (PostgresDialect) Name() string { return "postgres" }

func (PostgresDialect) QuoteIdentifier(identifier string) string {
	return quoteDottedIdentifier(identifier, `"`)
}

func (PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (PostgresDialect) CompileFunction(name string, args []string) (string, error) {
	return compileGenericFunction("postgres", name, args)
}

// MySQLDialect targets MySQL v8+.
type MySQLDialect struct{}

func (MySQLDialect) Name() string { return "mysql" }

func (MySQLDialect) QuoteIdentifier(identifier string) string {
	return quoteDottedIdentifier(identifier, "`")
}

func (MySQLDialect) Placeholder(int) string {
	return "?"
}

func (MySQLDialect) CompileFunction(name string, args []string) (string, error) {
	return compileGenericFunction("mysql", name, args)
}

// SQLiteDialect targets current SQLite releases.
type SQLiteDialect struct{}

func (SQLiteDialect) Name() string { return "sqlite" }

func (SQLiteDialect) QuoteIdentifier(identifier string) string {
	return quoteDottedIdentifier(identifier, `"`)
}

func (SQLiteDialect) Placeholder(int) string {
	return "?"
}

func (SQLiteDialect) CompileFunction(name string, args []string) (string, error) {
	return compileGenericFunction("sqlite", name, args)
}

// DollarDialect is a backward-compatible alias for PostgreSQL-style SQL.
type DollarDialect = PostgresDialect

// QuestionDialect is a backward-compatible alias for question-mark SQL.
type QuestionDialect = SQLiteDialect

func compileGenericFunction(dialect string, name string, args []string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("function name cannot be empty")
	}

	switch strings.ToLower(name) {
	case "count":
		switch len(args) {
		case 0:
			return "COUNT(*)", nil
		case 1:
			return "COUNT(" + args[0] + ")", nil
		default:
			return "", fmt.Errorf("function %q expects zero or one argument", name)
		}
	case "sum", "avg", "min", "max", "lower", "upper", "trim", "ltrim", "rtrim", "length", "abs", "date":
		if len(args) != 1 {
			return "", fmt.Errorf("function %q expects exactly one argument", name)
		}
		return strings.ToUpper(name) + "(" + args[0] + ")", nil
	case "now":
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", name)
		}
		if dialect == "sqlite" {
			return "CURRENT_TIMESTAMP", nil
		}
		return "NOW()", nil
	case "concat":
		if len(args) == 0 {
			return "", fmt.Errorf("function %q expects at least one argument", name)
		}
		if dialect == "mysql" {
			return "CONCAT(" + strings.Join(args, ", ") + ")", nil
		}
		return "(" + strings.Join(args, " || ") + ")", nil
	case "substring":
		if len(args) != 2 && len(args) != 3 {
			return "", fmt.Errorf("function %q expects two or three arguments", name)
		}
		if dialect == "sqlite" {
			return "SUBSTR(" + strings.Join(args, ", ") + ")", nil
		}
		return "SUBSTRING(" + strings.Join(args, ", ") + ")", nil
	case "replace":
		if len(args) != 3 {
			return "", fmt.Errorf("function %q expects exactly three arguments", name)
		}
		return "REPLACE(" + strings.Join(args, ", ") + ")", nil
	case "coalesce":
		if len(args) == 0 {
			return "", fmt.Errorf("function %q expects at least one argument", name)
		}
		return "COALESCE(" + strings.Join(args, ", ") + ")", nil
	case "nullif":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		return "NULLIF(" + strings.Join(args, ", ") + ")", nil
	case "ceil":
		if len(args) != 1 {
			return "", fmt.Errorf("function %q expects exactly one argument", name)
		}
		if dialect == "sqlite" {
			return "", unsupportedFunctionError(dialect, name)
		}
		return "CEIL(" + args[0] + ")", nil
	case "floor":
		if len(args) != 1 {
			return "", fmt.Errorf("function %q expects exactly one argument", name)
		}
		if dialect == "sqlite" {
			return "", unsupportedFunctionError(dialect, name)
		}
		return "FLOOR(" + args[0] + ")", nil
	case "mod":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		return "(" + args[0] + " % " + args[1] + ")", nil
	case "round":
		if len(args) != 1 && len(args) != 2 {
			return "", fmt.Errorf("function %q expects one or two arguments", name)
		}
		return "ROUND(" + strings.Join(args, ", ") + ")", nil
	case "left":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		if dialect == "sqlite" {
			return "SUBSTR(" + args[0] + ", 1, " + args[1] + ")", nil
		}
		return "LEFT(" + strings.Join(args, ", ") + ")", nil
	case "right":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		if dialect == "sqlite" {
			return "SUBSTR(" + args[0] + ", -" + args[1] + ")", nil
		}
		return "RIGHT(" + strings.Join(args, ", ") + ")", nil
	case "current_date":
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", name)
		}
		return "CURRENT_DATE", nil
	case "localtime":
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", name)
		}
		if dialect == "sqlite" {
			return "TIME('now', 'localtime')", nil
		}
		return "LOCALTIME", nil
	case "current_time":
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", name)
		}
		return "CURRENT_TIME", nil
	case "localtimestamp":
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", name)
		}
		if dialect == "sqlite" {
			return "DATETIME('now', 'localtime')", nil
		}
		return "LOCALTIMESTAMP", nil
	case "current_timestamp":
		if len(args) != 0 {
			return "", fmt.Errorf("function %q expects no arguments", name)
		}
		return "CURRENT_TIMESTAMP", nil
	case "date_trunc":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		if dialect != "postgres" {
			return "", unsupportedFunctionError(dialect, name)
		}
		return "DATE_TRUNC(" + strings.Join(args, ", ") + ")", nil
	case "extract":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		if dialect != "postgres" {
			return "", unsupportedFunctionError(dialect, name)
		}
		return "DATE_PART(" + strings.Join(args, ", ") + ")", nil
	case "date_bin":
		if len(args) != 3 {
			return "", fmt.Errorf("function %q expects exactly three arguments", name)
		}
		if dialect != "postgres" {
			return "", unsupportedFunctionError(dialect, name)
		}
		return "DATE_BIN(" + strings.Join(args, ", ") + ")", nil
	case "json_extract":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		switch dialect {
		case "postgres":
			return "JSON_QUERY(" + strings.Join(args, ", ") + ")", nil
		case "mysql":
			return "JSON_EXTRACT(" + strings.Join(args, ", ") + ")", nil
		case "sqlite":
			return "json_extract(" + strings.Join(args, ", ") + ")", nil
		default:
			return "", unsupportedFunctionError(dialect, name)
		}
	case "json_query":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		switch dialect {
		case "postgres":
			return "JSON_QUERY(" + strings.Join(args, ", ") + ")", nil
		case "mysql":
			return "JSON_EXTRACT(" + strings.Join(args, ", ") + ")", nil
		case "sqlite":
			return "json_extract(" + strings.Join(args, ", ") + ")", nil
		default:
			return "", unsupportedFunctionError(dialect, name)
		}
	case "json_value":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		switch dialect {
		case "postgres", "mysql":
			return "JSON_VALUE(" + strings.Join(args, ", ") + ")", nil
		case "sqlite":
			return "json_extract(" + strings.Join(args, ", ") + ")", nil
		default:
			return "", unsupportedFunctionError(dialect, name)
		}
	case "json_exists":
		if len(args) != 2 {
			return "", fmt.Errorf("function %q expects exactly two arguments", name)
		}
		switch dialect {
		case "postgres":
			return "JSON_EXISTS(" + strings.Join(args, ", ") + ")", nil
		case "mysql":
			return "JSON_CONTAINS_PATH(" + args[0] + ", 'one', " + args[1] + ")", nil
		case "sqlite":
			return "(json_type(" + args[0] + ", " + args[1] + ") IS NOT NULL)", nil
		default:
			return "", unsupportedFunctionError(dialect, name)
		}
	case "json_array_length":
		if len(args) != 1 && len(args) != 2 {
			return "", fmt.Errorf("function %q expects one or two arguments", name)
		}
		switch dialect {
		case "postgres":
			if len(args) == 1 {
				return "JSON_ARRAY_LENGTH(" + args[0] + ")", nil
			}
			return "JSON_ARRAY_LENGTH(JSON_QUERY(" + args[0] + ", " + args[1] + "))", nil
		case "mysql":
			return "JSON_LENGTH(" + strings.Join(args, ", ") + ")", nil
		case "sqlite":
			return "json_array_length(" + strings.Join(args, ", ") + ")", nil
		default:
			return "", unsupportedFunctionError(dialect, name)
		}
	case "json_type":
		if len(args) != 1 && len(args) != 2 {
			return "", fmt.Errorf("function %q expects one or two arguments", name)
		}
		switch dialect {
		case "postgres":
			if len(args) == 1 {
				return "JSON_TYPEOF(" + args[0] + ")", nil
			}
			return "JSON_TYPEOF(JSON_QUERY(" + args[0] + ", " + args[1] + "))", nil
		case "mysql":
			if len(args) == 1 {
				return "JSON_TYPE(" + args[0] + ")", nil
			}
			return "JSON_TYPE(JSON_EXTRACT(" + args[0] + ", " + args[1] + "))", nil
		case "sqlite":
			return "json_type(" + strings.Join(args, ", ") + ")", nil
		default:
			return "", unsupportedFunctionError(dialect, name)
		}
	case "json_array":
		if dialect == "sqlite" {
			return "json_array(" + strings.Join(args, ", ") + ")", nil
		}
		return "JSON_ARRAY(" + strings.Join(args, ", ") + ")", nil
	case "json_object":
		if len(args)%2 != 0 {
			return "", fmt.Errorf("function %q expects key/value pairs", name)
		}
		switch dialect {
		case "postgres":
			if len(args) == 0 {
				return "JSON_OBJECT()", nil
			}
			pairs := make([]string, 0, len(args)/2)
			for i := 0; i < len(args); i += 2 {
				pairs = append(pairs, args[i]+" VALUE "+args[i+1])
			}
			return "JSON_OBJECT(" + strings.Join(pairs, ", ") + ")", nil
		case "mysql":
			return "JSON_OBJECT(" + strings.Join(args, ", ") + ")", nil
		case "sqlite":
			return "json_object(" + strings.Join(args, ", ") + ")", nil
		default:
			return "", unsupportedFunctionError(dialect, name)
		}
	default:
		return strings.ToUpper(name) + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func unsupportedFunctionError(dialect, name string) error {
	return qb.NewError(
		fmt.Errorf("function %q is not supported by the %s dialect", name, dialect),
		qb.WithCode(qb.CodeUnsupportedFeature),
	)
}

func quoteDottedIdentifier(identifier, quote string) string {
	parts := strings.Split(identifier, ".")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.ReplaceAll(part, quote, quote+quote)
		parts[i] = quote + part + quote
	}
	return strings.Join(parts, ".")
}
