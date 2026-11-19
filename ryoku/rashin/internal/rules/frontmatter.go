package rules

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// value is one front matter value: a scalar, or a list when list is set.
type value struct {
	scalar string
	list   []string
	isList bool
}

// strings returns a list as it is and a non-empty scalar as a one-item list.
func (v value) strings() []string {
	if v.isList {
		return v.list
	}
	if v.scalar == "" {
		return nil
	}
	return []string{v.scalar}
}

// splitFrontmatter separates YAML front matter from the body. A file without
// front matter is all body.
func splitFrontmatter(text string) (front []string, body string, ok bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimPrefix(text, "\ufeff")
	if !strings.HasPrefix(text, "---\n") {
		return nil, text, true
	}
	lines := strings.Split(text[len("---\n"):], "\n")
	for i, line := range lines {
		if strings.TrimRight(line, " \t") == "---" {
			return lines[:i], strings.Join(lines[i+1:], "\n"), true
		}
	}
	return nil, "", false
}

// parseFrontmatter reads the keys rule files use: scalars (plain, single or
// double quoted, block | and >), flow lists ["a", "b"] and block lists of
// "- item" lines. Comments and nested maps are skipped.
func parseFrontmatter(lines []string) map[string]value {
	keys := map[string]value{}
	for i := 0; i < len(lines); {
		line := lines[i]
		i++
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		rest := strings.TrimSpace(line[colon+1:])
		// The lines that belong to this key: indented ones, list items and
		// comments, up to the next key.
		var more []string
		for i < len(lines) {
			next := lines[i]
			if next != "" && next[0] != ' ' && next[0] != '\t' && next[0] != '-' && next[0] != '#' {
				break
			}
			more = append(more, next)
			i++
		}
		switch {
		case rest == "" || strings.HasPrefix(rest, "#"):
			if items, ok := blockList(more); ok {
				keys[key] = value{list: items, isList: true}
			} else {
				keys[key] = value{}
			}
		case rest[0] == '[':
			keys[key] = value{list: flowList(rest), isList: true}
		case rest[0] == '|' || rest[0] == '>':
			keys[key] = value{scalar: blockScalar(rest[0] == '>', more)}
		default:
			keys[key] = value{scalar: scalar(rest)}
		}
	}
	return keys
}

// blockList reads "- item" lines; comments and blank lines between them are
// skipped.
func blockList(lines []string) ([]string, bool) {
	var items []string
	found := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if !strings.HasPrefix(t, "-") {
			return nil, false
		}
		found = true
		items = append(items, scalar(strings.TrimSpace(t[1:])))
	}
	return items, found
}

// flowList reads ["a", 'b', c]. Commas inside quotes stay in the item.
func flowList(text string) []string {
	text = strings.TrimSpace(text)
	if end := closingBracket(text); end > 0 {
		text = text[1:end]
	} else {
		text = strings.TrimPrefix(text, "[")
	}
	var items []string
	var cur strings.Builder
	quote := byte(0)
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case quote == '"' && c == '\\' && i+1 < len(text):
			cur.WriteByte(c)
			cur.WriteByte(text[i+1])
			i++
		case quote != 0:
			cur.WriteByte(c)
			if c == quote {
				if quote == '\'' && i+1 < len(text) && text[i+1] == '\'' {
					cur.WriteByte('\'')
					i++
					continue
				}
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			cur.WriteByte(c)
		case c == ',':
			if s := strings.TrimSpace(cur.String()); s != "" {
				items = append(items, scalar(s))
			}
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		items = append(items, scalar(s))
	}
	return items
}

// closingBracket is the index of the ] that closes a flow list, quotes
// respected, or -1.
func closingBracket(text string) int {
	quote := byte(0)
	for i := 1; i < len(text); i++ {
		c := text[i]
		switch {
		case quote == '"' && c == '\\':
			i++
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ']':
			return i
		}
	}
	return -1
}

// blockScalar reads a | (literal) or > (folded) block.
func blockScalar(folded bool, lines []string) string {
	indent := -1
	var kept []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			kept = append(kept, "")
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
		kept = append(kept, line)
	}
	for i, line := range kept {
		if len(line) >= indent && indent > 0 {
			kept[i] = line[indent:]
		}
	}
	text := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if folded {
		text = strings.ReplaceAll(text, "\n\n", "\x00")
		text = strings.ReplaceAll(text, "\n", " ")
		text = strings.ReplaceAll(text, "\x00", "\n")
	}
	return text
}

// scalar reads one scalar: quoted forms unescaped, a plain one without its
// trailing comment.
func scalar(text string) string {
	text = strings.TrimSpace(text)
	if len(text) >= 2 && text[0] == '"' {
		if end := closingQuote(text); end > 0 {
			return unescapeDouble(text[1:end])
		}
	}
	if len(text) >= 2 && text[0] == '\'' {
		var b strings.Builder
		for i := 1; i < len(text); i++ {
			if text[i] == '\'' {
				if i+1 < len(text) && text[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				return b.String()
			}
			b.WriteByte(text[i])
		}
		return b.String()
	}
	if i := strings.Index(text, " #"); i >= 0 {
		text = strings.TrimSpace(text[:i])
	}
	return text
}

func closingQuote(text string) int {
	for i := 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}

// unescapeDouble reads YAML double-quoted escapes. An escape YAML does not
// know is kept as written.
func unescapeDouble(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch e := s[i]; e {
		case '\\', '"', '/', ' ':
			b.WriteByte(e)
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '0':
			b.WriteByte(0)
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'e':
			b.WriteByte(0x1b)
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case 'x', 'u', 'U':
			n := 2
			if e == 'u' {
				n = 4
			} else if e == 'U' {
				n = 8
			}
			if i+1+n <= len(s) {
				if r, err := strconv.ParseUint(s[i+1:i+1+n], 16, 32); err == nil && utf8.ValidRune(rune(r)) {
					b.WriteRune(rune(r))
					i += n
					continue
				}
			}
			b.WriteByte('\\')
			b.WriteByte(e)
		default:
			b.WriteByte('\\')
			b.WriteByte(e)
		}
	}
	return b.String()
}

// quote writes a YAML double-quoted scalar. Go's quoting uses only escapes
// YAML reads the same way.
func quote(s string) string { return strconv.Quote(s) }

// truthy reads a YAML boolean.
func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}
