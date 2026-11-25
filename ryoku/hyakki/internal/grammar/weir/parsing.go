package weir

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
)

// Rule syntax needs only lexical classes and spans, not document annotations.
type parserToken struct {
	Span        text.Span
	kind        doc.TokenKindTag
	quote       bool
	punctuation text.PunctuationKind
	currency    text.Currency
}

type parser struct {
	tokens []parserToken
	source []rune
	i      int
}

func newParser(code string) *parser {
	r := []rune(code)
	p := &parser{source: r}
	if capacity := len(r) / 3; capacity > 0 {
		p.tokens = make([]parserToken, 0, capacity)
	}
	for cursor := 0; cursor < len(r); {
		found := doc.LexWeirToken(r[cursor:])
		end := cursor + found.NextIndex
		p.tokens = append(p.tokens, parserToken{
			Span: text.Span{Start: cursor, End: end},
			kind: found.Token.Kind, quote: found.Token.IsQuote(),
			punctuation: found.Token.Punctuation.Kind, currency: found.Token.Punctuation.Currency,
		})
		cursor = end
	}
	return p
}
func (p *parser) literal(i int) string { return p.tokens[i].Span.GetContentString(p.source) }
func (p *parser) at(s string) bool     { return p.i < len(p.tokens) && p.literal(p.i) == s }
func (p *parser) spaces() {
	for p.i < len(p.tokens) && p.tokens[p.i].kind == doc.Space {
		p.i++
	}
}
func (p *parser) space() error {
	if p.i >= len(p.tokens) {
		return ErrEndOfInput
	}
	if p.tokens[p.i].kind != doc.Space {
		return Error{Kind: UnexpectedToken, Value: p.literal(p.i)}
	}
	p.i++
	return nil
}
func (p *parser) quoted() (string, error) {
	p.spaces()
	if p.i >= len(p.tokens) {
		return "", ErrEndOfInput
	}
	if !p.tokens[p.i].quote {
		return "", Error{Kind: UnexpectedToken, Value: p.literal(p.i)}
	}
	start := p.tokens[p.i].Span.End
	p.i++
	for p.i < len(p.tokens) {
		t := p.tokens[p.i]
		if t.quote {
			backslashes := 0
			for j := t.Span.Start - 1; j >= 0 && p.source[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				p.i++
				return string(p.source[start:t.Span.Start]), nil
			}
		}
		p.i++
	}
	return "", ErrEndOfInput
}
func Parse(code string) (*Ast, error) { return ParseStr(code, true) }
func parseStrUncached(code string, optimize bool) (*Ast, error) {
	p := newParser(code)
	a := &Ast{}
	for p.i < len(p.tokens) {
		for p.i < len(p.tokens) && (p.tokens[p.i].kind == doc.Space || p.tokens[p.i].kind == doc.Newline) {
			p.i++
		}
		if p.i == len(p.tokens) {
			break
		}
		end := p.i
		for end < len(p.tokens) && p.tokens[end].kind != doc.Newline {
			end++
		}
		line := &parser{tokens: p.tokens[:end], source: p.source, i: p.i}
		key := line.literal(line.i)
		line.i++
		var s AstStmtNode
		switch key {
		case "#":
			s = AstStmtNode{Kind: Comment, Name: string(p.source[p.tokens[p.i].Span.Start:p.tokens[end-1].Span.End])}
			line.i = end
		case "let", "expr":
			if err := line.space(); err != nil {
				return nil, err
			}
			if line.i >= end {
				return nil, ErrEndOfInput
			}
			s.Name = line.literal(line.i)
			line.i++
			if key == "expr" {
				s.Kind = SetExpr
				if line.i < end {
					// Harper skips this token without requiring whitespace.
					line.i++
				}
				nodes, err := line.seq("")
				if err != nil {
					return nil, err
				}
				s.Expr = AstExprNode{Kind: Seq, Children: nodes}
			} else {
				s.Kind = DeclareVariable
				if err := line.space(); err != nil {
					return nil, err
				}
				if line.at("[") {
					line.i++
					s.Value.IsArray = true
					for !line.at("]") {
						if line.i >= end {
							return nil, ErrUnmatchedBrace
						}
						v, err := line.quoted()
						if err != nil {
							return nil, err
						}
						s.Value.Array = append(s.Value.Array, AstVariable{String: v})
						line.spaces()
						if line.at(",") {
							line.i++
							line.spaces()
						} else if !line.at("]") {
							return nil, ErrExpectedComma
						}
					}
					line.i++
					line.i = end
				} else {
					v, err := line.quoted()
					if err != nil {
						return nil, err
					}
					s.Value.String = v
					line.spaces()
				}
			}
		case "test", "allows":
			v, err := line.quoted()
			if err != nil {
				return nil, err
			}
			s.Expect = v
			if key == "allows" {
				s.Kind = Allows
			} else {
				s.Kind = Test
				if err := line.space(); err != nil {
					return nil, err
				}
				s.ToBe, err = line.quoted()
				if err != nil {
					return nil, err
				}
			}
		default:
			kind := UnexpectedToken
			if line.tokens[p.i].kind != doc.Word {
				kind = UnsupportedToken
			}
			return nil, Error{Kind: kind, Value: key}
		}
		if line.i != end {
			return nil, Error{Kind: UnexpectedToken, Value: line.literal(line.i)}
		}
		a.Stmts = append(a.Stmts, s)
		p.i = end + 1
	}
	if optimize {
		for Optimize(a) {
		}
	}
	return a, nil
}
func ParseExprStr(code string, optimize bool) (AstExprNode, error) {
	p := newParser(code)
	children, err := p.seq("")
	if err != nil {
		return AstExprNode{}, err
	}
	n := AstExprNode{Kind: Seq, Children: children}
	if optimize {
		for OptimizeExpr(&n) {
		}
	}
	return n, nil
}
func (p *parser) seq(close string) ([]AstExprNode, error) {
	out := []AstExprNode{}
	for p.i < len(p.tokens) && !p.at(close) {
		n, err := p.single()
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if close != "" && p.i == len(p.tokens) {
		return nil, ErrUnmatchedBrace
	}
	return out, nil
}

func (p *parser) single() (AstExprNode, error) {
	if p.i >= len(p.tokens) {
		return AstExprNode{}, ErrEndOfInput
	}
	t := p.tokens[p.i]
	s := p.literal(p.i)
	p.i++
	if t.kind == doc.Space {
		return AstExprNode{Kind: Whitespace}, nil
	}
	switch s {
	case "@", "$":
		if p.i >= len(p.tokens) {
			return AstExprNode{}, ErrEndOfInput
		}
		name := p.literal(p.i)
		p.i++
		k := ExprRef
		if s == "$" {
			k = DerivativeOf
		}
		return AstExprNode{Kind: k, Text: name}, nil
	case "*":
		return AstExprNode{Kind: Anything}, nil
	case "!":
		n, err := p.single()
		return AstExprNode{Kind: Not, Children: []AstExprNode{n}}, err
	case "(":
		children, err := p.seq(")")
		if err != nil {
			return AstExprNode{}, err
		}
		p.i++
		return AstExprNode{Kind: Seq, Children: children}, nil
	case "[", "<":
		close := "]"
		kind := Arr
		if s == "<" {
			close = ">"
			kind = Filter
		}
		children := []AstExprNode{}
		p.spaces()
		for !p.at(close) {
			if p.i >= len(p.tokens) {
				return AstExprNode{}, ErrUnmatchedBrace
			}
			c, err := p.single()
			if err != nil {
				return AstExprNode{}, err
			}
			children = append(children, c)
			p.spaces()
			if p.at(",") {
				p.i++
				p.spaces()
			} else if !p.at(close) {
				return AstExprNode{}, ErrExpectedComma
			}
		}
		p.i++
		return AstExprNode{Kind: kind, Children: children}, nil
	}
	if t.kind == doc.Word {
		if up, ok := pos.ParseUPOS(s); ok {
			return AstExprNode{Kind: UPOSSet, UPOS: []pos.UPOS{up}}, nil
		}
		if s == "PROG" {
			return AstExprNode{Kind: Progressive}, nil
		}
		return AstExprNode{Kind: Word, Text: s}, nil
	}
	if t.kind == doc.Punctuation {
		return AstExprNode{Kind: Punctuation, Punctuation: text.Punctuation{Kind: t.punctuation, Currency: t.currency}}, nil
	}
	return AstExprNode{}, Error{Kind: UnsupportedToken, Value: s}
}
func WeirExprToExpr(code string) (expr.Expr, error) {
	n, err := ParseExprStr(code, true)
	if err != nil {
		return nil, err
	}
	return n.ToExpr(nil)
}
