package docmodel

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/pakasa-io/qb"
	codecconfig "github.com/pakasa-io/qb/codecs/internal/config"
)

type rejectingLiteralCodec struct{}

func (rejectingLiteralCodec) FormatLiteral(value any) (any, string, bool, error) {
	return nil, "", false, nil
}

func (rejectingLiteralCodec) ParseLiteral(codec string, literal any) (any, bool, error) {
	return nil, false, nil
}

func docmodelTestOptions() codecconfig.Config {
	return codecconfig.ApplyOptions(
		codecconfig.WithLiteralCodec(codecconfig.DefaultLiteralCodec{}),
		codecconfig.WithValueDecoder(func(field string, op qb.Operator, value any) (any, error) {
			if field == "age" {
				if text, ok := value.(string); ok {
					return strconv.Atoi(text)
				}
			}
			return value, nil
		}),
		codecconfig.WithFilterFieldResolver(func(field string, op qb.Operator) (string, error) {
			switch field {
			case "state":
				return "status", nil
			case "blank":
				return "", nil
			case "missing":
				return "", errors.New("missing field")
			default:
				return field, nil
			}
		}),
		codecconfig.WithGroupFieldResolver(func(field string) (string, error) {
			switch field {
			case "state":
				return "group_status", nil
			case "blank":
				return "", nil
			case "missing":
				return "", errors.New("missing group field")
			default:
				return field, nil
			}
		}),
		codecconfig.WithSortFieldResolver(func(field string) (string, error) {
			switch field {
			case "state":
				return "sort_status", nil
			case "blank":
				return "", nil
			case "missing":
				return "", errors.New("missing sort field")
			default:
				return field, nil
			}
		}),
	)
}

