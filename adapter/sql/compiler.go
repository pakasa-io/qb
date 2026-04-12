package sqladapter

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pakasa-io/qb"
)

// Dialect controls identifier quoting and placeholder formatting.
type Dialect interface {
	QuoteIdentifier(string) string
	Placeholder(int) string
}

// Statement is a parameterized SQL fragment.
type Statement struct {
	SQL  string
	Args []any
}

// Compiler turns qb.Query values into SQL fragments.
type Compiler struct {
	dialect      Dialect
	transformers []qb.QueryTransformer
}

// Option customizes the compiler.
type Option func(*Compiler)

// New creates a SQL compiler with the ANSI question-mark dialect.
func New(opts ...Option) Compiler {
	compiler := Compiler{dialect: QuestionDialect{}}
	for _, opt := range opts {
		opt(&compiler)
	}
	return compiler
}

// WithDialect sets the SQL dialect used by the compiler.
func WithDialect(dialect Dialect) Option {
	return func(compiler *Compiler) {
		if dialect != nil {
			compiler.dialect = dialect
		}
	}
}

// WithQueryTransformer adds a query rewrite or validation hook.
func WithQueryTransformer(transformer qb.QueryTransformer) Option {
	return func(compiler *Compiler) {
		if transformer != nil {
			compiler.transformers = append(compiler.transformers, transformer)
		}
	}
}

// Capabilities reports which query features the compiler supports.
func (c Compiler) Capabilities() qb.Capabilities {
	return qb.Capabilities{
		Operators: map[qb.Operator]struct{}{
			qb.OpEq:       {},
			qb.OpNe:       {},
			qb.OpGt:       {},
			qb.OpGte:      {},
			qb.OpLt:       {},
			qb.OpLte:      {},
			qb.OpIn:       {},
			qb.OpNotIn:    {},
			qb.OpLike:     {},
			qb.OpContains: {},
			qb.OpPrefix:   {},
			qb.OpSuffix:   {},
			qb.OpIsNull:   {},
			qb.OpNotNull:  {},
		},
		SupportsSelect:  true,
		SupportsGroupBy: true,
		SupportsSort:    true,
		SupportsLimit:   true,
		SupportsOffset:  true,
		SupportsPage:    true,
		SupportsSize:    true,
	}
}

// Compile renders the query as a SQL fragment.
func (c Compiler) Compile(query qb.Query) (Statement, error) {
	transformed, err := qb.TransformQuery(query, c.transformers...)
	if err != nil {
		return Statement{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageCompile))
	}

	if err := c.Capabilities().Validate(qb.StageCompile, transformed); err != nil {
		return Statement{}, err
	}

	clauses := make([]string, 0, 4)
	args := make([]any, 0)
	argIndex := 1

	if len(transformed.Selects) > 0 {
		selects, err := c.compileFields(transformed.Selects, "select")
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "SELECT "+selects)
	}

	if transformed.Filter != nil {
		filter, filterArgs, nextArg, err := c.compileExpr(transformed.Filter, argIndex)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "WHERE "+filter)
		args = append(args, filterArgs...)
		argIndex = nextArg
	}

	if len(transformed.GroupBy) > 0 {
		groupBy, err := c.compileFields(transformed.GroupBy, "group_by")
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "GROUP BY "+groupBy)
	}

	if len(transformed.Sorts) > 0 {
		sorts, err := c.compileSorts(transformed.Sorts)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "ORDER BY "+sorts)
	}

	limit, offset, err := transformed.ResolvedPagination()
	if err != nil {
		return Statement{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageCompile))
	}

	if limit != nil {
		clauses = append(clauses, fmt.Sprintf("LIMIT %d", *limit))
	}

	if offset != nil {
		clauses = append(clauses, fmt.Sprintf("OFFSET %d", *offset))
	}

	return Statement{
		SQL:  strings.Join(clauses, " "),
		Args: args,
	}, nil
}

// QuestionDialect renders placeholders as '?' and quotes identifiers with
// double quotes.
type QuestionDialect struct{}

func (QuestionDialect) QuoteIdentifier(identifier string) string {
	return quoteDottedIdentifier(identifier)
}

func (QuestionDialect) Placeholder(int) string {
	return "?"
}

// DollarDialect renders PostgreSQL-style numbered placeholders.
type DollarDialect struct{}

func (DollarDialect) QuoteIdentifier(identifier string) string {
	return quoteDottedIdentifier(identifier)
}

