package screenask

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	codeFence = regexp.MustCompile("(?m)^\\s*```[A-Za-z0-9_-]*\\s*$")
	dashRun   = regexp.MustCompile(`-{3,}`)
	listMark  = regexp.MustCompile(`(?m)^[ \t]*(?:#{1,6}[ \t]+|[*+][ \t]+|-[ \t]+|\d+[.)][ \t]+)`)
)

// unfence drops code-fence lines a model wraps a reply in.
func unfence(s string) string { return codeFence.ReplaceAllString(s, "") }

// hidden reports runes that must never reach a prompt or a bubble: control
// characters, and format runes that reorder or hide text (bidi overrides,
// zero-width joiners, byte-order marks).
func hidden(r rune) bool {
	return r != '\n' && r != '\t' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r))
}

// clean strips hidden runes, markdown emphasis, list and heading marks and em
// dashes, and collapses whitespace. Line breaks become spaces unless keepLines.
func clean(s string, keepLines bool) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	s = strings.Map(func(r rune) rune {
		if hidden(r) || r == '*' || r == '`' {
			return -1
		}
		return r
	}, s)
	s = listMark.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, " \u2014 ", ", ")
	s = strings.ReplaceAll(s, "\u2014", ", ")
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, line := range lines {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			out = append(out, line)
		}
	}
	if keepLines {
		return strings.Join(out, "\n")
	}
	return strings.Join(out, " ")
}

// cleanText is clean on one line, cut to at most n runes on a word boundary.
func cleanText(s string, n int) string { return cut(clean(s, false), n) }

// foreign makes window text safe to place inside a fenced block: one line, no
// hidden runes, and no run of three dashes, so it can never close or open a
// fence.
func foreign(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if hidden(r) || r == '\n' || r == '\t' {
			return ' '
		}
		return r
	}, s)
	s = dashRun.ReplaceAllString(s, "--")
	return cut(strings.Join(strings.Fields(s), " "), n)
}

// cut shortens s to at most n runes, preferring to break between words.
func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndexByte(string(r), ' '); i > len(string(r))/2 {
		return strings.TrimRight(string(r)[:i], " ,;:")
	}
	return string(r)
}

// tokens lowercases s and splits it into its letter and digit runs.
func tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
