package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	i18n "ryoku-i18n"
)

// plugin_audit.go is the static security audit `ryoku plugin validate` and
// `ryoku plugin add` run over a plugin tree, on top of the manifest checks in
// plugin.go. It reads bytes only; it never executes anything from the plugin.
// Each finding carries a stable rule id, the file, the line, and one sentence.
// Blocking findings fail validation (exit 1) unless downgraded with --allow;
// warnings are printed and never fail. The rules mirror R1..R11 in docs and in
// the scaffold's AGENTS.md.

// Finding is one audit result. Line is 1-based, or 0 for a whole-file finding
// (symlink, binary, large-tree).
type Finding struct {
	Rule    string `json:"rule"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// auditResult groups findings by severity. --json marshals this shape directly.
type auditResult struct {
	Blocking []Finding `json:"blocking"`
	Warnings []Finding `json:"warnings"`
}

// blockingRules is the set that fails validation. Everything else is a warning.
var blockingRules = map[string]bool{
	"symlink":         true,
	"escalation":      true,
	"pipe-shell":      true,
	"internal-import": true,
	"config-write":    true,
	"secret":          true,
	"binary":          true,
}

// commandAllowlist is the set of external programs a plugin may run without
// declaring them in dependencies.commands (contract 2). Everything else the
// plugin runs must be in its own bin/ or in dependencies.commands.
var commandAllowlist = map[string]bool{
	"ryoku": true, "ryoku-shell": true, "ryoku-plugins-place": true,
	"ryostore": true, "notify-send": true, "xdg-open": true,
	"wl-copy": true, "wl-paste": true, "sh": true, "bash": true, "jq": true,
	"cat": true, "grep": true, "sed": true, "awk": true, "head": true,
	"tail": true, "sleep": true, "date": true, "nmcli": true, "pactl": true,
	"playerctl": true, "brightnessctl": true, "pkexec": true,
}

var (
	escalationRe   = regexp.MustCompile(`\b(sudo|doas|su)\b`)
	pipeShellRe    = regexp.MustCompile(`(?:curl|wget)[^|\n]*\|\s*(?:sh|bash|zsh)|eval\s+"\$\(curl`)
	importShellRe  = regexp.MustCompile(`^\s*import\s+shell\.`)
	importUiRe     = regexp.MustCompile(`^\s*import\s+Ryoku\.Ui`)
	importRelRe    = regexp.MustCompile(`^\s*import\s+"(\.\./[^"]*)"`)
	secretRe       = regexp.MustCompile(`(?:sk|ghp|gho|xox[abp])-[A-Za-z0-9_-]{16,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY`)
	cmdArrayRe     = regexp.MustCompile(`command\s*:\s*\[\s*([^,\]\n]+)`)
	execDetachedRe = regexp.MustCompile(`execDetached\s*\(\s*\[\s*([^,\]\n]+)`)
	undeclaredHost = regexp.MustCompile(`https?://([^/"'\s]+)`)
	shcArrayRe     = regexp.MustCompile(`["'](?:sh|bash)["']\s*,\s*["']-c["']\s*,\s*([^\]\)]+)`)
	shcInlineRe    = regexp.MustCompile(`\b(?:sh|bash)\s+-c\b(.*)`)
	outsideWriteRe = regexp.MustCompile(`~/\.config/|\$HOME/\.`)
	cmdPunctRe     = regexp.MustCompile(`["'\[\](),+]`)
)

// auditManifest is the manifest data the audit reasons about: what the plugin
// declared it needs. Parsed tolerantly, so the audit still runs when the
// manifest is malformed (the manifest checks report that separately).
type auditManifest struct {
	Privileged []string
	Network    []string
	Commands   []string
}

func readAuditManifest(dir string) auditManifest {
	var am auditManifest
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return am
	}
	var raw struct {
		Dependencies struct {
			Commands []string `json:"commands"`
		} `json:"dependencies"`
		Capabilities struct {
			Network    []string `json:"network"`
			Privileged []string `json:"privileged"`
		} `json:"capabilities"`
	}
	if json.Unmarshal(b, &raw) != nil {
		return am
	}
	am.Commands = raw.Dependencies.Commands
	am.Network = raw.Capabilities.Network
	am.Privileged = raw.Capabilities.Privileged
	return am
}

