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

// FunctionDialect optionally customizes how function calls render for a
// specific SQL dialect.
type FunctionDialect interface {
	CompileFunction(name string, args []string) (string, error)
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

	if len(transformed.Selects) > 0 || len(transformed.SelectExprs) > 0 {
		selects, selectArgs, nextArg, err := c.compileProjectionList(transformed.Selects, transformed.SelectExprs, "select", argIndex)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "SELECT "+selects)
		args = append(args, selectArgs...)
		argIndex = nextArg
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

	if len(transformed.GroupBy) > 0 || len(transformed.GroupExprs) > 0 {
		groupBy, groupArgs, nextArg, err := c.compileProjectionList(transformed.GroupBy, transformed.GroupExprs, "group_by", argIndex)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "GROUP BY "+groupBy)
		args = append(args, groupArgs...)
		argIndex = nextArg
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

func (QuestionDialect) CompileFunction(name string, args []string) (string, error) {
	return compileFunctionName(name, args)
}

// DollarDialect renders PostgreSQL-style numbered placeholders.
type DollarDialect struct{}

func (DollarDialect) QuoteIdentifier(identifier string) string {
	return quoteDottedIdentifier(identifier)
}

func (DollarDialect) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (DollarDialect) CompileFunction(name string, args []string) (string, error) {
	return compileFunctionName(name, args)
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
	leftExpr, field := predicateLeftExpr(predicate)
	if leftExpr == nil {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("predicate field cannot be empty"),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidQuery),
			qb.WithOperator(predicate.Op),
		)
	}

	leftSQL, leftArgs, nextArg, err := c.compileValueExpr(leftExpr, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	switch predicate.Op {
	case qb.OpEq:
		if isNilPredicateValue(predicate.Value) {
			return leftSQL + " IS NULL", leftArgs, nextArg, nil
		}
		rightSQL, rightArgs, updatedArg, err := c.compileComparableValue(predicate.Value, nextArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		return leftSQL + " = " + rightSQL, append(leftArgs, rightArgs...), updatedArg, nil
	case qb.OpNe:
		if isNilPredicateValue(predicate.Value) {
			return leftSQL + " IS NOT NULL", leftArgs, nextArg, nil
		}
		rightSQL, rightArgs, updatedArg, err := c.compileComparableValue(predicate.Value, nextArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		return leftSQL + " <> " + rightSQL, append(leftArgs, rightArgs...), updatedArg, nil
	case qb.OpGt:
		return c.compileBinaryPredicate(leftSQL, leftArgs, nextArg, predicate.Value, ">", argIndex)
	case qb.OpGte:
		return c.compileBinaryPredicate(leftSQL, leftArgs, nextArg, predicate.Value, ">=", argIndex)
	case qb.OpLt:
		return c.compileBinaryPredicate(leftSQL, leftArgs, nextArg, predicate.Value, "<", argIndex)
	case qb.OpLte:
		return c.compileBinaryPredicate(leftSQL, leftArgs, nextArg, predicate.Value, "<=", argIndex)
	case qb.OpLike:
		return c.compileLikePredicate(leftSQL, leftArgs, nextArg, predicate.Value, "", "", argIndex)
	case qb.OpContains:
		return c.compileLikePredicate(leftSQL, leftArgs, nextArg, predicate.Value, "%", "%", argIndex)
	case qb.OpPrefix:
		return c.compileLikePredicate(leftSQL, leftArgs, nextArg, predicate.Value, "", "%", argIndex)
	case qb.OpSuffix:
		return c.compileLikePredicate(leftSQL, leftArgs, nextArg, predicate.Value, "%", "", argIndex)
	case qb.OpIsNull:
		return leftSQL + " IS NULL", leftArgs, nextArg, nil
	case qb.OpNotNull:
		return leftSQL + " IS NOT NULL", leftArgs, nextArg, nil
	case qb.OpIn, qb.OpNotIn:
		values, ok := anyList(predicate.Value)
		if !ok || len(values) == 0 {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("%s requires a non-empty list", predicate.Op),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidValue),
				qb.WithField(field),
				qb.WithOperator(predicate.Op),
			)
		}

		placeholders := make([]string, len(values))
		args := append([]any(nil), leftArgs...)
		updatedArg := nextArg

		for i, value := range values {
			part, partArgs, nextValueArg, err := c.compileComparableValue(value, updatedArg)
			if err != nil {
				return "", nil, argIndex, err
			}
			placeholders[i] = part
			args = append(args, partArgs...)
			updatedArg = nextValueArg
		}

		operator := " IN "
		if predicate.Op == qb.OpNotIn {
			operator = " NOT IN "
		}

		return leftSQL + operator + "(" + strings.Join(placeholders, ", ") + ")", args, updatedArg, nil
	default:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("unsupported operator %q", predicate.Op),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeUnsupportedOperator),
			qb.WithField(field),
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

func (c Compiler) compileProjectionList(fields []string, exprs []qb.ValueExpr, kind string, argIndex int) (string, []any, int, error) {
	parts := make([]string, 0, len(fields)+len(exprs))
	args := make([]any, 0)
	nextArg := argIndex

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("%s field cannot be empty", kind),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		parts = append(parts, c.dialect.QuoteIdentifier(field))
	}

	for _, expr := range exprs {
		part, partArgs, updatedArg, err := c.compileValueExpr(expr, nextArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		parts = append(parts, part)
		args = append(args, partArgs...)
		nextArg = updatedArg
	}

	return strings.Join(parts, ", "), args, nextArg, nil
}

func (c Compiler) compileBinaryPredicate(leftSQL string, leftArgs []any, nextArg int, value any, operator string, originalArg int) (string, []any, int, error) {
	rightSQL, rightArgs, updatedArg, err := c.compileComparableValue(value, nextArg)
	if err != nil {
		return "", nil, originalArg, err
	}
	return leftSQL + " " + operator + " " + rightSQL, append(leftArgs, rightArgs...), updatedArg, nil
}

func (c Compiler) compileLikePredicate(leftSQL string, leftArgs []any, nextArg int, value any, prefix, suffix string, originalArg int) (string, []any, int, error) {
	if expr, ok := qb.AsValueExpr(value); ok {
		pattern := expr
		if prefix != "" || suffix != "" {
			pattern = qb.Call("concat", qb.Lit(prefix), expr, qb.Lit(suffix))
		}
		rightSQL, rightArgs, updatedArg, err := c.compileValueExpr(pattern, nextArg)
		if err != nil {
			return "", nil, originalArg, err
		}
		return leftSQL + " LIKE " + rightSQL, append(leftArgs, rightArgs...), updatedArg, nil
	}

	rightSQL, rightArgs, updatedArg, err := c.compileComparableValue(prefix+fmt.Sprint(value)+suffix, nextArg)
	if err != nil {
		return "", nil, originalArg, err
	}
	return leftSQL + " LIKE " + rightSQL, append(leftArgs, rightArgs...), updatedArg, nil
}

func (c Compiler) compileComparableValue(value any, argIndex int) (string, []any, int, error) {
	if expr, ok := qb.AsValueExpr(value); ok {
		return c.compileValueExpr(expr, argIndex)
	}
	return c.compileValueExpr(qb.Lit(value), argIndex)
}

func (c Compiler) compileValueExpr(expr qb.ValueExpr, argIndex int) (string, []any, int, error) {
	switch typed := expr.(type) {
	case nil:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("expression cannot be nil"),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	case qb.Ref:
		field := string(typed)
		if strings.TrimSpace(field) == "" {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("expression field cannot be empty"),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		return c.dialect.QuoteIdentifier(field), nil, argIndex, nil
	case qb.Literal:
		if typed.Value == nil {
			return "NULL", nil, argIndex, nil
		}
		return c.dialect.Placeholder(argIndex), []any{typed.Value}, argIndex + 1, nil
	case qb.CallExpr:
		name := strings.TrimSpace(typed.Name)
		if name == "" {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("function name cannot be empty"),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}

		args := make([]string, len(typed.Args))
		values := make([]any, 0)
		nextArg := argIndex
		for i, arg := range typed.Args {
			part, partArgs, updatedArg, err := c.compileValueExpr(arg, nextArg)
			if err != nil {
				return "", nil, argIndex, err
			}
			args[i] = part
			values = append(values, partArgs...)
			nextArg = updatedArg
		}

		sql, err := c.compileFunction(name, args)
		if err != nil {
			return "", nil, argIndex, err
		}

		return sql, values, nextArg, nil
	default:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("unsupported value expression %T", expr),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
}

func (c Compiler) compileFunction(name string, args []string) (string, error) {
	if dialect, ok := c.dialect.(FunctionDialect); ok {
		return dialect.CompileFunction(name, args)
	}
	return compileFunctionName(name, args)
}

func compileFunctionName(name string, args []string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("function name cannot be empty")
	}

	switch strings.ToLower(strings.TrimSpace(name)) {
	case "concat":
		return "(" + strings.Join(args, " || ") + ")", nil
	default:
		return strings.ToUpper(strings.TrimSpace(name)) + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func predicateLeftExpr(predicate qb.Predicate) (qb.ValueExpr, string) {
	if predicate.Left != nil {
		return predicate.Left, predicateFieldName(predicate)
	}
	if predicate.Field == "" {
		return nil, ""
	}
	return qb.Field(predicate.Field), predicate.Field
}

func predicateFieldName(predicate qb.Predicate) string {
	if predicate.Field != "" {
		return predicate.Field
	}
	field, ok := qb.SingleRef(predicate.Left)
	if !ok {
		return ""
	}
	return field
}

func isNilPredicateValue(value any) bool {
	if value == nil {
		return true
	}

	expr, ok := qb.AsValueExpr(value)
	if !ok {
		return false
	}

	literal, ok := expr.(qb.Literal)
	return ok && literal.Value == nil
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
