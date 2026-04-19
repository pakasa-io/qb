// Package sql compiles qb.Query values into parameterized SQL fragments.
//
// The package includes PostgreSQL, MySQL, and SQLite dialects plus compiler
// options for dialect selection and query transformer pipelines. It is the
// primary adapter for turning the qb AST into backend-specific SQL while
// keeping the core query model independent from any driver.
package sql
