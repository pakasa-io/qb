package gormadapter_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/pakasa-io/qb"
	gormadapter "github.com/pakasa-io/qb/adapter/gorm"
	"github.com/pakasa-io/qb/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type user struct {
	ID        int
	Status    string
	Role      string
	Age       int
	CreatedAt int
	CompanyID int
	Company   company
	Orders    []order
}

type company struct {
	ID   int
	Name string
}

type order struct {
	ID     int
	UserID int
}

func TestApply(t *testing.T) {
	query, err := qb.New().
		Where(qb.And(
			qb.Field("status").Eq("active"),
			qb.Field("role").In("admin", "owner"),
			qb.Not(qb.Field("deleted_at").IsNull()),
		)).
		SortBy("created_at", qb.Desc).
		Limit(10).
		Offset(20).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	result, err := applyAndFind(t, gormadapter.New(), query)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	wantSQL := "SELECT * FROM `users` WHERE `status` = ? AND `role` IN (?,?) AND `deleted_at` IS NOT NULL ORDER BY `created_at` DESC LIMIT 10 OFFSET 20"
	if result.Statement.SQL.String() != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, result.Statement.SQL.String())
	}

	wantArgs := []any{"active", "admin", "owner"}
	if len(result.Statement.Vars) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(result.Statement.Vars))
	}

	for i := range wantArgs {
		if result.Statement.Vars[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], result.Statement.Vars[i])
		}
	}
}

func TestApplyWithTransformer(t *testing.T) {
	userSchema := schema.MustNew(
		schema.Define("status", schema.Storage("users.status"), schema.Aliases("state")),
		schema.Define("created_at", schema.Storage("users.created_at"), schema.Aliases("createdAt"), schema.Sortable()),
	)

	query, err := qb.New().
		Where(qb.Field("state").Eq("active")).
		SortBy("createdAt", qb.Desc).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	result, err := applyAndFind(
		t,
		gormadapter.New(gormadapter.WithQueryTransformer(userSchema.ToStorage)),
		query,
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	wantSQL := "SELECT * FROM `users` WHERE `users`.`status` = ? ORDER BY `users`.`created_at` DESC"
	if result.Statement.SQL.String() != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, result.Statement.SQL.String())
	}
}

func TestApplyWithCustomPredicateCompiler(t *testing.T) {
	query, err := qb.New().
		Where(qb.Field("status").Contains("Act")).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	result, err := applyAndFind(
		t,
		gormadapter.New(
			gormadapter.WithPredicateCompiler(qb.OpContains, func(field string, predicate qb.Predicate) (clause.Expression, error) {
				operand, ok := predicate.Right.(qb.ScalarOperand)
				if !ok {
					return nil, errors.New("expected scalar operand")
				}
				literal, ok := operand.Expr.(qb.Literal)
				if !ok {
					return nil, errors.New("expected literal operand")
				}
				return clause.Expr{
					SQL:  "LOWER(" + field + ") LIKE LOWER(?)",
					Vars: []interface{}{"%" + literal.Value.(string) + "%"},
				}, nil
			}),
		),
		query,
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	wantSQL := "SELECT * FROM `users` WHERE LOWER(status) LIKE LOWER(?)"
	if result.Statement.SQL.String() != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, result.Statement.SQL.String())
	}
}

func TestScopeAddsError(t *testing.T) {
	query := qb.Query{
		Filter: qb.Predicate{
			Left:  qb.F("status"),
			Op:    qb.Operator("bogus"),
			Right: qb.ScalarOperand{Expr: qb.V("active")},
		},
	}

	db := dryRunDB(t)
	result := db.Model(&user{}).Scopes(gormadapter.New().Scope(query)).Find(&[]user{})
	if result.Error == nil {
		t.Fatal("expected scope error")
	}

	if !strings.Contains(result.Error.Error(), `operator "bogus" is not supported`) {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	var diagnostic *qb.Error
	if !errors.As(result.Error, &diagnostic) {
		t.Fatalf("expected qb.Error, got %T", result.Error)
	}

	if diagnostic.Stage != qb.StageApply || diagnostic.Code != qb.CodeUnsupportedOperator {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}
}

func TestApplyWithTransformerError(t *testing.T) {
	query, err := qb.New().
		Where(qb.Field("status").Eq("active")).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	wantErr := errors.New("boom")
	_, err = gormadapter.New(
		gormadapter.WithQueryTransformer(func(query qb.Query) (qb.Query, error) {
			return qb.Query{}, wantErr
		}),
	).Apply(dryRunDB(t), query)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected transformer error, got %v", err)
	}
}

