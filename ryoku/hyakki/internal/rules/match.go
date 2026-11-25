package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dlclark/regexp2"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// Kinds of output a rule watches: Scope entries.
const (
	SourceText     = "text"
	SourceThinking = "thinking"
	SourceTool     = "tool"
)

// MatchTimeout bounds one regular expression run, so a pattern that
// backtracks badly costs a stream at most this much.
const MatchTimeout = 100 * time.Millisecond

// excerptRunes is how much text around a match the model and the user see.
const excerptRunes = 80

// source is one checked output: a reply's text, its reasoning, or one tool
// call (per file for the file tools, whose added text is what is checked).
type source struct {
	Kind string
	Tool string
	Path string
	Text string
}

// subject names a source for the model.
func (s source) subject() string {
	switch s.Kind {
	case SourceThinking:
		return "reasoning"
	case SourceTool:
		return s.Tool + " call"
	}
	return "reply"
}

// scopeEntry is one entry of a rule's Scope.
type scopeEntry struct {
	kind  string
	tool  string
	globs []*regexp.Regexp
}

// scope is a parsed Scope; empty is text and every tool.
type scope []scopeEntry

// parseScope reads "text, thinking, tool, tool:edit(*.go)". Commas inside
// parentheses and braces belong to the glob.
func parseScope(text string) (scope, error) {
	var out scope
	for _, part := range splitTop(text) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch {
		case part == SourceText || part == SourceThinking || part == SourceTool:
			out = append(out, scopeEntry{kind: part})
		case strings.HasPrefix(part, "tool:"):
			spec := part[len("tool:"):]
			e := scopeEntry{kind: SourceTool}
			if open := strings.IndexByte(spec, '('); open >= 0 {
				if !strings.HasSuffix(spec, ")") {
					return nil, fmt.Errorf("scope %q: missing )", part)
				}
				glob := spec[open+1 : len(spec)-1]
				spec = spec[:open]
				g, err := compileGlob(glob)
				if err != nil {
					return nil, fmt.Errorf("scope %q: %w", part, err)
				}
				e.globs = g
			}
			e.tool = canonicalTool(strings.TrimSpace(spec))
			if e.tool == "" {
				return nil, fmt.Errorf("scope %q: no tool name", part)
			}
			out = append(out, e)
		default:
			return nil, fmt.Errorf("scope %q: use text, thinking, tool or tool:<name>(<glob>)", part)
		}
	}
	return out, nil
}

