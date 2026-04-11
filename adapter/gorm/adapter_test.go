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
		schema.Define("status", schema.Aliases("state")),
		schema.Define("created_at", schema.Aliases("createdAt"), schema.Sortable()),
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
		gormadapter.New(gormadapter.WithQueryTransformer(userSchema.Normalize)),
		query,
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	wantSQL := "SELECT * FROM `users` WHERE `status` = ? ORDER BY `created_at` DESC"
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
				return clause.Expr{
					SQL:  "LOWER(" + field + ") LIKE LOWER(?)",
					Vars: []interface{}{"%" + predicate.Value.(string) + "%"},
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
			Field: "status",
			Op:    qb.Operator("bogus"),
			Value: "active",
		},
	}

	db := dryRunDB(t)
	result := db.Model(&user{}).Scopes(gormadapter.New().Scope(query)).Find(&[]user{})
	if result.Error == nil {
		t.Fatal("expected scope error")
	}

	if !strings.Contains(result.Error.Error(), `unsupported operator "bogus"`) {
		t.Fatalf("unexpected error: %v", result.Error)
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
