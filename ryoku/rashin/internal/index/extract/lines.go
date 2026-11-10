package extract

import (
	"regexp"
	"strings"
)

var (
	pythonDef       = regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pythonClass     = regexp.MustCompile(`^\s*class\s+([A-Za-z_]\w*)`)
	jsFunction      = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`)
	jsClass         = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?class\s+([A-Za-z_$][\w$]*)`)
	jsConst         = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)?\s*=>`)
	jsExport        = regexp.MustCompile(`^\s*export\s+(?:const|let|var)\s+([A-Za-z_$][\w$]*)`)
	jsMethod        = regexp.MustCompile(`^\s*(?:async\s+)?([A-Za-z_$][\w$]*)\s*\([^;]*\)\s*\{`)
	rustItem        = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(fn|struct|enum|trait|type|const|static|mod)\s+([A-Za-z_]\w*)`)
	cItem           = regexp.MustCompile(`^\s*(?:[A-Za-z_]\w*[\s*&]+)+([A-Za-z_]\w*)\s*\([^;]*\)\s*(?:const\s*)?\{`)
	luaItem         = regexp.MustCompile(`^\s*(?:local\s+)?function\s+([A-Za-z_][\w.:]*)`)
	shellItem       = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][\w-]*)\s*(?:\(\s*\))?\s*\{?\s*$`)
	fishItem        = regexp.MustCompile(`^\s*function\s+([A-Za-z_][\w-]*)`)
	qmlProperty     = regexp.MustCompile(`^\s*(?:readonly\s+|required\s+|default\s+)?property\s+(?:alias\s+|[A-Za-z_.<>]+\s+)([A-Za-z_]\w*)`)
	qmlFunction     = regexp.MustCompile(`^\s*function\s+([A-Za-z_]\w*)\s*\(`)
	qmlSignal       = regexp.MustCompile(`^\s*signal\s+([A-Za-z_]\w*)`)
	qmlID           = regexp.MustCompile(`^\s*id\s*:\s*([A-Za-z_]\w*)`)
	qmlRoot         = regexp.MustCompile(`^\s*([A-Z][A-Za-z0-9_.]*)\s*\{`)
	markdownHeading = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	cssSelector     = regexp.MustCompile(`^\s*([^@][^{]+)\s*\{`)
	topKey          = regexp.MustCompile(`^["']?([A-Za-z_][\w.-]*)["']?\s*(?:=|:)`)
	jsonKey         = regexp.MustCompile(`^\s*"([A-Za-z_][\w.-]*)"\s*:`)
	kdlNode         = regexp.MustCompile(`^\s*([A-Za-z_][\w-]*)\b`)
)

const (
	maxStructuredSymbols = 256
	smallStructuredBytes = 64 << 10
)

func extractLines(language string, source []byte) Result {
	lines := strings.Split(string(source), "\n")
	result := Result{Language: language}
	var className string
	var classIndent int
	jsonDepth := 0
	structuredSymbols := 0
	for i, line := range lines {
		lineNumber := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if className != "" && indentation(line) <= classIndent && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "#") {
			className = ""
		}
		switch language {
		case "python":
			if match := pythonClass.FindStringSubmatch(line); match != nil {
				className, classIndent = match[1], indentation(line)
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "class", "", indentationEnd(lines, i), 0))
			} else if match := pythonDef.FindStringSubmatch(line); match != nil {
				parent, name := "", match[1]
				if className != "" && indentation(line) > classIndent {
					parent = className
					name = parent + "." + name
				}
				depth := 0
				if parent != "" {
					depth = 1
				}
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, name, map[bool]string{true: "method", false: "function"}[parent != ""], parent, indentationEnd(lines, i), depth))
			}
			result.Imports = append(result.Imports, pythonImports(trimmed, lineNumber)...)
		case "javascript", "typescript":
			if match := jsClass.FindStringSubmatch(line); match != nil {
				className, classIndent = match[1], indentation(line)
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "class", "", braceEnd(lines, i), 0))
			} else if match := jsFunction.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "function", "", braceEnd(lines, i), 0))
			} else if match := jsConst.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "function", "", expressionEnd(lines, i), 0))
			} else if match := jsExport.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "const", "", expressionEnd(lines, i), 0))
			} else if className != "" && indentation(line) > classIndent {
				if match := jsMethod.FindStringSubmatch(line); match != nil && match[1] != "if" && match[1] != "for" && match[1] != "while" {
					result.Symbols = append(result.Symbols, lineSymbol(lines, i, className+"."+match[1], "method", className, braceEnd(lines, i), 1))
				}
			}
			result.Imports = append(result.Imports, jsImports(trimmed, lineNumber)...)
		case "rust":
			if match := rustItem.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[2], match[1], "", braceOrLineEnd(lines, i), 0))
			}
			if strings.HasPrefix(trimmed, "use ") {
				target := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(trimmed, "use ")), ";")
				if cut, _, ok := strings.Cut(target, "::"); ok {
					target = cut
				}
				result.Imports = append(result.Imports, Import{Target: target, Line: lineNumber})
			}
		case "c", "cpp":
			if match := cItem.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "function", "", braceEnd(lines, i), 0))
			}
			if strings.HasPrefix(trimmed, "#include \"") {
				result.Imports = append(result.Imports, Import{Target: betweenQuotes(trimmed), Line: lineNumber})
			}
		case "lua":
			if match := luaItem.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "function", "", keywordEnd(lines, i, "end"), 0))
			}
			if target := luaRequire(trimmed); target != "" {
				result.Imports = append(result.Imports, Import{Target: target, Line: lineNumber})
			}
		case "bash":
			if match := shellItem.FindStringSubmatch(line); match != nil && (strings.Contains(line, "function") || strings.Contains(line, "()")) {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "function", "", shellEnd(lines, i), 0))
			}
			if target := shellSource(trimmed); target != "" {
				result.Imports = append(result.Imports, Import{Target: target, Line: lineNumber})
			}
		case "fish":
			if match := fishItem.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "function", "", keywordEnd(lines, i, "end"), 0))
			}
			if target := shellSource(trimmed); target != "" {
				result.Imports = append(result.Imports, Import{Target: target, Line: lineNumber})
			}
		case "qml":
			if strings.HasPrefix(trimmed, "import ") {
				fields := strings.Fields(trimmed)
				if len(fields) > 1 {
					result.Imports = append(result.Imports, Import{Target: strings.Trim(fields[1], "\"'"), Line: lineNumber})
				}
			}
			if match := qmlRoot.FindStringSubmatch(line); match != nil && len(result.Symbols) == 0 {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "component", "", braceEnd(lines, i), 0))
			}
			if match := qmlID.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "id", "", lineNumber, 1))
			}
			if match := qmlProperty.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "property", "", lineNumber, 1))
			}
			if match := qmlFunction.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "function", "", braceEnd(lines, i), 1))
			}
			if match := qmlSignal.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "signal", "", lineNumber, 1))
			}
		case "markdown":
			if match := markdownHeading.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, strings.TrimSpace(match[2]), "heading", "", headingEnd(lines, i, len(match[1])), len(match[1])-1))
			}
		case "json":
			if structuredSymbols < maxStructuredSymbols && (jsonDepth == 1 || (len(source) <= smallStructuredBytes && jsonDepth > 1)) {
				if match := jsonKey.FindStringSubmatch(line); match != nil {
					result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "key", "", lineNumber, max(0, jsonDepth-1)))
					structuredSymbols++
				}
			}
		case "yaml", "toml", "ini":
			if structuredSymbols < maxStructuredSymbols && indentation(line) == 0 {
				if match := topKey.FindStringSubmatch(line); match != nil {
					result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "key", "", lineNumber, 0))
					structuredSymbols++
				}
			}
		case "css":
			if strings.HasPrefix(trimmed, "@import ") {
				if target := betweenQuotes(trimmed); target != "" {
					result.Imports = append(result.Imports, Import{Target: target, Line: lineNumber})
				}
			} else if match := cssSelector.FindStringSubmatch(line); match != nil {
				result.Symbols = append(result.Symbols, lineSymbol(lines, i, strings.TrimSpace(match[1]), "selector", "", braceEnd(lines, i), 0))
			}
		case "kdl":
			if indentation(line) == 0 {
				if match := kdlNode.FindStringSubmatch(line); match != nil {
					result.Symbols = append(result.Symbols, lineSymbol(lines, i, match[1], "node", "", braceOrLineEnd(lines, i), 0))
					if match[1] == "include" || match[1] == "import" {
						if target := betweenQuotes(trimmed); target != "" {
							result.Imports = append(result.Imports, Import{Target: target, Line: lineNumber})
						}
					}
				}
			}
		}
		if language == "json" {
			jsonDepth += jsonBraceDelta(line)
			if jsonDepth < 0 {
				jsonDepth = 0
			}
		}
	}
	return result
}