// auditPlugin runs the static audit over the plugin tree at dir, returning the
// findings split into blocking and warnings. allow downgrades the named blocking
// rules to warnings for this run. It never executes plugin code.
func auditPlugin(dir string, allow map[string]bool) auditResult {
	am := readAuditManifest(dir)
	var findings []Finding
	add := func(rule, p string, line int, msg string) {
		findings = append(findings, Finding{Rule: rule, Path: p, Line: line, Message: msg})
	}

	var total int64
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			add("symlink", relSlash, 0, i18n.T("symlinks are not allowed in a plugin (R2)"))
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			total += info.Size()
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".so" || ext == ".dylib" || ext == ".exe" {
			add("binary", relSlash, 0, i18n.T("a compiled binary; a plugin ships scripts only (R10)"))
			return nil
		}
		content, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		if hasBinaryMagic(content) {
			add("binary", relSlash, 0, i18n.T("a compiled binary; a plugin ships scripts only (R10)"))
			return nil
		}
		auditFile(relSlash, content, am, add)
		return nil
	})
	if total > 20*1024*1024 {
		add("large-tree", ".", 0, fmt.Sprintf(i18n.T("the plugin tree is %d MB; keep it under 20 MB"), total/(1024*1024)))
	}

	res := auditResult{Blocking: []Finding{}, Warnings: []Finding{}}
	for _, f := range findings {
		if blockingRules[f.Rule] && !allow[f.Rule] {
			res.Blocking = append(res.Blocking, f)
		} else {
			res.Warnings = append(res.Warnings, f)
		}
	}
	sortFindings(res.Blocking)
	sortFindings(res.Warnings)
	return res
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Path != fs[j].Path {
			return fs[i].Path < fs[j].Path
		}
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Rule < fs[j].Rule
	})
}

// auditFile applies the per-file rules to one file's content. class is the file
// class (qml/js/sh/json/md/other); code-pattern rules run only on qml/js/sh,
// over the comment-stripped text, while secret scanning runs over every text
// file's raw lines (a secret is a secret even in a comment).
func auditFile(p string, content []byte, am auditManifest, add func(rule, p string, line int, msg string)) {
	class := classify(p, content)
	rawLines := strings.Split(string(content), "\n")

	// secret: every text file, raw.
	for i, ln := range rawLines {
		if secretRe.MatchString(ln) {
			add("secret", p, i+1, i18n.T("looks like a hardcoded secret or key (R10)"))
		}
	}

	if class != "qml" && class != "js" && class != "sh" {
		return
	}
	// A file with NUL bytes is not source we can reason about line by line.
	if bytes.IndexByte(content, 0) >= 0 {
		return
	}

	codeLines := codeText(rawLines, class)
	qmlLike := class == "qml" || class == "js"
	for i, code := range codeLines {
		line := i + 1

		if m := escalationRe.FindString(code); m != "" {
			add("escalation", p, line, fmt.Sprintf(i18n.T("uses %q; a plugin must never escalate privilege (R6)"), m))
		}
		if idx := strings.Index(code, "pkexec"); idx >= 0 && !pkexecAllowed(code[idx:], am.Privileged) {
			add("escalation", p, line, i18n.T("runs pkexec without an exact match in capabilities.privileged (R6)"))
		}
		if pipeShellRe.MatchString(code) {
			add("pipe-shell", p, line, i18n.T("downloads and pipes code into a shell (R7)"))
		}

		if qmlLike {
			switch {
			case importShellRe.MatchString(code):
				add("internal-import", p, line, i18n.T("imports shell internals; only Ryoku.PluginKit is allowed (R4)"))
			case importUiRe.MatchString(code):
				add("internal-import", p, line, i18n.T("imports Ryoku.Ui internals; only Ryoku.PluginKit is allowed (R4)"))
			default:
				if mm := importRelRe.FindStringSubmatch(code); mm != nil && importEscapes(p, mm[1]) {
					add("internal-import", p, line, i18n.T("a relative import climbs out of the plugin folder (R4)"))
				}
			}
		}

		auditConfigWrite(p, line, code, add)

		if qmlLike {
			for _, name := range extractCommands(code) {
				if !commandAllowed(name, am.Commands) {
					add("undeclared-command", p, line, fmt.Sprintf(i18n.T("runs %q, not in bin/, dependencies.commands, or the allowlist"), name))
				}
			}
		}
		for _, hm := range undeclaredHost.FindAllStringSubmatch(code, -1) {
			h := stripPort(hm[1])
			if h != "" && !hostAllowed(h, am.Network) {
				add("undeclared-host", p, line, fmt.Sprintf(i18n.T("contacts %q, not listed in capabilities.network"), h))
			}
		}
		if isDynamicShell(code) {
			add("dynamic-shell", p, line, i18n.T("sh/bash -c with a built-up argument invites injection (R9)"))
		}
		if outsideWriteRe.MatchString(code) {
			add("outside-write", p, line, i18n.T("writes outside the plugin state dir (R8)"))
		}
	}
}

