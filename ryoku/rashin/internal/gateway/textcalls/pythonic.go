package textcalls

// Derived from oh-my-pi (MIT), packages/ai/src/dialect/rendering.ts@c7fff67

import (
	"fmt"
	"strings"
)

// pyLead only claims a bare list after seeing the first complete function
// name and opening parenthesis. Ordinary bracketed prose stays visible.
func pyLead(s string) match {
	if !strings.HasPrefix(s, "[") {
		return matchNo
	}
	s = strings.TrimLeft(s[1:], spaces)
	if s == "" {
		return matchMaybe
	}
	if !isIdentStart(s[0]) {
		return matchNo
	}
	i := 1
	for i < len(s) && (isIdent(s[i]) || s[i] == '.' || s[i] == '-' || s[i] == ':') {
		i++
	}
	name := s[:i]
	s = strings.TrimLeft(s[i:], spaces)
	if s == "" {
		return matchMaybe
	}
	if s[0] == '(' && validName(name) {
		return matchYes
	}
	return matchNo
}

func pyFirstName(s string) string {
	s = strings.TrimLeft(s, spaces)
	if strings.HasPrefix(s, "[") {
		s = strings.TrimLeft(s[1:], spaces)
	}
	i := strings.IndexByte(s, '(')
	if i < 0 || i > maxName {
		return ""
	}
	n := strings.TrimSpace(s[:i])
	if validName(n) {
		return n
	}
	return ""
}

// pyListEnd returns the end of the outer list only when quotes, escapes and
// nested containers are balanced. A closing bracket in a string is data.
func pyListEnd(s string) int {
	if !strings.HasPrefix(strings.TrimLeft(s, spaces), "[") {
		return -1
	}
	var stack []byte
	var quote byte
	triple := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				if triple {
					if i+2 < len(s) && s[i+1] == quote && s[i+2] == quote {
						quote = 0
						triple = false
						i += 2
					}
				} else {
					quote = 0
				}
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			triple = i+2 < len(s) && s[i+1] == c && s[i+2] == c
			if triple {
				i += 2
			}
		case '(', '[', '{':
			stack = append(stack, c)
		case ')', ']', '}':
			if len(stack) == 0 || !matching(stack[len(stack)-1], c) {
				return -1
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func matching(open, close byte) bool {
	return open == '(' && close == ')' || open == '[' && close == ']' || open == '{' && close == '}'
}

// pyParts splits at top-level commas while preserving nested JSON/Python
// containers and quoted strings. The scanner waits for a complete list first.
func pyParts(s string) ([]string, error) {
	var parts []string
	var stack []byte
	var quote byte
	triple := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				if triple {
					if i+2 < len(s) && s[i+1] == quote && s[i+2] == quote {
						quote = 0
						triple = false
						i += 2
					}
				} else {
					quote = 0
				}
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			triple = i+2 < len(s) && s[i+1] == c && s[i+2] == c
			if triple {
				i += 2
			}
		case '(', '[', '{':
			stack = append(stack, c)
		case ')', ']', '}':
			if len(stack) == 0 || !matching(stack[len(stack)-1], c) {
				return nil, errSyntax
			}
			stack = stack[:len(stack)-1]
		case ',':
			if len(stack) == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if quote != 0 || len(stack) != 0 {
		return nil, errSyntax
	}
	if tail := strings.TrimSpace(s[start:]); tail != "" {
		parts = append(parts, tail)
	}
	return parts, nil
}

func pyValue(s string) (any, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 6 && (strings.HasPrefix(s, `"""`) && strings.HasSuffix(s, `"""`) || strings.HasPrefix(s, `'''`) && strings.HasSuffix(s, `'''`)) {
		return s[3 : len(s)-3], nil
	}
	return parseRelaxed(s)
}

func (s *scanner) parsePy(body string) ([]parsed, error) {
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "[") {
		if pyListEnd(body) != len(body) {
			return nil, errSyntax
		}
		body = body[1 : len(body)-1]
	}
	pieces, err := pyParts(body)
	if err != nil || len(pieces) == 0 {
		return nil, errSyntax
	}
	calls := make([]parsed, 0, len(pieces))
	for _, piece := range pieces {
		i := strings.IndexByte(piece, '(')
		if i < 0 || !strings.HasSuffix(piece, ")") {
			return nil, errSyntax
		}
		name := s.toolName(piece[:i])
		if !validName(name) {
			return nil, errSyntax
		}
		args, err := pyParts(piece[i+1 : len(piece)-1])
		if err != nil {
			return nil, err
		}
		sh := s.tools[name]
		var out object
		position := 0
		keyword := false
		for _, arg := range args {
			eq := pyAssignment(arg)
			key, raw := "", arg
			if eq >= 0 {
				key, raw = strings.TrimSpace(arg[:eq]), arg[eq+1:]
				if !validName(key) || strings.ContainsAny(key, ".:-") {
					return nil, errSyntax
				}
				keyword = true
			} else {
				if keyword || sh == nil || position >= len(sh.props) {
					return nil, errSyntax
				}
				key = sh.props[position].key
				position++
			}
			v, err := pyValue(raw)
			if err != nil {
				return nil, fmt.Errorf("%w: argument %s", errSyntax, key)
			}
			out.set(key, v)
		}
		calls = append(calls, parsed{name: name, args: out})
	}
	return calls, nil
}

func pyAssignment(s string) int {
	var quote byte
	var stack []byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0 && c == '\\':
			i++
		case quote != 0 && c == quote:
			quote = 0
		case quote != 0:
		case c == '"' || c == '\'':
			quote = c
		case c == '[' || c == '{' || c == '(':
			stack = append(stack, c)
		case c == ']' || c == '}' || c == ')':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case c == '=' && len(stack) == 0:
			if validName(strings.TrimSpace(s[:i])) {
				return i
			}
			return -1
		}
	}
	return -1
}
