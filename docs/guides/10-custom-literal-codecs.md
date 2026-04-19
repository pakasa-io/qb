# 10. Custom Literal Codecs

Use a custom literal codec when JSON or YAML documents need to round-trip
application-specific literal types instead of plain strings or numbers.

This is a deeper companion to [09. Codec Output And Literal Round-Trips](./09-complex-codec-output.md).

## When You Need A Custom Codec

Use `codecs.WithLiteralCodec(...)` when:

- your query literals include domain types that are not covered by the built-in codec
- JSON or YAML payloads need stable `$literal` and `$codec` wrappers
- parser input should rehydrate domain values instead of leaving them as generic strings

The built-in `codecs.DefaultLiteralCodec` already handles:

- `time.Time`
- `time.Duration`
- `uuid.UUID`

## Implement The Interface

The interface has two methods:

```go
type LiteralCodec interface {
	FormatLiteral(value any) (literal any, codec string, handled bool, err error)
	ParseLiteral(codec string, literal any) (value any, handled bool, err error)
}
```

## Delegate To The Default Codec

If you replace the literal codec entirely, you also replace the built-in time,
duration, and UUID support. In practice, most custom codecs should delegate
unknown values back to `codecs.DefaultLiteralCodec`.

```go
type Status string

type userLiteralCodec struct {
	fallback codecs.DefaultLiteralCodec
}

func (c userLiteralCodec) FormatLiteral(value any) (any, string, bool, error) {
	switch typed := value.(type) {
	case Status:
		return string(typed), "status", true, nil
	default:
		return c.fallback.FormatLiteral(value)
	}
}

func (c userLiteralCodec) ParseLiteral(codec string, literal any) (any, bool, error) {
	switch codec {
	case "status":
		return Status(fmt.Sprint(literal)), true, nil
	default:
		return c.fallback.ParseLiteral(codec, literal)
	}
}
```

## Use The Codec On Both Marshal And Parse

```go
literalCodec := userLiteralCodec{
	fallback: codecs.DefaultLiteralCodec{},
}

payload, err := jsoncodec.Marshal(
	query,
	codecs.WithLiteralCodec(literalCodec),
)
if err != nil {
	panic(err)
}

parsed, err := jsoncodec.Parse(
	payload,
	codecs.WithLiteralCodec(literalCodec),
)
if err != nil {
	panic(err)
}
```

If you only configure the codec on one side, you will either emit wrappers that
cannot be parsed back or fail to emit wrappers at all.

## What The Payload Looks Like

When the codec handles a value, JSON and YAML encoders emit a typed literal
wrapper:

```json
{
  "$literal": "active",
  "$codec": "status"
}
```

The parser accepts the same shape in normalized documents:

```go
query, err := codecs.Parse(
	map[string]any{
		"$where": map[string]any{
			"status": map[string]any{
				"$eq": map[string]any{
					"$literal": "active",
					"$codec":   "status",
				},
			},
		},
	},
	codecs.WithLiteralCodec(literalCodec),
)
```

Structured cursor values can use the same wrapper shape.

## Reversible Text Mode Is The Simpler Option

If the type already implements `encoding.TextMarshaler` and
`encoding.TextUnmarshaler`, you may not need a full custom codec.

```go
type Status string

func (s Status) MarshalText() ([]byte, error) {
	return []byte("X-" + string(s)), nil
}

func (s *Status) UnmarshalText(text []byte) error {
	*s = Status(strings.TrimPrefix(string(text), "X-"))
	return nil
}

if err := codecs.RegisterReversibleTextType("status_text", new(Status)); err != nil {
	panic(err)
}

if err := codecs.SetDefaultLiteralCodecMode(codecs.LiteralCodecModeReversibleText); err != nil {
	panic(err)
}
```

That extends `codecs.DefaultLiteralCodec` instead of replacing it.

## Transport Limits

- JSON and YAML preserve codec wrappers and can round-trip custom literal types.
- Query-string encoding flattens leaves to strings and does not emit `$codec` metadata.
- For query strings, prefer schema decoders and string-preserving parsing patterns when exact typing matters.

## Failure Modes

- returning `handled=false` without a fallback means the value is treated as an ordinary literal
- returning an error from `FormatLiteral` fails marshaling
- returning `handled=false` from `ParseLiteral` for an incoming `$codec` value makes parsing fail with `unsupported literal codec`

## Matching References

- [codecs/literal tests](../../codecs/literal_test.go)
- [codec config literal tests](../../codecs/internal/config/literal_test.go)
- [JSON codec round-trip tests](../../codecs/json/codec_test.go)
- [parser literal wrapper tests](../../codecs/internal/docmodel/parser_test.go)
