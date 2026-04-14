package sqladapter

import (
	"fmt"
	"strings"

	"github.com/pakasa-io/qb"
)

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

// New creates a SQL compiler using the current package default dialect unless
// a specific dialect override is provided.
func New(opts ...Option) Compiler {
	compiler := Compiler{dialect: DefaultDialect()}
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
			qb.OpILike:    {},
			qb.OpRegexp:   {},
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

	clauses := make([]string, 0, 5)
	args := make([]any, 0)
	argIndex := 1

	if len(transformed.Selects) > 0 {
		selects, selectArgs, nextArg, err := c.compileScalarList(transformed.Selects, "select", argIndex)
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

	if len(transformed.GroupBy) > 0 {
		groupBy, groupArgs, nextArg, err := c.compileScalarList(transformed.GroupBy, "group_by", argIndex)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "GROUP BY "+groupBy)
		args = append(args, groupArgs...)
		argIndex = nextArg
	}

	if len(transformed.Sorts) > 0 {
		sorts, sortArgs, nextArg, err := c.compileSorts(transformed.Sorts, argIndex)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "ORDER BY "+sorts)
		args = append(args, sortArgs...)
		argIndex = nextArg
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
	leftSQL, leftArgs, nextArg, err := c.compileScalar(predicate.Left, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	field := predicateField(predicate.Left)

	switch predicate.Op {
	case qb.OpEq:
		if operandIsNull(predicate.Right) {
			return leftSQL + " IS NULL", leftArgs, nextArg, nil
		}
		return c.compileBinary(leftSQL, leftArgs, predicate.Right, "=", nextArg, field, predicate.Op)
	case qb.OpNe:
		if operandIsNull(predicate.Right) {
			return leftSQL + " IS NOT NULL", leftArgs, nextArg, nil
		}
		return c.compileBinary(leftSQL, leftArgs, predicate.Right, "<>", nextArg, field, predicate.Op)
	case qb.OpGt:
		return c.compileBinary(leftSQL, leftArgs, predicate.Right, ">", nextArg, field, predicate.Op)
	case qb.OpGte:
		return c.compileBinary(leftSQL, leftArgs, predicate.Right, ">=", nextArg, field, predicate.Op)
	case qb.OpLt:
		return c.compileBinary(leftSQL, leftArgs, predicate.Right, "<", nextArg, field, predicate.Op)
	case qb.OpLte:
		return c.compileBinary(leftSQL, leftArgs, predicate.Right, "<=", nextArg, field, predicate.Op)
	case qb.OpLike:
		return c.compileLike(leftSQL, leftArgs, predicate.Right, "", "", nextArg, field, predicate.Op)
	case qb.OpILike:
		return c.compilePattern(leftSQL, leftArgs, predicate.Right, "ILIKE", nextArg, field, predicate.Op)
	case qb.OpRegexp:
		return c.compileRegexp(leftSQL, leftArgs, predicate.Right, nextArg, field, predicate.Op)
	case qb.OpContains:
		return c.compileLike(leftSQL, leftArgs, predicate.Right, "%", "%", nextArg, field, predicate.Op)
	case qb.OpPrefix:
		return c.compileLike(leftSQL, leftArgs, predicate.Right, "", "%", nextArg, field, predicate.Op)
	case qb.OpSuffix:
		return c.compileLike(leftSQL, leftArgs, predicate.Right, "%", "", nextArg, field, predicate.Op)
	case qb.OpIsNull:
		return leftSQL + " IS NULL", leftArgs, nextArg, nil
	case qb.OpNotNull:
		return leftSQL + " IS NOT NULL", leftArgs, nextArg, nil
	case qb.OpIn, qb.OpNotIn:
		list, ok := predicate.Right.(qb.ListOperand)
		if !ok || len(list.Items) == 0 {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("%s requires a non-empty list", predicate.Op),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidValue),
				qb.WithField(field),
				qb.WithOperator(predicate.Op),
			)
		}

		parts := make([]string, len(list.Items))
		args := append([]any(nil), leftArgs...)
		updatedArg := nextArg
		for i, item := range list.Items {
			part, partArgs, nextValueArg, err := c.compileScalar(item, updatedArg)
			if err != nil {
				return "", nil, argIndex, err
			}
			parts[i] = part
			args = append(args, partArgs...)
			updatedArg = nextValueArg
		}

		operator := " IN "
		if predicate.Op == qb.OpNotIn {
			operator = " NOT IN "
		}

		return leftSQL + operator + "(" + strings.Join(parts, ", ") + ")", args, updatedArg, nil
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

func (c Compiler) compileBinary(leftSQL string, leftArgs []any, operand qb.Operand, operator string, argIndex int, field string, op qb.Operator) (string, []any, int, error) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	rightSQL, rightArgs, nextArg, err := c.compileScalar(right.Expr, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	return leftSQL + " " + operator + " " + rightSQL, append(leftArgs, rightArgs...), nextArg, nil
}

func (c Compiler) compileLike(leftSQL string, leftArgs []any, operand qb.Operand, prefix, suffix string, argIndex int, field string, op qb.Operator) (string, []any, int, error) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	scalar := right.Expr
	if prefix != "" || suffix != "" {
		if literal, ok := scalar.(qb.Literal); ok {
			scalar = qb.V(prefix + fmt.Sprint(literal.Value) + suffix)
		} else {
			parts := make([]any, 0, 3)
			if prefix != "" {
				parts = append(parts, qb.V(prefix))
			}
			parts = append(parts, scalar)
			if suffix != "" {
				parts = append(parts, qb.V(suffix))
			}
			scalar = qb.Func("concat", parts...)
		}
	}

	rightSQL, rightArgs, nextArg, err := c.compileScalar(scalar, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	return leftSQL + " LIKE " + rightSQL, append(leftArgs, rightArgs...), nextArg, nil
}

func (c Compiler) compilePattern(leftSQL string, leftArgs []any, operand qb.Operand, operator string, argIndex int, field string, op qb.Operator) (string, []any, int, error) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	if op == qb.OpILike && c.dialect.Name() != "postgres" {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("operator %q is not supported by the %s dialect", op, c.dialect.Name()),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeUnsupportedFeature),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	rightSQL, rightArgs, nextArg, err := c.compileScalar(right.Expr, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	return leftSQL + " " + operator + " " + rightSQL, append(leftArgs, rightArgs...), nextArg, nil
}

func (c Compiler) compileRegexp(leftSQL string, leftArgs []any, operand qb.Operand, argIndex int, field string, op qb.Operator) (string, []any, int, error) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	rightSQL, rightArgs, nextArg, err := c.compileScalar(right.Expr, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	switch c.dialect.Name() {
	case "postgres":
		return leftSQL + " ~ " + rightSQL, append(leftArgs, rightArgs...), nextArg, nil
	case "mysql":
		return "REGEXP_LIKE(" + leftSQL + ", " + rightSQL + ")", append(leftArgs, rightArgs...), nextArg, nil
	default:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("operator %q is not supported by the %s dialect", op, c.dialect.Name()),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeUnsupportedFeature),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}
}

