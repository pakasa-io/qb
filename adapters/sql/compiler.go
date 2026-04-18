package adapter

import (
	"fmt"
	"strings"

	"github.com/pakasa-io/qb"
	sqlrender "github.com/pakasa-io/qb/adapters/internal/sqlrender"
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
	return c.dialect.Capabilities()
}

// Compile renders the query as a SQL fragment.
func (c Compiler) Compile(query qb.Query) (Statement, error) {
	transformed, err := qb.TransformQuery(query, c.transformers...)
	if err != nil {
		return Statement{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageCompile))
	}

	renderer := sqlrender.New(c.dialect, qb.StageCompile)
	if err := renderer.Capabilities().Validate(qb.StageCompile, transformed); err != nil {
		return Statement{}, err
	}

	clauses := make([]string, 0, 5)
	args := make([]any, 0)
	argIndex := 1

	if len(transformed.Projections) > 0 {
		selects, selectArgs, nextArg, err := renderer.CompileProjectionList(transformed.Projections, argIndex)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "SELECT "+selects)
		args = append(args, selectArgs...)
		argIndex = nextArg
	}

	if transformed.Filter != nil {
		filter, filterArgs, nextArg, err := renderer.CompileExpr(transformed.Filter, argIndex)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "WHERE "+filter)
		args = append(args, filterArgs...)
		argIndex = nextArg
	}

	if len(transformed.GroupBy) > 0 {
		groupBy, groupArgs, nextArg, err := renderer.CompileScalarList(transformed.GroupBy, "group_by", argIndex)
		if err != nil {
			return Statement{}, err
		}
		clauses = append(clauses, "GROUP BY "+groupBy)
		args = append(args, groupArgs...)
		argIndex = nextArg
	}

	if len(transformed.Sorts) > 0 {
		sorts, sortArgs, nextArg, err := renderer.CompileSorts(transformed.Sorts, argIndex)
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

	return Statement{SQL: strings.Join(clauses, " "), Args: args}, nil
}
