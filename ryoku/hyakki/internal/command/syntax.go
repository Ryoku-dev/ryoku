package command

import "strings"

// shellSource is the common lexical boundary for read-only, tier, elevation,
// and secret inspection. Keep newlines: a shell comment ends at one, not at EOF.
func shellSource(src string) (string, string) {
	var out strings.Builder
	quote := byte(0)
	wordStart := true
	for i := 0; i < len(src); i++ {
		c := src[i]
		if quote == '\'' {
			out.WriteByte(c)
			if c == '\'' {
				quote = 0
			}
			continue
		}
		if c == '\\' {
			if i+1 == len(src) {
				return "", "unfinished escape"
			}
			if quote == '"' && src[i+1] == '"' {
				return "", "escaped shell quote cannot be inspected safely"
			}
			out.WriteByte(c)
			i++
			out.WriteByte(src[i])
			continue
		}
		if c == quote && quote != 0 {
			quote = 0
			out.WriteByte(c)
			continue
		}
		if c == '$' && i+1 < len(src) && src[i+1] == '\'' {
			return "", "ANSI-C shell quoting cannot be inspected safely"
		}
		if quote == 0 && c == '#' && wordStart {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if i == len(src) {
				break
			}
			out.WriteByte('\n')
			wordStart = true
			continue
		}
		if quote == 0 && (c == '\'' || c == '"') {
			quote = c
		}
		out.WriteByte(c)
		if quote == 0 {
			wordStart = c == ' ' || c == '\t' || c == '\n' || c == ';' || c == '|' || c == '&' || c == '(' || c == ')' || c == '{' || c == '}' || c == '<' || c == '>'
		} else {
			wordStart = false
		}
	}
	if quote != 0 {
		return "", "unterminated shell quote"
	}
	return out.String(), ""
}
