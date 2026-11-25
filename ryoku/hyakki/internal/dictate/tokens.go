package dictate

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type kind uint8

const (
	kWord      kind = iota // letters, with inner apostrophes, hyphens and dots
	kPunct                 // one punctuation character
	kProtected             // a URL, email, path, code span or number: never changed
	kFixed                 // a dictionary spelling: written as stored
	kBreak                 // a line break, "\n", or a paragraph break, "\n\n"
)

type token struct {
	text  string
	kind  kind
	space bool // whitespace came before it
}

func (t token) content() bool { return t.kind == kWord || t.kind == kProtected || t.kind == kFixed }

func (t token) is(word string) bool { return t.kind == kWord && strings.EqualFold(t.text, word) }

// mark reports whether t is a punctuation token found in set.
func (t token) mark(set string) bool { return t.kind == kPunct && strings.Contains(set, t.text) }

const (
	openers   = "([{“‘«"
	closers   = ".,;:!?)]}”’»…"
	quotes    = "\"'"
	stops     = ".!?"
	pauses    = ",;:"
	dashes    = "—–-"
	edgeMarks = pauses + stops + "…" + dashes
)

// boundary reports a clause edge: a pause, a stop, a dash or a line break.
func boundary(t token) bool { return t.kind == kBreak || t.mark(edgeMarks) }

// sentenceEnd reports a stop or a line break.
func sentenceEnd(t token) bool { return t.kind == kBreak || t.mark(stops) }

var (
	urlPattern   = regexp.MustCompile(`^(?i:(?:https?|ftps?|sftp|ssh|git|file|wss?)://\S+|www\.[^\s.]+\.\S+)$`)
	emailPattern = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+$`)
	drivePattern = regexp.MustCompile(`^[A-Za-z]:\\`)
	relPattern   = regexp.MustCompile(`^[\w.\-]+(?:/[\w.\-]+)+$`)
)

// protectedCore reports a value dictation must keep byte for byte: a URL,
// an email address, a path, or anything holding a digit.
func protectedCore(s string) bool {
	switch {
	case urlPattern.MatchString(s), emailPattern.MatchString(s), drivePattern.MatchString(s):
		return true
	case len(s) > 1 && s[0] == '/', strings.HasPrefix(s, "~/"), strings.HasPrefix(s, "./"), strings.HasPrefix(s, "../"):
		return true
	case relPattern.MatchString(s):
		last := s[strings.LastIndexByte(s, '/')+1:]
		if dot := strings.LastIndexByte(last, '.'); dot > 0 && dot < len(last)-1 {
			return true
		}
	}
	return strings.IndexFunc(s, unicode.IsDigit) >= 0
}

// codeSpanEnd returns the end of a code span starting at i: a fenced block
// between triple backticks, or a non-empty run between single backticks on
// one line. It returns -1 when no span starts there.
func codeSpanEnd(s string, i int) int {
	if strings.HasPrefix(s[i:], "```") {
		if j := strings.Index(s[i+3:], "```"); j >= 0 {
			return i + 3 + j + 3
		}
		return -1
	}
	if j := strings.IndexAny(s[i+1:], "`\n"); j > 0 && s[i+1+j] == '`' {
		return i + 1 + j + 1
	}
	return -1
}

// tokenize splits text into tokens, keeping protected values whole.
func tokenize(text string) []token {
	var out []token
	space := false
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == '\n' {
			lines, j := 0, i
			for j < len(text) {
				if text[j] == '\n' {
					lines++
				} else if text[j] != ' ' && text[j] != '\t' && text[j] != '\r' {
					break
				}
				j++
			}
			brk := "\n"
			if lines > 1 {
				brk = "\n\n"
			}
			out = append(out, token{text: brk, kind: kBreak})
			space, i = false, j
			continue
		}
		if unicode.IsSpace(r) {
			space, i = true, i+size
			continue
		}
		if r == '`' {
			if end := codeSpanEnd(text, i); end > 0 {
				out = append(out, token{text: text[i:end], kind: kProtected, space: space})
				space, i = false, end
				continue
			}
		}
		j := i
		for j < len(text) {
			r2, s2 := utf8.DecodeRuneInString(text[j:])
			if unicode.IsSpace(r2) || r2 == '`' && j > i && codeSpanEnd(text, j) > 0 {
				break
			}
			j += s2
		}
		out = appendChunk(out, text[i:j], space)
		space, i = false, j
	}
	return out
}

// appendChunk splits one whitespace-free chunk: opening marks, the core, then
// closing marks. A protected core stays one token; any other core splits into
// words and single punctuation characters.
func appendChunk(out []token, chunk string, space bool) []token {
	emit := func(t token) {
		t.space = space
		space = false
		out = append(out, t)
	}
	for chunk != "" {
		r, size := utf8.DecodeRuneInString(chunk)
		if !strings.ContainsRune(openers+quotes, r) {
			break
		}
		emit(token{text: chunk[:size], kind: kPunct})
		chunk = chunk[size:]
	}
	var tail []string
	for chunk != "" {
		r, size := utf8.DecodeLastRuneInString(chunk)
		if !strings.ContainsRune(closers+quotes, r) {
			break
		}
		tail = append(tail, chunk[len(chunk)-size:])
		chunk = chunk[:len(chunk)-size]
	}
	if chunk != "" && protectedCore(chunk) {
		emit(token{text: chunk, kind: kProtected})
	} else {
		runes := []rune(chunk)
		start := -1
		for k, r := range runes {
			inWord := wordRune(r) || joiner(r) && k > 0 && k+1 < len(runes) && wordRune(runes[k-1]) && wordRune(runes[k+1])
			if inWord {
				if start < 0 {
					start = k
				}
				continue
			}
			if start >= 0 {
				emit(token{text: string(runes[start:k]), kind: kWord})
				start = -1
			}
			emit(token{text: string(r), kind: kPunct})
		}
		if start >= 0 {
			emit(token{text: string(runes[start:]), kind: kWord})
		}
	}
	for k := len(tail) - 1; k >= 0; k-- {
		emit(token{text: tail[k], kind: kPunct})
	}
	return out
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r) || r == '_'
}

func joiner(r rune) bool { return r == '\'' || r == '’' || r == '-' || r == '.' }

// render joins tokens with normal spacing: none before closing marks or after
// opening ones, one after a pause or stop, otherwise as dictated.
func render(toks []token) string {
	var b strings.Builder
	var prev token
	for k, t := range toks {
		if t.kind == kBreak {
			b.WriteString(t.text)
			prev = t
			continue
		}
		if k > 0 && prev.kind != kBreak && needSpace(prev, t) {
			b.WriteByte(' ')
		}
		b.WriteString(t.text)
		prev = t
	}
	return b.String()
}

func needSpace(prev, t token) bool {
	if t.mark(closers) {
		return false
	}
	if prev.mark(openers) {
		return false
	}
	if prev.mark(closers) && !prev.mark(")]}”’»") && (t.content() || t.mark(openers)) {
		return true
	}
	return t.space
}

// words lowercases the content tokens of a phrase; ok is false when the
// phrase holds anything but words and protected values.
func words(phrase string) (out []string, ok bool) {
	for _, t := range tokenize(phrase) {
		if !t.content() {
			return nil, false
		}
		out = append(out, strings.ToLower(t.text))
	}
	return out, len(out) > 0
}
