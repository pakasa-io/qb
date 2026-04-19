# 09. Codec Output And Literal Round-Trips

The transport packages can also emit JSON, YAML, and query-string documents from
an existing `qb.Query`.

## Marshal Back To JSON And YAML

```go
payload, err := jsoncodec.Marshal(query)
if err != nil {
	panic(err)
}

yamlPayload, err := yamlcodec.Marshal(query)
if err != nil {
	panic(err)
}
```

## Encode To Query Strings

```go
raw, err := querystring.Encode(query)
if err != nil {
	panic(err)
}

values, err := querystring.EncodeValues(query)
if err != nil {
	panic(err)
}
```

## Compact Output

Encoders support canonical and compact output modes:

```go
payload, err := jsoncodec.Marshal(query, codecs.WithMode(codecs.Compact))
raw, err := querystring.Encode(query, codecs.WithMode(codecs.Compact))
```

Compact mode uses string shorthands only when the query can be represented
losslessly that way. Expression-bearing selects, groups, and sorts still fall
back to canonical arrays or structured objects.

## Built-In Typed Literals

The default literal codec preserves common non-JSON-native values such as
`time.Time`, `time.Duration`, and `uuid.UUID` during JSON and YAML round trips:

```go
joinedAt := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

query, err := qb.New().
	Where(qb.F("users.joined_at").Gte(joinedAt)).
	Query()

payload, err := jsoncodec.Marshal(query)
parsed, err := jsoncodec.Parse(payload)
```

For query strings, the encoder flattens leaves to plain strings and does not
preserve `$codec` metadata. Use schema decoders or string-preserving query-string
parsing patterns when that transport needs type control.

## Reversible Text Types

You can register your own transport-safe literal type when it implements
`encoding.TextMarshaler` and `encoding.TextUnmarshaler`:

```go
type Status string

func (s Status) MarshalText() ([]byte, error) { return []byte("X-" + string(s)), nil }

func (s *Status) UnmarshalText(text []byte) error {
	*s = Status(strings.TrimPrefix(string(text), "X-"))
	return nil
}

_ = codecs.RegisterReversibleTextType("status_text", new(Status))
_ = codecs.SetDefaultLiteralCodecMode(codecs.LiteralCodecModeReversibleText)
```

Once enabled, `jsoncodec.Marshal` and `jsoncodec.Parse` can round-trip that type
through codec-tagged literals.

For a full custom codec implementation, see
[10. Custom Literal Codecs](./10-custom-literal-codecs.md).

## Matching Examples

- [codecs/json round-trip tests](../../codecs/json/codec_test.go)
- [codecs/yaml round-trip tests](../../codecs/yaml/codec_test.go)
- [codecs/query-string round-trip tests](../../codecs/qs/encode_test.go)
- [literal codec tests](../../codecs/literal_test.go)
