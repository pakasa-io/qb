package gormadapter

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapter/sql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PredicateCompiler builds a GORM expression for a simple field predicate.
type PredicateCompiler func(field string, predicate qb.Predicate) (clause.Expression, error)

// Adapter applies qb.Query values to a *gorm.DB chain.
type Adapter struct {
	transformers      []qb.QueryTransformer
	predicateCompiler map[qb.Operator]PredicateCompiler
}

// Option customizes the adapter.
type Option func(*Adapter)

// New creates a GORM adapter with the default operator compilers.
func New(opts ...Option) Adapter {
	adapter := Adapter{
		predicateCompiler: defaultPredicateCompilers(),
	}

	for _, opt := range opts {
		opt(&adapter)
	}

	return adapter
}

// WithQueryTransformer adds a rewrite or validation hook.
func WithQueryTransformer(transformer qb.QueryTransformer) Option {
	return func(adapter *Adapter) {
		if transformer != nil {
			adapter.transformers = append(adapter.transformers, transformer)
		}
	}
}

// WithPredicateCompiler overrides the compiler used for a specific operator.
func WithPredicateCompiler(op qb.Operator, compiler PredicateCompiler) Option {
	return func(adapter *Adapter) {
		if compiler != nil {
			adapter.predicateCompiler[op] = compiler
		}
	}
}

// Capabilities reports which query features the adapter supports.
func (a Adapter) Capabilities() qb.Capabilities {
	operators := make(map[qb.Operator]struct{}, len(a.predicateCompiler))
	for op := range a.predicateCompiler {
		operators[op] = struct{}{}
	}

	return qb.Capabilities{
		Operators:       operators,
		SupportsSelect:  true,
		SupportsInclude: true,
		SupportsGroupBy: true,
		SupportsSort:    true,
		SupportsLimit:   true,
		SupportsOffset:  true,
		SupportsPage:    true,
		SupportsSize:    true,
	}
}

