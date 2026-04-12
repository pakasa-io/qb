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

	if transformed.Filter != nil {
		filter, err := a.compileExpr(transformed.Filter)
		if err != nil {
			return nil, err
		}
		db = db.Where(filter)
	}

	if len(transformed.Selects) > 0 {
		db = db.Select(append([]string(nil), transformed.Selects...))
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

func (a Adapter) compileExpr(expr qb.Expr) (clause.Expression, error) {
	switch typed := expr.(type) {
	case qb.Predicate:
		return a.compilePredicate(typed)
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
			compiled, err := a.compileExpr(term)
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
		compiled, err := a.compileExpr(typed.Expr)
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

func (a Adapter) compilePredicate(predicate qb.Predicate) (clause.Expression, error) {
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
			qb.WithField(predicate.Field),
			qb.WithOperator(predicate.Op),
		)
	}

	return compiler(predicate.Field, predicate)
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
