package sqlrender

import (
	"fmt"
	"strings"

	"github.com/pakasa-io/qb"
)

type Dialect interface {
	Name() string
	QuoteIdentifier(string) string
	Placeholder(int) string
	CompileFunction(name string, args []string) (string, error)
	CompileCast(expr string, typeName string) (string, error)
	CompilePredicate(op qb.Operator, left string, right string) (string, bool, error)
	Capabilities() qb.Capabilities
}

type Renderer struct {
	dialect Dialect
	stage   qb.ErrorStage
}

func New(dialect Dialect, stage qb.ErrorStage) Renderer {
	return Renderer{dialect: dialect, stage: stage}
}

func (r Renderer) Dialect() Dialect {
	return r.dialect
}

func (r Renderer) Capabilities() qb.Capabilities {
	return r.dialect.Capabilities()
}

func (r Renderer) CompileProjectionList(values []qb.Projection, argIndex int) (string, []any, int, error) {
	parts := make([]string, 0, len(values))
	args := make([]any, 0)
	nextArg := argIndex

	for _, value := range values {
		part, partArgs, updatedArg, err := r.compileProjection(value, nextArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		parts = append(parts, part)
		args = append(args, partArgs...)
		nextArg = updatedArg
	}

	return strings.Join(parts, ", "), args, nextArg, nil
}

func (r Renderer) CompileScalarList(values []qb.Scalar, kind string, argIndex int) (string, []any, int, error) {
	parts := make([]string, 0, len(values))
	args := make([]any, 0)
	nextArg := argIndex

	for _, value := range values {
		part, partArgs, updatedArg, err := r.CompileScalar(value, nextArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		if part == "" {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("%s expression cannot be empty", kind),
				qb.WithStage(r.stage),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		parts = append(parts, part)
		args = append(args, partArgs...)
		nextArg = updatedArg
	}

	return strings.Join(parts, ", "), args, nextArg, nil
}

func (r Renderer) CompileSorts(values []qb.Sort, argIndex int) (string, []any, int, error) {
	parts := make([]string, 0, len(values))
	args := make([]any, 0)
	nextArg := argIndex

	for _, sort := range values {
		if sort.Expr == nil {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("sort expression cannot be nil"),
				qb.WithStage(r.stage),
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
				qb.WithStage(r.stage),
				qb.WithCode(qb.CodeInvalidQuery),
				qb.WithField(predicateField(sort.Expr)),
			)
		}

		part, partArgs, updatedArg, err := r.CompileScalar(sort.Expr, nextArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		parts = append(parts, part+" "+strings.ToUpper(string(direction)))
		args = append(args, partArgs...)
		nextArg = updatedArg
	}

	return strings.Join(parts, ", "), args, nextArg, nil
}

func (r Renderer) CompileExpr(expr qb.Expr, argIndex int) (string, []any, int, error) {
	switch typed := expr.(type) {
	case qb.Predicate:
		return r.compilePredicate(typed, argIndex)
	case qb.Group:
		if len(typed.Terms) == 0 {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("empty %s group", typed.Kind),
				qb.WithStage(r.stage),
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
			part, partArgs, updatedArg, err := r.CompileExpr(term, nextArg)
			if err != nil {
				return "", nil, argIndex, err
			}
			parts = append(parts, part)
			args = append(args, partArgs...)
			nextArg = updatedArg
		}
		return "(" + strings.Join(parts, operator) + ")", args, nextArg, nil
	case qb.Negation:
		part, args, nextArg, err := r.CompileExpr(typed.Expr, argIndex)
		if err != nil {
			return "", nil, argIndex, err
		}
		return "NOT (" + part + ")", args, nextArg, nil
	default:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("unsupported expression %T", expr),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
}

func (r Renderer) CompileScalar(expr qb.Scalar, argIndex int) (string, []any, int, error) {
	switch typed := expr.(type) {
	case nil:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("scalar expression cannot be nil"),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	case qb.Ref:
		if strings.TrimSpace(typed.Name) == "" {
			return "", nil, argIndex, qb.NewError(
				fmt.Errorf("field reference cannot be empty"),
				qb.WithStage(r.stage),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		return r.dialect.QuoteIdentifier(typed.Name), nil, argIndex, nil
	case qb.Literal:
		if typed.Value == nil {
			return "NULL", nil, argIndex, nil
		}
		return r.dialect.Placeholder(argIndex), []any{typed.Value}, argIndex + 1, nil
	case qb.Call:
		args := make([]string, len(typed.Args))
		values := make([]any, 0)
		nextArg := argIndex
		for i, arg := range typed.Args {
			part, partArgs, updatedArg, err := r.CompileScalar(arg, nextArg)
			if err != nil {
				return "", nil, argIndex, err
			}
			args[i] = part
			values = append(values, partArgs...)
			nextArg = updatedArg
		}

		sql, err := r.dialect.CompileFunction(typed.Name, args)
		if err != nil {
			return "", nil, argIndex, qb.WrapError(
				err,
				qb.WithDefaultStage(r.stage),
				qb.WithDefaultCode(qb.CodeInvalidQuery),
			)
		}
		return sql, values, nextArg, nil
	case qb.Cast:
		part, partArgs, nextArg, err := r.CompileScalar(typed.Expr, argIndex)
		if err != nil {
			return "", nil, argIndex, err
		}
		sql, err := r.dialect.CompileCast(part, typed.Type)
		if err != nil {
			return "", nil, argIndex, qb.WrapError(
				err,
				qb.WithDefaultStage(r.stage),
				qb.WithDefaultCode(qb.CodeInvalidQuery),
			)
		}
		return sql, partArgs, nextArg, nil
	default:
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("unsupported scalar expression %T", expr),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
}

func (r Renderer) compileProjection(value qb.Projection, argIndex int) (string, []any, int, error) {
	if value.Expr == nil {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("select expression cannot be nil"),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}

	sql, args, nextArg, err := r.CompileScalar(value.Expr, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	if strings.TrimSpace(value.Alias) != "" {
		sql += " AS " + r.dialect.QuoteIdentifier(value.Alias)
	}

	return sql, args, nextArg, nil
}

func (r Renderer) compilePredicate(predicate qb.Predicate, argIndex int) (string, []any, int, error) {
	leftSQL, leftArgs, nextArg, err := r.CompileScalar(predicate.Left, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	field := predicateField(predicate.Left)

	switch predicate.Op {
	case qb.OpEq:
		if operandIsNull(predicate.Right) {
			return leftSQL + " IS NULL", leftArgs, nextArg, nil
		}
		return r.compileBinary(leftSQL, leftArgs, predicate.Right, "=", nextArg, field, predicate.Op)
	case qb.OpNe:
		if operandIsNull(predicate.Right) {
			return leftSQL + " IS NOT NULL", leftArgs, nextArg, nil
		}
		return r.compileBinary(leftSQL, leftArgs, predicate.Right, "<>", nextArg, field, predicate.Op)
	case qb.OpGt:
		return r.compileBinary(leftSQL, leftArgs, predicate.Right, ">", nextArg, field, predicate.Op)
	case qb.OpGte:
		return r.compileBinary(leftSQL, leftArgs, predicate.Right, ">=", nextArg, field, predicate.Op)
	case qb.OpLt:
		return r.compileBinary(leftSQL, leftArgs, predicate.Right, "<", nextArg, field, predicate.Op)
	case qb.OpLte:
		return r.compileBinary(leftSQL, leftArgs, predicate.Right, "<=", nextArg, field, predicate.Op)
	case qb.OpLike:
		return r.compileLike(leftSQL, leftArgs, predicate.Right, "", "", nextArg, field, predicate.Op)
	case qb.OpContains:
		return r.compileLike(leftSQL, leftArgs, predicate.Right, "%", "%", nextArg, field, predicate.Op)
	case qb.OpPrefix:
		return r.compileLike(leftSQL, leftArgs, predicate.Right, "", "%", nextArg, field, predicate.Op)
	case qb.OpSuffix:
		return r.compileLike(leftSQL, leftArgs, predicate.Right, "%", "", nextArg, field, predicate.Op)
	case qb.OpIsNull:
		return leftSQL + " IS NULL", leftArgs, nextArg, nil
	case qb.OpNotNull:
		return leftSQL + " IS NOT NULL", leftArgs, nextArg, nil
	case qb.OpIn, qb.OpNotIn:
		return r.compileIn(leftSQL, leftArgs, predicate.Right, nextArg, field, predicate.Op)
	default:
		return r.compileDialectPredicate(leftSQL, leftArgs, predicate.Right, nextArg, field, predicate.Op)
	}
}

func (r Renderer) compileBinary(leftSQL string, leftArgs []any, operand qb.Operand, operator string, argIndex int, field string, op qb.Operator) (string, []any, int, error) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	rightSQL, rightArgs, nextArg, err := r.CompileScalar(right.Expr, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	return leftSQL + " " + operator + " " + rightSQL, append(leftArgs, rightArgs...), nextArg, nil
}

func (r Renderer) compileIn(leftSQL string, leftArgs []any, operand qb.Operand, argIndex int, field string, op qb.Operator) (string, []any, int, error) {
	list, ok := operand.(qb.ListOperand)
	if !ok || len(list.Items) == 0 {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("%s requires a non-empty list", op),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	parts := make([]string, len(list.Items))
	args := append([]any(nil), leftArgs...)
	updatedArg := argIndex
	for i, item := range list.Items {
		part, partArgs, nextValueArg, err := r.CompileScalar(item, updatedArg)
		if err != nil {
			return "", nil, argIndex, err
		}
		parts[i] = part
		args = append(args, partArgs...)
		updatedArg = nextValueArg
	}

	operator := " IN "
	if op == qb.OpNotIn {
		operator = " NOT IN "
	}

	return leftSQL + operator + "(" + strings.Join(parts, ", ") + ")", args, updatedArg, nil
}

func (r Renderer) compileLike(leftSQL string, leftArgs []any, operand qb.Operand, prefix, suffix string, argIndex int, field string, op qb.Operator) (string, []any, int, error) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(r.stage),
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

	rightSQL, rightArgs, nextArg, err := r.CompileScalar(scalar, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	return leftSQL + " LIKE " + rightSQL, append(leftArgs, rightArgs...), nextArg, nil
}

func (r Renderer) compileDialectPredicate(leftSQL string, leftArgs []any, operand qb.Operand, argIndex int, field string, op qb.Operator) (string, []any, int, error) {
	if !r.dialect.Capabilities().SupportsOperator(op) {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("operator %q is not supported", op),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeUnsupportedOperator),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	rightSQL, rightArgs, nextArg, err := r.CompileScalar(right.Expr, argIndex)
	if err != nil {
		return "", nil, argIndex, err
	}

	sql, handled, err := r.dialect.CompilePredicate(op, leftSQL, rightSQL)
	if err != nil {
		return "", nil, argIndex, qb.WrapError(
			err,
			qb.WithDefaultStage(r.stage),
			qb.WithDefaultCode(qb.CodeInvalidQuery),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}
	if !handled {
		return "", nil, argIndex, qb.NewError(
			fmt.Errorf("operator %q does not have a renderer for dialect %s", op, r.dialect.Name()),
			qb.WithStage(r.stage),
			qb.WithCode(qb.CodeUnsupportedOperator),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}
	return sql, append(leftArgs, rightArgs...), nextArg, nil
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