// Apply applies the query to a GORM chain and returns the updated chain.
func (a Adapter) Apply(db *gorm.DB, query qb.Query) (*gorm.DB, error) {
	if db == nil {
		return nil, qb.NewError(
			fmt.Errorf("db cannot be nil"),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeInvalidInput),
		)
	}

	transformed, err := qb.TransformQuery(query, a.transformers...)
	if err != nil {
		return nil, qb.WrapError(err, qb.WithDefaultStage(qb.StageApply))
	}

	if err := a.Capabilities().Validate(qb.StageApply, transformed); err != nil {
		return nil, err
	}

	dialect := lookupDialect(db.Dialector.Name())

	if transformed.Filter != nil {
		filter, err := a.compileExpr(transformed.Filter, dialect)
		if err != nil {
			return nil, err
		}
		db = db.Where(filter)
	}

	if len(transformed.Selects) > 0 {
		if refsOnly(transformed.Selects) {
			fields := make([]string, len(transformed.Selects))
			for i, item := range transformed.Selects {
				fields[i] = item.(qb.Ref).Name
			}
			db = db.Select(fields)
		} else {
			sql, vars, err := compileScalarList(transformed.Selects, dialect, qb.StageApply, "select")
			if err != nil {
				return nil, err
			}
			db = db.Select(sql, vars...)
		}
	}

	for _, include := range transformed.Includes {
		if strings.TrimSpace(include) == "" {
			return nil, qb.NewError(
				fmt.Errorf("include cannot be empty"),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		db = db.Preload(include)
	}

	if len(transformed.GroupBy) > 0 {
		if refsOnly(transformed.GroupBy) {
			columns := make([]clause.Column, 0, len(transformed.GroupBy))
			for _, item := range transformed.GroupBy {
				ref := item.(qb.Ref)
				columns = append(columns, Column(ref.Name))
			}
			db.Statement.AddClause(clause.GroupBy{Columns: columns})
		} else {
			sql, vars, err := compileScalarList(transformed.GroupBy, dialect, qb.StageApply, "group_by")
			if err != nil {
				return nil, err
			}
			if len(vars) > 0 {
				return nil, qb.NewError(
					fmt.Errorf("group_by expressions cannot contain parameterized literals"),
					qb.WithStage(qb.StageApply),
					qb.WithCode(qb.CodeUnsupportedFeature),
				)
			}
			db = db.Group(sql)
		}
	}

	for _, sort := range transformed.Sorts {
		if sort.Expr == nil {
			return nil, qb.NewError(
				fmt.Errorf("sort expression cannot be nil"),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}

		direction := sort.Direction
		if direction == "" {
			direction = qb.Asc
		}
		if direction != qb.Asc && direction != qb.Desc {
			return nil, qb.NewError(
				fmt.Errorf("unsupported sort direction %q", sort.Direction),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
				qb.WithField(predicateField(sort.Expr)),
			)
		}

		if ref, ok := sort.Expr.(qb.Ref); ok {
			db = db.Order(clause.OrderByColumn{
				Column: Column(ref.Name),
				Desc:   direction == qb.Desc,
			})
			continue
		}

		sql, vars, err := compileScalar(sort.Expr, dialect, qb.StageApply)
		if err != nil {
			return nil, err
		}
		orderSQL := sql + " " + strings.ToUpper(string(direction))
		db = db.Order(clause.OrderBy{
			Expression: clause.Expr{
				SQL:                orderSQL,
				Vars:               vars,
				WithoutParentheses: true,
			},
		})
	}

	limit, offset, err := transformed.ResolvedPagination()
	if err != nil {
		return nil, qb.WrapError(err, qb.WithDefaultStage(qb.StageApply))
	}

	if limit != nil {
		db = db.Limit(*limit)
	}

	if offset != nil {
		db = db.Offset(*offset)
	}

	return db, nil
}

// Scope returns an idiomatic GORM scope.
func (a Adapter) Scope(query qb.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		tx, err := a.Apply(db, query)
		if err != nil {
			if db == nil {
				return nil
			}
			tx = db.Session(&gorm.Session{})
			_ = tx.AddError(err)
			return tx
		}

		return tx
	}
}

// Column converts a qb field reference into a GORM column reference.
func Column(field string) clause.Column {
	parts := strings.Split(field, ".")
	if len(parts) == 1 {
		return clause.Column{Name: field}
	}

	return clause.Column{
		Table: strings.Join(parts[:len(parts)-1], "."),
		Name:  parts[len(parts)-1],
	}
}

func (a Adapter) compileExpr(expr qb.Expr, dialect sqladapter.Dialect) (clause.Expression, error) {
	switch typed := expr.(type) {
	case qb.Predicate:
		return a.compilePredicate(typed, dialect)
	case qb.Group:
		if len(typed.Terms) == 0 {
			return nil, qb.NewError(
				fmt.Errorf("empty %s group", typed.Kind),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}

		exprs := make([]clause.Expression, 0, len(typed.Terms))
		for _, term := range typed.Terms {
			compiled, err := a.compileExpr(term, dialect)
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, compiled)
		}

		switch typed.Kind {
		case qb.AndGroup:
			return clause.And(exprs...), nil
		case qb.OrGroup:
			return clause.Or(exprs...), nil
		default:
			return nil, qb.NewError(
				fmt.Errorf("unsupported group kind %q", typed.Kind),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
	case qb.Negation:
		compiled, err := a.compileExpr(typed.Expr, dialect)
		if err != nil {
			return nil, err
		}
		return clause.Not(compiled), nil
	default:
		return nil, qb.NewError(
			fmt.Errorf("unsupported expression %T", expr),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
}

func (a Adapter) compilePredicate(predicate qb.Predicate, dialect sqladapter.Dialect) (clause.Expression, error) {
	field, ok := plainField(predicate.Left)
	if ok && operandIsPlain(predicate.Right) {
		compiler, exists := a.predicateCompiler[predicate.Op]
		if !exists {
			return nil, qb.NewError(
				fmt.Errorf("unsupported operator %q", predicate.Op),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeUnsupportedOperator),
				qb.WithField(field),
				qb.WithOperator(predicate.Op),
			)
		}
		return compiler(field, predicate)
	}

	sql, vars, err := compilePredicateRaw(predicate, dialect)
	if err != nil {
		return nil, err
	}
	return clause.Expr{SQL: sql, Vars: vars}, nil
}

func defaultPredicateCompilers() map[qb.Operator]PredicateCompiler {
	return map[qb.Operator]PredicateCompiler{
		qb.OpEq: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Eq{Column: Column(field), Value: value}, nil
		},
		qb.OpNe: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Neq{Column: Column(field), Value: value}, nil
		},
		qb.OpGt: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Gt{Column: Column(field), Value: value}, nil
		},
		qb.OpGte: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Gte{Column: Column(field), Value: value}, nil
		},
		qb.OpLt: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Lt{Column: Column(field), Value: value}, nil
		},
		qb.OpLte: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Lte{Column: Column(field), Value: value}, nil
		},
		qb.OpIn: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			values, ok := literalList(predicate.Right)
			if !ok || len(values) == 0 {
				return nil, fmt.Errorf("qb/gorm: %s requires a non-empty literal list", predicate.Op)
			}
			return clause.IN{Column: Column(field), Values: values}, nil
		},
		qb.OpNotIn: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			values, ok := literalList(predicate.Right)
			if !ok || len(values) == 0 {
				return nil, fmt.Errorf("qb/gorm: %s requires a non-empty literal list", predicate.Op)
			}
			return clause.Not(clause.IN{Column: Column(field), Values: values}), nil
		},
		qb.OpLike: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Like{Column: Column(field), Value: value}, nil
		},
		qb.OpContains: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Like{Column: Column(field), Value: "%" + fmt.Sprint(value) + "%"}, nil
		},
		qb.OpPrefix: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Like{Column: Column(field), Value: fmt.Sprint(value) + "%"}, nil
		},
		qb.OpSuffix: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			value, ok := scalarOperandLiteralValue(predicate.Right)
			if !ok {
				return nil, fmt.Errorf("qb/gorm: %s requires a scalar literal operand", predicate.Op)
			}
			return clause.Like{Column: Column(field), Value: "%" + fmt.Sprint(value)}, nil
		},
		qb.OpIsNull: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Eq{Column: Column(field), Value: nil}, nil
		},
		qb.OpNotNull: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Neq{Column: Column(field), Value: nil}, nil
		},
	}
}

