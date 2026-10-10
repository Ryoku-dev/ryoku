package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type kdlValue struct {
	Text string
	Raw  string
	Kind string
}

func (v kdlValue) String() string { return v.Text }

func (v kdlValue) Bool() (bool, bool) {
	switch strings.ToLower(v.Text) {
	case "true", "#true":
		return true, true
	case "false", "#false":
		return false, true
	}
	return false, false
}

func (v kdlValue) Float() (float64, bool) {
	s := strings.ReplaceAll(strings.TrimSpace(v.Text), "_", "")
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func (v kdlValue) Int() (int, bool) {
	f, ok := v.Float()
	return int(f), ok && f == float64(int(f))
}

type kdlNode struct {
	Name     string
	Args     []kdlValue
	Props    map[string]kdlValue
	Children []*kdlNode
	Raw      string
	File     string
	Line     int
}

func (n *kdlNode) Child(name string) *kdlNode {
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

func (n *kdlNode) ChildrenNamed(name string) []*kdlNode {
	var out []*kdlNode
	for _, child := range n.Children {
		if child.Name == name {
			out = append(out, child)
		}
	}
	return out
}

func (n *kdlNode) Arg(i int) (kdlValue, bool) {
	if i < 0 || i >= len(n.Args) {
		return kdlValue{}, false
	}
	return n.Args[i], true
}

type kdlTokenKind uint8

const (
	kdlEOF kdlTokenKind = iota
	kdlWord
	kdlString
	kdlLBrace
	kdlRBrace
	kdlEqual
	kdlEnd
	kdlSlashDash
)

type kdlToken struct {
	kind       kdlTokenKind
	raw, value string
	start, end int
	line       int
}

type kdlLexer struct {
	s    string
	i    int
	line int
}

func lexKDL(s string) ([]kdlToken, error) {
	l := kdlLexer{s: strings.TrimPrefix(s, "\ufeff"), line: 1}
	var out []kdlToken
	for {
		t, err := l.next()
		if err != nil {
			return nil, err
		}
		out = append(out, t)
		if t.kind == kdlEOF {
			return out, nil
		}
	}
}

func (l *kdlLexer) next() (kdlToken, error) {
	for l.i < len(l.s) {
		start := l.i
		switch l.s[l.i] {
		case ' ', '\t', '\r':
			l.i++
			continue
		case '\n', ';':
			line := l.line
			if l.s[l.i] == '\n' {
				l.line++
			}
			l.i++
			return kdlToken{kind: kdlEnd, raw: l.s[start:l.i], start: start, end: l.i, line: line}, nil
		case '{':
			l.i++
			return kdlToken{kind: kdlLBrace, raw: "{", start: start, end: l.i, line: l.line}, nil
		case '}':
			l.i++
			return kdlToken{kind: kdlRBrace, raw: "}", start: start, end: l.i, line: l.line}, nil
		case '=':
			l.i++
			return kdlToken{kind: kdlEqual, raw: "=", start: start, end: l.i, line: l.line}, nil
		case '"':
			return l.quoted()
		case '/':
			if strings.HasPrefix(l.s[l.i:], "//") {
				l.skipLineComment()
				continue
			}
			if strings.HasPrefix(l.s[l.i:], "/*") {
				if err := l.skipBlockComment(); err != nil {
					return kdlToken{}, err
				}
				continue
			}
			if strings.HasPrefix(l.s[l.i:], "/-") {
				l.i += 2
				return kdlToken{kind: kdlSlashDash, raw: "/-", start: start, end: l.i, line: l.line}, nil
			}
		}
		if l.s[l.i] == 'r' || l.s[l.i] == '#' {
			if tok, ok, err := l.rawString(); ok || err != nil {
				return tok, err
			}
		}
		return l.word()
	}
	return kdlToken{kind: kdlEOF, start: l.i, end: l.i, line: l.line}, nil
}

func (l *kdlLexer) skipLineComment() {
	for l.i < len(l.s) && l.s[l.i] != '\n' {
		l.i++
	}
}

func (l *kdlLexer) skipBlockComment() error {
	depth := 0
	for l.i < len(l.s) {
		switch {
		case strings.HasPrefix(l.s[l.i:], "/*"):
			depth++
			l.i += 2
		case strings.HasPrefix(l.s[l.i:], "*/"):
			depth--
			l.i += 2
			if depth == 0 {
				return nil
			}
		default:
			if l.s[l.i] == '\n' {
				l.line++
			}
			_, size := utf8.DecodeRuneInString(l.s[l.i:])
			l.i += size
		}
	}
	return fmt.Errorf("line %d: unterminated block comment", l.line)
}

func (l *kdlLexer) quoted() (kdlToken, error) {
	start, line := l.i, l.line
	l.i++
	var value strings.Builder
	for l.i < len(l.s) {
		c := l.s[l.i]
		if c == '"' {
			l.i++
			return kdlToken{kind: kdlString, raw: l.s[start:l.i], value: value.String(), start: start, end: l.i, line: line}, nil
		}
		if c == '\n' {
			l.line++
		}
		if c != '\\' {
			r, size := utf8.DecodeRuneInString(l.s[l.i:])
			value.WriteRune(r)
			l.i += size
			continue
		}
		l.i++
		if l.i >= len(l.s) {
			break
		}
		esc := l.s[l.i]
		l.i++
		switch esc {
		case 'n':
			value.WriteByte('\n')
		case 'r':
			value.WriteByte('\r')
		case 't':
			value.WriteByte('\t')
		case '\\', '"', '/':
			value.WriteByte(esc)
		case 'b':
			value.WriteByte('\b')
		case 'f':
			value.WriteByte('\f')
		case 'u':
			if l.i < len(l.s) && l.s[l.i] == '{' {
				l.i++
				end := strings.IndexByte(l.s[l.i:], '}')
				if end < 0 {
					return kdlToken{}, fmt.Errorf("line %d: unterminated unicode escape", line)
				}
				hex := l.s[l.i : l.i+end]
				v, err := strconv.ParseInt(hex, 16, 32)
				if err != nil {
					return kdlToken{}, fmt.Errorf("line %d: invalid unicode escape", line)
				}
				value.WriteRune(rune(v))
				l.i += end + 1
			} else {
				if l.i+4 > len(l.s) {
					return kdlToken{}, fmt.Errorf("line %d: short unicode escape", line)
				}
				v, err := strconv.ParseInt(l.s[l.i:l.i+4], 16, 32)
				if err != nil {
					return kdlToken{}, fmt.Errorf("line %d: invalid unicode escape", line)
				}
				value.WriteRune(rune(v))
				l.i += 4
			}
		default:
			value.WriteByte(esc)
		}
	}
	return kdlToken{}, fmt.Errorf("line %d: unterminated string", line)
}

func (l *kdlLexer) rawString() (kdlToken, bool, error) {
	start, line, p := l.i, l.line, l.i
	if l.s[p] == 'r' {
		p++
	}
	hashes := 0
	for p < len(l.s) && l.s[p] == '#' {
		hashes++
		p++
	}
	if p >= len(l.s) || l.s[p] != '"' || (l.s[start] == '#' && hashes == 0) {
		return kdlToken{}, false, nil
	}
	p++
	body := p
	close := "\"" + strings.Repeat("#", hashes)
	end := strings.Index(l.s[p:], close)
	if end < 0 {
		return kdlToken{}, true, fmt.Errorf("line %d: unterminated raw string", line)
	}
	value := l.s[body : p+end]
	for _, c := range value {
		if c == '\n' {
			l.line++
		}
	}
	l.i = p + end + len(close)
	return kdlToken{kind: kdlString, raw: l.s[start:l.i], value: value, start: start, end: l.i, line: line}, true, nil
}

func (l *kdlLexer) word() (kdlToken, error) {
	start, line := l.i, l.line
	for l.i < len(l.s) {
		r, size := utf8.DecodeRuneInString(l.s[l.i:])
		if unicode.IsSpace(r) || strings.ContainsRune("{};=\"", r) {
			break
		}
		if r == '/' && l.i+1 < len(l.s) && (l.s[l.i+1] == '/' || l.s[l.i+1] == '*' || l.s[l.i+1] == '-') {
			break
		}
		l.i += size
	}
	if l.i == start {
		return kdlToken{}, fmt.Errorf("line %d: unexpected byte %q", line, l.s[l.i])
	}
	raw := l.s[start:l.i]
	return kdlToken{kind: kdlWord, raw: raw, value: raw, start: start, end: l.i, line: line}, nil
}

type kdlParser struct {
	s    string
	file string
	toks []kdlToken
	i    int
}

func parseKDL(file, s string) ([]*kdlNode, error) {
	toks, err := lexKDL(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	p := &kdlParser{s: strings.TrimPrefix(s, "\ufeff"), file: file, toks: toks}
	return p.document(false)
}

func (p *kdlParser) document(child bool) ([]*kdlNode, error) {
	var nodes []*kdlNode
	for {
		for p.peek().kind == kdlEnd {
			p.i++
		}
		switch p.peek().kind {
		case kdlEOF:
			if child {
				return nil, fmt.Errorf("%s:%d: missing closing brace", p.file, p.peek().line)
			}
			return nodes, nil
		case kdlRBrace:
			if !child {
				return nil, fmt.Errorf("%s:%d: unexpected closing brace", p.file, p.peek().line)
			}
			p.i++
			return nodes, nil
		case kdlSlashDash:
			p.i++
			if err := p.skipNode(); err != nil {
				return nil, err
			}
			continue
		}
		n, err := p.node()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
}

func (p *kdlParser) node() (*kdlNode, error) {
	startTok := p.peek()
	if startTok.kind != kdlWord && startTok.kind != kdlString {
		return nil, fmt.Errorf("%s:%d: expected node name", p.file, startTok.line)
	}
	p.i++
	n := &kdlNode{Name: startTok.value, Props: map[string]kdlValue{}, File: p.file, Line: startTok.line}
	end := startTok.end
	for {
		t := p.peek()
		switch t.kind {
		case kdlEnd:
			p.i++
			n.Raw = strings.TrimSpace(p.s[startTok.start:end])
			return n, nil
		case kdlEOF, kdlRBrace:
			n.Raw = strings.TrimSpace(p.s[startTok.start:end])
			return n, nil
		case kdlLBrace:
			p.i++
			children, err := p.document(true)
			if err != nil {
				return nil, err
			}
			n.Children = children
			end = p.toks[p.i-1].end
			if p.peek().kind == kdlEnd {
				p.i++
			}
			n.Raw = strings.TrimSpace(p.s[startTok.start:end])
			return n, nil
		case kdlSlashDash:
			p.i++
			if p.peek().kind == kdlWord && p.i+1 < len(p.toks) && p.toks[p.i+1].kind == kdlEqual {
				p.i += 2
			}
			if p.peek().kind == kdlWord || p.peek().kind == kdlString {
				end = p.peek().end
				p.i++
			}
			continue
		case kdlWord, kdlString:
			p.i++
			end = t.end
			if p.peek().kind == kdlEqual {
				key := t.value
				p.i++
				v, err := p.value()
				if err != nil {
					return nil, err
				}
				n.Props[key] = v
				end = p.toks[p.i-1].end
			} else {
				n.Args = append(n.Args, tokenValue(t))
			}
		default:
			return nil, fmt.Errorf("%s:%d: unexpected token %q", p.file, t.line, t.raw)
		}
	}
}

func (p *kdlParser) value() (kdlValue, error) {
	t := p.peek()
	if t.kind != kdlWord && t.kind != kdlString {
		return kdlValue{}, fmt.Errorf("%s:%d: expected value", p.file, t.line)
	}
	p.i++
	return tokenValue(t), nil
}

func tokenValue(t kdlToken) kdlValue {
	kind := "word"
	if t.kind == kdlString {
		kind = "string"
	} else if _, err := strconv.ParseFloat(strings.ReplaceAll(t.value, "_", ""), 64); err == nil {
		kind = "number"
	} else if t.value == "true" || t.value == "false" || t.value == "#true" || t.value == "#false" {
		kind = "bool"
	}
	return kdlValue{Text: t.value, Raw: t.raw, Kind: kind}
}

func (p *kdlParser) skipNode() error {
	if p.peek().kind != kdlWord && p.peek().kind != kdlString {
		return fmt.Errorf("%s:%d: /- must precede a node", p.file, p.peek().line)
	}
	depth := 0
	for {
		t := p.peek()
		p.i++
		switch t.kind {
		case kdlLBrace:
			depth++
		case kdlRBrace:
			if depth == 0 {
				p.i--
				return nil
			}
			depth--
			if depth == 0 {
				return nil
			}
		case kdlEnd:
			if depth == 0 {
				return nil
			}
		case kdlEOF:
			return nil
		}
	}
}

func (p *kdlParser) peek() kdlToken {
	if p.i >= len(p.toks) {
		return kdlToken{kind: kdlEOF}
	}
	return p.toks[p.i]
}
