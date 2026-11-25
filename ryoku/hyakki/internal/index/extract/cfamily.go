package extract

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
)

var (
	cAccess    = regexp.MustCompile(`\b(?:public|private|protected|signals|slots|Q_SIGNALS|Q_SLOTS)(?:\s+(?:slots|Q_SLOTS))?\s*:`)
	cMacroLine = regexp.MustCompile(`^[A-Z][A-Z0-9_]*(?:\s*\([^;{}]*\))?$`)
	cAttribute = regexp.MustCompile(`\[\[[^\]]*\]\]`)
	cNamespace = regexp.MustCompile(`^(?:inline\s+)?namespace\b`)
	cUsing     = regexp.MustCompile(`^using\s+([A-Za-z_]\w*)\s*=`)
	cTypedefFn = regexp.MustCompile(`\(\s*\*\s*([A-Za-z_]\w*)\s*\)\s*\(`)
	cTypedef   = regexp.MustCompile(`([A-Za-z_]\w*)\s*(?:\[[^\]]*\]\s*)*$`)
	cTail      = regexp.MustCompile(`^(?:\s*(?:const|volatile|noexcept(?:\s*\([^()]*\))?|throw\s*\([^()]*\)|override|final|mutable|&&|&|try|[A-Z][A-Z0-9_]*(?:\s*\([^()]*\))?|->\s*[\w:<>,*&\s]+?))*\s*$`)
	cDeleted   = regexp.MustCompile(`\s*=\s*(?:0|default|delete)\s*$`)
	cName      = regexp.MustCompile(`^(?:[A-Za-z_]\w*::)*(?:~?[A-Za-z_]\w*|operator.+)$`)
	cConstName = regexp.MustCompile(`\s([A-Za-z_]\w*)$`)
	cppHint    = regexp.MustCompile(`(?m)^\s*(?:class\s+\w|namespace\b|template\s*<|(?:public|private|protected)\s*:)|\bstd::`)
)

var cNotNames = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true, "sizeof": true,
	"catch": true, "do": true, "else": true, "case": true, "new": true, "delete": true,
	"throw": true, "static_assert": true, "typeid": true, "alignof": true, "decltype": true,
	"defined": true, "noexcept": true, "requires": true, "co_await": true, "co_return": true,
	"co_yield": true, "asm": true, "__asm__": true, "typeof": true, "__typeof__": true,
}

// cScope is one open brace: whether statements directly inside it are read,
// the class they belong to, and the symbol the closing brace ends
type cScope struct {
	collect bool
	class   string
	depth   int
	symbol  int
	typedef bool
	keep    bool
}

// looksCpp tells a C++ header from a C one by the constructs only C++ has
func looksCpp(source []byte) bool {
	return cppHint.Match(source)
}

