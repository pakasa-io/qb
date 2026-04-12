package gormadapter

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pakasa-io/qb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PredicateCompiler builds a GORM expression for a predicate.
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

	dialectName := db.Dialector.Name()

	if transformed.Filter != nil {
		filter, err := a.compileExpr(transformed.Filter, dialectName)
		if err != nil {
			return nil, err
		}
		db = db.Where(filter)
	}

	if len(transformed.SelectExprs) == 0 && len(transformed.Selects) > 0 {
		db = db.Select(append([]string(nil), transformed.Selects...))
	} else if len(transformed.Selects) > 0 || len(transformed.SelectExprs) > 0 {
		selectSQL, vars, err := a.compileProjectionList(transformed.Selects, transformed.SelectExprs, dialectName)
		if err != nil {
			return nil, err
		}
		db = db.Select(selectSQL, vars...)
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

	if len(transformed.GroupBy) > 0 || len(transformed.GroupExprs) > 0 {
		if len(transformed.GroupExprs) == 0 {
			columns := make([]clause.Column, 0, len(transformed.GroupBy))
			for _, field := range transformed.GroupBy {
				if strings.TrimSpace(field) == "" {
					return nil, qb.NewError(
						fmt.Errorf("group_by field cannot be empty"),
						qb.WithStage(qb.StageApply),
						qb.WithCode(qb.CodeInvalidQuery),
					)
				}
				columns = append(columns, Column(field))
			}
			db.Statement.AddClause(clause.GroupBy{Columns: columns})
		} else {
			groupBySQL, vars, err := a.compileProjectionList(transformed.GroupBy, transformed.GroupExprs, dialectName)
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
			db = db.Group(groupBySQL)
		}
	}

	for _, sort := range transformed.Sorts {
		if sort.Field == "" {
			return nil, qb.NewError(
				fmt.Errorf("sort field cannot be empty"),
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
				qb.WithField(sort.Field),
			)
		}

		db = db.Order(clause.OrderByColumn{
			Column: Column(sort.Field),
			Desc:   direction == qb.Desc,
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

func (a Adapter) compileExpr(expr qb.Expr, dialectName string) (clause.Expression, error) {
	switch typed := expr.(type) {
	case qb.Predicate:
		return a.compilePredicate(typed, dialectName)
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
			compiled, err := a.compileExpr(term, dialectName)
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
		compiled, err := a.compileExpr(typed.Expr, dialectName)
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

func (a Adapter) compilePredicate(predicate qb.Predicate, dialectName string) (clause.Expression, error) {
	field := predicateFieldName(predicate)
	if predicate.Left != nil || containsValueExpr(predicate.Value) {
		sql, vars, err := a.compilePredicateSQL(predicate, dialectName)
		if err != nil {
			return nil, err
		}
		return clause.Expr{SQL: sql, Vars: vars}, nil
	}

	if predicate.Field == "" {
		return nil, qb.NewError(
			fmt.Errorf("predicate field cannot be empty"),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeInvalidQuery),
			qb.WithOperator(predicate.Op),
		)
	}

	compiler, ok := a.predicateCompiler[predicate.Op]
	if !ok {
		return nil, qb.NewError(
			fmt.Errorf("unsupported operator %q", predicate.Op),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeUnsupportedOperator),
			qb.WithField(field),
			qb.WithOperator(predicate.Op),
		)
	}

	return compiler(predicate.Field, predicate)
}

func (a Adapter) compilePredicateSQL(predicate qb.Predicate, dialectName string) (string, []any, error) {
	leftExpr, field := predicateLeftExpr(predicate)
	if leftExpr == nil {
		return "", nil, qb.NewError(
			fmt.Errorf("predicate field cannot be empty"),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeInvalidQuery),
			qb.WithOperator(predicate.Op),
		)
	}

	leftSQL, leftVars, err := a.compileValueExpr(leftExpr, dialectName)
	if err != nil {
		return "", nil, err
	}

	switch predicate.Op {
	case qb.OpEq:
		if isNilPredicateValue(predicate.Value) {
			return leftSQL + " IS NULL", leftVars, nil
		}
		return a.compileBinaryPredicateSQL(leftSQL, leftVars, predicate.Value, "=", dialectName)
	case qb.OpNe:
		if isNilPredicateValue(predicate.Value) {
			return leftSQL + " IS NOT NULL", leftVars, nil
		}
		return a.compileBinaryPredicateSQL(leftSQL, leftVars, predicate.Value, "<>", dialectName)
	case qb.OpGt:
		return a.compileBinaryPredicateSQL(leftSQL, leftVars, predicate.Value, ">", dialectName)
	case qb.OpGte:
		return a.compileBinaryPredicateSQL(leftSQL, leftVars, predicate.Value, ">=", dialectName)
	case qb.OpLt:
		return a.compileBinaryPredicateSQL(leftSQL, leftVars, predicate.Value, "<", dialectName)
	case qb.OpLte:
		return a.compileBinaryPredicateSQL(leftSQL, leftVars, predicate.Value, "<=", dialectName)
	case qb.OpLike:
		return a.compileLikePredicateSQL(leftSQL, leftVars, predicate.Value, "", "", dialectName)
	case qb.OpContains:
		return a.compileLikePredicateSQL(leftSQL, leftVars, predicate.Value, "%", "%", dialectName)
	case qb.OpPrefix:
		return a.compileLikePredicateSQL(leftSQL, leftVars, predicate.Value, "", "%", dialectName)
	case qb.OpSuffix:
		return a.compileLikePredicateSQL(leftSQL, leftVars, predicate.Value, "%", "", dialectName)
	case qb.OpIsNull:
		return leftSQL + " IS NULL", leftVars, nil
	case qb.OpNotNull:
		return leftSQL + " IS NOT NULL", leftVars, nil
	case qb.OpIn, qb.OpNotIn:
		values, ok := anyList(predicate.Value)
		if !ok || len(values) == 0 {
			return "", nil, qb.NewError(
				fmt.Errorf("%s requires a non-empty list", predicate.Op),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidValue),
				qb.WithField(field),
				qb.WithOperator(predicate.Op),
			)
		}

		parts := make([]string, len(values))
		vars := append([]any(nil), leftVars...)
		for i, value := range values {
			part, partVars, err := a.compileComparableValue(value, dialectName)
			if err != nil {
				return "", nil, err
			}
			parts[i] = part
			vars = append(vars, partVars...)
		}

		operator := " IN "
		if predicate.Op == qb.OpNotIn {
			operator = " NOT IN "
		}

		return leftSQL + operator + "(" + strings.Join(parts, ", ") + ")", vars, nil
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

func defaultPredicateCompilers() map[qb.Operator]PredicateCompiler {
	return map[qb.Operator]PredicateCompiler{
		qb.OpEq: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Eq{Column: Column(field), Value: predicate.Value}, nil
		},
		qb.OpNe: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Neq{Column: Column(field), Value: predicate.Value}, nil
		},
		qb.OpGt: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Gt{Column: Column(field), Value: predicate.Value}, nil
		},
		qb.OpGte: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Gte{Column: Column(field), Value: predicate.Value}, nil
		},
		qb.OpLt: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Lt{Column: Column(field), Value: predicate.Value}, nil
		},
		qb.OpLte: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Lte{Column: Column(field), Value: predicate.Value}, nil
		},
		qb.OpIn: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			values, ok := anyList(predicate.Value)
			if !ok || len(values) == 0 {
				return nil, fmt.Errorf("qb/gorm: %s requires a non-empty list", predicate.Op)
			}
			return clause.IN{Column: Column(field), Values: values}, nil
		},
		qb.OpNotIn: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			values, ok := anyList(predicate.Value)
			if !ok || len(values) == 0 {
				return nil, fmt.Errorf("qb/gorm: %s requires a non-empty list", predicate.Op)
			}
			return clause.Not(clause.IN{Column: Column(field), Values: values}), nil
		},
		qb.OpLike: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Like{Column: Column(field), Value: predicate.Value}, nil
		},
		qb.OpContains: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Like{Column: Column(field), Value: "%" + fmt.Sprint(predicate.Value) + "%"}, nil
		},
		qb.OpPrefix: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Like{Column: Column(field), Value: fmt.Sprint(predicate.Value) + "%"}, nil
		},
		qb.OpSuffix: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Like{Column: Column(field), Value: "%" + fmt.Sprint(predicate.Value)}, nil
		},
		qb.OpIsNull: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Eq{Column: Column(field), Value: nil}, nil
		},
		qb.OpNotNull: func(field string, predicate qb.Predicate) (clause.Expression, error) {
			return clause.Neq{Column: Column(field), Value: nil}, nil
		},
	}
}

