package remind

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// token is one word as the parser compares it, and its byte span in the
// text it came from, so a reminder keeps the user's own spelling.
type token struct {
	word       string
	start, end int
}

// strip replaces a decoder's annotations, such as "[BLANK_AUDIO]" or
// "(coughs)", with spaces of the same length, so spans stay put. An
// annotation left open runs to the end of the text.
func strip(text string) string {
	if !strings.ContainsAny(text, "[(*") {
		return text
	}
	var b strings.Builder
	var close rune
	for _, r := range text {
		switch {
		case close != 0:
			if r == close {
				close = 0
			}
		case r == '[':
			close = ']'
		case r == '(':
			close = ')'
		case r == '*':
			close = '*'
		default:
			b.WriteRune(r)
			continue
		}
		b.WriteString(strings.Repeat(" ", len(string(r))))
	}
	return b.String()
}

func asciiDigit(r rune) bool { return r >= '0' && r <= '9' }

// tokenize splits text into words as the routing matchers fold them: lower
// case, accents folded, apostrophes dropped ("o'clock" is "oclock"). Digits
// and letters part ("4pm" is "4" "pm"), and a time's colon or stop parts
// its hour from its minutes. One digit after a stop stays a fraction
// ("1.5").
func tokenize(text string) []token {
	var out []token
	var b strings.Builder
	start, class := -1, 0 // class: 1 letters, 2 digits
	flush := func(end int) {
		if b.Len() > 0 {
			out = append(out, token{word: b.String(), start: start, end: end})
			b.Reset()
		}
		start, class = -1, 0
	}
	for i, r := range text {
		switch {
		case r == '\'' || r == '’':
		case asciiDigit(r):
			if class == 1 {
				flush(i)
			}
			if start < 0 {
				start = i
			}
			class = 2
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if class == 2 {
				flush(i)
			}
			if start < 0 {
				start = i
			}
			class = 1
			if r < unicode.MaxASCII {
				b.WriteRune(unicode.ToLower(r))
				continue
			}
			for _, f := range norm.NFKD.String(string(r)) {
				if !unicode.Is(unicode.Mn, f) {
					b.WriteRune(unicode.ToLower(f))
				}
			}
		case r == '.' && class == 2 && fraction(text[i+1:]) && !strings.Contains(b.String(), "."):
			b.WriteRune(r)
		default:
			flush(i)
		}
	}
	flush(len(text))
	return out
}

// fraction reports whether rest starts with exactly one digit.
func fraction(rest string) bool {
	return len(rest) > 0 && asciiDigit(rune(rest[0])) && (len(rest) == 1 || !asciiDigit(rune(rest[1])))
}

var (
	smallNumbers = map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9,
		"ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15,
		"sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19,
	}
	tensNumbers = map[string]int{"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90}
)

// number reads a whole number at ws[p:]: up to five digits, or a number
// word up to ninety-nine ("forty five"). It returns the value and how many
// words it took.
func number(ws []string, p int) (int, int) {
	if p >= len(ws) {
		return 0, 0
	}
	w := ws[p]
	if w != "" && len(w) <= 5 && strings.IndexFunc(w, func(r rune) bool { return !asciiDigit(r) }) < 0 {
		n, err := strconv.Atoi(w)
		if err != nil {
			return 0, 0
		}
		return n, 1
	}
	if n, ok := smallNumbers[w]; ok {
		return n, 1
	}
	t, ok := tensNumbers[w]
	if !ok {
		return 0, 0
	}
	if p+1 < len(ws) {
		if o, ok := smallNumbers[ws[p+1]]; ok && o < 10 {
			return t + o, 2
		}
	}
	return t, 1
}

// has reports whether ws[p:] starts with the words in phrase.
func has(ws []string, p int, phrase ...string) bool {
	if p < 0 || p+len(phrase) > len(ws) {
		return false
	}
	for i, w := range phrase {
		if ws[p+i] != w {
			return false
		}
	}
	return true
}

// Polite words around a request, which change nothing.
var (
	politeLeads = [][]string{
		{"please"}, {"hey"}, {"ok"}, {"okay"}, {"oh"}, {"and"}, {"so"}, {"just"},
		{"can", "you"}, {"could", "you"}, {"would", "you"}, {"will", "you"},
		{"i", "want", "you", "to"}, {"i", "need", "you", "to"}, {"id", "like", "you", "to"},
		{"go", "ahead", "and"},
	}
	politeTails = [][]string{{"please"}, {"thanks"}, {"thank", "you"}, {"for", "me"}, {"now"}}
)

// trim is the span of ws left once polite words are taken off both ends.
func trim(ws []string) (int, int) {
	lo, hi := 0, len(ws)
	for changed := true; changed; {
		changed = false
		for _, p := range politeLeads {
			if lo+len(p) < hi && has(ws, lo, p...) {
				lo += len(p)
				changed = true
			}
		}
		for _, p := range politeTails {
			if hi-len(p) > lo && has(ws, hi-len(p), p...) {
				hi -= len(p)
				changed = true
			}
		}
	}
	return lo, hi
}

// request is one utterance as the parsers read it.
type request struct {
	text   string
	toks   []token
	ws     []string
	lo, hi int
}

func read(c string) request {
	text := strip(c)
	toks := tokenize(text)
	ws := make([]string, len(toks))
	for i, t := range toks {
		ws[i] = t.word
	}
	lo, hi := trim(ws)
	return request{text: text, toks: toks, ws: ws, lo: lo, hi: hi}
}

// core is the request's words with polite words trimmed, joined by spaces.
func (q request) core() string { return strings.Join(q.ws[q.lo:q.hi], " ") }

// span is the user's own text for words [p, e), trimmed of surrounding
// punctuation.
func (q request) span(p, e int) string {
	if p >= e {
		return ""
	}
	s := q.text[q.toks[p].start:q.toks[e-1].end]
	return strings.TrimSpace(s)
}
