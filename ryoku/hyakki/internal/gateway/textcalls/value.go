package textcalls

// Derived from oh-my-pi (MIT), packages/utils/src/json-parse.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/utils/src/json-lexer.ts@c7fff67

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Decoded values are string, number, bool, nil, []any and object. Objects
// keep their key order so a call replayed as text reads the way the model
// wrote it, and numbers keep their text so large integers survive.
type number string

type member struct {
	key string
	val any
}

type object []member

func (o object) get(key string) (any, bool) {
	for _, m := range o {
		if m.key == key {
			return m.val, true
		}
	}
	return nil, false
}

// set replaces a key in place, so a repeated key keeps its first position
// and its last value, as JSON.parse does.
func (o *object) set(key string, val any) {
	for i := range *o {
		if (*o)[i].key == key {
			(*o)[i].val = val
			return
		}
	}
	*o = append(*o, member{key, val})
}

var errSyntax = errors.New("textcalls: malformed JSON")

// parseRelaxed decodes JSON with the repairs small models need: single
// quotes, unquoted keys, comments, Python literals, stray commas, invalid
// escapes and bareword values. It refuses truncated input and trailing
// garbage, so a half-written call never runs.
func parseRelaxed(src string) (any, error) {
	l := &lexer{src: src}
	l.ws()
	if l.end() {
		return nil, errSyntax
	}
	v, err := l.value(false, 0)
	if err != nil {
		return nil, err
	}
	l.ws()
	if !l.end() {
		return nil, fmt.Errorf("%w: trailing text at %d", errSyntax, l.pos)
	}
	return v, nil
}

// jsonPrefix returns the end of the first balanced JSON object or array in
// s (strings and escapes respected), or -1 when it never closes.
func jsonPrefix(s string) int {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i + 1
			}
			if depth < 0 {
				return -1
			}
		}
	}
	return -1
}

const maxDepth = 128

type lexer struct {
	src string
	pos int
}

func (l *lexer) end() bool { return l.pos >= len(l.src) }

func (l *lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isIdent(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$'
}

func isNumberStart(c byte) bool { return c == '-' || c == '+' || c == '.' || c >= '0' && c <= '9' }

func isNumberChar(c byte) bool {
	return c >= '0' && c <= '9' || c == '-' || c == '+' || c == '.' ||
		c|0x20 >= 'a' && c|0x20 <= 'f' || c|0x20 == 'x'
}

// skipInsignificant skips whitespace and // or /* */ comments from i.
func skipInsignificant(s string, i int) int {
	for {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i+1 < len(s) && s[i] == '/' {
			switch s[i+1] {
			case '/':
				i += 2
				for i < len(s) && s[i] != '\n' {
					i++
				}
				continue
			case '*':
				i += 2
				for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
					i++
				}
				i = min(i+2, len(s))
				continue
			}
		}
		return i
	}
}

func (l *lexer) ws() { l.pos = skipInsignificant(l.src, l.pos) }

func (l *lexer) value(bareword bool, depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("%w: nested too deep", errSyntax)
	}
	c := l.peek()
	switch {
	case c == '{':
		return l.object(depth)
	case c == '[':
		return l.array(depth)
	case c == '"' || c == '\'':
		return l.str(c)
	case isNumberStart(c):
		return l.number()
	}
	if v, ok := l.keyword(); ok {
		return v, nil
	}
	if bareword {
		return l.bareword()
	}
	return nil, fmt.Errorf("%w: unexpected token at %d", errSyntax, l.pos)
}

func (l *lexer) object(depth int) (any, error) {
	l.pos++
	out := object{}
	for {
		l.ws()
		if l.end() {
			return nil, fmt.Errorf("%w: unterminated object", errSyntax)
		}
		switch l.peek() {
		case '}':
			l.pos++
			return out, nil
		case ',':
			l.pos++
			continue
		}
		key, err := l.key()
		if err != nil {
			return nil, err
		}
		l.ws()
		if l.peek() != ':' {
			return nil, fmt.Errorf("%w: expected ':' at %d", errSyntax, l.pos)
		}
		l.pos++
		l.ws()
		if l.end() {
			return nil, fmt.Errorf("%w: missing value", errSyntax)
		}
		v, err := l.value(true, depth+1)
		if err != nil {
			return nil, err
		}
		out.set(key, v)
		l.ws()
		switch l.peek() {
		case ',':
			l.pos++
		case '}':
			l.pos++
			return out, nil
		default:
			return nil, fmt.Errorf("%w: expected ',' or '}' at %d", errSyntax, l.pos)
		}
	}
}

