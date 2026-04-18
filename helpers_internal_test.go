package qb

import "testing"

func TestFunctionHelperConstructorsAndAliases(t *testing.T) {
	callTests := []struct {
		name     string
		call     Call
		wantName string
		wantArgs int
	}{
		{"count", Count(), "count", 0},
		{"sum", Sum(F("amount")), "sum", 1},
		{"avg", Avg(F("rating")), "avg", 1},
		{"min", Min(F("age")), "min", 1},
		{"max", Max(F("age")), "max", 1},
		{"ref count", F("id").Count(), "count", 1},
		{"literal sum", V(1).Sum(), "sum", 1},
		{"call avg", Lower(F("name")).Avg(), "avg", 1},
		{"date", Date(F("created_at")), "date", 1},
		{"now", Now(), "now", 0},
		{"current date", CurrentDate(), "current_date", 0},
		{"local time", LocalTime(), "localtime", 0},
		{"current time", CurrentTime(), "current_time", 0},
		{"local timestamp", LocalTimestamp(), "localtimestamp", 0},
		{"current timestamp", CurrentTimestamp(), "current_timestamp", 0},
		{"date trunc", DateTrunc("day", F("created_at")), "date_trunc", 2},
		{"extract", Extract("year", F("created_at")), "extract", 2},
		{"date bin", DateBin("1 day", F("created_at"), F("origin")), "date_bin", 3},
		{"ref date", F("created_at").Date(), "date", 1},
		{"literal trunc", V("2026-01-01").DateTrunc("day"), "date_trunc", 2},
		{"call extract", Now().Extract("epoch"), "extract", 2},
		{"coalesce", Coalesce(F("status"), "draft"), "coalesce", 2},
		{"nullif", NullIf(F("status"), "draft"), "nullif", 2},
		{"ref coalesce", F("status").Coalesce("draft"), "coalesce", 2},
		{"literal nullif", V("a").NullIf("b"), "nullif", 2},
		{"call coalesce", Lower(F("name")).Coalesce("n/a"), "coalesce", 2},
		{"json extract", JsonExtract(F("profile"), "$.name"), "json_extract", 2},
		{"json query", JsonQuery(F("profile"), "$.name"), "json_query", 2},
		{"json value", JsonValue(F("profile"), "$.name"), "json_value", 2},
		{"json exists", JsonExists(F("profile"), "$.name"), "json_exists", 2},
		{"json array length", JsonArrayLength(F("profile"), "$.items"), "json_array_length", 2},
		{"json type", JsonType(F("profile"), "$.items"), "json_type", 2},
		{"json array", JsonArray("a", "b"), "json_array", 2},
		{"json object", JsonObject("a", 1), "json_object", 2},
		{"ref json extract", F("profile").JsonExtract("$.name"), "json_extract", 2},
		{"literal json exists", V("{}").JsonExists("$.name"), "json_exists", 2},
		{"call json type", Lower(F("payload")).JsonType("$.name"), "json_type", 2},
		{"abs", Abs(F("score")), "abs", 1},
		{"ceil", Ceil(F("score")), "ceil", 1},
		{"floor", Floor(F("score")), "floor", 1},
		{"mod", Mod(F("score"), 10), "mod", 2},
		{"round", Round(F("score"), 2), "round", 2},
		{"round double", RoundDouble(F("score"), 2), "round_double", 2},
		{"ref abs", F("score").Abs(), "abs", 1},
		{"literal round", V(1.2).Round(1), "round", 2},
		{"call mod", Lower(F("name")).Mod(3), "mod", 2},
		{"cast round double", F("score").Cast("double").RoundDouble(2), "round_double", 2},
		{"lower", Lower(F("name")), "lower", 1},
		{"upper", Upper(F("name")), "upper", 1},
		{"trim", Trim(F("name")), "trim", 1},
		{"ltrim", LTrim(F("name")), "ltrim", 1},
		{"rtrim", RTrim(F("name")), "rtrim", 1},
		{"length", Length(F("name")), "length", 1},
		{"concat", Concat(F("first"), F("last")), "concat", 2},
		{"substring", Substring(F("name"), 1, 2), "substring", 3},
		{"replace", Replace(F("name"), "a", "b"), "replace", 3},
		{"left", Left(F("name"), 2), "left", 2},
		{"right", Right(F("name"), 2), "right", 2},
		{"ref upper", F("name").Upper(), "upper", 1},
		{"literal concat", V("a").Concat("b"), "concat", 2},
		{"call substring", Lower(F("name")).Substring(1, 2), "substring", 3},
		{"field alias", Field("name").Lower(), "lower", 1},
	}

	for _, tc := range callTests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.call.Name != tc.wantName || len(tc.call.Args) != tc.wantArgs {
				t.Fatalf("unexpected call %#v", tc.call)
			}
		})
	}

	if got := CastTo("42", " int "); got.Type != "int" {
		t.Fatalf("unexpected CastTo() type: %#v", got)
	}

	projections := []Projection{
		F("name").As("name_alias"),
		V("x").As("literal_alias"),
		Lower(F("name")).As("lower_alias"),
		F("score").Cast("double").As("score_alias"),
	}

	for _, projection := range projections {
		if projection.Alias == "" || projection.Expr == nil {
			t.Fatalf("unexpected projection alias: %#v", projection)
		}
	}
}

