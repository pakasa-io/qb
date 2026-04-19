# 06. SQL, GORM, And Dialects

`qb` keeps the query model backend-agnostic. Adapters decide how to compile or
apply it.

## Compile To SQL

```go
statement, err := sqladapter.New().Compile(query)
if err != nil {
	panic(err)
}

fmt.Println(statement.SQL)
fmt.Println(statement.Args)
```

## Switch Dialects Per Compiler

```go
dialects := []sqladapter.Dialect{
	sqladapter.PostgresDialect{},
	sqladapter.MySQLDialect{},
	sqladapter.SQLiteDialect{},
}

for _, dialect := range dialects {
	statement, err := sqladapter.New(
		sqladapter.WithDialect(dialect),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(dialect.Name(), statement.SQL)
}
```

## Change The Process Default Dialect

```go
original := sqladapter.DefaultDialect()
defer sqladapter.SetDefaultDialect(original)

if err := sqladapter.SetDefaultDialectByName("mysql"); err != nil {
	panic(err)
}

statement, err := sqladapter.New().Compile(query)
```

## Apply Queries To GORM

`Include(...)` is meaningful for the GORM adapter because it becomes `Preload(...)`:

```go
query, err := qb.New().
	Select("id", "status", "company_id").
	Include("Company", "Orders").
	Where(qb.F("status").Eq("active")).
	SortBy("created_at", qb.Desc).
	Page(1).
	Size(10).
	Query()

tx, err := gormadapter.New().Apply(db.Model(&user{}), query)
if err != nil {
	panic(err)
}

result := tx.Find(&[]user{})
```

If you prefer standard GORM scopes:

```go
db.Scopes(gormadapter.New().Scope(query)).Find(&users)
```

## Relations Stay Outside The Core

`qb` can filter on fields like `customer.company.name`, but it does not own join
generation. Typical pattern:

- schema maps API fields to storage names such as `companies.name`
- SQL callers add the required `JOIN`s around the compiled fragment
- GORM callers configure `Joins(...)` or model associations themselves

`Include(...)` is not a generic SQL join mechanism. The SQL compiler validates
capabilities and rejects includes by default.

## Matching Examples

- [examples/adapters/dialect-switching](../../examples/adapters/dialect-switching/main.go)
- [examples/adapters/default-dialect](../../examples/adapters/default-dialect/main.go)
- [examples/adapters/gorm-apply](../../examples/adapters/gorm-apply/main.go)
- [examples/schema/gorm-public-api](../../examples/schema/gorm-public-api/main.go)