func (DollarDialect) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (c Compiler) compileExpr(expr qb.Expr, argIndex int) (string, []any, int, error) {
	switch typed := expr.(type) {
	case qb.Predicate:
		return c.compilePredicate(typed, argIndex)
	case qb.Group:
		if len(typed.Terms) == 0 {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("empty %s group", typed.Kind),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}

		operator := " AND "
		if typed.Kind == qb.OrGroup {
			operator = " OR "
		}

		parts := make([]string, 0, len(typed.Terms))
		args := make([]any, 0)
		nextArg := argIndex

		for _, term := range typed.Terms {
			part, partArgs, updatedArg, err := c.compileExpr(term, nextArg)
			if err != nil {
				return "", nil, argIndex, err
			}

			parts = append(parts, part)
			args = append(args, partArgs...)
			nextArg = updatedArg
		}

		return "(" + strings.Join(parts, operator) + ")", args, nextArg, nil
	case qb.Negation:
		part, args, nextArg, err := c.compileExpr(typed.Expr, argIndex)
		if err != nil {
			return "", nil, argIndex, err
		}
		return "NOT (" + part + ")", args, nextArg, nil
	default:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("unsupported expression %T", expr),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
}

func (c Compiler) compilePredicate(predicate qb.Predicate, argIndex int) (string, []any, int, error) {
	if predicate.Field == "" {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("predicate field cannot be empty"),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidQuery),
			qb.WithOperator(predicate.Op),
		)
	}

	field := c.dialect.QuoteIdentifier(predicate.Field)

	switch predicate.Op {
	case qb.OpEq:
		if predicate.Value == nil {
			return field + " IS NULL", nil, argIndex, nil
		}
		return field + " = " + c.dialect.Placeholder(argIndex), []any{predicate.Value}, argIndex + 1, nil
	case qb.OpNe:
		if predicate.Value == nil {
			return field + " IS NOT NULL", nil, argIndex, nil
		}
		return field + " <> " + c.dialect.Placeholder(argIndex), []any{predicate.Value}, argIndex + 1, nil
	case qb.OpGt:
		return field + " > " + c.dialect.Placeholder(argIndex), []any{predicate.Value}, argIndex + 1, nil
	case qb.OpGte:
		return field + " >= " + c.dialect.Placeholder(argIndex), []any{predicate.Value}, argIndex + 1, nil
	case qb.OpLt:
		return field + " < " + c.dialect.Placeholder(argIndex), []any{predicate.Value}, argIndex + 1, nil
	case qb.OpLte:
		return field + " <= " + c.dialect.Placeholder(argIndex), []any{predicate.Value}, argIndex + 1, nil
	case qb.OpLike:
		return field + " LIKE " + c.dialect.Placeholder(argIndex), []any{predicate.Value}, argIndex + 1, nil
	case qb.OpContains:
		return field + " LIKE " + c.dialect.Placeholder(argIndex), []any{"%" + fmt.Sprint(predicate.Value) + "%"}, argIndex + 1, nil
	case qb.OpPrefix:
		return field + " LIKE " + c.dialect.Placeholder(argIndex), []any{fmt.Sprint(predicate.Value) + "%"}, argIndex + 1, nil
	case qb.OpSuffix:
		return field + " LIKE " + c.dialect.Placeholder(argIndex), []any{"%" + fmt.Sprint(predicate.Value)}, argIndex + 1, nil
	case qb.OpIsNull:
		return field + " IS NULL", nil, argIndex, nil
	case qb.OpNotNull:
		return field + " IS NOT NULL", nil, argIndex, nil
	case qb.OpIn, qb.OpNotIn:
		values, ok := anyList(predicate.Value)
		if !ok || len(values) == 0 {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("%s requires a non-empty list", predicate.Op),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidValue),
				qb.WithField(predicate.Field),
				qb.WithOperator(predicate.Op),
			)
		}

		placeholders := make([]string, len(values))
		args := make([]any, len(values))
		nextArg := argIndex

		for i, value := range values {
			placeholders[i] = c.dialect.Placeholder(nextArg)
			args[i] = value
			nextArg++
		}

		operator := " IN "
		if predicate.Op == qb.OpNotIn {
			operator = " NOT IN "
		}

		return field + operator + "(" + strings.Join(placeholders, ", ") + ")", args, nextArg, nil
	default:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("unsupported operator %q", predicate.Op),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeUnsupportedOperator),
			qb.WithField(predicate.Field),
			qb.WithOperator(predicate.Op),
		)
	}
}

func (c Compiler) compileSorts(sorts []qb.Sort) (string, error) {
	parts := make([]string, 0, len(sorts))

	for _, sort := range sorts {
		if sort.Field == "" {
			return "", qb.NewError(
				fmt.Errorf("sort field cannot be empty"),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}

		direction := sort.Direction
		if direction == "" {
			direction = qb.Asc
		}

		if direction != qb.Asc && direction != qb.Desc {
			return "", qb.NewError(
				fmt.Errorf("unsupported sort direction %q", sort.Direction),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
				qb.WithField(sort.Field),
			)
		}

		parts = append(parts, c.dialect.QuoteIdentifier(sort.Field)+" "+strings.ToUpper(string(direction)))
	}

	return strings.Join(parts, ", "), nil
}

func (c Compiler) compileFields(fields []string, kind string) (string, error) {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			return "", qb.NewError(
				fmt.Errorf("%s field cannot be empty", kind),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		parts = append(parts, c.dialect.QuoteIdentifier(field))
	}
	return strings.Join(parts, ", "), nil
}

func quoteDottedIdentifier(identifier string) string {
	parts := strings.Split(identifier, ".")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.ReplaceAll(part, `"`, `""`)
		parts[i] = `"` + part + `"`
	}
	return strings.Join(parts, ".")
}

func anyList(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return append([]any(nil), typed...), true
	case []string:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = item
		}
		return out, true
	default:
		if typed == nil {
			return nil, false
		}

		rv := reflect.ValueOf(typed)
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return nil, false
		}

		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return nil, false
		}

		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, true
	}
}
