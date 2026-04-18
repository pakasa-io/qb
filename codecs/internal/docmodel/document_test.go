package docmodel

import (
	"reflect"
	"testing"
	"time"

	"github.com/pakasa-io/qb"
	codecconfig "github.com/pakasa-io/qb/codecs/internal/config"
	"github.com/pakasa-io/qb/codecs/internal/dsl"
)

func TestBuildDocumentCompactAndCanonical(t *testing.T) {
	compactQuery := qb.Query{
		Projections: []qb.Projection{qb.Project(qb.F("id")), qb.Project(qb.F("status"))},
		Includes:    []string{"orders"},
		Filter: qb.And(
			qb.F("status").Eq("active"),
			qb.F("age").Gte(21),
		),
		GroupBy: []qb.Scalar{qb.F("status")},
		Sorts:   []qb.Sort{{Expr: qb.F("created_at"), Direction: qb.Desc}},
		Page:    intPtr(2),
		Size:    intPtr(25),
	}

	doc, err := BuildDocument(compactQuery, TransportJSON, codecconfig.WithMode(codecconfig.Compact))
	if err != nil {
		t.Fatalf("BuildDocument(compact) error = %v", err)
	}

	if len(doc) != 7 {
		t.Fatalf("unexpected compact document: %#v", doc)
	}

	if doc[0].Key != "$select" || doc[0].Value != "id,status" {
		t.Fatalf("unexpected compact select encoding: %#v", doc[0])
	}

	if doc[1].Key != "$include" || !reflect.DeepEqual(doc[1].Value, []any{"orders"}) {
		t.Fatalf("unexpected compact include encoding: %#v", doc[1])
	}

	where, ok := doc[2].Value.(OrderedObject)
	if !ok || where[0].Key != "age" || where[1].Key != "status" {
		t.Fatalf("unexpected compact where encoding: %#v", doc[2].Value)
	}

	if doc[3].Value != "status" || doc[4].Value != "-created_at" {
		t.Fatalf("unexpected compact group/sort encoding: %#v", doc)
	}

	if doc[5].Value != 2 || doc[6].Value != 25 {
		t.Fatalf("unexpected compact page/size encoding: %#v", doc)
	}

	now := time.Date(2026, 4, 18, 12, 34, 56, 0, time.UTC)
	canonicalQuery := qb.Query{
		Projections: []qb.Projection{
			qb.Project(qb.Lower(qb.F("name"))).As("normalized_name"),
			qb.Project(qb.V(now)).As("created_at"),
		},
		Filter: qb.Lower(qb.F("name")).Eq("JOHN"),
		Sorts:  []qb.Sort{{Expr: qb.Lower(qb.F("name")), Direction: qb.Asc}},
		Cursor: &qb.Cursor{Values: map[string]any{"created_at": now, "id": 7}},
		Size:   intPtr(10),
	}

	doc, err = BuildDocument(canonicalQuery, TransportJSON, codecconfig.WithLiteralCodec(codecconfig.DefaultLiteralCodec{}))
	if err != nil {
		t.Fatalf("BuildDocument(canonical) error = %v", err)
	}

	selectValues, ok := doc[0].Value.([]any)
	if !ok || len(selectValues) != 2 {
		t.Fatalf("unexpected canonical projections: %#v", doc[0].Value)
	}

	secondProjection := selectValues[1].(OrderedObject)
	if secondProjection[0].Key != "$expr" || secondProjection[1].Key != "$as" {
		t.Fatalf("unexpected structured projection: %#v", secondProjection)
	}

	structuredLiteral := secondProjection[0].Value.(OrderedObject)
	if structuredLiteral[0].Key != "$literal" || structuredLiteral[1].Key != "$codec" || structuredLiteral[1].Value != "time" {
		t.Fatalf("unexpected structured literal: %#v", structuredLiteral)
	}

	where = doc[1].Value.(OrderedObject)
	if where[0].Key != "lower(name)" || where[0].Value != "JOHN" {
		t.Fatalf("unexpected canonical where encoding: %#v", where)
	}

	cursorObject := doc[4].Value.(OrderedObject)
	if cursorObject[0].Key != "created_at" || cursorObject[1].Key != "id" {
		t.Fatalf("unexpected cursor ordering: %#v", cursorObject)
	}
}

