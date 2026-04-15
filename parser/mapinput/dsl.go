package mapinput

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/pakasa-io/qb"
)

type dslTokenKind int

const (
	dslEOF dslTokenKind = iota
	dslIdent
	dslNumber
	dslString
	dslDot
	dslComma
	dslLParen
	dslRParen
	dslAt
	dslCast
)

type dslToken struct {
	kind dslTokenKind
	text string
}

type dslParser struct {
	input  string
	tokens []dslToken
	pos    int
}

func parseStandaloneScalar(input string, allowAlias bool) (qb.Scalar, string, error) {
	parser, err := newDSLParser(input)
	if err != nil {
		return nil, "", err
	}
	expr, alias, err := parser.parseExpression(allowAlias)
	if err != nil {
		return nil, "", err
	}
	if parser.peek().kind != dslEOF {
		return nil, "", fmt.Errorf("unexpected token %q", parser.peek().text)
	}
	return expr, alias, nil
}

func parseSortItem(input string) (qb.Scalar, qb.Direction, error) {
	parser, err := newDSLParser(input)
	if err != nil {
		return nil, "", err
	}
	expr, _, err := parser.parseExpression(false)
	if err != nil {
		return nil, "", err
	}

	direction := qb.Asc
	if token := parser.peek(); token.kind == dslIdent {
		switch strings.ToLower(token.text) {
		case "asc":
			parser.next()
			direction = qb.Asc
		case "desc":
			parser.next()
			direction = qb.Desc
		}
	}
	if parser.peek().kind != dslEOF {
		return nil, "", fmt.Errorf("unexpected token %q", parser.peek().text)
	}
	return expr, direction, nil
}

func newDSLParser(input string) (*dslParser, error) {
	tokens, err := tokenizeDSL(input)
	if err != nil {
		return nil, err
	}
	return &dslParser{input: input, tokens: tokens}, nil
}

func (p *dslParser) parseExpression(allowAlias bool) (qb.Scalar, string, error) {
	expr, err := p.parseCastExpression()
	if err != nil {
		return nil, "", err
	}
	alias := ""
	if allowAlias && p.peek().kind == dslIdent && strings.EqualFold(p.peek().text, "as") {
		p.next()
		token := p.next()
		if token.kind != dslIdent {
			return nil, "", fmt.Errorf("expected alias identifier")
		}
		alias = token.text
	}
	return expr, alias, nil
}

func (p *dslParser) parseCastExpression() (qb.Scalar, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == dslCast {
		p.next()
		token := p.next()
		if token.kind != dslIdent {
			return nil, fmt.Errorf("expected cast type")
		}
		expr = qb.CastTo(expr, token.text)
	}
	return expr, nil
}

