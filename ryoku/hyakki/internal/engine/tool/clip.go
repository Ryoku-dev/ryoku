package tool

import (
	"fmt"
	"unicode/utf8"
)

// ClipBytes cuts s to at most max bytes on a rune boundary and reports
// whether it cut.
func ClipBytes(s string, max int) (string, bool) {
	if max < 0 || len(s) <= max {
		return s, false
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

// ClipRunes cuts s to at most max runes and reports whether it cut.
func ClipRunes(s string, max int) (string, bool) {
	if max < 0 || len(s) <= max {
		return s, false
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i], true
		}
		n++
	}
	return s, false
}

// HeadTail keeps the first head and last tail bytes of s, on rune
// boundaries, with a line saying how many bytes were left out between them.
// Text that fits in head+tail comes back whole. This is the shape every
// tool uses for output over Limits.Told, so the model learns one marker.
func HeadTail(s string, head, tail int) string {
	if len(s) <= head+tail {
		return s
	}
	start, _ := ClipBytes(s, head)
	from := len(s) - tail
	for from < len(s) && !utf8.RuneStart(s[from]) {
		from++
	}
	return fmt.Sprintf("%s\n[%d bytes left out]\n%s", start, from-len(start), s[from:])
}