func (l *lexer) array(depth int) (any, error) {
	l.pos++
	out := []any{}
	for {
		l.ws()
		if l.end() {
			return nil, fmt.Errorf("%w: unterminated array", errSyntax)
		}
		switch l.peek() {
		case ']':
			l.pos++
			return out, nil
		case ',':
			l.pos++
			continue
		}
		v, err := l.value(true, depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		l.ws()
		switch l.peek() {
		case ',':
			l.pos++
		case ']':
			l.pos++
			return out, nil
		default:
			return nil, fmt.Errorf("%w: expected ',' or ']' at %d", errSyntax, l.pos)
		}
	}
}

func (l *lexer) key() (string, error) {
	c := l.peek()
	if c == '"' || c == '\'' {
		return l.str(c)
	}
	start := l.pos
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == ':' || c == ',' || c == '}' || isSpace(c) {
			break
		}
		l.pos++
	}
	if l.pos == start {
		return "", fmt.Errorf("%w: expected a key at %d", errSyntax, l.pos)
	}
	return l.src[start:l.pos], nil
}

// str reads a quoted string. A double quote closes at once, as in JSON; a
// single quote closes only before a value terminator, so 'it's' survives.
func (l *lexer) str(quote byte) (string, error) {
	s := l.src
	i := l.pos + 1
	var b strings.Builder
	run := i
	for i < len(s) {
		c := s[i]
		if c == quote {
			if quote == '"' || l.closes(i+1) {
				b.WriteString(s[run:i])
				l.pos = i + 1
				return b.String(), nil
			}
			i++
			continue
		}
		if c != '\\' {
			i++
			continue
		}
		b.WriteString(s[run:i])
		i++
		if i >= len(s) {
			break
		}
		switch e := s[i]; e {
		case '"', '\'', '\\', '/':
			b.WriteByte(e)
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			r, n := unicodeEscape(s, i+1)
			if n == 0 {
				b.WriteString(`\u`)
				break
			}
			b.WriteRune(r)
			i += n
		default:
			b.WriteByte('\\')
			b.WriteByte(e)
		}
		i++
		run = i
	}
	return "", fmt.Errorf("%w: unterminated string", errSyntax)
}

// unicodeEscape decodes the four hex digits at i, joining a surrogate pair
// written as two escapes. n is the bytes used past the 'u', 0 when invalid.
func unicodeEscape(s string, i int) (rune, int) {
	hi, ok := hex4(s, i)
	if !ok {
		return 0, 0
	}
	r := rune(hi)
	if utf16.IsSurrogate(r) && i+10 <= len(s) && s[i+4] == '\\' && s[i+5] == 'u' {
		if lo, ok := hex4(s, i+6); ok {
			if pair := utf16.DecodeRune(r, rune(lo)); pair != utf8.RuneError {
				return pair, 10
			}
		}
	}
	if utf16.IsSurrogate(r) {
		r = utf8.RuneError
	}
	return r, 4
}

func hex4(s string, i int) (int, bool) {
	if i+4 > len(s) {
		return 0, false
	}
	v, err := strconv.ParseUint(s[i:i+4], 16, 32)
	if err != nil {
		return 0, false
	}
	return int(v), true
}

func (l *lexer) closes(from int) bool {
	k := skipInsignificant(l.src, from)
	if k >= len(l.src) {
		return true
	}
	switch l.src[k] {
	case ',', '}', ']', ':':
		return true
	}
	return false
}

func (l *lexer) number() (any, error) {
	start := l.pos
	for l.pos < len(l.src) && isNumberChar(l.src[l.pos]) {
		l.pos++
	}
	n, ok := canonicalNumber(l.src[start:l.pos])
	if !ok {
		return nil, fmt.Errorf("%w: bad number %q", errSyntax, l.src[start:l.pos])
	}
	return n, nil
}

// canonicalNumber turns relaxed number text (+1, .5, 5., 0x1F, 0b101) into
// JSON number text; non-finite values fail.
func canonicalNumber(t string) (number, bool) {
	if t == "" {
		return "", false
	}
	if jsonNumber(t) {
		return number(t), true
	}
	if i, err := strconv.ParseInt(t, 0, 64); err == nil {
		return number(strconv.FormatInt(i, 10)), true
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", false
	}
	return number(strconv.FormatFloat(f, 'g', -1, 64)), true
}

