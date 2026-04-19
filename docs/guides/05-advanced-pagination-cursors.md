# 05. Pagination And Cursors

`qb` supports offset-style pagination directly and cursor pagination as metadata
that your rewrite pipeline can translate into filters.

## Page And Size

```go
query, err := qb.New().
	SortBy("created_at", qb.Desc).
	Page(3).
	Size(25).
	Query()
```

`ResolvedPagination()` turns that into `LIMIT 25 OFFSET 50`.

## Opaque Cursor Tokens

The builder stores the token. Your transformer decides how to decode and apply
it:

```go
query, err := qb.New().
	SortBy("created_at", qb.Desc).
	Size(25).
	CursorToken(token).
	Query()

statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(rewriteCursorToken),
).Compile(query)
```

```go
func rewriteCursorToken(query qb.Query) (qb.Query, error) {
	if query.Cursor == nil {
		return query, nil
	}

	cursor, err := decodeCursor(query.Cursor.Token)
	if err != nil {
		return qb.Query{}, err
	}

	query.Filter = qb.F("created_at").Lt(cursor.CreatedAt)
	query.Cursor = nil
	return query, nil
}
```

## Composite Cursors

Structured cursor values work well for multi-column sorts:

```go
query, err := qb.New().
	SortBy("created_at", qb.Desc).
	SortBy("id", qb.Desc).
	Size(25).
	CursorValues(map[string]any{
		"created_at": createdAt,
		"id":         int64(981),
	}).
	Query()
```

```go
func rewriteCompositeCursor(query qb.Query) (qb.Query, error) {
	if query.Cursor == nil {
		return query, nil
	}

	createdAt := query.Cursor.Values["created_at"]
	id := query.Cursor.Values["id"]
	query.Filter = qb.Or(
		qb.F("created_at").Lt(createdAt),
		qb.And(
			qb.F("created_at").Eq(createdAt),
			qb.F("id").Lt(id),
		),
	)
	query.Cursor = nil
	return query, nil
}
```

## Schema-Normalized Cursor Payloads

When cursor fields come from external input, let the schema normalize and decode
them before your cursor rewrite runs:

```go
query, err := codecs.Parse(
	map[string]any{
		"$sort": []any{"createdAt desc", "id desc"},
		"$cursor": map[string]any{
			"createdAt": "2026-04-11T12:00:00Z",
			"id":        "981",
		},
		"$size": 25,
	},
	codecs.WithSortFieldResolver(orderSchema.ResolveSortField),
	codecs.WithValueDecoder(orderSchema.DecodeValue),
)

statement, err := sqladapter.New(
	sqladapter.WithQueryTransformer(orderSchema.ToStorage),
	sqladapter.WithQueryTransformer(rewriteCursor),
).Compile(query)
```

## Cursor Rules

- cursor pagination requires `Size`
- cursor pagination cannot be combined with `Page`
- cursor pagination cannot be combined with legacy `Limit` and `Offset`
- adapters do not interpret cursor metadata directly; your transformer must clear `query.Cursor`

## Matching Examples

- [examples/core/cursor-token](../../examples/core/cursor-token/main.go)
- [examples/core/composite-cursor](../../examples/core/composite-cursor/main.go)
- [examples/schema/cursor-normalization](../../examples/schema/cursor-normalization/main.go)
