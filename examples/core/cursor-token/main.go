package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pakasa-io/qb"
	sqladapter "github.com/pakasa-io/qb/adapters/sql"
)

type cursorToken struct {
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	token := encodeCursor(cursorToken{
		CreatedAt: time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
	})

	query, err := qb.New().
		SortBy("created_at", qb.Desc).
		Size(25).
		CursorToken(token).
		Query()
	if err != nil {
		panic(err)
	}

	statement, err := sqladapter.New(
		sqladapter.WithQueryTransformer(rewriteCursorToken),
	).Compile(query)
	if err != nil {
		panic(err)
	}

	fmt.Println(statement.SQL)
	fmt.Printf("%#v\n", statement.Args)
}

func rewriteCursorToken(query qb.Query) (qb.Query, error) {
	if query.Cursor == nil {
		return query, nil
	}

	cursor, err := decodeCursor(query.Cursor.Token)
	if err != nil {
		return qb.Query{}, err
	}

	query.Filter = qb.Field("created_at").Lt(cursor.CreatedAt)
	query.Cursor = nil
	return query, nil
}

func encodeCursor(cursor cursorToken) string {
	data, err := json.Marshal(cursor)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(token string) (cursorToken, error) {
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return cursorToken{}, err
	}

	var cursor cursorToken
	err = json.Unmarshal(data, &cursor)
	return cursor, err
}