func jsonBraceDelta(line string) int {
	delta := 0
	quoted := false
	escaped := false
	for _, char := range line {
		if quoted {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				quoted = false
			}
			continue
		}
		if char == '"' {
			quoted = true
		} else if char == '{' || char == '[' {
			delta++
		} else if char == '}' || char == ']' {
			delta--
		}
	}
	return delta
}

func lineSymbol(lines []string, index int, name, kind, parent string, end, depth int) Symbol {
	start := index + 1
	if end < start {
		end = start
	}
	return Symbol{Name: name, Kind: kind, Signature: strings.TrimSpace(lines[index]), Start: start, End: end, Parent: parent, Doc: leadingDoc(lines, index), Depth: depth}
}
func leadingDoc(lines []string, index int) string {
	var doc []string
	for i := index - 1; i >= 0; i-- {
		text := strings.TrimSpace(lines[i])
		if strings.HasPrefix(text, "//") {
			doc = append([]string{strings.TrimSpace(strings.TrimPrefix(text, "//"))}, doc...)
			continue
		}
		if strings.HasPrefix(text, "#") {
			doc = append([]string{strings.TrimSpace(strings.TrimPrefix(text, "#"))}, doc...)
			continue
		}
		break
	}
	return strings.Join(doc, "\n")
}
func indentation(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' {
			n++
		} else if r == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}
