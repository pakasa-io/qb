package sqladapter

import (
	"fmt"
	"strings"
	"sync/atomic"
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
	return compileGenericFunction(name, args, false)
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
	return compileGenericFunction(name, args, true)
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
	return compileGenericFunction(name, args, false)
}

// DollarDialect is a backward-compatible alias for PostgreSQL-style SQL.
type DollarDialect = PostgresDialect

// QuestionDialect is a backward-compatible alias for question-mark SQL.
type QuestionDialect = SQLiteDialect

func compileGenericFunction(name string, args []string, mysql bool) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("function name cannot be empty")
	}

	switch strings.ToLower(name) {
	case "concat":
		if mysql {
			return "CONCAT(" + strings.Join(args, ", ") + ")", nil
		}
		return "(" + strings.Join(args, " || ") + ")", nil
	default:
		return strings.ToUpper(name) + "(" + strings.Join(args, ", ") + ")", nil
	}
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