func (p *dslParser) parsePrimary() (qb.Scalar, error) {
	token := p.peek()
	switch token.kind {
	case dslAt:
		p.next()
		ref, err := p.parseReference(true)
		if err != nil {
			return nil, err
		}
		return ref, nil
	case dslIdent:
		if strings.EqualFold(token.text, "true") || strings.EqualFold(token.text, "false") || strings.EqualFold(token.text, "null") {
			p.next()
			switch strings.ToLower(token.text) {
			case "true":
				return qb.V(true), nil
			case "false":
				return qb.V(false), nil
			default:
				return qb.V(nil), nil
			}
		}
		if p.peekN(1).kind == dslLParen {
			return p.parseCall()
		}
		ref, err := p.parseReference(false)
		if err != nil {
			return nil, err
		}
		return ref, nil
	case dslNumber:
		p.next()
		if strings.Contains(token.text, ".") {
			number, err := strconv.ParseFloat(token.text, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number %q", token.text)
			}
			return qb.V(number), nil
		}
		number, err := strconv.ParseInt(token.text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", token.text)
		}
		return qb.V(number), nil
	case dslString:
		p.next()
		return qb.V(token.text), nil
	case dslLParen:
		p.next()
		expr, err := p.parseCastExpression()
		if err != nil {
			return nil, err
		}
		if p.next().kind != dslRParen {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		return expr, nil
	default:
		return nil, fmt.Errorf("unexpected token %q", token.text)
	}
}

func (p *dslParser) parseCall() (qb.Scalar, error) {
	name := p.next()
	if name.kind != dslIdent {
		return nil, fmt.Errorf("expected function name")
	}
	if p.next().kind != dslLParen {
		return nil, fmt.Errorf("expected opening parenthesis")
	}

	args := []any{}
	if p.peek().kind != dslRParen {
		for {
			arg, err := p.parseCastExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().kind != dslComma {
				break
			}
			p.next()
		}
	}

	if p.next().kind != dslRParen {
		return nil, fmt.Errorf("expected closing parenthesis")
	}
	return qb.Func(name.text, args...), nil
}

func (p *dslParser) parseReference(explicit bool) (qb.Ref, error) {
	token := p.next()
	if token.kind != dslIdent {
		return qb.Ref{}, fmt.Errorf("expected identifier")
	}
	parts := []string{token.text}
	for p.peek().kind == dslDot {
		p.next()
		token = p.next()
		if token.kind != dslIdent {
			return qb.Ref{}, fmt.Errorf("expected identifier after '.'")
		}
		parts = append(parts, token.text)
	}
	name := strings.Join(parts, ".")
	if explicit {
		name = "@" + name
	}
	return qb.F(strings.TrimPrefix(name, "@")), nil
}

func (p *dslParser) peek() dslToken {
	return p.peekN(0)
}

func (p *dslParser) peekN(offset int) dslToken {
	index := p.pos + offset
	if index >= len(p.tokens) {
		return dslToken{kind: dslEOF}
	}
	return p.tokens[index]
}

func (p *dslParser) next() dslToken {
	token := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return token
}

func tokenizeDSL(input string) ([]dslToken, error) {
	tokens := make([]dslToken, 0, len(input)/2)
	for i := 0; i < len(input); {
		ch := rune(input[i])
		if unicode.IsSpace(ch) {
			i++
			continue
		}
		switch ch {
		case '.':
			tokens = append(tokens, dslToken{kind: dslDot, text: "."})
			i++
		case ',':
			tokens = append(tokens, dslToken{kind: dslComma, text: ","})
			i++
		case '(':
			tokens = append(tokens, dslToken{kind: dslLParen, text: "("})
			i++
		case ')':
			tokens = append(tokens, dslToken{kind: dslRParen, text: ")"})
			i++
		case '@':
			tokens = append(tokens, dslToken{kind: dslAt, text: "@"})
			i++
		case ':':
			if i+1 >= len(input) || input[i+1] != ':' {
				return nil, fmt.Errorf("unexpected ':'")
			}
			tokens = append(tokens, dslToken{kind: dslCast, text: "::"})
			i += 2
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
					tokens = append(tokens, dslToken{kind: dslString, text: builder.String()})
					goto nextToken
				}
				builder.WriteByte(input[i])
				i++
			}
			return nil, fmt.Errorf("unterminated string starting at %d", start)
		default:
			if isNumberStart(input, i) {
				start := i
				i++
				for i < len(input) && (unicode.IsDigit(rune(input[i])) || input[i] == '.') {
					i++
				}
				tokens = append(tokens, dslToken{kind: dslNumber, text: input[start:i]})
				continue
			}
			if isIdentStart(ch) {
				start := i
				i++
				for i < len(input) && isIdentContinue(rune(input[i])) {
					i++
				}
				tokens = append(tokens, dslToken{kind: dslIdent, text: input[start:i]})
				continue
			}
			return nil, fmt.Errorf("unexpected character %q", string(ch))
		}
		continue
	nextToken:
		continue
	}
	tokens = append(tokens, dslToken{kind: dslEOF})
	return tokens, nil
}

func isNumberStart(input string, index int) bool {
	if input[index] == '-' {
		return index+1 < len(input) && unicode.IsDigit(rune(input[index+1]))
	}
	return unicode.IsDigit(rune(input[index]))
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentContinue(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
