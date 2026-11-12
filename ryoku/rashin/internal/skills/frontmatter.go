package skills

import (
	"strconv"
	"strings"
)

// frontmatter reads the top-level scalar keys of a SKILL.md's YAML
// frontmatter: plain, quoted, folded (>) and literal (|) values. Nested maps
// and lists are skipped, as no key Rashin reads holds one. A file without
// frontmatter has no keys.
func frontmatter(text string) map[string]string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	lines := strings.Split(text[len("---\n"):], "\n")
	end := -1
	for i, line := range lines {
		if strings.TrimRight(line, " \t") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil
	}
	lines = lines[:end]
	keys := map[string]string{}
	for i := 0; i < len(lines); {
		line := lines[i]
		i++
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || (value != "" && value[0] != ' ' && value[0] != '\t') {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// the lines indented under the key belong to its value
		var more []string
		for i < len(lines) && (strings.TrimSpace(lines[i]) == "" || lines[i][0] == ' ' || lines[i][0] == '\t') {
			more = append(more, lines[i])
			i++
		}
		switch {
		case value == "":
			// a nested map or list
		case value[0] == '>' || value[0] == '|':
			keys[key] = block(value, more)
		case value[0] == '"' || value[0] == '\'':
			keys[key] = quoted(strings.Join(append([]string{value}, trimmed(more)...), " "))
		default:
			keys[key] = plain(value, more)
		}
	}
	return keys
}

// block is a folded (>) or literal (|) block scalar's value with its
// chomping (- strips the final line break, + keeps every trailing one).
func block(header string, lines []string) string {
	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			indent = len(line) - len(strings.TrimLeft(line, " "))
			break
		}
	}
	var body []string
	for _, line := range lines {
		switch {
		case strings.TrimSpace(line) == "":
			body = append(body, "")
		case len(line) >= indent && indent >= 0:
			body = append(body, line[indent:])
		default:
			body = append(body, strings.TrimLeft(line, " \t"))
		}
	}
	var b strings.Builder
	folded := header[0] == '>'
	for i, line := range body {
		if i > 0 {
			previous := body[i-1]
			plainLines := line != "" && previous != "" && line[0] != ' ' && previous[0] != ' '
			// a folded block joins lines with a space, and the break before
			// a blank line goes into it; a more indented line keeps its break
			switch {
			case folded && plainLines:
				b.WriteByte(' ')
			case folded && line == "" && previous != "" && previous[0] != ' ':
			default:
				b.WriteByte('\n')
			}
		}
		b.WriteString(line)
	}
	value := strings.TrimRight(b.String(), "\n")
	switch {
	case strings.Contains(header, "-"):
		return value
	case strings.Contains(header, "+"):
		return value + strings.Repeat("\n", trailingBlank(body)+1)
	default:
		return value + "\n"
	}
}

// trailingBlank counts the blank lines that end a block.
func trailingBlank(lines []string) int {
	n := 0
	for i := len(lines) - 1; i >= 0 && strings.TrimSpace(lines[i]) == ""; i-- {
		n++
	}
	return n
}

// quoted is a double- or single-quoted scalar's value; one that does not
// close is read as written.
func quoted(value string) string {
	if value[0] == '\'' {
		if end := strings.LastIndex(value, "'"); end > 0 {
			return strings.ReplaceAll(value[1:end], "''", "'")
		}
		return value
	}
	end := closingQuote(value)
	if end < 0 {
		return value
	}
	if unquoted, err := strconv.Unquote(value[:end+1]); err == nil {
		return unquoted
	}
	return value[1:end]
}

// closingQuote is the index of the double quote that ends value, or -1.
func closingQuote(value string) int {
	for i := 1; i < len(value); i++ {
		switch value[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}

// plain is a plain scalar's value: a trailing comment dropped, and lines it
// runs on to folded into one.
func plain(value string, more []string) string {
	if at := strings.Index(value, " #"); at >= 0 {
		value = strings.TrimSpace(value[:at])
	}
	return strings.Join(append([]string{value}, trimmed(more)...), " ")
}

// trimmed are lines without their indent, the blank ones dropped.
func trimmed(lines []string) []string {
	var out []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// truthy reads a YAML boolean.
func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "on":
		return true
	}
	return false
}