func compilePredicateRaw(predicate qb.Predicate, dialect sqladapter.Dialect) (string, []any, error) {
	leftSQL, leftArgs, err := compileScalar(predicate.Left, dialect, qb.StageApply)
	if err != nil {
		return "", nil, err
	}

	field := predicateField(predicate.Left)

	switch predicate.Op {
	case qb.OpEq:
		if operandIsNull(predicate.Right) {
			return leftSQL + " IS NULL", leftArgs, nil
		}
		return compileBinaryRaw(leftSQL, leftArgs, predicate.Right, "=", dialect, field, predicate.Op)
	case qb.OpNe:
		if operandIsNull(predicate.Right) {
			return leftSQL + " IS NOT NULL", leftArgs, nil
		}
		return compileBinaryRaw(leftSQL, leftArgs, predicate.Right, "<>", dialect, field, predicate.Op)
	case qb.OpGt:
		return compileBinaryRaw(leftSQL, leftArgs, predicate.Right, ">", dialect, field, predicate.Op)
	case qb.OpGte:
		return compileBinaryRaw(leftSQL, leftArgs, predicate.Right, ">=", dialect, field, predicate.Op)
	case qb.OpLt:
		return compileBinaryRaw(leftSQL, leftArgs, predicate.Right, "<", dialect, field, predicate.Op)
	case qb.OpLte:
		return compileBinaryRaw(leftSQL, leftArgs, predicate.Right, "<=", dialect, field, predicate.Op)
	case qb.OpLike:
		return compileLikeRaw(leftSQL, leftArgs, predicate.Right, "", "", dialect, field, predicate.Op)
	case qb.OpContains:
		return compileLikeRaw(leftSQL, leftArgs, predicate.Right, "%", "%", dialect, field, predicate.Op)
	case qb.OpPrefix:
		return compileLikeRaw(leftSQL, leftArgs, predicate.Right, "", "%", dialect, field, predicate.Op)
	case qb.OpSuffix:
		return compileLikeRaw(leftSQL, leftArgs, predicate.Right, "%", "", dialect, field, predicate.Op)
	case qb.OpIsNull:
		return leftSQL + " IS NULL", leftArgs, nil
	case qb.OpNotNull:
		return leftSQL + " IS NOT NULL", leftArgs, nil
	case qb.OpIn, qb.OpNotIn:
		list, ok := predicate.Right.(qb.ListOperand)
		if !ok || len(list.Items) == 0 {
			return "", nil, qb.NewError(
				fmt.Errorf("%s requires a non-empty list", predicate.Op),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidValue),
				qb.WithField(field),
				qb.WithOperator(predicate.Op),
			)
		}

		parts := make([]string, len(list.Items))
		args := append([]any(nil), leftArgs...)
		for i, item := range list.Items {
			part, partArgs, err := compileScalar(item, dialect, qb.StageApply)
			if err != nil {
				return "", nil, err
			}
			parts[i] = part
			args = append(args, partArgs...)
		}

		operator := " IN "
		if predicate.Op == qb.OpNotIn {
			operator = " NOT IN "
		}

		return leftSQL + operator + "(" + strings.Join(parts, ", ") + ")", args, nil
	default:
		return "", nil, qb.NewError(
			fmt.Errorf("unsupported operator %q", predicate.Op),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeUnsupportedOperator),
			qb.WithField(field),
			qb.WithOperator(predicate.Op),
		)
	}
}

func compileBinaryRaw(leftSQL string, leftArgs []any, operand qb.Operand, operator string, dialect sqladapter.Dialect, field string, op qb.Operator) (string, []any, error) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	rightSQL, rightArgs, err := compileScalar(right.Expr, dialect, qb.StageApply)
	if err != nil {
		return "", nil, err
	}

	return leftSQL + " " + operator + " " + rightSQL, append(leftArgs, rightArgs...), nil
}

