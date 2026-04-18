package dsl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/pakasa-io/qb"
)

// ErrStructuredRequired indicates the scalar cannot be rendered losslessly as plain DSL.
var ErrStructuredRequired = errors.New("dsl: structured scalar required")

// LiteralTokenDecoder rebuilds `!#:<codec>:<literal>` tokens into literal values.
type LiteralTokenDecoder func(codec string, literal string) (any, error)

// LiteralFormatter formats non-JSON-native values into typed literal tokens.
type LiteralFormatter func(value any) (literal string, codec string, handled bool, err error)

// Context controls how references are rendered.
type Context int

const (
	StandaloneContext Context = iota
	MixedContext
)

// Mode controls canonical vs compact DSL formatting behavior.
type Mode int

const (
	CanonicalMode Mode = iota
	CompactMode
)

type FormatOptions struct {
	Context          Context
	Mode             Mode
	LiteralFormatter LiteralFormatter
}

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenNumber
	tokenString
	tokenTypedLiteral
	tokenDot
	tokenComma
	tokenLParen
	tokenRParen
	tokenAt
	tokenCast
)

type token struct {
	kind tokenKind
	text string
}

type parser struct {
	tokens             []token
	pos                int
	decodeLiteralToken LiteralTokenDecoder
}

// ParseStandaloneScalar parses a DSL scalar, optionally allowing `as alias`.
func ParseStandaloneScalar(input string, allowAlias bool, decodeLiteralToken LiteralTokenDecoder) (qb.Scalar, string, error) {
	p, err := newParser(input, decodeLiteralToken)
	if err != nil {
		return nil, "", err
	}
	expr, alias, err := p.parseExpression(allowAlias)
	if err != nil {
		return nil, "", err
	}
	if p.peek().kind != tokenEOF {
		return nil, "", fmt.Errorf("unexpected token %q", p.peek().text)
	}
	return expr, alias, nil
}

// ParseSort parses a scalar expression with an optional trailing direction.
func ParseSort(input string, decodeLiteralToken LiteralTokenDecoder) (qb.Scalar, qb.Direction, error) {
	p, err := newParser(input, decodeLiteralToken)
	if err != nil {
		return nil, "", err
	}
	expr, _, err := p.parseExpression(false)
	if err != nil {
		return nil, "", err
	}

	direction := qb.Asc
	if tok := p.peek(); tok.kind == tokenIdent {
		switch strings.ToLower(tok.text) {
		case "asc":
			p.next()
			direction = qb.Asc
		case "desc":
			p.next()
			direction = qb.Desc
		}
	}
	if p.peek().kind != tokenEOF {
		return nil, "", fmt.Errorf("unexpected token %q", p.peek().text)
	}
	return expr, direction, nil
}

// FormatScalar renders a scalar expression as DSL.
func FormatScalar(expr qb.Scalar, opts FormatOptions) (string, error) {
	if expr == nil {
		return "", nil
	}
	if opts.Context != MixedContext {
		opts.Context = StandaloneContext
	}
	return formatScalar(expr, opts)
}

// FormatProjection renders a projection as DSL plus optional alias.
func FormatProjection(projection qb.Projection, opts FormatOptions) (string, error) {
	text, err := FormatScalar(projection.Expr, opts)
	if err != nil {
		return "", err
	}
	if projection.Alias != "" {
		return text + " as " + projection.Alias, nil
	}
	return text, nil
}

// FormatSort renders a sort as DSL with trailing direction.
func FormatSort(sort qb.Sort, opts FormatOptions) (string, error) {
	text, err := FormatScalar(sort.Expr, opts)
	if err != nil {
		return "", err
	}
	direction := sort.Direction
	if direction == "" {
		direction = qb.Asc
	}
	return text + " " + string(direction), nil
}

func newParser(input string, decode LiteralTokenDecoder) (*parser, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	return &parser{tokens: tokens, decodeLiteralToken: decode}, nil
}

func (p *parser) parseExpression(allowAlias bool) (qb.Scalar, string, error) {
	expr, err := p.parseCastExpression()
	if err != nil {
		return nil, "", err
	}
	alias := ""
	if allowAlias && p.peek().kind == tokenIdent && strings.EqualFold(p.peek().text, "as") {
		p.next()
		tok := p.next()
		if tok.kind != tokenIdent {
			return nil, "", fmt.Errorf("expected alias identifier")
		}
		alias = tok.text
	}
	return expr, alias, nil
}