// extractCFamily outlines C and C++ without a parser: comments, literals and
// preprocessor lines are blanked first so braces and semicolons can be
// trusted, then each statement is classified at the brace it opens or the
// semicolon that ends it
func extractCFamily(source []byte, header bool) Result {
	text := string(source)
	lines := strings.Split(text, "\n")
	clean := blankC(source)
	starts := []int{0}
	for i, c := range clean {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	lineOf := func(offset int) int {
		return sort.Search(len(starts), func(i int) bool { return starts[i] > offset })
	}
	var result Result
	for i, line := range lines {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "#include \"") {
			result.Imports = append(result.Imports, Import{Target: betweenQuotes(trimmed), Line: i + 1})
		}
	}
	add := func(start, end int, name, kind, parent, signature string, depth int) int {
		startLine := lineOf(start)
		if len([]rune(signature)) > 300 {
			signature = string([]rune(signature)[:300])
		}
		result.Symbols = append(result.Symbols, Symbol{Name: name, Kind: kind, Signature: signature, Start: startLine, End: max(lineOf(end), startLine), Parent: parent, Doc: cDoc(lines, startLine-1), Depth: depth})
		return len(result.Symbols) - 1
	}
	stack := []cScope{{collect: true, symbol: -1}}
	stmt, paren := -1, 0
	for pos := 0; pos < len(clean); pos++ {
		top := stack[len(stack)-1]
		switch c := clean[pos]; c {
		case '(':
			paren++
		case ')':
			paren = max(paren-1, 0)
		case '{':
			scope := cScope{symbol: -1, keep: paren > 0}
			if top.collect && stmt >= 0 && paren == 0 {
				scope = cOpen(clean, stmt, pos, top, add)
			}
			stack = append(stack, scope)
			if !scope.keep {
				stmt, paren = -1, 0
			}
		case '}':
			closed := top
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			if closed.symbol >= 0 {
				symbol := &result.Symbols[closed.symbol]
				symbol.End = max(lineOf(pos), symbol.Start)
				if closed.typedef {
					symbol.Name = cTypedefName(clean[pos+1:])
					if symbol.Name != "" {
						symbol.Signature += " " + symbol.Name
						if symbol.Parent != "" {
							symbol.Name = symbol.Parent + "::" + symbol.Name
						}
					}
				}
			}
			if !closed.keep {
				stmt, paren = -1, 0
			}
		case ';':
			if top.collect && stmt >= 0 {
				cDeclare(clean, stmt, pos, top, header, add)
			}
			stmt, paren = -1, 0
		case ' ', '\t', '\n', '\r', '\f', '\v':
		default:
			if stmt < 0 {
				stmt = pos
			}
		}
	}
	kept := result.Symbols[:0]
	for _, symbol := range result.Symbols {
		if symbol.Name != "" {
			kept = append(kept, symbol)
		}
	}
	result.Symbols = kept
	return result
}

type cAdd func(start, end int, name, kind, parent, signature string, depth int) int

// cHead returns a statement with access labels cut off the front, its
// offset in the source, and the same text with templates and attributes
// stripped and whitespace collapsed for classification
func cHead(clean []byte, from, to int) (int, string, string) {
	head := string(clean[from:to])
	if found := cAccess.FindAllStringIndex(head, -1); found != nil {
		last := found[len(found)-1]
		if last[1] >= len(head) || head[last[1]] != ':' {
			from += last[1]
			head = head[last[1]:]
		}
	}
	trimmed := strings.TrimLeft(head, " \t\r\n\f\v")
	from += len(head) - len(trimmed)
	signature := strings.NewReplacer("( ", "(", " )", ")").Replace(strings.Join(strings.Fields(trimmed), " "))
	norm := stripTemplate(signature)
	norm = cAttribute.ReplaceAllString(norm, " ")
	for _, word := range []string{"__attribute__", "__declspec", "alignas"} {
		norm = stripCall(norm, word)
	}
	return from, signature, strings.Join(strings.Fields(norm), " ")
}

// cOpen classifies the statement a brace opens and returns the scope it starts
func cOpen(clean []byte, stmt, pos int, top cScope, add cAdd) cScope {
	skip := cScope{symbol: -1}
	start, signature, norm := cHead(clean, stmt, pos)
	switch {
	case norm == "":
		return skip
	case cNamespace.MatchString(norm), norm == "extern":
		return cScope{collect: true, class: top.class, depth: top.depth, symbol: -1}
	}
	if keyword, name, typedef, ok := cRecord(norm); ok {
		qualified := name
		if qualified != "" && top.class != "" {
			qualified = top.class + "::" + name
		}
		if name == "" && !typedef {
			return skip
		}
		kind := keyword
		if name == "" {
			signature = "typedef " + keyword
		}
		index := add(start, pos, qualified, kind, top.class, signature, top.depth)
		if keyword == "enum" || name == "" {
			return cScope{symbol: index, typedef: name == "", class: top.class}
		}
		return cScope{collect: true, class: qualified, depth: top.depth + 1, symbol: index}
	}
	if name := cConstant(norm, top.class != ""); name != "" {
		symbolName, _, parent, depth := cMember(name, top)
		return cScope{symbol: add(start, pos, symbolName, "const", parent, signature, depth)}
	}
	prefix, name, params, tail, ok := cFunction(norm)
	if !ok {
		return skip
	}
	if strings.HasPrefix(strings.TrimSpace(tail), ":") && !strings.HasPrefix(strings.TrimSpace(tail), "::") {
		if last := norm[len(norm)-1]; last == '_' || last == '>' || isAlnum(last) {
			return cScope{symbol: -1, keep: true}
		}
		tail = ""
	}
	if !cTail.MatchString(tail) || !cCallable(prefix, name, top.class) {
		return skip
	}
	symbolName, kind, parent, depth := cMember(name, top)
	sig := strings.TrimSpace(prefix + " " + name + params + " " + strings.TrimSpace(tail))
	if cut := strings.Index(signature, name); cut >= 0 && !strings.ContainsAny(signature[:cut], "{") {
		sig = strings.TrimSpace(signature[:cut] + name + params + " " + strings.TrimSpace(tail))
	}
	return cScope{symbol: add(start, pos, symbolName, kind, parent, sig, depth)}
}