func (a Adapter) compileProjectionList(fields []string, exprs []qb.ValueExpr, dialectName string) (string, []any, error) {
	parts := make([]string, 0, len(fields)+len(exprs))
	vars := make([]any, 0)

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			return "", nil, qb.NewError(
				fmt.Errorf("projection field cannot be empty"),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		parts = append(parts, quoteIdentifier(dialectName, field))
	}

	for _, expr := range exprs {
		part, partVars, err := a.compileValueExpr(expr, dialectName)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, part)
		vars = append(vars, partVars...)
	}

	return strings.Join(parts, ", "), vars, nil
}

func (a Adapter) compileBinaryPredicateSQL(leftSQL string, leftVars []any, value any, operator string, dialectName string) (string, []any, error) {
	rightSQL, rightVars, err := a.compileComparableValue(value, dialectName)
	if err != nil {
		return "", nil, err
	}
	return leftSQL + " " + operator + " " + rightSQL, append(leftVars, rightVars...), nil
}

func (a Adapter) compileLikePredicateSQL(leftSQL string, leftVars []any, value any, prefix, suffix, dialectName string) (string, []any, error) {
	if expr, ok := qb.AsValueExpr(value); ok {
		pattern := expr
		if prefix != "" || suffix != "" {
			pattern = qb.Call("concat", qb.Lit(prefix), expr, qb.Lit(suffix))
		}
		rightSQL, rightVars, err := a.compileValueExpr(pattern, dialectName)
		if err != nil {
			return "", nil, err
		}
		return leftSQL + " LIKE " + rightSQL, append(leftVars, rightVars...), nil
	}

	rightSQL, rightVars, err := a.compileComparableValue(prefix+fmt.Sprint(value)+suffix, dialectName)
	if err != nil {
		return "", nil, err
	}
	return leftSQL + " LIKE " + rightSQL, append(leftVars, rightVars...), nil
}