// classify maps a file to its class by extension, falling back to a shebang
// sniff so a bin/ script with no extension still reads as shell.
func classify(p string, content []byte) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".qml":
		return "qml"
	case ".js", ".mjs":
		return "js"
	case ".sh", ".bash", ".zsh":
		return "sh"
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "md"
	}
	if bytes.HasPrefix(content, []byte("#!")) {
		first := content
		if i := bytes.IndexByte(content, '\n'); i >= 0 {
			first = content[:i]
		}
		if bytes.Contains(first, []byte("sh")) {
			return "sh"
		}
	}
	return "other"
}

// hasBinaryMagic reports whether b starts with an ELF or Mach-O magic number.
func hasBinaryMagic(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	if bytes.HasPrefix(b, []byte{0x7f, 'E', 'L', 'F'}) {
		return true
	}
	switch {
	case bytes.HasPrefix(b, []byte{0xFE, 0xED, 0xFA, 0xCE}),
		bytes.HasPrefix(b, []byte{0xFE, 0xED, 0xFA, 0xCF}),
		bytes.HasPrefix(b, []byte{0xCE, 0xFA, 0xED, 0xFE}),
		bytes.HasPrefix(b, []byte{0xCF, 0xFA, 0xED, 0xFE}),
		bytes.HasPrefix(b, []byte{0xCA, 0xFE, 0xBA, 0xBE}),
		bytes.HasPrefix(b, []byte{0xBE, 0xBA, 0xFE, 0xCA}):
		return true
	}
	return false
}

// codeText returns each raw line with its comment content blanked (spaces),
// preserving line numbers and column positions, so the pattern rules match code
// and not comments. Handles sh (#), and C-style // and /* */ for qml/js.
func codeText(rawLines []string, class string) []string {
	out := make([]string, len(rawLines))
	if class == "sh" {
		for i, ln := range rawLines {
			out[i] = stripShellComment(ln)
		}
		return out
	}
	inBlock := false
	for i, ln := range rawLines {
		out[i], inBlock = stripCLine(ln, inBlock)
	}
	return out
}

func stripShellComment(ln string) string {
	var b strings.Builder
	var quote byte
	prevSpace := true
	for i := 0; i < len(ln); i++ {
		c := ln[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == quote {
				quote = 0
			}
			prevSpace = false
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			b.WriteByte(c)
			prevSpace = false
			continue
		}
		if c == '#' && prevSpace {
			for j := i; j < len(ln); j++ {
				b.WriteByte(' ')
			}
			return b.String()
		}
		b.WriteByte(c)
		prevSpace = c == ' ' || c == '\t'
	}
	return b.String()
}

