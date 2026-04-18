package sqladapter

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/pakasa-io/qb"
)

type functionCompiler func(args []string) (string, error)

type castCompiler func(expr string, typeName string) (string, error)

type predicateCompiler func(left string, right string) (string, error)

// Dialect controls SQL quoting, placeholders, functions, and special operators.
type Dialect interface {
	Name() string
	QuoteIdentifier(string) string
	Placeholder(int) string
	CompileFunction(name string, args []string) (string, error)
	CompileCast(expr string, typeName string) (string, error)
	CompilePredicate(op qb.Operator, left string, right string) (string, bool, error)
	Capabilities() qb.Capabilities
}

type dialectHolder struct {
	dialect Dialect
}

var defaultDialect atomic.Value

func init() {
	defaultDialect.Store(dialectHolder{dialect: PostgresDialect{}})
}

// DefaultDialect returns the process-wide default SQL dialect.
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

// SetDefaultDialectByName changes the process-wide default SQL dialect by name.
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

// DollarDialect is a backward-compatible alias for PostgreSQL-style SQL.
type DollarDialect = PostgresDialect

// QuestionDialect is a backward-compatible alias for question-mark SQL.
type QuestionDialect = SQLiteDialect

type registeredDialect struct {
	name               string
	quote              string
	placeholder        func(int) string
	functions          map[string]functionCompiler
	cast               castCompiler
	predicateCompilers map[qb.Operator]predicateCompiler
	capabilities       qb.Capabilities
}

func (d registeredDialect) Name() string {
	return d.name
}

func (d registeredDialect) QuoteIdentifier(identifier string) string {
	return quoteDottedIdentifier(identifier, d.quote)
}

func (d registeredDialect) Placeholder(index int) string {
	return d.placeholder(index)
}

func (d registeredDialect) CompileFunction(name string, args []string) (string, error) {
	compiler, ok := d.functions[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return "", qb.UnsupportedFunction("", d.name, strings.ToLower(strings.TrimSpace(name)))
	}
	return compiler(args)
}

func (d registeredDialect) CompileCast(expr string, typeName string) (string, error) {
	if d.cast == nil {
		return "", fmt.Errorf("casts are not supported for dialect %q", d.name)
	}
	return d.cast(expr, typeName)
}

func (d registeredDialect) CompilePredicate(op qb.Operator, left string, right string) (string, bool, error) {
	compiler, ok := d.predicateCompilers[op]
	if !ok {
		return "", false, nil
	}
	sql, err := compiler(left, right)
	return sql, true, err
}

func (d registeredDialect) Capabilities() qb.Capabilities {
	return d.capabilities
}

func newCapabilities(functions map[string]functionCompiler, operators ...qb.Operator) qb.Capabilities {
	functionCaps := make(map[string]struct{}, len(functions))
	for name := range functions {
		functionCaps[name] = struct{}{}
	}

	operatorCaps := make(map[qb.Operator]struct{}, len(operators))
	for _, op := range operators {
		operatorCaps[op] = struct{}{}
	}

	return qb.Capabilities{
		Functions:       functionCaps,
		Operators:       operatorCaps,
		SupportsSelect:  true,
		SupportsGroupBy: true,
		SupportsSort:    true,
		SupportsLimit:   true,
		SupportsOffset:  true,
		SupportsPage:    true,
		SupportsSize:    true,
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