func (p *parser) parseCastExpression() (qb.Scalar, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokenCast {
		p.next()
		tok := p.next()
		if tok.kind != tokenIdent {
			return nil, fmt.Errorf("expected cast type")
		}
		expr = qb.CastTo(expr, tok.text)
	}
	return expr, nil
}

func (p *parser) parsePrimary() (qb.Scalar, error) {
	tok := p.peek()
	switch tok.kind {
	case tokenAt:
		p.next()
		return p.parseReference()
	case tokenIdent:
		if strings.EqualFold(tok.text, "true") || strings.EqualFold(tok.text, "false") || strings.EqualFold(tok.text, "null") {
			p.next()
			switch strings.ToLower(tok.text) {
			case "true":
				return qb.V(true), nil
			case "false":
				return qb.V(false), nil
			default:
				return qb.V(nil), nil
			}
		}
		if p.peekN(1).kind == tokenLParen {
			return p.parseCall()
		}
		return p.parseReference()
	case tokenNumber:
		p.next()
		if strings.Contains(tok.text, ".") {
			value, err := strconv.ParseFloat(tok.text, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number %q", tok.text)
			}
			return qb.V(value), nil
		}
		value, err := strconv.ParseInt(tok.text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", tok.text)
		}
		return qb.V(value), nil
	case tokenString:
		p.next()
		return qb.V(tok.text), nil
	case tokenTypedLiteral:
		p.next()
		codec, literal, err := splitTypedLiteralToken(tok.text)
		if err != nil {
			return nil, err
		}
		if codec != "" && p.decodeLiteralToken != nil {
			value, err := p.decodeLiteralToken(codec, literal)
			if err != nil {
				return nil, err
			}
			return qb.V(value), nil
		}
		return qb.V(literal), nil
	case tokenLParen:
		p.next()
		expr, err := p.parseCastExpression()
		if err != nil {
			return nil, err
		}
		if p.next().kind != tokenRParen {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		return expr, nil
	default:
		return nil, fmt.Errorf("unexpected token %q", tok.text)
	}
}

func (p *parser) parseReference() (qb.Scalar, error) {
	tok := p.next()
	if tok.kind != tokenIdent {
		return nil, fmt.Errorf("expected identifier")
	}
	parts := []string{tok.text}
	for p.peek().kind == tokenDot {
		p.next()
		tok = p.next()
		if tok.kind != tokenIdent {
			return nil, fmt.Errorf("expected identifier after '.'")
		}
		parts = append(parts, tok.text)
	}
	return qb.F(strings.Join(parts, ".")), nil
}

func (p *parser) parseCall() (qb.Scalar, error) {
	name := p.next()
	if name.kind != tokenIdent {
		return nil, fmt.Errorf("expected function name")
	}
	if p.next().kind != tokenLParen {
		return nil, fmt.Errorf("expected opening parenthesis")
	}

	args := []any{}
	if p.peek().kind != tokenRParen {
		for {
			arg, err := p.parseCastExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().kind != tokenComma {
				break
			}
			p.next()
		}
	}

	if p.next().kind != tokenRParen {
		return nil, fmt.Errorf("expected closing parenthesis")
	}
	return qb.Func(name.text, args...), nil
}

func (p *parser) peek() token { return p.peekN(0) }

func (p *parser) peekN(offset int) token {
	index := p.pos + offset
	if index >= len(p.tokens) {
		return token{kind: tokenEOF}
	}
	return p.tokens[index]
}

func (p *parser) next() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func tokenize(input string) ([]token, error) {
	tokens := make([]token, 0, len(input)/2)
	for i := 0; i < len(input); {
		ch := rune(input[i])
		if unicode.IsSpace(ch) {
			i++
			continue
		}

		if strings.HasPrefix(input[i:], "!#:") {
			start := i
			i += 3
			for i < len(input) {
				switch input[i] {
				case ',', ')':
					goto typedLiteralDone
				}
				if unicode.IsSpace(rune(input[i])) {
					goto typedLiteralDone
				}
				i++
			}
		typedLiteralDone:
			tokens = append(tokens, token{kind: tokenTypedLiteral, text: input[start:i]})
			continue
		}

		switch ch {
		case '.':
			tokens = append(tokens, token{kind: tokenDot, text: "."})
			i++
		case ',':
			tokens = append(tokens, token{kind: tokenComma, text: ","})
			i++
		case '(':
			tokens = append(tokens, token{kind: tokenLParen, text: "("})
			i++
		case ')':
			tokens = append(tokens, token{kind: tokenRParen, text: ")"})
			i++
		case '@':
			tokens = append(tokens, token{kind: tokenAt, text: "@"})
			i++
		case '\'':
			start := i
			i++
			var builder strings.Builder
			for i < len(input) {
				if input[i] == '\'' {
					if i+1 < len(input) && input[i+1] == '\'' {
						builder.WriteByte('\'')
						i += 2
						continue
					}
					i++
					tokens = append(tokens, token{kind: tokenString, text: builder.String()})
					goto nextToken
				}
				builder.WriteByte(input[i])
				i++
			}
			return nil, fmt.Errorf("unterminated string starting at %d", start)
		case ':':
			if i+1 < len(input) && input[i+1] == ':' {
				tokens = append(tokens, token{kind: tokenCast, text: "::"})
				i += 2
				continue
			}
			return nil, fmt.Errorf("unexpected ':'")
		default:
			if isIdentStart(ch) {
				start := i
				i++
				for i < len(input) && isIdentContinue(rune(input[i])) {
					i++
				}
				tokens = append(tokens, token{kind: tokenIdent, text: input[start:i]})
				continue
			}
			if unicode.IsDigit(ch) || ch == '-' {
				start := i
				i++
				for i < len(input) && (unicode.IsDigit(rune(input[i])) || input[i] == '.') {
					i++
				}
				tokens = append(tokens, token{kind: tokenNumber, text: input[start:i]})
				continue
			}
			return nil, fmt.Errorf("unexpected character %q", ch)
		}
	nextToken:
	}
	tokens = append(tokens, token{kind: tokenEOF})
	return tokens, nil
}

func formatScalar(expr qb.Scalar, opts FormatOptions) (string, error) {
	switch typed := expr.(type) {
	case qb.Ref:
		if opts.Context == MixedContext {
			return "@" + typed.Name, nil
		}
		return typed.Name, nil
	case qb.Literal:
		return formatLiteral(typed.Value, opts)
	case qb.Call:
		args := make([]string, len(typed.Args))
		for i, arg := range typed.Args {
			text, err := formatScalar(arg, opts)
			if err != nil {
				return "", err
			}
			args[i] = text
		}
		return typed.Name + "(" + strings.Join(args, ", ") + ")", nil
	case qb.Cast:
		text, err := formatScalar(typed.Expr, opts)
		if err != nil {
			return "", err
		}
		return text + "::" + typed.Type, nil
	default:
		return "", fmt.Errorf("unsupported scalar %T", expr)
	}
}

func formatLiteral(value any, opts FormatOptions) (string, error) {
	if opts.LiteralFormatter != nil {
		literal, codec, handled, err := opts.LiteralFormatter(value)
		if err != nil {
			return "", err
		}
		if handled {
			if opts.Mode == CompactMode && inlineSafeLiteral(literal) {
				if codec != "" {
					return "!#:" + codec + ":" + literal, nil
				}
				return "!#::" + literal, nil
			}
			return "", ErrStructuredRequired
		}
	}

	switch typed := value.(type) {
	case nil:
		return "null", nil
	case string:
		return quoteString(typed), nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%v", typed), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", typed), nil
	case float32, float64:
		return fmt.Sprintf("%v", typed), nil
	default:
		return "", ErrStructuredRequired
	}
}

func inlineSafeLiteral(value string) bool {
	if strings.TrimSpace(value) != value || value == "" {
		return false
	}
	for _, r := range value {
		switch r {
		case ',', ')', '\n', '\r':
			return false
		}
	}
	return true
}

func quoteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func splitTypedLiteralToken(token string) (codec string, literal string, err error) {
	if !strings.HasPrefix(token, "!#:") {
		return "", "", fmt.Errorf("invalid typed literal token %q", token)
	}
	rest := strings.TrimPrefix(token, "!#:")
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid typed literal token %q", token)
	}
	return parts[0], parts[1], nil
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentContinue(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