func TestBuildDocumentQueryStringNullsAndHelpers(t *testing.T) {
	query := qb.Query{
		Filter: qb.And(
			qb.F("deleted_at").Eq(nil),
			qb.F("status").Ne(nil),
		),
	}

	doc, err := BuildDocument(query, TransportQueryString)
	if err != nil {
		t.Fatalf("BuildDocument(querystring) error = %v", err)
	}

	where := doc[0].Value.(OrderedObject)
	if where[0].Key != "deleted_at" || where[1].Key != "status" {
		t.Fatalf("unexpected querystring where encoding: %#v", where)
	}

	left := normalizePredicateValue("active")
	right := normalizePredicateValue(map[string]any{"$gte": 18})
	if left["$eq"] != "active" || right["$gte"] != 18 {
		t.Fatalf("unexpected normalizePredicateValue() results: %#v %#v", left, right)
	}

	merged, ok := mergeWhereValue("status", map[string]any{"$gte": 18}, map[string]any{"$lte": 21})
	if !ok {
		t.Fatal("expected mergeWhereValue() to merge compatible operators")
	}

	if got := merged.(map[string]any)["$lte"]; got != 21 {
		t.Fatalf("unexpected merged predicate: %#v", merged)
	}

	if _, ok := mergeWhereValue("$and", map[string]any{}, map[string]any{}); ok {
		t.Fatal("expected mergeWhereValue() to reject top-level operator collisions")
	}

	ordered := orderWhereMap(map[string]any{
		"$or": []any{map[string]any{"b": 2}, map[string]any{"a": 1}},
		"b":   map[string]any{"$eq": 2},
		"a":   1,
	})
	if ordered[0].Key != "a" || ordered[1].Key != "b" || ordered[2].Key != "$or" {
		t.Fatalf("unexpected orderWhereMap() result: %#v", ordered)
	}

	if got := orderValue([]any{map[string]any{"b": 2, "a": 1}}, false).([]any); len(got) != 1 {
		t.Fatalf("unexpected orderValue() result: %#v", got)
	}

	if got := stringsToAny([]string{"a", "b"}); !reflect.DeepEqual(got, []any{"a", "b"}) {
		t.Fatalf("unexpected stringsToAny() result: %#v", got)
	}

	if !projectionsAreSimple([]qb.Projection{qb.Project(qb.F("id"))}) {
		t.Fatal("expected simple projections to be detected")
	}

	if !scalarsAreSimpleFields([]qb.Scalar{qb.F("user.name")}) {
		t.Fatal("expected dotted field to count as a simple scalar field")
	}

	if sortsAreSimple([]qb.Sort{{Expr: qb.F("id"), Direction: qb.Direction("sideways")}}) {
		t.Fatal("expected invalid sort direction to be rejected")
	}

	if defaultDirection("") != qb.Asc || operatorToken(qb.OpPrefix, TransportJSON, false) != "$prefix" {
		t.Fatal("unexpected helper output")
	}
}

func TestCanonicalPageSizeAndStructuredScalarHelpers(t *testing.T) {
	page, size, err := canonicalPageSize(qb.Query{Limit: intPtr(10), Offset: intPtr(20)})
	if err != nil || page == nil || size == nil || *page != 3 || *size != 10 {
		t.Fatalf("unexpected canonicalPageSize() result: page=%v size=%v err=%v", page, size, err)
	}

	tests := []qb.Query{
		{Offset: intPtr(5)},
		{Limit: intPtr(0)},
		{Limit: intPtr(10), Offset: intPtr(3)},
	}

	for _, query := range tests {
		if _, _, err := canonicalPageSize(query); err == nil {
			t.Fatalf("expected canonicalPageSize() to fail for %#v", query)
		}
	}

	options := codecconfig.ApplyOptions(codecconfig.WithLiteralCodec(codecconfig.DefaultLiteralCodec{}))

	if value, err := encodeScalar(qb.F("name"), TransportJSON, options, dsl.StandaloneContext); err != nil || value != "name" {
		t.Fatalf("unexpected encodeScalar() result: %#v %v", value, err)
	}

	structured, err := encodeStructuredScalar(qb.F("user.name").Cast("string"), TransportJSON, options)
	if err != nil {
		t.Fatalf("encodeStructuredScalar() error = %v", err)
	}

	castObject := structured.(OrderedObject)
	if castObject[0].Key != "$cast" || castObject[1].Key != "$expr" {
		t.Fatalf("unexpected cast encoding: %#v", castObject)
	}

	if value, codec, err := encodeLiteralWrapper(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC), TransportQueryString, options); err != nil || codec != "time" || value == nil {
		t.Fatalf("unexpected encodeLiteralWrapper() result: %#v %q %v", value, codec, err)
	}

	if value, err := encodeTransportLeaf(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC), TransportJSON, options, true); err != nil {
		t.Fatalf("encodeTransportLeaf() error = %v", err)
	} else if _, ok := value.(OrderedObject); !ok {
		t.Fatalf("expected wrapped transport leaf, got %#v", value)
	}

	if value, ok, err := encodeFilterKey(qb.V(map[string]any{"a": "b"}), options); err != nil || ok || value != "" {
		t.Fatalf("unexpected encodeFilterKey() result: %q %v %v", value, ok, err)
	}
}

