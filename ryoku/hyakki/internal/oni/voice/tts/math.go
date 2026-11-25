package tts

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	displayMath = regexp.MustCompile(`(?s)\$\$(.+?)\$\$|\\\[(.+?)\\\]|\\\((.+?)\\\)`)
	inlineMath  = regexp.MustCompile(`\$([^\s$](?:[^$\n]*[^\s$])?)\$`)
)

// speakMathSpans replaces $$…$$, \[…\], \(…\) and $…$ with words. A $…$
// pair counts as math only when its closing $ is not followed by a digit,
// so "$5 or $6" stays money.
func speakMathSpans(text string) string {
	text = displayMath.ReplaceAllStringFunc(text, func(m string) string {
		sub := displayMath.FindStringSubmatch(m)
		for _, s := range sub[1:] {
			if s != "" {
				return " " + mathWords(s) + " "
			}
		}
		return " "
	})
	var b strings.Builder
	last := 0
	for _, m := range inlineMath.FindAllStringSubmatchIndex(text, -1) {
		if m[1] < len(text) && text[m[1]] >= '0' && text[m[1]] <= '9' {
			continue
		}
		if m[0] < last {
			continue
		}
		b.WriteString(text[last:m[0]])
		b.WriteString(mathWords(text[m[2]:m[3]]))
		last = m[1]
	}
	b.WriteString(text[last:])
	return b.String()
}

// mathNames are the LaTeX commands said as words.
var mathNames = map[string]string{
	"times": "times", "cdot": "times", "div": "divided by", "pm": "plus or minus", "mp": "minus or plus",
	"le": "is less than or equal to", "leq": "is less than or equal to",
	"ge": "is greater than or equal to", "geq": "is greater than or equal to",
	"ne": "is not equal to", "neq": "is not equal to", "approx": "is approximately",
	"equiv": "is equivalent to", "sim": "is similar to", "propto": "is proportional to",
	"infty": "infinity", "sum": "the sum of", "prod": "the product of", "int": "the integral of",
	"lim": "the limit of", "to": "to", "rightarrow": "to", "Rightarrow": "implies", "implies": "implies",
	"iff": "if and only if", "in": "in", "notin": "not in", "subset": "subset of", "cup": "union", "cap": "intersection",
	"forall": "for all", "exists": "there exists", "partial": "partial", "nabla": "del",
	"cdots": "and so on", "ldots": "and so on", "dots": "and so on", "circ": "degrees", "degree": "degrees",
	"log": "log", "ln": "natural log", "exp": "exp", "sin": "sine", "cos": "cosine", "tan": "tangent",
	"max": "max", "min": "min", "mod": "mod", "bmod": "mod", "neg": "not", "land": "and", "lor": "or",
	"alpha": "alpha", "beta": "beta", "gamma": "gamma", "Gamma": "gamma", "delta": "delta", "Delta": "delta",
	"epsilon": "epsilon", "varepsilon": "epsilon", "zeta": "zeta", "eta": "eta", "theta": "theta", "Theta": "theta",
	"iota": "iota", "kappa": "kappa", "lambda": "lambda", "Lambda": "lambda", "mu": "mu", "nu": "nu",
	"xi": "xi", "Xi": "xi", "pi": "pi", "Pi": "pi", "rho": "rho", "sigma": "sigma", "Sigma": "sigma",
	"tau": "tau", "upsilon": "upsilon", "phi": "phi", "varphi": "phi", "Phi": "phi", "chi": "chi",
	"psi": "psi", "Psi": "psi", "omega": "omega", "Omega": "omega",
}

// mathSilent are LaTeX commands that only shape the layout.
var mathSilent = map[string]bool{
	"left": true, "right": true, "displaystyle": true, "textstyle": true, "limits": true,
	"big": true, "Big": true, "bigg": true, "Bigg": true, "quad": true, "qquad": true,
	",": true, ";": true, ":": true, "!": true, " ": true, "\\": true,
}

// mathText are LaTeX commands whose argument is said as it is.
var mathText = map[string]bool{
	"text": true, "textrm": true, "textbf": true, "textit": true, "mathrm": true, "mathbf": true,
	"mathit": true, "mathsf": true, "mathcal": true, "mathbb": true, "operatorname": true,
	"boldsymbol": true, "vec": true, "hat": true, "bar": true, "overline": true,
}

var mathOps = map[byte]string{
	'=': "equals", '+': "plus", '-': "minus", '<': "is less than", '>': "is greater than",
	'/': "over", '*': "times", '!': "factorial",
}

// mathWords says a LaTeX expression in words.
func mathWords(s string) string {
	var b strings.Builder
	say := func(w string) { b.WriteString(" " + w + " ") }
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\\':
			name, n := command(s[i+1:])
			i += 1 + n
			switch {
			case name == "frac" || name == "dfrac" || name == "tfrac":
				var num, den string
				num, i = group(s, i)
				den, i = group(s, i)
				say(mathWords(num) + " over " + mathWords(den))
			case name == "sqrt":
				root := ""
				if j := skipSpace(s, i); j < len(s) && s[j] == '[' {
					if k := strings.IndexByte(s[j:], ']'); k > 0 {
						root, i = s[j+1:j+k], j+k+1
					}
				}
				var arg string
				arg, i = group(s, i)
				switch strings.TrimSpace(root) {
				case "":
					say("the square root of " + mathWords(arg))
				case "3":
					say("the cube root of " + mathWords(arg))
				default:
					say("the " + mathWords(root) + " root of " + mathWords(arg))
				}
			case mathText[name]:
				var arg string
				arg, i = group(s, i)
				say(strings.Map(func(r rune) rune {
					if r == '{' || r == '}' || r == '\\' {
						return ' '
					}
					return r
				}, arg))
			case mathSilent[name]:
				b.WriteByte(' ')
			case mathNames[name] != "":
				say(mathNames[name])
			default:
				say(name)
			}
		case c == '^':
			var arg string
			arg, i = group(s, i+1)
			switch strings.TrimSpace(arg) {
			case "2":
				say("squared")
			case "3":
				say("cubed")
			case `\circ`:
				say("degrees")
			default:
				say("to the power of " + mathWords(arg))
			}
		case c == '_':
			var arg string
			arg, i = group(s, i+1)
			say("sub " + mathWords(arg))
		case c == '{' || c == '}' || c == '&' || c == '|' || c == '~':
			b.WriteByte(' ')
			i++
		case mathOps[c] != "":
			say(mathOps[c])
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return strings.TrimSpace(spaces.ReplaceAllString(b.String(), " "))
}

// command reads a LaTeX command name after its backslash: a run of
// letters, or one other character.
func command(s string) (string, int) {
	n := 0
	for n < len(s) && s[n] < unicode.MaxASCII && unicode.IsLetter(rune(s[n])) {
		n++
	}
	if n == 0 && len(s) > 0 {
		return s[:1], 1
	}
	return s[:n], n
}

// group reads one argument at i: a braced group, a command, or a single
// character. It returns the argument and where reading stopped.
func group(s string, i int) (string, int) {
	i = skipSpace(s, i)
	if i >= len(s) {
		return "", i
	}
	switch s[i] {
	case '{':
		depth := 0
		for j := i; j < len(s); j++ {
			switch s[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return s[i+1 : j], j + 1
				}
			}
		}
		return s[i+1:], len(s)
	case '\\':
		_, n := command(s[i+1:])
		return s[i : i+1+n], i + 1 + n
	}
	_, n := utf8.DecodeRuneInString(s[i:])
	return s[i : i+n], i + n
}

func skipSpace(s string, i int) int {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}