func TestPredicateHelpers(t *testing.T) {
	tests := []struct {
		name       string
		expr       Expr
		wantOp     Operator
		wantScalar bool
	}{
		{"top ilike", ILike(F("name"), "%a%"), OpILike, true},
		{"top regexp", Regexp(F("name"), "^[a-z]+$"), OpRegexp, true},
		{"ref ne", F("name").Ne("x"), OpNe, true},
		{"ref gt", F("age").Gt(18), OpGt, true},
		{"ref gte", F("age").Gte(18), OpGte, true},
		{"ref lt", F("age").Lt(18), OpLt, true},
		{"ref lte", F("age").Lte(18), OpLte, true},
		{"ref like", F("name").Like("%a%"), OpLike, true},
		{"ref ilike", F("name").ILike("%a%"), OpILike, true},
		{"ref regexp", F("name").Regexp("a"), OpRegexp, true},
		{"ref contains", F("name").Contains("a"), OpContains, true},
		{"ref prefix", F("name").Prefix("a"), OpPrefix, true},
		{"ref suffix", F("name").Suffix("a"), OpSuffix, true},
		{"ref in", F("name").In("a", "b"), OpIn, false},
		{"ref not in", F("name").NotIn("a", "b"), OpNotIn, false},
		{"ref is null", F("name").IsNull(), OpIsNull, false},
		{"ref not null", F("name").NotNull(), OpNotNull, false},
		{"call eq", Lower(F("name")).Eq("a"), OpEq, true},
		{"call ne", Lower(F("name")).Ne("a"), OpNe, true},
		{"call gt", Lower(F("name")).Gt("a"), OpGt, true},
		{"call gte", Lower(F("name")).Gte("a"), OpGte, true},
		{"call lt", Lower(F("name")).Lt("a"), OpLt, true},
		{"call lte", Lower(F("name")).Lte("a"), OpLte, true},
		{"call like", Lower(F("name")).Like("a"), OpLike, true},
		{"call ilike", Lower(F("name")).ILike("a"), OpILike, true},
		{"call regexp", Lower(F("name")).Regexp("a"), OpRegexp, true},
		{"call contains", Lower(F("name")).Contains("a"), OpContains, true},
		{"call prefix", Lower(F("name")).Prefix("a"), OpPrefix, true},
		{"call suffix", Lower(F("name")).Suffix("a"), OpSuffix, true},
		{"call in", Lower(F("name")).In("a", "b"), OpIn, false},
		{"call not in", Lower(F("name")).NotIn("a", "b"), OpNotIn, false},
		{"call is null", Lower(F("name")).IsNull(), OpIsNull, false},
		{"call not null", Lower(F("name")).NotNull(), OpNotNull, false},
		{"cast eq", F("score").Cast("double").Eq(1), OpEq, true},
		{"cast ne", F("score").Cast("double").Ne(1), OpNe, true},
		{"cast gt", F("score").Cast("double").Gt(1), OpGt, true},
		{"cast gte", F("score").Cast("double").Gte(1), OpGte, true},
		{"cast lt", F("score").Cast("double").Lt(1), OpLt, true},
		{"cast lte", F("score").Cast("double").Lte(1), OpLte, true},
		{"cast like", F("score").Cast("string").Like("1"), OpLike, true},
		{"cast ilike", F("score").Cast("string").ILike("1"), OpILike, true},
		{"cast regexp", F("score").Cast("string").Regexp("1"), OpRegexp, true},
		{"cast contains", F("score").Cast("string").Contains("1"), OpContains, true},
		{"cast prefix", F("score").Cast("string").Prefix("1"), OpPrefix, true},
		{"cast suffix", F("score").Cast("string").Suffix("1"), OpSuffix, true},
		{"cast in", F("score").Cast("double").In(1, 2), OpIn, false},
		{"cast not in", F("score").Cast("double").NotIn(1, 2), OpNotIn, false},
		{"cast is null", F("score").Cast("double").IsNull(), OpIsNull, false},
		{"cast not null", F("score").Cast("double").NotNull(), OpNotNull, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			predicate, ok := tc.expr.(Predicate)
			if !ok {
				t.Fatalf("expected Predicate, got %T", tc.expr)
			}

			if predicate.Op != tc.wantOp {
				t.Fatalf("unexpected operator: %#v", predicate)
			}

			switch predicate.Right.(type) {
			case ScalarOperand:
				if !tc.wantScalar {
					t.Fatalf("expected list-or-empty operand, got scalar: %#v", predicate)
				}
			case ListOperand:
				if tc.wantScalar {
					t.Fatalf("expected scalar operand, got list: %#v", predicate)
				}
			case nil:
				if tc.wantScalar {
					t.Fatalf("expected scalar operand, got nil: %#v", predicate)
				}
			default:
				t.Fatalf("unexpected operand type %T", predicate.Right)
			}
		})
	}
}