func TestEncodePredicateAndWhereHelpers(t *testing.T) {
	options := codecconfig.ApplyOptions(codecconfig.WithLiteralCodec(codecconfig.DefaultLiteralCodec{}))

	key, value, ok, err := encodeFieldPredicate(
		qb.Predicate{Left: qb.F("status"), Op: qb.OpEq, Right: qb.ScalarOperand{Expr: qb.V("active")}},
		TransportJSON,
		options,
	)
	if err != nil || !ok || key != "status" || value != "active" {
		t.Fatalf("unexpected encodeFieldPredicate(eq) result: %q %#v %v %v", key, value, ok, err)
	}

	key, value, ok, err = encodeFieldPredicate(
		qb.Predicate{Left: qb.F("deleted_at"), Op: qb.OpEq, Right: qb.ScalarOperand{Expr: qb.V(nil)}},
		TransportQueryString,
		options,
	)
	if err != nil || !ok || key != "deleted_at" {
		t.Fatalf("unexpected encodeFieldPredicate(null eq) result: %q %#v %v %v", key, value, ok, err)
	}

	key, value, ok, err = encodeFieldPredicate(
		qb.Predicate{Left: qb.F("deleted_at"), Op: qb.OpNotNull},
		TransportJSON,
		options,
	)
	if err != nil || !ok || key != "deleted_at" {
		t.Fatalf("unexpected encodeFieldPredicate(not null) result: %q %#v %v %v", key, value, ok, err)
	}

	if _, _, ok, err = encodeFieldPredicate(
		qb.Predicate{Left: qb.Lower(qb.F("name")), Op: qb.OpEq, Right: qb.ScalarOperand{Expr: qb.Lower(qb.F("other_name"))}},
		TransportJSON,
		options,
	); err != nil || ok {
		t.Fatalf("expected computed predicate to fall back to expression mode, got ok=%v err=%v", ok, err)
	}

	exprValue, err := encodeExpressionPredicate(
		qb.Predicate{Left: qb.Lower(qb.F("name")), Op: qb.OpEq, Right: qb.ScalarOperand{Expr: qb.V("john")}},
		TransportJSON,
		options,
	)
	if err != nil {
		t.Fatalf("encodeExpressionPredicate(eq) error = %v", err)
	}

	if _, ok := exprValue["$eq"]; !ok {
		t.Fatalf("unexpected expression predicate encoding: %#v", exprValue)
	}

	exprValue, err = encodeExpressionPredicate(
		qb.Predicate{Left: qb.F("status"), Op: qb.OpIn, Right: qb.ListOperand{Items: []qb.Scalar{qb.V("active"), qb.V("trial")}}},
		TransportJSON,
		options,
	)
	if err != nil {
		t.Fatalf("encodeExpressionPredicate(in) error = %v", err)
	}

	if _, ok := exprValue["$in"]; !ok {
		t.Fatalf("unexpected list expression predicate encoding: %#v", exprValue)
	}

	exprValue, err = encodeExpressionPredicate(
		qb.Predicate{Left: qb.F("deleted_at"), Op: qb.OpIsNull},
		TransportJSON,
		options,
	)
	if err != nil {
		t.Fatalf("encodeExpressionPredicate(isnull) error = %v", err)
	}

	if _, ok := exprValue["$isnull"]; !ok {
		t.Fatalf("unexpected unary expression predicate encoding: %#v", exprValue)
	}

	whereRaw, err := encodeWhereRaw(qb.Not(qb.F("deleted_at").Eq(nil)), TransportJSON, options)
	if err != nil {
		t.Fatalf("encodeWhereRaw(negation) error = %v", err)
	}

	if _, ok := whereRaw["$not"]; !ok {
		t.Fatalf("unexpected negation where encoding: %#v", whereRaw)
	}

	whereRaw, err = encodeWhereRaw(
		qb.Or(qb.F("status").Eq("active"), qb.F("status").Eq("trial")),
		TransportJSON,
		options,
	)
	if err != nil {
		t.Fatalf("encodeWhereRaw(or) error = %v", err)
	}

	if _, ok := whereRaw["$or"]; !ok {
		t.Fatalf("unexpected or-group encoding: %#v", whereRaw)
	}

	whereRaw, err = encodeAndWhere(
		[]qb.Expr{qb.F("status").Eq("active"), qb.F("status").Eq("trial")},
		TransportJSON,
		options,
	)
	if err != nil {
		t.Fatalf("encodeAndWhere(conflict) error = %v", err)
	}

	if _, ok := whereRaw["$and"]; !ok {
		t.Fatalf("unexpected conflicting and-group encoding: %#v", whereRaw)
	}

	if merged := mergeWhereMap(map[string]any{"status": "active"}, map[string]any{"age": 21}); !merged {
		t.Fatal("expected mergeWhereMap() to merge disjoint fields")
	}

	if token := operatorToken(qb.OpNotNull, TransportQueryString, true); token != "$notnull" {
		t.Fatalf("unexpected operatorToken() result: %q", token)
	}
}

func intPtr(v int) *int {
	return &v
}
