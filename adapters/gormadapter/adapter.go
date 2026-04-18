package gormadapter

import (
	"fmt"
	"strings"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sqladapter"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Adapter applies qb.Query values to a *gorm.DB chain.
type Adapter struct {
	transformers []qb.QueryTransformer
}

// Option customizes the adapter.
type Option func(*Adapter)

// New creates a GORM adapter.
func New(opts ...Option) Adapter {
	adapter := Adapter{}
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

// Capabilities reports query features supported by the adapter using the
// current default SQL dialect as the baseline.
func (a Adapter) Capabilities() qb.Capabilities {
	capabilities := sqladapter.NewRenderer(sqladapter.DefaultDialect(), qb.StageApply).Capabilities()
	capabilities.SupportsInclude = true
	return capabilities
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

	dialect := lookupDialect(db.Dialector.Name())
	renderer := sqladapter.NewRenderer(dialect, qb.StageApply)

	capabilities := renderer.Capabilities()
	capabilities.SupportsInclude = true
	if err := capabilities.Validate(qb.StageApply, transformed); err != nil {
		return nil, err
	}

	if transformed.Filter != nil {
		sql, vars, _, err := renderer.CompileExpr(transformed.Filter, 1)
		if err != nil {
			return nil, err
		}
		db = db.Where(clause.Expr{SQL: sql, Vars: vars})
	}

	if len(transformed.Projections) > 0 {
		sql, vars, _, err := renderer.CompileProjectionList(transformed.Projections, 1)
		if err != nil {
			return nil, err
		}
		db = db.Select(sql, vars...)
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
		sql, vars, _, err := renderer.CompileScalarList(transformed.GroupBy, "group_by", 1)
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

	if len(transformed.Sorts) > 0 {
		sql, vars, _, err := renderer.CompileSorts(transformed.Sorts, 1)
		if err != nil {
			return nil, err
		}
		db = db.Order(clause.OrderBy{
			Expression: clause.Expr{
				SQL:                sql,
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

func lookupDialect(name string) sqladapter.Dialect {
	dialect, err := sqladapter.LookupDialect(name)
	if err != nil {
		return sqladapter.DefaultDialect()
	}
	return dialect
}