// cDeclare records what a semicolon ends: member and header prototypes,
// type aliases; fields, variables and calls are left out
func cDeclare(clean []byte, stmt, pos int, top cScope, header bool, add cAdd) {
	start, signature, norm := cHead(clean, stmt, pos)
	if norm == "" || strings.HasPrefix(norm, "friend ") {
		return
	}
	qualify := func(name string) string {
		if top.class != "" {
			return top.class + "::" + name
		}
		return name
	}
	if match := cUsing.FindStringSubmatch(norm); match != nil {
		add(start, pos, qualify(match[1]), "type", top.class, signature+";", top.depth)
		return
	}
	if strings.HasPrefix(norm, "typedef ") {
		name := ""
		if match := cTypedefFn.FindStringSubmatch(norm); match != nil {
			name = match[1]
		} else if match := cTypedef.FindStringSubmatch(norm); match != nil {
			name = match[1]
		}
		if name != "" {
			add(start, pos, qualify(name), "type", top.class, signature+";", top.depth)
		}
		return
	}
	if name := cConstant(norm, top.class != ""); name != "" {
		symbolName, _, parent, depth := cMember(name, top)
		add(start, pos, symbolName, "const", parent, signature+";", depth)
		return
	}
	if top.class == "" && !header {
		return
	}
	prefix, name, _, tail, ok := cFunction(norm)
	if !ok {
		return
	}
	tail = cDeleted.ReplaceAllString(tail, "")
	if !cTail.MatchString(tail) || !cCallable(prefix, name, top.class) {
		return
	}
	symbolName, kind, parent, depth := cMember(name, top)
	add(start, pos, symbolName, kind, parent, signature+";", depth)
}

// cConstant names a constant a declaration defines: a constexpr or const
// variable, which inside a class body must also be static
func cConstant(norm string, member bool) string {
	words := strings.Fields(norm)
	constant, static := false, false
	for _, word := range words {
		constant = constant || word == "constexpr" || word == "const"
		static = static || word == "static" || word == "constexpr"
	}
	if !constant || member && !static || strings.HasPrefix(norm, "typedef ") || strings.HasPrefix(norm, "using ") || strings.Contains(norm, "operator") {
		return ""
	}
	declarator := norm
	if cut := strings.IndexByte(declarator, '='); cut >= 0 {
		declarator = declarator[:cut]
	}
	declarator = strings.TrimSpace(declarator)
	for strings.HasSuffix(declarator, "]") {
		cut := strings.LastIndexByte(declarator, '[')
		if cut < 0 {
			return ""
		}
		declarator = strings.TrimSpace(declarator[:cut])
	}
	if strings.ContainsAny(declarator, "(){}") {
		return ""
	}
	match := cConstName.FindStringSubmatch(declarator)
	if match == nil || cNotNames[match[1]] || match[1] == "const" || match[1] == "constexpr" {
		return ""
	}
	return match[1]
}

// cMember names a function by where it is declared: inside a class body or
// qualified as Class::name it is that class's method
func cMember(name string, top cScope) (string, string, string, int) {
	if top.class != "" {
		return top.class + "::" + name, "method", top.class, top.depth
	}
	if cut := strings.LastIndex(name, "::"); cut > 0 {
		return name, "method", name[:cut], 1
	}
	return name, "function", "", 0
}

