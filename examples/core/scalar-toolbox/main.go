package main

import (
	"fmt"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)

func main() {
	query, err := qb.New().
		SelectProjection(
			qb.F("users.name").LTrim().As("left_trimmed"),
			qb.F("users.name").RTrim().As("right_trimmed"),
			qb.F("users.name").Replace("  ", " ").As("normalized_spaces"),
			qb.F("users.name").Left(3).As("name_prefix"),
			qb.F("users.name").Right(2).As("name_suffix"),
			qb.F("users.nickname").NullIf("").Coalesce("anonymous").As("display_name"),
			qb.F("users.delta").Abs().As("abs_delta"),
			qb.F("users.score").Ceil().As("score_ceil"),
			qb.F("users.score").Floor().As("score_floor"),
			qb.F("users.id").Mod(10).As("bucket"),
			qb.F("users.score").Min().As("min_score"),
			qb.F("users.score").Max().As("max_score"),
		).
		Where(qb.And(
			qb.F("users.nickname").NotNull(),
			qb.F("users.name").Like("Jo%"),
		)).
		GroupByExpr(
			qb.F("users.name").LTrim(),
			qb.F("users.name").RTrim(),
			qb.F("users.name").Replace("  ", " "),
			qb.F("users.name").Left(3),
			qb.F("users.name").Right(2),
			qb.F("users.nickname").NullIf("").Coalesce("anonymous"),
			qb.F("users.delta").Abs(),
			qb.F("users.score").Ceil(),
			qb.F("users.score").Floor(),
			qb.F("users.id").Mod(10),
		).
		SortByExpr(qb.F("users.id").Mod(10), qb.Asc).
		Query()
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New().Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}
