package fs

import (
	"fmt"
	"strings"
)

// Derived from oh-my-pi (MIT), crates/edit/src/store.rs and
// crates/edit/src/modes/hashline/format.rs@c7fff67

// fileTag is a file's hashline tag: the low 16 bits of XXH32 (seed 0) of its
// text with each line's trailing spaces, tabs and carriage returns dropped,
// as four uppercase hex digits. text is already BOM-stripped and LF-only.
func fileTag(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for len(text) > 0 {
		line := text
		nl := strings.IndexByte(text, '\n')
		if nl >= 0 {
			line = text[:nl]
			text = text[nl+1:]
		} else {
			text = ""
		}
		b.WriteString(strings.TrimRight(line, " \t\r"))
		if nl >= 0 {
			b.WriteByte('\n')
		}
	}
	return fmt.Sprintf("%04X", xxhash32([]byte(b.String()), 0)&0xffff)
}

// header is the line read prints above a file's rows in the full profile.
func header(shown, tag string) string { return "[" + shown + "#" + tag + "]" }

// parseHeader splits a `[PATH#TAG]` or `[PATH]` row. A valid trailing
// four-hex tag lets the path itself hold `#`.
func parseHeader(line string) (path, tag string, ok bool) {
	line = strings.TrimRight(line, " \t\r")
	if len(line) < 3 || line[0] != '[' || line[len(line)-1] != ']' {
		return "", "", false
	}
	body := line[1 : len(line)-1]
	if body == "" {
		return "", "", false
	}
	if i := strings.LastIndexByte(body, '#'); i >= 0 {
		p, t := body[:i], body[i+1:]
		if p == "" || orphanBracket(p) {
			return "", "", false
		}
		if !hexTag(t) {
			// a placeholder tag (#AUTO, #????, bare #) still names the file;
			// the edit is rejected with the current tag
			return p, "", true
		}
		return p, strings.ToUpper(t), true
	}
	if orphanBracket(body) {
		return "", "", false
	}
	return body, "", true
}

func hexTag(t string) bool {
	if len(t) != 4 {
		return false
	}
	for _, c := range t {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// orphanBracket reports a `]` without its `[`: two bracket groups on one row
// are noise, not a path.
func orphanBracket(path string) bool {
	depth := 0
	for _, c := range path {
		switch c {
		case '[':
			depth++
		case ']':
			if depth == 0 {
				return true
			}
			depth--
		}
	}
	return false
}

// stripHeaderPath accepts a path the model copied with its header around it.
func stripHeaderPath(path string) string {
	if p, _, ok := parseHeader(strings.TrimSpace(path)); ok {
		return p
	}
	return path
}

// stripRowNumbers removes `N:` prefixes a model copied from read output into
// content: only when every line carries one and they count up by one, so a
// file that really starts lines with numbers is left alone.
func stripRowNumbers(text string) (string, bool) {
	rows := strings.Split(text, "\n")
	trailing := false
	if len(rows) > 1 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
		trailing = true
	}
	if len(rows) < 2 {
		return text, false
	}
	want := -1
	out := make([]string, len(rows))
	for i, r := range rows {
		n, rest, ok := rowNumber(r)
		if !ok || (want >= 0 && n != want) {
			return text, false
		}
		want = n + 1
		out[i] = rest
	}
	s := strings.Join(out, "\n")
	if trailing {
		s += "\n"
	}
	return s, true
}

func rowNumber(r string) (int, string, bool) {
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(r) || r[i] != ':' {
		return 0, "", false
	}
	n := 0
	for _, c := range r[:i] {
		n = n*10 + int(c-'0')
	}
	return n, r[i+1:], true
}