// cCallable rejects heads that only look like functions: a control keyword,
// a macro call with no return type, or an expression
func cCallable(prefix, name, class string) bool {
	last := name
	if cut := strings.LastIndex(last, "::"); cut >= 0 {
		last = last[cut+2:]
	}
	if cNotNames[last] || !cName.MatchString(name) {
		return false
	}
	if strings.ContainsAny(prefix, "=\"'.;{}") || strings.Contains(prefix, "->") {
		return false
	}
	for _, word := range strings.Fields(prefix) {
		if word == "return" || word == "throw" || word == "new" || word == "friend" || word == "typedef" {
			return false
		}
	}
	if strings.TrimSpace(prefix) != "" {
		return true
	}
	bare := strings.TrimPrefix(last, "~")
	owner := class
	if cut := strings.LastIndex(name, "::"); cut >= 0 {
		owner = name[:cut]
	}
	if cut := strings.LastIndex(owner, "::"); cut >= 0 {
		owner = owner[cut+2:]
	}
	return owner != "" && (bare == owner || strings.HasPrefix(last, "operator"))
}

// cRecord reads a class, struct, union or enum head; an anonymous one
// returns an empty name
func cRecord(norm string) (string, string, bool, bool) {
	if strings.ContainsAny(norm, "(=") {
		return "", "", false, false
	}
	typedef := strings.HasPrefix(norm, "typedef ")
	rest := strings.TrimPrefix(norm, "typedef ")
	keyword, rest, _ := strings.Cut(rest, " ")
	switch keyword {
	case "class", "struct", "union", "enum":
	default:
		return "", "", false, false
	}
	if keyword == "enum" {
		rest = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(rest, "class "), "struct "), " ")
	}
	for i := 0; i < len(rest); i++ {
		if rest[i] == ':' {
			if i+1 < len(rest) && rest[i+1] == ':' {
				i++
				continue
			}
			rest = rest[:i]
			break
		}
	}
	if cut := strings.IndexByte(rest, '<'); cut >= 0 {
		rest = rest[:cut]
	}
	fields := strings.Fields(rest)
	for len(fields) > 0 && fields[len(fields)-1] == "final" {
		fields = fields[:len(fields)-1]
	}
	if len(fields) == 0 {
		return keyword, "", typedef, true
	}
	name := fields[len(fields)-1]
	if !cName.MatchString(name) || strings.HasPrefix(name, "operator") {
		return "", "", false, false
	}
	return keyword, name, typedef, true
}

// cFunction finds the declarator of a function head: the text before the
// name, the qualified name, the parameter list and what follows it
func cFunction(head string) (string, string, string, string, bool) {
	angle, paren := 0, 0
	for i := 0; i < len(head); i++ {
		c := head[i]
		switch {
		case c == '_' || isAlpha(c):
			j := i
			for j < len(head) && (head[j] == '_' || isAlnum(head[j])) {
				j++
			}
			if head[i:j] == "operator" && angle == 0 && paren == 0 && (i == 0 || head[i-1] == ' ' || head[i-1] == ':' || head[i-1] == '*' || head[i-1] == '&') {
				k := j
				for k < len(head) && head[k] == ' ' {
					k++
				}
				if strings.HasPrefix(head[k:], "()") {
					k += 2
				}
				for k < len(head) && head[k] != '(' {
					k++
				}
				if k >= len(head) {
					return "", "", "", "", false
				}
				nameStart := cNameStart(head, i)
				name := cPlainName(strings.TrimSpace(head[nameStart:k]))
				end := cClose(head, k)
				if end < 0 {
					return "", "", "", "", false
				}
				return strings.TrimSpace(head[:nameStart]), name, head[k : end+1], head[end+1:], true
			}
			i = j - 1
		case c == '<':
			if angle > 0 || (i > 0 && (head[i-1] == '_' || isAlnum(head[i-1]) || head[i-1] == ' ')) {
				angle++
			}
		case c == '>':
			if angle > 0 {
				angle--
			}
		case c == '(' && angle == 0 && paren == 0:
			nameEnd := i
			for nameEnd > 0 && head[nameEnd-1] == ' ' {
				nameEnd--
			}
			nameStart := cNameStart(head, nameEnd)
			name := cPlainName(head[nameStart:nameEnd])
			end := cClose(head, i)
			if end < 0 {
				return "", "", "", "", false
			}
			switch name {
			case "decltype", "alignas", "__attribute__", "__declspec", "noexcept", "throw", "typeof", "__typeof__", "sizeof", "alignof":
				i = end
				continue
			}
			if name == "" {
				return "", "", "", "", false
			}
			return strings.TrimSpace(head[:nameStart]), name, head[i : end+1], head[end+1:], true
		case c == '(':
			paren++
		case c == ')':
			paren--
		}
	}
	return "", "", "", "", false
}