func (c Compiler) compileScalarList(values []qb.Scalar, kind string, argIndex int) (string, []any, int, error) {
	parts := make([]string, 0, len(values))
	args := make([]any, 0)
	nextArg := argIndex

	for _, value := range values {
		part, partArgs, updatedArg, err := c.compileScalar(value, nextArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		if part == "" {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("%s expression cannot be empty", kind),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		parts = append(parts, part)
		args = append(args, partArgs...)
		nextArg = updatedArg
	}

	return strings.Join(parts, ", "), args, nextArg, nil
}

func (c Compiler) compileSorts(values []qb.Sort, argIndex int) (string, []any, int, error) {
	parts := make([]string, 0, len(values))
	args := make([]any, 0)
	nextArg := argIndex

	for _, sort := range values {
		if sort.Expr == nil {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("sort expression cannot be nil"),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}

		direction := sort.Direction
		if direction == "" {
			direction = qb.Asc
		}
		if direction != qb.Asc && direction != qb.Desc {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("unsupported sort direction %q", sort.Direction),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
				qb.WithField(predicateField(sort.Expr)),
			)
		}

		part, partArgs, updatedArg, err := c.compileScalar(sort.Expr, nextArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		parts = append(parts, part+" "+strings.ToUpper(string(direction)))
		args = append(args, partArgs...)
		nextArg = updatedArg
	}

	return strings.Join(parts, ", "), args, nextArg, nil
}

func (c Compiler) compileScalar(expr qb.Scalar, argIndex int) (string, []any, int, error) {
	switch typed := expr.(type) {
	case nil:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("scalar expression cannot be nil"),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	case qb.Ref:
		if strings.TrimSpace(typed.Name) == "" {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("field reference cannot be empty"),
				qb.WithStage(qb.StageCompile),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		return c.dialect.QuoteIdentifier(typed.Name), nil, argIndex, nil
	case qb.Literal:
		if typed.Value == nil {
			return "NULL", nil, argIndex, nil
		}
		return c.dialect.Placeholder(argIndex), []any{typed.Value}, argIndex + 1, nil
	case qb.Call:
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
			part, partArgs, updatedArg, err := c.compileScalar(arg, nextArg)
			if err != nil {
				return "", nil, argIndex, err
			}
			args[i] = part
			values = append(values, partArgs...)
			nextArg = updatedArg
		}

		sql, err := c.dialect.CompileFunction(name, args)
		if err != nil {
			return "", nil, argIndex, qb.WrapError(
				err,
				qb.WithDefaultStage(qb.StageCompile),
				qb.WithDefaultCode(qb.CodeInvalidQuery),
			)
		}
		return sql, values, nextArg, nil
	default:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("unsupported scalar expression %T", expr),
			qb.WithStage(qb.StageCompile),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
}

func operandIsNull(operand qb.Operand) bool {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return false
	}

	literal, ok := right.Expr.(qb.Literal)
	return ok && literal.Value == nil
}

func predicateField(expr qb.Scalar) string {
	if expr == nil {
		return ""
	}

	field, ok := qb.SingleRef(expr)
	if !ok {
		return ""
	}
	return field
}