func stripCLine(ln string, inBlock bool) (string, bool) {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(ln); i++ {
		c := ln[i]
		if inBlock {
			if c == '*' && i+1 < len(ln) && ln[i+1] == '/' {
				b.WriteString("  ")
				i++
				inBlock = false
			} else {
				b.WriteByte(' ')
			}
			continue
		}
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(ln) {
				i++
				b.WriteByte(ln[i])
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch {
		case c == '"' || c == '\'' || c == '`':
			quote = c
			b.WriteByte(c)
		case c == '/' && i+1 < len(ln) && ln[i+1] == '/':
			for j := i; j < len(ln); j++ {
				b.WriteByte(' ')
			}
			return b.String(), false
		case c == '/' && i+1 < len(ln) && ln[i+1] == '*':
			b.WriteString("  ")
			i++
			inBlock = true
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), inBlock
}

// pkexecAllowed reports whether the command text starting at a pkexec token is
// covered by a declared privileged string: a declared string, whitespace
// normalised, must be a prefix of the normalised command text (contract 2).
func pkexecAllowed(fromPkexec string, privileged []string) bool {
	norm := normalizeCmd(fromPkexec)
	for _, p := range privileged {
		p = normalizeCmd(p)
		if p != "" && strings.HasPrefix(norm, p) {
			return true
		}
	}
	return false
}

func normalizeCmd(s string) string {
	s = cmdPunctRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// auditConfigWrite flags references to files a plugin must never write: shell
// config, ssh keys, /etc, and shell rc files. plugins.json is allowed when the
// line hands it to ryoku-plugins-place, the sanctioned placement tool.
func auditConfigWrite(p string, line int, code string, add func(rule, p string, line int, msg string)) {
	if strings.Contains(code, "shell.json") {
		add("config-write", p, line, i18n.T("references shell.json; settings go through pluginApi.saveSetting (R5)"))
	}
	if strings.Contains(code, "plugins.json") && !strings.Contains(code, "ryoku-plugins-place") {
		add("config-write", p, line, i18n.T("references plugins.json; settings go through pluginApi.saveSetting (R5)"))
	}
	if strings.Contains(code, "~/.ssh") {
		add("config-write", p, line, i18n.T("touches ~/.ssh; write only under stateDir (R8)"))
	}
	if strings.Contains(code, "/etc/") {
		add("config-write", p, line, i18n.T("writes under /etc; write only under stateDir (R8)"))
	}
	for _, rc := range []string{".bashrc", ".zshrc", "config.fish"} {
		if strings.Contains(code, rc) {
			add("config-write", p, line, i18n.T("touches a shell rc file (")+rc+i18n.T("); write only under stateDir (R8)"))
			break
		}
	}
}

// extractCommands returns argv[0] of each Process command array and execDetached
// call on a line, skipping the plugin's own bin/ scripts and pluginDir paths.
func extractCommands(code string) []string {
	var names []string
	for _, re := range []*regexp.Regexp{cmdArrayRe, execDetachedRe} {
		for _, m := range re.FindAllStringSubmatch(code, -1) {
			first := strings.TrimSpace(m[1])
			if strings.Contains(first, "pluginDir") {
				continue
			}
			val, ok := unquote(first)
			if !ok || val == "" {
				continue
			}
			if strings.HasPrefix(val, "bin/") || strings.HasPrefix(val, "./bin/") {
				continue
			}
			name := val
			if strings.Contains(name, "/") {
				name = path.Base(name)
			}
			names = append(names, name)
		}
	}
	return names
}

func unquote(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		q := s[0]
		if end := strings.IndexByte(s[1:], q); end >= 0 {
			return s[1 : 1+end], true
		}
	}
	return "", false
}

func commandAllowed(name string, deps []string) bool {
	if commandAllowlist[name] {
		return true
	}
	// ryoku-wm-<name> is the provider binary a plugin reaches the compositor through.
	if strings.HasPrefix(name, "ryoku-wm-") {
		return true
	}
	for _, d := range deps {
		if d == name {
			return true
		}
	}
	return false
}

func stripPort(host string) string {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		return host[:i]
	}
	return host
}

func hostAllowed(host string, network []string) bool {
	host = strings.ToLower(host)
	for _, n := range network {
		if strings.ToLower(strings.TrimSpace(n)) == host {
			return true
		}
	}
	return false
}

// isDynamicShell reports whether a line invokes sh/bash -c with an argument that
// is not a single string literal (R9): the array form ["sh","-c", <expr>], or an
// inline `sh -c` whose tail is built with string concatenation.
func isDynamicShell(code string) bool {
	for _, m := range shcArrayRe.FindAllStringSubmatch(code, -1) {
		arg := strings.TrimRight(strings.TrimSpace(m[1]), " )]")
		if !isSingleStringLiteral(arg) {
			return true
		}
	}
	if m := shcInlineRe.FindStringSubmatch(code); m != nil && strings.Contains(m[1], "+") {
		return true
	}
	return false
}

func isSingleStringLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}
	q := s[0]
	if q != '"' && q != '\'' {
		return false
	}
	if s[len(s)-1] != q {
		return false
	}
	return !strings.ContainsRune(s[1:len(s)-1], rune(q))
}

// importEscapes resolves a relative import against the importing file's dir and
// reports whether it lands outside the plugin root.
func importEscapes(filePath, imp string) bool {
	target := path.Clean(path.Join(path.Dir(filePath), imp))
	return target == ".." || strings.HasPrefix(target, "../")
}

// printAudit renders findings: one line per finding (blocking first, then
// warnings), then the summary. --json marshals the auditResult directly.
func printAudit(res auditResult, asJSON bool) {
	if asJSON {
		b, _ := json.Marshal(res)
		fmt.Println(string(b))
		return
	}
	for _, f := range res.Blocking {
		fmt.Printf("%s  %s:%d  %s\n", f.Rule, f.Path, f.Line, f.Message)
	}
	for _, f := range res.Warnings {
		fmt.Printf("%s  %s:%d  %s\n", f.Rule, f.Path, f.Line, f.Message)
	}
	fmt.Printf(i18n.T("%d blocking, %d warnings\n"), len(res.Blocking), len(res.Warnings))
}