// cNameStart walks back from end over a qualified name, template arguments
// and a destructor tilde included
func cNameStart(head string, end int) int {
	i := end
	for i > 0 {
		c := head[i-1]
		switch {
		case c == '_' || isAlnum(c) || c == ':' || c == '~':
			i--
		case c == '>':
			depth := 0
			j := i - 1
			for ; j >= 0; j-- {
				if head[j] == '>' {
					depth++
				} else if head[j] == '<' {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			if j <= 0 {
				return i
			}
			i = j
		default:
			return i
		}
	}
	return i
}

// cPlainName drops template arguments from a qualified name
func cPlainName(name string) string {
	if strings.HasPrefix(name, "operator") || strings.Contains(name, "::operator") {
		return strings.Join(strings.Fields(name), " ")
	}
	var out strings.Builder
	depth := 0
	for i := range len(name) {
		switch c := name[i]; {
		case c == '<':
			depth++
		case c == '>':
			depth--
		case depth == 0 && c != ' ':
			out.WriteByte(c)
		}
	}
	return out.String()
}

// cClose finds the parenthesis closing the one at open, or -1
func cClose(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// cTypedefName reads the name an anonymous typedef gets after its closing brace
func cTypedefName(rest []byte) string {
	end := bytes.IndexByte(rest, ';')
	if end < 0 {
		return ""
	}
	fields := strings.FieldsFunc(string(rest[:end]), func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '*' })
	if len(fields) == 0 || !cName.MatchString(fields[0]) {
		return ""
	}
	return fields[0]
}

// stripTemplate removes leading template parameter lists
func stripTemplate(head string) string {
	for strings.HasPrefix(head, "template") {
		rest := strings.TrimLeft(head[len("template"):], " ")
		if !strings.HasPrefix(rest, "<") {
			return head
		}
		depth := 0
		end := -1
		for i := range len(rest) {
			if rest[i] == '<' {
				depth++
			} else if rest[i] == '>' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		if end < 0 {
			return head
		}
		head = strings.TrimLeft(rest[end+1:], " ")
	}
	return head
}

// stripCall removes word and the balanced argument list after it
func stripCall(text, word string) string {
	for {
		at := strings.Index(text, word)
		if at < 0 {
			return text
		}
		open := at + len(word)
		for open < len(text) && text[open] == ' ' {
			open++
		}
		if open >= len(text) || text[open] != '(' {
			return text
		}
		end := cClose(text, open)
		if end < 0 {
			return text
		}
		text = text[:at] + " " + text[end+1:]
	}
}

// cDoc reads the comment directly above a declaration
func cDoc(lines []string, index int) string {
	var doc []string
	i := index - 1
	if i >= 0 && strings.HasSuffix(strings.TrimSpace(lines[i]), "*/") {
		for ; i >= 0; i-- {
			text := strings.TrimSpace(lines[i])
			opens := strings.HasPrefix(text, "/*")
			text = strings.TrimSuffix(text, "*/")
			text = strings.TrimLeft(text, "/*!")
			doc = append([]string{strings.TrimSpace(text)}, doc...)
			if opens {
				break
			}
		}
		return strings.TrimSpace(strings.Join(doc, "\n"))
	}
	for ; i >= 0; i-- {
		text := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(text, "//") {
			break
		}
		doc = append([]string{strings.TrimSpace(strings.TrimLeft(text, "/!<"))}, doc...)
	}
	return strings.Join(doc, "\n")
}

// blankC returns source with comments, literals and preprocessor lines turned
// to spaces, every newline kept so offsets stay on their lines; a line that
// is only a macro call, such as Q_OBJECT, is blanked too since it ends no
// statement of its own
func blankC(source []byte) []byte {
	out := bytes.Clone(source)
	blank := func(from, to int) {
		for k := from; k < to && k < len(out); k++ {
			if out[k] != '\n' {
				out[k] = ' '
			}
		}
	}
	lineStart := true
	for i := 0; i < len(source); {
		c := source[i]
		switch c {
		case '\n':
			lineStart = true
			i++
			continue
		case ' ', '\t', '\r', '\f', '\v':
			i++
			continue
		}
		if lineStart && c == '#' {
			end := i
			for end < len(source) && source[end] != '\n' {
				if source[end] == '\\' && end+1 < len(source) && (source[end+1] == '\n' || (source[end+1] == '\r' && end+2 < len(source) && source[end+2] == '\n')) {
					end += 2
					continue
				}
				if source[end] == '/' && end+1 < len(source) && source[end+1] == '*' {
					if close := bytes.Index(source[end+2:], []byte("*/")); close >= 0 {
						end += close + 4
						continue
					}
					end = len(source)
					break
				}
				end++
			}
			blank(i, end)
			i = end
			continue
		}
		lineStart = false
		switch {
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			end := bytes.IndexByte(source[i:], '\n')
			if end < 0 {
				end = len(source) - i
			}
			blank(i, i+end)
			i += end
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			end := bytes.Index(source[i+2:], []byte("*/"))
			if end < 0 {
				blank(i, len(source))
				i = len(source)
				break
			}
			blank(i, i+end+4)
			i += end + 4
		case c == '"' && i > 0 && source[i-1] == 'R' && (i < 2 || !isAlnum(source[i-2]) && source[i-2] != '_' || bytes.HasSuffix(source[:i-1], []byte("u8")) || source[i-2] == 'u' || source[i-2] == 'U' || source[i-2] == 'L'):
			open := bytes.IndexByte(source[i:min(len(source), i+18)], '(')
			if open < 0 {
				end := cLiteralEnd(source, i, '"')
				blank(i, end)
				i = end
				break
			}
			marker := append(append([]byte(")"), source[i+1:i+open]...), '"')
			end := bytes.Index(source[i+open:], marker)
			if end < 0 {
				blank(i, len(source))
				i = len(source)
				break
			}
			stop := i + open + end + len(marker)
			blank(i, stop)
			i = stop
		case c == '"':
			end := cLiteralEnd(source, i, '"')
			blank(i, end)
			i = end
		case c == '\'' && !cDigitSeparator(source, i):
			end := cLiteralEnd(source, i, '\'')
			blank(i, end)
			i = end
		default:
			i++
		}
	}
	lineFrom := 0
	for i := range len(out) + 1 {
		if i == len(out) || out[i] == '\n' {
			if cMacroLine.Match(bytes.TrimSpace(out[lineFrom:i])) {
				blank(lineFrom, i)
			}
			lineFrom = i + 1
		}
	}
	return out
}

// cLiteralEnd returns the offset after the literal opened at start; an
// unterminated one ends at its line
func cLiteralEnd(source []byte, start int, quote byte) int {
	for j := start + 1; j < len(source); j++ {
		switch source[j] {
		case '\\':
			j++
		case quote:
			return j + 1
		case '\n':
			return j
		}
	}
	return len(source)
}

// cDigitSeparator tells a C++14 digit separator from a character literal
func cDigitSeparator(source []byte, at int) bool {
	if at+1 >= len(source) || !isAlnum(source[at+1]) {
		return false
	}
	start := at
	for start > 0 && (isAlnum(source[start-1]) || source[start-1] == '.') {
		start--
	}
	return start < at && source[start] >= '0' && source[start] <= '9'
}

func isAlpha(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isAlnum(c byte) bool {
	return isAlpha(c) || c >= '0' && c <= '9'
}