func TestApplyWithSelectIncludeGroupByAndPageSize(t *testing.T) {
	query, err := qb.New().
		Select("status", "role").
		Include("Company", "Orders").
		GroupBy("status", "role").
		SortBy("status", qb.Asc).
		Page(2).
		Size(10).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	tx, err := gormadapter.New().Apply(dryRunDB(t).Model(&user{}), query)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if len(tx.Statement.Selects) != 2 || tx.Statement.Selects[0] != "status" || tx.Statement.Selects[1] != "role" {
		t.Fatalf("unexpected selects: %#v", tx.Statement.Selects)
	}

	if len(tx.Statement.Preloads) != 2 {
		t.Fatalf("unexpected preloads: %#v", tx.Statement.Preloads)
	}

	if _, ok := tx.Statement.Preloads["Company"]; !ok {
		t.Fatalf("expected Company preload, got %#v", tx.Statement.Preloads)
	}

	if _, ok := tx.Statement.Preloads["Orders"]; !ok {
		t.Fatalf("expected Orders preload, got %#v", tx.Statement.Preloads)
	}

	groupClause, ok := tx.Statement.Clauses["GROUP BY"]
	if !ok {
		t.Fatal("expected GROUP BY clause")
	}

	groupBy, ok := groupClause.Expression.(clause.GroupBy)
	if !ok {
		t.Fatalf("unexpected GROUP BY expression: %T", groupClause.Expression)
	}

	if len(groupBy.Columns) != 2 || groupBy.Columns[0].Name != "status" || groupBy.Columns[1].Name != "role" {
		t.Fatalf("unexpected GROUP BY columns: %#v", groupBy.Columns)
	}

	result := tx.Find(&[]user{})
	if result.Error != nil {
		t.Fatalf("Find() error = %v", result.Error)
	}

	wantSQL := "SELECT `status`,`role` FROM `users` GROUP BY `status`,`role` ORDER BY `status` LIMIT 10 OFFSET 10"
	if result.Statement.SQL.String() != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, result.Statement.SQL.String())
	}
}

func TestApplyWithFunctionExpressions(t *testing.T) {
	query, err := qb.New().
		SelectExpr(qb.Lower(qb.F("name")), qb.F("age")).
		GroupByExpr(qb.Lower(qb.F("name"))).
		SortByExpr(qb.Lower(qb.F("name")), qb.Asc).
		Where(qb.And(
			qb.Lower(qb.F("name")).Eq("john"),
			qb.F("name").Eq(qb.Lower("JOHN")),
		)).
		Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	result, err := applyAndFind(t, gormadapter.New(), query)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	wantSQL := "SELECT LOWER(\"name\"), \"age\" FROM `users` WHERE LOWER(\"name\") = ? AND \"name\" = LOWER(?) GROUP BY LOWER(\"name\") ORDER BY LOWER(\"name\") ASC"
	if result.Statement.SQL.String() != wantSQL {
		t.Fatalf("SQL mismatch\nwant: %s\ngot:  %s", wantSQL, result.Statement.SQL.String())
	}

	wantArgs := []any{"john", "JOHN"}
	if len(result.Statement.Vars) != len(wantArgs) {
		t.Fatalf("arg count mismatch: want %d, got %d", len(wantArgs), len(result.Statement.Vars))
	}

	for i := range wantArgs {
		if result.Statement.Vars[i] != wantArgs[i] {
			t.Fatalf("arg %d mismatch: want %#v, got %#v", i, wantArgs[i], result.Statement.Vars[i])
		}
	}
}

func TestApplyRejectsUnsupportedDialectSpecificFunctions(t *testing.T) {
	tests := []struct {
		name  string
		query qb.Query
	}{
		{
			name: "ilike on sqlite",
			query: mustBuildQuery(t, qb.New().
				Where(qb.F("name").ILike("jo%"))),
		},
		{
			name: "regexp on sqlite",
			query: mustBuildQuery(t, qb.New().
				Where(qb.F("name").Regexp("jo.*"))),
		},
		{
			name: "ceil on sqlite",
			query: mustBuildQuery(t, qb.New().
				SelectExpr(qb.F("score").Ceil())),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gormadapter.New().Apply(dryRunDB(t).Model(&user{}), tt.query)
			if err == nil {
				t.Fatal("expected apply error")
			}

			var diagnostic *qb.Error
			if !errors.As(err, &diagnostic) {
				t.Fatalf("expected qb.Error, got %T", err)
			}

			if diagnostic.Code != qb.CodeUnsupportedFeature {
				t.Fatalf("unexpected diagnostic: %+v", diagnostic)
			}
		})
	}
}

func applyAndFind(t *testing.T, adapter gormadapter.Adapter, query qb.Query) (*gorm.DB, error) {
	t.Helper()

	db := dryRunDB(t)
	tx, err := adapter.Apply(db.Model(&user{}), query)
	if err != nil {
		return nil, err
	}

	result := tx.Find(&[]user{})
	if result.Error != nil {
		return nil, result.Error
	}

	return result, nil
}

func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	return db
}

func mustBuildQuery(t *testing.T, builder qb.Builder) qb.Query {
	t.Helper()

	query, err := builder.Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	return query
}