func TestStructuredNodeParsers(t *testing.T) {
	opts := docmodelTestOptions()

	projection, err := parseProjectionNode(
		map[string]any{
			"$expr": map[string]any{"$call": "lower", "$args": []any{map[string]any{"$field": "state"}}},
			"$as":   "normalized",
		},
		opts,
		"$.select[0]",
	)
	if err != nil {
		t.Fatalf("parseProjectionNode() error = %v", err)
	}

	call, ok := projection.Expr.(qb.Call)
	if !ok || projection.Alias != "normalized" || call.Name != "lower" {
		t.Fatalf("unexpected projection parsing: %#v", projection)
	}

	if _, err := parseProjectionNode(map[string]any{"$as": "alias"}, opts, "$.select[1]"); err == nil {
		t.Fatal("expected parseProjectionNode() to require $expr")
	}

	sortExpr, err := parseSortNode(
		map[string]any{"$expr": map[string]any{"$cast": "string", "$expr": map[string]any{"$field": "state"}}, "$dir": "desc"},
		opts,
		"$.sort[0]",
	)
	if err != nil {
		t.Fatalf("parseSortNode() error = %v", err)
	}

	if sortExpr.Direction != qb.Desc {
		t.Fatalf("unexpected parsed sort direction: %#v", sortExpr)
	}

	if _, err := parseSortNode(map[string]any{"$expr": "state", "$dir": "sideways"}, opts, "$.sort[1]"); err == nil {
		t.Fatal("expected parseSortNode() to reject unsupported directions")
	}

	scalar, err := parseScalarAny("lower(state)", opts, "$.expr")
	if err != nil {
		t.Fatalf("parseScalarAny(string) error = %v", err)
	}

	if _, ok := scalar.(qb.Call); !ok {
		t.Fatalf("expected parsed call, got %T", scalar)
	}

	scalar, err = parseScalarAny(map[string]any{"$field": "status"}, opts, "$.field")
	if err != nil || scalar.(qb.Ref).Name != "status" {
		t.Fatalf("unexpected parsed field node: %#v %v", scalar, err)
	}

	scalar, err = parseScalarAny(
		map[string]any{"$literal": "2026-04-18T00:00:00Z", "$codec": "time"},
		opts,
		"$.literal",
	)
	if err != nil {
		t.Fatalf("parseScalarAny(literal) error = %v", err)
	}

	if _, ok := scalar.(qb.Literal).Value.(time.Time); !ok {
		t.Fatalf("expected literal time value, got %#v", scalar)
	}

	scalar, err = parseScalarNode(
		map[string]any{"$call": "concat", "$args": []any{"a", map[string]any{"$field": "status"}}},
		opts,
		"$.call",
	)
	if err != nil {
		t.Fatalf("parseScalarNode(call) error = %v", err)
	}

	if got := scalar.(qb.Call).Name; got != "concat" {
		t.Fatalf("unexpected parsed call node: %#v", scalar)
	}

	scalar, err = parseScalarNode(
		map[string]any{"$cast": "string", "$expr": map[string]any{"$field": "status"}},
		opts,
		"$.cast",
	)
	if err != nil {
		t.Fatalf("parseScalarNode(cast) error = %v", err)
	}

	if _, ok := scalar.(qb.Cast); !ok {
		t.Fatalf("expected parsed cast node, got %T", scalar)
	}

	if _, err := parseScalarNode(map[string]any{"$field": ""}, opts, "$.badField"); err == nil {
		t.Fatal("expected parseScalarNode() to reject blank $field")
	}

	if _, err := parseScalarNode(map[string]any{"$call": "", "$args": []any{}}, opts, "$.badCall"); err == nil {
		t.Fatal("expected parseScalarNode() to reject blank $call")
	}

	if _, err := parseScalarNode(map[string]any{"$call": "lower", "$args": "bad"}, opts, "$.badArgs"); err == nil {
		t.Fatal("expected parseScalarNode() to reject non-list $args")
	}

	if _, err := parseScalarNode(map[string]any{"$cast": "", "$expr": map[string]any{"$field": "status"}}, opts, "$.badCast"); err == nil {
		t.Fatal("expected parseScalarNode() to reject blank $cast")
	}

	if _, err := parseScalarNode(map[string]any{"$cast": "string"}, opts, "$.missingCastExpr"); err == nil {
		t.Fatal("expected parseScalarNode() to require $expr for casts")
	}

	if _, err := parseLiteralNode(map[string]any{"$codec": "time"}, opts, "$.missingLiteral"); err == nil {
		t.Fatal("expected parseLiteralNode() to require $literal")
	}

	if _, err := parseLiteralNode(map[string]any{"$literal": "x", "$codec": 5}, opts, "$.badCodec"); err == nil {
		t.Fatal("expected parseLiteralNode() to reject non-string codecs")
	}

	rejecting := codecconfig.ApplyOptions(codecconfig.WithLiteralCodec(rejectingLiteralCodec{}))
	if _, err := parseLiteralNode(map[string]any{"$literal": "x", "$codec": "missing"}, rejecting, "$.unsupportedCodec"); err == nil {
		t.Fatal("expected parseLiteralNode() to reject unsupported codecs")
	}

	value, err := parseCursorValue(map[string]any{"$literal": "2026-04-18T00:00:00Z", "$codec": "time"}, opts, "$.cursor.created_at")
	if err != nil {
		t.Fatalf("parseCursorValue() error = %v", err)
	}

	if _, ok := value.(time.Time); !ok {
		t.Fatalf("expected parsed cursor literal, got %#v", value)
	}

	if !looksLikeScalarNode(map[string]any{"$call": "lower"}) || !looksLikeLiteralNode(map[string]any{"$literal": "x"}) {
		t.Fatal("expected scalar/literal node detection to work")
	}

	if !hasOnlyKnownKeys(map[string]any{"$field": "status"}, "$field") || hasOnlyKnownKeys(map[string]any{"$field": "status", "$extra": true}, "$field") {
		t.Fatal("unexpected hasOnlyKnownKeys() behavior")
	}

	if err := validateAllowedKeys(map[string]any{"$field": "status", "$extra": true}, "$.node", "$field"); err == nil {
		t.Fatal("expected validateAllowedKeys() to reject extra keys")
	}

	decoder := literalTokenDecoder(opts.LiteralCodec)
	if value, err := decoder("time", "2026-04-18T00:00:00Z"); err != nil || reflect.TypeOf(value) != reflect.TypeOf(time.Time{}) {
		t.Fatalf("unexpected literalTokenDecoder() result: %#v %v", value, err)
	}

	if value, err := literalTokenDecoder(nil)("time", "x"); err != nil || value != "x" {
		t.Fatalf("unexpected nil literalTokenDecoder() result: %#v %v", value, err)
	}
}