func compileLikeRaw(leftSQL string, leftArgs []any, operand qb.Operand, prefix, suffix string, dialect sqladapter.Dialect, field string, op qb.Operator) (string, []any, error) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return "", nil, qb.NewError(
			fmt.Errorf("%s requires a scalar operand", op),
			qb.WithStage(qb.StageApply),
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
			args := make([]any, 0, 3)
			if prefix != "" {
				args = append(args, qb.V(prefix))
			}
			args = append(args, scalar)
			if suffix != "" {
				args = append(args, qb.V(suffix))
			}
			scalar = qb.Func("concat", args...)
		}
	}

	rightSQL, rightArgs, err := compileScalar(scalar, dialect, qb.StageApply)
	if err != nil {
		return "", nil, err
	}

	return leftSQL + " LIKE " + rightSQL, append(leftArgs, rightArgs...), nil
}

func compileScalarList(values []qb.Scalar, dialect sqladapter.Dialect, stage qb.ErrorStage, kind string) (string, []any, error) {
	parts := make([]string, 0, len(values))
	args := make([]any, 0)
	for _, value := range values {
		part, partArgs, err := compileScalar(value, dialect, stage)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, part)
		args = append(args, partArgs...)
	}
	return strings.Join(parts, ", "), args, nil
}

func compileScalar(expr qb.Scalar, dialect sqladapter.Dialect, stage qb.ErrorStage) (string, []any, error) {
	switch typed := expr.(type) {
	case nil:
		return "", nil, qb.NewError(
			fmt.Errorf("scalar expression cannot be nil"),
			qb.WithStage(stage),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	case qb.Ref:
		if strings.TrimSpace(typed.Name) == "" {
			return "", nil, qb.NewError(
				fmt.Errorf("field reference cannot be empty"),
				qb.WithStage(stage),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		return dialect.QuoteIdentifier(typed.Name), nil, nil
	case qb.Literal:
		if typed.Value == nil {
			return "NULL", nil, nil
		}
		return "?", []any{typed.Value}, nil
	case qb.Call:
		args := make([]string, len(typed.Args))
		vars := make([]any, 0)
		for i, arg := range typed.Args {
			part, partVars, err := compileScalar(arg, dialect, stage)
			if err != nil {
				return "", nil, err
			}
			args[i] = part
			vars = append(vars, partVars...)
		}

		sql, err := dialect.CompileFunction(typed.Name, args)
		if err != nil {
			return "", nil, qb.NewError(
				err,
				qb.WithStage(stage),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}

		return sql, vars, nil
	default:
		return "", nil, qb.NewError(
			fmt.Errorf("unsupported scalar expression %T", expr),
			qb.WithStage(stage),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
}

func plainField(expr qb.Scalar) (string, bool) {
	ref, ok := expr.(qb.Ref)
	if !ok {
		return "", false
	}
	if strings.TrimSpace(ref.Name) == "" {
		return "", false
	}
	return ref.Name, true
}

func refsOnly(values []qb.Scalar) bool {
	for _, value := range values {
		if _, ok := value.(qb.Ref); !ok {
			return false
		}
	}
	return true
}

func operandIsPlain(operand qb.Operand) bool {
	switch typed := operand.(type) {
	case nil:
		return true
	case qb.ScalarOperand:
		_, ok := typed.Expr.(qb.Literal)
		return ok
	case qb.ListOperand:
		if len(typed.Items) == 0 {
			return false
		}
		for _, item := range typed.Items {
			if _, ok := item.(qb.Literal); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func scalarOperandLiteralValue(operand qb.Operand) (any, bool) {
	right, ok := operand.(qb.ScalarOperand)
	if !ok {
		return nil, false
	}

	literal, ok := right.Expr.(qb.Literal)
	if !ok {
		return nil, false
	}

	return literal.Value, true
}

func literalList(operand qb.Operand) ([]interface{}, bool) {
	list, ok := operand.(qb.ListOperand)
	if !ok {
		return nil, false
	}

	values := make([]interface{}, len(list.Items))
	for i, item := range list.Items {
		literal, ok := item.(qb.Literal)
		if !ok {
			return nil, false
		}
		values[i] = literal.Value
	}

	return values, true
}

func operandIsNull(operand qb.Operand) bool {
	value, ok := scalarOperandLiteralValue(operand)
	return ok && value == nil
}

func predicateField(expr qb.Scalar) string {
	field, ok := qb.SingleRef(expr)
	if !ok {
		return ""
	}
	return field
}

func lookupDialect(name string) sqladapter.Dialect {
	dialect, err := sqladapter.LookupDialect(name)
	if err != nil {
		return sqladapter.DefaultDialect()
	}
	return dialect
}

func anyList(value any) ([]interface{}, bool) {
	switch typed := value.(type) {
	case []interface{}:
		return append([]interface{}(nil), typed...), true
	case []string:
		out := make([]interface{}, len(typed))
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

		out := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, true
	}
}