func (a Adapter) compileComparableValue(value any, dialectName string) (string, []any, error) {
	if expr, ok := qb.AsValueExpr(value); ok {
		return a.compileValueExpr(expr, dialectName)
	}
	return a.compileValueExpr(qb.Lit(value), dialectName)
}

func (a Adapter) compileValueExpr(expr qb.ValueExpr, dialectName string) (string, []any, error) {
	switch typed := expr.(type) {
	case nil:
		return "", nil, qb.NewError(
			fmt.Errorf("expression cannot be nil"),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	case qb.Ref:
		field := string(typed)
		if strings.TrimSpace(field) == "" {
			return "", nil, qb.NewError(
				fmt.Errorf("expression field cannot be empty"),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		return quoteIdentifier(dialectName, field), nil, nil
	case qb.Literal:
		if typed.Value == nil {
			return "NULL", nil, nil
		}
		return "?", []any{typed.Value}, nil
	case qb.CallExpr:
		name := strings.TrimSpace(typed.Name)
		if name == "" {
			return "", nil, qb.NewError(
				fmt.Errorf("function name cannot be empty"),
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}

		args := make([]string, len(typed.Args))
		vars := make([]any, 0)
		for i, arg := range typed.Args {
			part, partVars, err := a.compileValueExpr(arg, dialectName)
			if err != nil {
				return "", nil, err
			}
			args[i] = part
			vars = append(vars, partVars...)
		}

		sql, err := compileFunctionForDialect(dialectName, name, args)
		if err != nil {
			return "", nil, qb.NewError(
				err,
				qb.WithStage(qb.StageApply),
				qb.WithCode(qb.CodeInvalidQuery),
			)
		}
		return sql, vars, nil
	default:
		return "", nil, qb.NewError(
			fmt.Errorf("unsupported value expression %T", expr),
			qb.WithStage(qb.StageApply),
			qb.WithCode(qb.CodeInvalidQuery),
		)
	}
}

func quoteIdentifier(dialectName, identifier string) string {
	parts := strings.Split(identifier, ".")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		switch dialectName {
		case "mysql", "sqlite":
			parts[i] = "`" + strings.ReplaceAll(part, "`", "``") + "`"
		default:
			parts[i] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
		}
	}
	return strings.Join(parts, ".")
}

func compileFunctionForDialect(dialectName, name string, args []string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("function name cannot be empty")
	}

	switch strings.ToLower(name) {
	case "concat":
		switch dialectName {
		case "mysql":
			return "CONCAT(" + strings.Join(args, ", ") + ")", nil
		default:
			return "(" + strings.Join(args, " || ") + ")", nil
		}
	default:
		return strings.ToUpper(name) + "(" + strings.Join(args, ", ") + ")", nil
	}
}

func containsValueExpr(value any) bool {
	if _, ok := qb.AsValueExpr(value); ok {
		return true
	}

	values, ok := anyList(value)
	if !ok {
		return false
	}

	for _, item := range values {
		if _, ok := qb.AsValueExpr(item); ok {
			return true
		}
	}

	return false
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
