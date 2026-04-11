package gormadapter

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pakasa-io/qb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// QueryTransformer rewrites or validates a query before it is applied.
type QueryTransformer func(qb.Query) (qb.Query, error)

// PredicateCompiler builds a GORM expression for a predicate.
type PredicateCompiler func(field string, predicate qb.Predicate) (clause.Expression, error)

// Adapter applies qb.Query values to a *gorm.DB chain.
type Adapter struct {
	transformers      []QueryTransformer
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
func WithQueryTransformer(transformer QueryTransformer) Option {
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

// Apply applies the query to a GORM chain and returns the updated chain.
func (a Adapter) Apply(db *gorm.DB, query qb.Query) (*gorm.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("qb/gorm: db cannot be nil")
	}

	var err error
	for _, transformer := range a.transformers {
		query, err = transformer(query)
		if err != nil {
			return nil, err
		}
	}

	if query.Filter != nil {
		filter, err := a.compileExpr(query.Filter)
		if err != nil {
			return nil, err
		}
		db = db.Where(filter)
	}

	for _, sort := range query.Sorts {
		if sort.Field == "" {
			return nil, fmt.Errorf("qb/gorm: sort field cannot be empty")
		}

		direction := sort.Direction
		if direction == "" {
			direction = qb.Asc
		}

		if direction != qb.Asc && direction != qb.Desc {
			return nil, fmt.Errorf("qb/gorm: unsupported sort direction %q", sort.Direction)
		}

		db = db.Order(clause.OrderByColumn{
			Column: Column(sort.Field),
			Desc:   direction == qb.Desc,
		})
	}

	if query.Limit != nil {
		if *query.Limit < 0 {
			return nil, fmt.Errorf("qb/gorm: limit cannot be negative")
		}
		db = db.Limit(*query.Limit)
	}

	if query.Offset != nil {
		if *query.Offset < 0 {
			return nil, fmt.Errorf("qb/gorm: offset cannot be negative")
		}
		db = db.Offset(*query.Offset)
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
			return nil, fmt.Errorf("qb/gorm: empty %s group", typed.Kind)
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
			return nil, fmt.Errorf("qb/gorm: unsupported group kind %q", typed.Kind)
		}
	case qb.Negation:
		compiled, err := a.compileExpr(typed.Expr)
		if err != nil {
			return nil, err
		}
		return clause.Not(compiled), nil
	default:
		return nil, fmt.Errorf("qb/gorm: unsupported expression %T", expr)
	}
}

func (a Adapter) compilePredicate(predicate qb.Predicate) (clause.Expression, error) {
	if predicate.Field == "" {
		return nil, fmt.Errorf("qb/gorm: predicate field cannot be empty")
	}

	compiler, ok := a.predicateCompiler[predicate.Op]
	if !ok {
		return nil, fmt.Errorf("qb/gorm: unsupported operator %q", predicate.Op)
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