// jsonNumber reports whether t is a number in strict JSON grammar.
func jsonNumber(t string) bool {
	i := 0
	if i < len(t) && t[i] == '-' {
		i++
	}
	switch {
	case i < len(t) && t[i] == '0':
		i++
	case i < len(t) && t[i] >= '1' && t[i] <= '9':
		for i < len(t) && t[i] >= '0' && t[i] <= '9' {
			i++
		}
	default:
		return false
	}
	if i < len(t) && t[i] == '.' {
		i++
		if i >= len(t) || t[i] < '0' || t[i] > '9' {
			return false
		}
		for i < len(t) && t[i] >= '0' && t[i] <= '9' {
			i++
		}
	}
	if i < len(t) && (t[i] == 'e' || t[i] == 'E') {
		i++
		if i < len(t) && (t[i] == '+' || t[i] == '-') {
			i++
		}
		if i >= len(t) || t[i] < '0' || t[i] > '9' {
			return false
		}
		for i < len(t) && t[i] >= '0' && t[i] <= '9' {
			i++
		}
	}
	return i == len(t)
}

var keywords = []struct {
	word string
	val  any
}{
	{"true", true}, {"false", false}, {"null", nil},
	{"True", true}, {"False", false}, {"None", nil},
}

func (l *lexer) keyword() (any, bool) {
	for _, k := range keywords {
		if strings.HasPrefix(l.src[l.pos:], k.word) {
			next := l.pos + len(k.word)
			if next < len(l.src) && isIdent(l.src[next]) {
				continue
			}
			l.pos = next
			return k.val, true
		}
	}
	return nil, false
}

var nonFinite = map[string]bool{"NaN": true, "Infinity": true, "-Infinity": true, "+Infinity": true, "undefined": true}

// bareword recovers an unquoted string value such as {"path": src/*.go} up
// to the next ',', '}', ']' or newline. A quote, bracket or key-like colon
// inside fails, so a missed comma never swallows the next field.
func (l *lexer) bareword() (any, error) {
	s := l.src
	start := l.pos
	i := start
	for ; i < len(s); i++ {
		c := s[i]
		if c == ',' || c == '}' || c == ']' || c == '\n' || c == '\r' {
			break
		}
		if c == '/' && i > start && isSpace(s[i-1]) && i+1 < len(s) && (s[i+1] == '/' || s[i+1] == '*') {
			break
		}
		if c == '"' || c == '{' || c == '[' {
			return nil, fmt.Errorf("%w: unexpected token at %d", errSyntax, start)
		}
		if c == ':' && (i+1 >= len(s) || s[i+1] != '/' && s[i+1] != '\\') {
			return nil, fmt.Errorf("%w: unexpected token at %d", errSyntax, start)
		}
	}
	if i >= len(s) {
		return nil, fmt.Errorf("%w: unterminated value", errSyntax)
	}
	word := strings.TrimRight(s[start:i], " \t")
	if word == "" || nonFinite[word] {
		return nil, fmt.Errorf("%w: unexpected token at %d", errSyntax, start)
	}
	l.pos = i
	return word, nil
}

// marshal writes v as compact JSON without HTML escaping.
func marshal(v any) string {
	return string(appendJSON(nil, v))
}

func appendJSON(b []byte, v any) []byte {
	switch v := v.(type) {
	case nil:
		return append(b, "null"...)
	case bool:
		if v {
			return append(b, "true"...)
		}
		return append(b, "false"...)
	case number:
		return append(b, v...)
	case string:
		return appendString(b, v)
	case []any:
		b = append(b, '[')
		for i, e := range v {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSON(b, e)
		}
		return append(b, ']')
	case object:
		b = append(b, '{')
		for i, m := range v {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendString(b, m.key)
			b = append(b, ':')
			b = appendJSON(b, m.val)
		}
		return append(b, '}')
	}
	return append(b, "null"...)
}

func appendString(b []byte, s string) []byte {
	const hexDigits = "0123456789abcdef"
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '"' || c == '\\':
				b = append(b, '\\', c)
			case c == '\n':
				b = append(b, '\\', 'n')
			case c == '\r':
				b = append(b, '\\', 'r')
			case c == '\t':
				b = append(b, '\\', 't')
			case c < 0x20:
				b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
			default:
				b = append(b, c)
			}
			i++
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 {
			b = append(b, `\ufffd`...)
		} else {
			b = append(b, s[i:i+n]...)
		}
		i += n
	}
	return append(b, '"')
}