// splitTop splits on commas outside parentheses and braces.
func splitTop(text string) []string {
	var parts []string
	depth, start := 0, 0
	for i, c := range text {
		switch c {
		case '(', '{':
			depth++
		case ')', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, text[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, text[start:])
}

func (sc scope) allows(src source) bool {
	if len(sc) == 0 {
		return src.Kind == SourceText || src.Kind == SourceTool
	}
	for _, e := range sc {
		if e.kind != src.Kind {
			continue
		}
		if e.kind != SourceTool || e.tool == "" {
			return true
		}
		if e.tool != canonicalTool(src.Tool) {
			continue
		}
		if len(e.globs) == 0 || matchGlobs(e.globs, src.Path) {
			return true
		}
	}
	return false
}

// broad reports a scope that takes prose or every tool, which the forge
// narrows when a rule only ever matched file edits.
func (sc scope) broad() bool {
	if len(sc) == 0 {
		return true
	}
	for _, e := range sc {
		if e.kind == SourceText || (e.kind == SourceTool && e.tool == "") {
			return true
		}
	}
	return false
}

// canonicalTool maps Ask's tool names to the agent tools they match, so a
// rule scoped to write also watches write_file.
func canonicalTool(name string) string {
	switch name {
	case tool.WriteFile:
		return tool.Write
	case tool.RunCommand:
		return tool.Bash
	case tool.ReadFile:
		return tool.Read
	case tool.ListDir:
		return tool.Ls
	}
	return name
}

// compileGlob turns a file glob into matchers: * stays within a folder, **
// crosses folders, {a,b} picks one, and a glob without a slash matches the
// file name alone.
func compileGlob(glob string) ([]*regexp.Regexp, error) {
	glob = strings.TrimSpace(glob)
	if glob == "" {
		return nil, fmt.Errorf("empty glob")
	}
	var out []*regexp.Regexp
	for _, g := range expandBraces(glob) {
		var b strings.Builder
		b.WriteString("^")
		if !strings.Contains(g, "/") {
			b.WriteString("(?:.*/)?")
		}
		for i := 0; i < len(g); i++ {
			c := g[i]
			switch {
			case c == '*' && i+1 < len(g) && g[i+1] == '*':
				i++
				if i+1 < len(g) && g[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			case c == '*':
				b.WriteString("[^/]*")
			case c == '?':
				b.WriteString("[^/]")
			case c == '[':
				end := strings.IndexByte(g[i:], ']')
				if end < 0 {
					b.WriteString(`\[`)
					continue
				}
				class := g[i+1 : i+end]
				if strings.HasPrefix(class, "!") {
					class = "^" + class[1:]
				}
				b.WriteString("[" + class + "]")
				i += end
			default:
				b.WriteString(regexp.QuoteMeta(string(c)))
			}
		}
		b.WriteString("$")
		re, err := regexp.Compile(b.String())
		if err != nil {
			return nil, fmt.Errorf("bad glob %q", glob)
		}
		out = append(out, re)
	}
	return out, nil
}

// expandBraces expands the first {a,b} group, recursively.
func expandBraces(glob string) []string {
	open := strings.IndexByte(glob, '{')
	if open < 0 {
		return []string{glob}
	}
	depth := 0
	for i := open; i < len(glob); i++ {
		switch glob[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				var out []string
				for _, alt := range splitTop(glob[open+1 : i]) {
					out = append(out, expandBraces(glob[:open]+alt+glob[i+1:])...)
				}
				return out
			}
		}
	}
	return []string{glob}
}

func matchGlobs(globs []*regexp.Regexp, path string) bool {
	if path == "" {
		return false
	}
	path = strings.ReplaceAll(path, "\\", "/")
	for _, g := range globs {
		if g.MatchString(path) {
			return true
		}
	}
	return false
}

// condition is one compiled Condition.
type condition struct {
	re *regexp2.Regexp
	// multiline patterns can match across lines, so a stream re-reads more
	// than the line it grew on.
	multiline bool
}

// compileCondition compiles a JavaScript-flavoured pattern with lookaround.
func compileCondition(expr string) (condition, error) {
	re, err := regexp2.Compile(expr, regexp2.RE2)
	if err != nil {
		return condition{}, err
	}
	re.MatchTimeout = MatchTimeout
	multi := strings.Contains(expr, "(?s)") || strings.ContainsRune(expr, '\n') ||
		strings.Contains(expr, `\n`) || strings.Contains(expr, `\r`) ||
		strings.Contains(expr, `\s`) || strings.Contains(expr, `\W`) ||
		strings.Contains(expr, `\D`) || strings.Contains(expr, `[^`)
	return condition{re: re, multiline: multi}, nil
}

// find runs the condition on text and returns the excerpt around the first
// match. A timed out run counts as no match.
func (c condition) find(text string) (string, bool) {
	m, err := c.re.FindStringMatch(text)
	if err != nil || m == nil {
		return "", false
	}
	runes := []rune(text)
	start := max(0, m.Index-excerptRunes/2)
	end := min(len(runes), m.Index+m.Length+excerptRunes/2)
	excerpt := strings.TrimSpace(string(runes[start:end]))
	if m.Length > excerptRunes*2 {
		excerpt = strings.TrimSpace(m.String())
		if r := []rune(excerpt); len(r) > excerptRunes*2 {
			excerpt = string(r[:excerptRunes*2]) + "..."
		}
	}
	return excerpt, true
}

// compiled is a rule ready to match.
type compiled struct {
	rule       protocol.Rule
	builtin    bool
	conditions []condition
	scope      scope
	globs      []*regexp.Regexp
}

// compile checks a rule and prepares it. A rule that does not compile
// carries the reason in Rule.Error and never matches.
func compile(r protocol.Rule, builtin bool) *compiled {
	c := &compiled{rule: r, builtin: builtin}
	if err := c.prepare(); err != nil {
		c.rule.Error = err.Error()
		c.conditions = nil
	}
	return c
}

func (c *compiled) prepare() error {
	r := c.rule
	if r.Interrupt != "" && !oneOf(r.Interrupt, protocol.RuleInterruptNever, protocol.RuleInterruptProseOnly, protocol.RuleInterruptToolOnly, protocol.RuleInterruptAlways) {
		return fmt.Errorf("interrupt must be never, prose-only, tool-only or always, not %q", r.Interrupt)
	}
	if r.Repeat != "" && !oneOf(r.Repeat, protocol.RuleRepeatOnce, protocol.RuleRepeatAfterGap, protocol.RuleRepeatAlways) {
		return fmt.Errorf("repeat must be once, after-gap or always, not %q", r.Repeat)
	}
	sc, err := parseScope(r.Scope)
	if err != nil {
		return err
	}
	c.scope = sc
	for _, g := range r.Globs {
		m, err := compileGlob(g)
		if err != nil {
			return err
		}
		c.globs = append(c.globs, m...)
	}
	for _, expr := range r.Condition {
		cond, err := compileCondition(expr)
		if err != nil {
			return fmt.Errorf("condition %q: %v", expr, err)
		}
		c.conditions = append(c.conditions, cond)
	}
	return nil
}

func oneOf(v string, options ...string) bool {
	for _, o := range options {
		if v == o {
			return true
		}
	}
	return false
}

// streams reports a rule that matches while a reply streams.
func (c *compiled) streams() bool {
	return c.rule.Enabled && c.rule.Error == "" && len(c.conditions) > 0 && strings.TrimSpace(c.rule.Question) == ""
}

// judged reports a rule a judge checks after the turn.
func (c *compiled) judged() bool {
	return c.rule.Enabled && c.rule.Error == "" && strings.TrimSpace(c.rule.Question) != ""
}

// applies reports whether src is in the rule's scope and, for a tool call
// when the rule has Globs, touches a matching file.
func (c *compiled) applies(src source) bool {
	if !c.scope.allows(src) {
		return false
	}
	if src.Kind == SourceTool && len(c.globs) > 0 {
		return matchGlobs(c.globs, src.Path)
	}
	return true
}

// find runs every condition over text.
func (c *compiled) find(text string) (string, bool) {
	for _, cond := range c.conditions {
		if excerpt, ok := cond.find(text); ok {
			return excerpt, true
		}
	}
	return "", false
}

// A condition that crosses lines still sees the whole bounded stream.
func (c *compiled) findStream(text string, start int) (string, bool) {
	for _, cond := range c.conditions {
		candidate := text
		if !cond.multiline && start < len(text) {
			candidate = text[start:]
		}
		if excerpt, ok := cond.find(candidate); ok {
			return excerpt, true
		}
	}
	return "", false
}

// interrupts reports whether a match in src stops the reply.
func (c *compiled) interrupts(src source) bool {
	switch c.rule.Interrupt {
	case protocol.RuleInterruptNever:
		return false
	case protocol.RuleInterruptProseOnly:
		return src.Kind != SourceTool
	case protocol.RuleInterruptToolOnly:
		return src.Kind == SourceTool
	}
	return true
}

// fileTools write files; their calls are checked as the text they add.
var fileTools = map[string]bool{tool.Write: true, tool.WriteFile: true, tool.Edit: true}

var (
	pathKeys    = []string{"path", "file_path", "file", "filename", "filePath"}
	contentKeys = []string{"content", "text", "data", "new", "new_string", "new_text", "newText", "replace", "replacement", "with"}
	patchKeys   = []string{"input", "patch", "diff", "edits", "ops"}
)

// hashlineHeader opens one file's section of an anchored edit: [PATH#TAG].
var hashlineHeader = regexp.MustCompile(`^\[(.+)#[0-9A-Fa-f]{4}\]\s*$`)

// callSources turns one tool call into what rules check: the text a file
// tool adds, per file, or the other tools' arguments as "key: value" lines.
func callSources(name, args string) []source {
	var v map[string]any
	if json.Unmarshal([]byte(args), &v) != nil {
		return []source{{Kind: SourceTool, Tool: name, Text: args}}
	}
	path := firstString(v, pathKeys)
	if !fileTools[name] {
		return []source{{Kind: SourceTool, Tool: name, Path: path, Text: renderArgs(v)}}
	}
	var out []source
	var added []string
	for _, k := range contentKeys {
		if s, ok := v[k].(string); ok && s != "" {
			added = append(added, s)
		}
	}
	for _, k := range patchKeys {
		switch p := v[k].(type) {
		case string:
			files, rest := patchSources(name, path, p)
			out = append(out, files...)
			if rest != "" {
				added = append(added, rest)
			}
		case []any:
			for _, item := range p {
				if m, ok := item.(map[string]any); ok {
					for _, ck := range contentKeys {
						if s, ok := m[ck].(string); ok && s != "" {
							added = append(added, s)
						}
					}
				}
			}
		}
	}
	if len(added) > 0 {
		out = append([]source{{Kind: SourceTool, Tool: name, Path: path, Text: strings.Join(added, "\n")}}, out...)
	}
	if len(out) == 0 {
		out = []source{{Kind: SourceTool, Tool: name, Path: path, Text: renderArgs(v)}}
	}
	return out
}

// patchSources reads an anchored edit: [PATH#TAG] headers open a file and
// "+" lines are what it adds. Text without headers or "+" lines is returned
// whole as rest.
func patchSources(name, path, patch string) ([]source, string) {
	var out []source
	cur := source{Kind: SourceTool, Tool: name, Path: path}
	var lines []string
	headers, plus := false, false
	flush := func() {
		if len(lines) > 0 {
			cur.Text = strings.Join(lines, "\n")
			out = append(out, cur)
		}
		lines = nil
	}
	for _, line := range strings.Split(patch, "\n") {
		if m := hashlineHeader.FindStringSubmatch(line); m != nil {
			flush()
			headers = true
			cur = source{Kind: SourceTool, Tool: name, Path: m[1]}
			continue
		}
		if strings.HasPrefix(line, "+") {
			plus = true
			lines = append(lines, line[1:])
		}
	}
	flush()
	if !headers && !plus {
		return nil, patch
	}
	return out, ""
}

func firstString(v map[string]any, keys []string) string {
	for _, k := range keys {
		if s, ok := v[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// renderArgs writes arguments as sorted "key: value" lines: strings as they
// are, so a pattern sees a command as typed, other values as JSON.
func renderArgs(v map[string]any) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(": ")
		if s, ok := v[k].(string); ok {
			b.WriteString(s)
		} else {
			raw, _ := json.Marshal(v[k])
			b.Write(raw)
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