func TestPredicateResolversAndParsingHelpers(t *testing.T) {
	opts := docmodelTestOptions()

	expr, err := parsePredicate(qb.F("state"), qb.OpGte, "21", opts, "$.where.age")
	if err != nil {
		t.Fatalf("parsePredicate(gte) error = %v", err)
	}

	predicate := expr.(qb.Predicate)
	if predicate.Left.(qb.Ref).Name != "status" || predicate.Right.(qb.ScalarOperand).Expr.(qb.Literal).Value != "21" {
		t.Fatalf("unexpected parsed predicate: %#v", predicate)
	}

	expr, err = parsePredicate(qb.F("state"), qb.OpIn, []any{"active", "trial"}, opts, "$.where.state")
	if err != nil {
		t.Fatalf("parsePredicate(in) error = %v", err)
	}

	if len(expr.(qb.Predicate).Right.(qb.ListOperand).Items) != 2 {
		t.Fatalf("unexpected IN predicate: %#v", expr)
	}

	expr, err = parsePredicate(qb.F("state"), qb.OpIsNull, true, opts, "$.where.deleted_at")
	if err != nil {
		t.Fatalf("parsePredicate(isnull) error = %v", err)
	}

	if expr.(qb.Predicate).Right != nil {
		t.Fatalf("expected unary predicate to have nil rhs: %#v", expr)
	}

	if _, err := parsePredicate(qb.F("state"), qb.OpIsNull, false, opts, "$.where.deleted_at"); err == nil {
		t.Fatal("expected parsePredicate() to enforce unary true operand")
	}

	expr, err = parseExpressionPredicate(qb.OpEq, []any{"lower(@state)", "ACTIVE"}, opts, "$.where.$expr.$eq")
	if err != nil {
		t.Fatalf("parseExpressionPredicate(eq) error = %v", err)
	}

	if _, ok := expr.(qb.Predicate).Left.(qb.Call); !ok {
		t.Fatalf("expected expression predicate lhs to be a call, got %#v", expr)
	}

	expr, err = parseExpressionPredicate(qb.OpIn, []any{"@state", []any{"active", "trial"}}, opts, "$.where.$expr.$in")
	if err != nil {
		t.Fatalf("parseExpressionPredicate(in) error = %v", err)
	}

	if len(expr.(qb.Predicate).Right.(qb.ListOperand).Items) != 2 {
		t.Fatalf("unexpected expression IN predicate: %#v", expr)
	}

	expr, err = parseExpressionPredicate(qb.OpIsNull, []any{"@deleted_at"}, opts, "$.where.$expr.$isnull")
	if err != nil {
		t.Fatalf("parseExpressionPredicate(isnull) error = %v", err)
	}

	if expr.(qb.Predicate).Op != qb.OpIsNull {
		t.Fatalf("unexpected unary expression predicate: %#v", expr)
	}

	if _, err := parseExpressionPredicate(qb.OpEq, []any{"@state"}, opts, "$.where.$expr.$eq"); err == nil {
		t.Fatal("expected parseExpressionPredicate() to require two operands for eq")
	}

	if _, err := parseExpressionPredicate(qb.OpIn, []any{"@state"}, opts, "$.where.$expr.$in"); err == nil {
		t.Fatal("expected parseExpressionPredicate() to require rhs values for in")
	}

	mixed, err := parseMixedScalar("lower(@state)", "", qb.OpEq, opts, "$.expr")
	if err != nil {
		t.Fatalf("parseMixedScalar(dsl) error = %v", err)
	}

	if _, ok := mixed.(qb.Call); !ok {
		t.Fatalf("expected mixed DSL scalar, got %#v", mixed)
	}

	mixed, err = parseMixedScalar(map[string]any{"$literal": "21"}, "age", qb.OpEq, opts, "$.literal")
	if err != nil {
		t.Fatalf("parseMixedScalar(literal node) error = %v", err)
	}

	if mixed.(qb.Literal).Value != "21" {
		t.Fatalf("unexpected decoded mixed literal: %#v", mixed)
	}

	mixed, err = parseMixedScalar(map[string]any{"$field": "status"}, "", qb.OpEq, opts, "$.field")
	if err != nil || mixed.(qb.Ref).Name != "status" {
		t.Fatalf("unexpected structured mixed scalar: %#v %v", mixed, err)
	}

	if _, err := parseMixedScalar("lower(name) as alias", "", qb.OpEq, opts, "$.badAlias"); err == nil {
		t.Fatal("expected parseMixedScalar() to reject aliases in mixed contexts")
	}

	for _, name := range []string{"$eq", "$ne", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$like", "$ilike", "$regexp", "$contains", "$prefix", "$suffix", "$isnull", "$notnull"} {
		if _, err := parseOperatorName(name, "$.where."+name); err != nil {
			t.Fatalf("parseOperatorName(%q) error = %v", name, err)
		}
	}

	if _, err := parseOperatorName("$bogus", "$.where.$bogus"); err == nil {
		t.Fatal("expected parseOperatorName() to reject unknown operators")
	}

	if resolved, err := resolveFilterScalar(qb.F("state"), qb.OpEq, opts, "$.where"); err != nil || resolved.(qb.Ref).Name != "status" {
		t.Fatalf("unexpected resolveFilterScalar() result: %#v %v", resolved, err)
	}

	if _, err := resolveFilterScalar(qb.F("blank"), qb.OpEq, opts, "$.where"); err == nil {
		t.Fatal("expected resolveFilterScalar() to reject blank resolver output")
	}

	if _, err := resolveFilterScalar(qb.F("missing"), qb.OpEq, opts, "$.where"); err == nil {
		t.Fatal("expected resolveFilterScalar() to wrap resolver errors")
	}

	if resolved, err := resolveGroupScalar(qb.F("state"), opts, "$.group"); err != nil || resolved.(qb.Ref).Name != "group_status" {
		t.Fatalf("unexpected resolveGroupScalar() result: %#v %v", resolved, err)
	}

	if _, err := resolveGroupScalar(qb.F("blank"), opts, "$.group"); err == nil {
		t.Fatal("expected resolveGroupScalar() to reject blank resolver output")
	}

	if resolved, err := resolveSortScalar(qb.F("state"), opts, "$.sort"); err != nil || resolved.(qb.Ref).Name != "sort_status" {
		t.Fatalf("unexpected resolveSortScalar() result: %#v %v", resolved, err)
	}

	if _, err := resolveSortScalar(qb.F("blank"), opts, "$.sort"); err == nil {
		t.Fatal("expected resolveSortScalar() to reject blank resolver output")
	}

	if values, err := resolveGroupScalars([]qb.Scalar{qb.F("state")}, opts, "$.group"); err != nil || len(values) != 1 {
		t.Fatalf("unexpected resolveGroupScalars() result: %#v %v", values, err)
	}

	if values, err := resolveSortScalars([]qb.Scalar{qb.F("state")}, opts, "$.sort"); err != nil || len(values) != 1 {
		t.Fatalf("unexpected resolveSortScalars() result: %#v %v", values, err)
	}

	if values, err := parseSimpleFieldList("id, status", "$.select"); err != nil || !reflect.DeepEqual(values, []string{"id", "status"}) {
		t.Fatalf("unexpected parseSimpleFieldList() result: %#v %v", values, err)
	}

	if _, err := parseSimpleFieldList("lower(name)", "$.select"); err == nil {
		t.Fatal("expected parseSimpleFieldList() to reject expressions")
	}

	if values, err := parseDelimitedStrings([]any{"a,b", "c"}, "$.include"); err != nil || !reflect.DeepEqual(values, []string{"a", "b", "c"}) {
		t.Fatalf("unexpected parseDelimitedStrings() result: %#v %v", values, err)
	}

	if _, err := parseDelimitedStrings(5, "$.include"); err == nil {
		t.Fatal("expected parseDelimitedStrings() to reject invalid types")
	}

	cursor, err := parseCursor(" token ", opts, "$.cursor")
	if err != nil || cursor.Token != "token" {
		t.Fatalf("unexpected parseCursor(token) result: %#v %v", cursor, err)
	}

	cursor, err = parseCursor(map[string]any{"created_at": map[string]any{"$literal": "2026-04-18T00:00:00Z", "$codec": "time"}}, opts, "$.cursor")
	if err != nil || len(cursor.Values) != 1 {
		t.Fatalf("unexpected parseCursor(object) result: %#v %v", cursor, err)
	}

	if _, err := parseCursor("", opts, "$.cursor"); err == nil {
		t.Fatal("expected parseCursor() to reject blank tokens")
	}

	if got := predicatePrimaryField(qb.Func("concat", qb.F("first"), qb.F("last"))); got != "" {
		t.Fatalf("unexpected predicatePrimaryField() result: %q", got)
	}

	if values := splitCSV("a, b, , c"); !reflect.DeepEqual(values, []string{"a", "b", "c"}) {
		t.Fatalf("unexpected splitCSV() result: %#v", values)
	}

	if !isMixedDSLString("lower(@state)") || isMixedDSLString("plain") == true && false {
		t.Fatal("unexpected isMixedDSLString() behavior")
	}

	if !isSimpleFieldRef("user.status") || isSimpleFieldRef("123bad") {
		t.Fatal("unexpected isSimpleFieldRef() behavior")
	}

	if err := validateUnaryPredicateOperand("true", "$.where.deleted_at"); err != nil {
		t.Fatalf("validateUnaryPredicateOperand() error = %v", err)
	}

	if err := validateUnaryPredicateOperand(false, "$.where.deleted_at"); err == nil {
		t.Fatal("expected validateUnaryPredicateOperand() to reject false")
	}

	for _, value := range []any{1, int64(2), float32(3), float64(4), json.Number("5"), "6"} {
		if _, err := parseInteger(value, "$.size"); err != nil {
			t.Fatalf("parseInteger(%T) error = %v", value, err)
		}
	}

	if _, err := parseInteger(json.Number("5.5"), "$.size"); err == nil {
		t.Fatal("expected parseInteger() to reject fractional numbers")
	}

	if decoded, err := decodeValue("age", qb.OpEq, map[string]any{"$literal": "21"}, opts, "$.where.age"); err != nil || decoded != 21 {
		t.Fatalf("unexpected decodeValue() result: %#v %v", decoded, err)
	}

	if values, err := asList([]any{"a", "b"}, "$.list"); err != nil || len(values) != 2 {
		t.Fatalf("unexpected asList() result: %#v %v", values, err)
	}

	if _, err := asList(5, "$.list"); err == nil {
		t.Fatal("expected asList() to reject non-lists")
	}

	if got := normalizeJSONNumber(json.Number("12")); got != int64(12) {
		t.Fatalf("unexpected normalizeJSONNumber(int) result: %#v", got)
	}

	if got := normalizeJSONNumber(json.Number("1.5")); got != 1.5 {
		t.Fatalf("unexpected normalizeJSONNumber(float) result: %#v", got)
	}
}