func indentationEnd(lines []string, index int) int {
	indent := indentation(lines[index])
	end := index + 1
	for i := index + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentation(lines[i]) <= indent {
			break
		}
		end = i + 1
	}
	return end
}
func braceEnd(lines []string, index int) int {
	depth := 0
	seen := false
	for i := index; i < len(lines); i++ {
		for _, r := range lines[i] {
			if r == '{' {
				depth++
				seen = true
			}
			if r == '}' {
				depth--
				if seen && depth <= 0 {
					return i + 1
				}
			}
		}
	}
	return index + 1
}
func braceOrLineEnd(lines []string, index int) int {
	if strings.Contains(lines[index], "{") {
		return braceEnd(lines, index)
	}
	return index + 1
}
func expressionEnd(lines []string, index int) int {
	if strings.Contains(lines[index], "{") {
		return braceEnd(lines, index)
	}
	for i := index; i < len(lines); i++ {
		if strings.Contains(lines[i], ";") {
			return i + 1
		}
	}
	return index + 1
}
func keywordEnd(lines []string, index int, keyword string) int {
	for i := index + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == keyword {
			return i + 1
		}
	}
	return index + 1
}
func shellEnd(lines []string, index int) int {
	if strings.Contains(lines[index], "{") {
		return braceEnd(lines, index)
	}
	return keywordEnd(lines, index, "end")
}
func headingEnd(lines []string, index, level int) int {
	for i := index + 1; i < len(lines); i++ {
		if m := markdownHeading.FindStringSubmatch(lines[i]); m != nil && len(m[1]) <= level {
			return i
		}
	}
	return len(lines)
}
func betweenQuotes(text string) string {
	start := strings.IndexAny(text, "\"'")
	if start < 0 {
		return ""
	}
	quote := text[start]
	end := strings.IndexByte(text[start+1:], quote)
	if end < 0 {
		return ""
	}
	return text[start+1 : start+1+end]
}
func pythonImports(text string, line int) []Import {
	var out []Import
	if strings.HasPrefix(text, "import ") {
		for _, v := range strings.Split(strings.TrimPrefix(text, "import "), ",") {
			fields := strings.Fields(strings.TrimSpace(v))
			if len(fields) == 0 {
				continue
			}
			out = append(out, Import{Target: fields[0], Line: line})
		}
	} else if strings.HasPrefix(text, "from ") {
		fields := strings.Fields(text)
		if len(fields) > 1 {
			out = append(out, Import{Target: fields[1], Line: line})
		}
	}
	return out
}
func jsImports(text string, line int) []Import {
	if !strings.HasPrefix(text, "import ") && !strings.Contains(text, "require(") && !strings.HasPrefix(text, "export ") {
		return nil
	}
	target := betweenQuotes(text)
	if target == "" {
		return nil
	}
	return []Import{{Target: target, Line: line}}
}
func luaRequire(text string) string {
	if !strings.Contains(text, "require") {
		return ""
	}
	return betweenQuotes(text)
}
func shellSource(text string) string {
	fields := strings.Fields(text)
	if len(fields) < 2 || (fields[0] != "source" && fields[0] != ".") {
		return ""
	}
	return strings.Trim(fields[1], "\"'")
}
