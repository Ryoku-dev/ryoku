package command

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ryoku-rashin/internal/paths"
)

// readonly.go decides, without touching disk, whether an agent tool call only
// reads the machine. In approvals mode "read-only" the daemon auto-approves a
// call this accepts and asks for everything else. It is deny-first: anything it
// cannot prove is a pure read returns false, so an unfamiliar command or an
// unparseable line asks rather than runs. The reasons are short because the
// chat UI prints them next to the tool row.

// readOnlyToolCall reports whether an ACP tool call only reads the machine and
// may run without asking, and a short reason when it may not.
func readOnlyToolCall(kind, title string, rawInput json.RawMessage) (ok bool, reason string) {
	switch kind {
	case "read", "search":
		paths := roReadPaths(rawInput)
		if len(paths) == 0 {
			return false, "no path or query to inspect"
		}
		for _, p := range paths {
			if m, bad := roSecretPath(p); bad {
				return false, "touches a secret (" + m + ")"
			}
		}
		return true, ""
	case "think":
		return true, ""
	case "fetch":
		return roFetchReadOnly(rawInput)
	case "execute":
		cmd, has := roCommandString(rawInput)
		if !has {
			cmd = title
		}
		return readOnlyCommand(cmd)
	case "other", "":
		cmd, has := roCommandString(rawInput)
		if !has {
			return false, "no command to inspect"
		}
		return readOnlyCommand(cmd)
	default:
		// edit, delete, move, switch_mode, and anything unknown change state.
		return false, "`" + kind + "` may change the machine"
	}
}

// roSecretFile gates command arguments through the same predicate used by the
// native read_file and list_dir tools.
func roSecretFile(path string) (string, bool) {
	return SecretPath(path)
}

// roSecretPath also treats generic secret-search terms as sensitive for ACP
// read and search tools, where the input may be a query rather than a path.
func roSecretPath(path string) (string, bool) {
	if marker, blocked := SecretPath(path); blocked {
		return marker, true
	}
	low := strings.ToLower(path)
	for _, marker := range []string{"credentials", "secrets", "token"} {
		if strings.Contains(low, marker) {
			return marker, true
		}
	}
	return "", false
}

// roReadPaths pulls every path or pattern a read/search tool call names, so a
// secret in any of them is caught. Titles are display text, not inspected input.
func roReadPaths(raw json.RawMessage) []string {
	m := roDecode(raw)
	var out []string
	for _, key := range []string{"path", "file_path", "filePath", "pattern", "query"} {
		if s, ok := roString(m, key); ok && s != "" {
			out = append(out, s)
		}
	}
	if v, ok := m["paths"]; ok {
		var arr []string
		if json.Unmarshal(v, &arr) == nil {
			out = append(out, arr...)
		} else if s, ok := roStringOrArray(v); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// roFetchReadOnly allows a web fetch that is a plain GET with no body. A method
// other than GET, or any request body or upload, changes the far end and asks.
func roFetchReadOnly(raw json.RawMessage) (bool, string) {
	m := roDecode(raw)
	rawURL, ok := roString(m, "url")
	if !ok || strings.TrimSpace(rawURL) == "" {
		return false, "no URL to inspect"
	}
	u, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false, "invalid URL"
	}
	if s, ok := roString(m, "method"); ok {
		if method := strings.ToUpper(strings.TrimSpace(s)); method != "" && method != "GET" {
			return false, "`" + method + "` is not a plain read"
		}
	}
	for _, key := range []string{"body", "data", "json", "form", "files", "upload"} {
		if v, ok := m[key]; ok && !roEmptyJSON(v) {
			return false, "sends a request body"
		}
	}
	return true, ""
}

func roDecode(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

func roString(m map[string]json.RawMessage, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s, true
	}
	return "", false
}

// roStringOrArray reads a JSON value that is either a string or an array of
// strings, joining an array with spaces the way a shell would.
func roStringOrArray(v json.RawMessage) (string, bool) {
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s, true
	}
	var arr []string
	if json.Unmarshal(v, &arr) == nil {
		return strings.Join(arr, " "), true
	}
	return "", false
}

// roCommandString reads the command an execute-style tool runs from its
// "command" (string or array) or "cmd" field.
func roCommandString(raw json.RawMessage) (string, bool) {
	m := roDecode(raw)
	if v, ok := m["command"]; ok {
		if s, ok := roStringOrArray(v); ok && strings.TrimSpace(s) != "" {
			return s, true
		}
	}
	if s, ok := roString(m, "cmd"); ok && strings.TrimSpace(s) != "" {
		return s, true
	}
	return "", false
}

func roEmptyJSON(v json.RawMessage) bool {
	switch strings.TrimSpace(string(v)) {
	case "", "null", `""`, "[]", "{}", "0", "false":
		return true
	}
	return false
}

// readOnlyCommand reports whether a shell command line only reads. It lexes the
// line quote-aware, splits it into segments, rejects any construct that could
// run or write something unseen, checks each segment's command against an
// allowlist with per-command argument rules, and refuses any segment or
// redirect that reads a credential file.
func readOnlyCommand(cmd string) (ok bool, reason string) {
	if strings.TrimSpace(cmd) == "" {
		return false, "empty command"
	}
	segs, reads, rj := roLex(cmd)
	if rj != "" {
		return false, rj
	}
	for _, t := range reads {
		if m, bad := roSecretFile(t); bad {
			return false, "reads a secret (" + m + ")"
		}
	}
	checked := false
	for _, argv := range segs {
		if len(argv) == 0 {
			continue
		}
		stripped, assignmentReason := roStripAssignments(argv)
		if assignmentReason != "" {
			return false, assignmentReason
		}
		if len(stripped) == 0 {
			return false, "sets variables without running a command"
		}
		checked = true
		if m, bad := roArgvSecret(stripped); bad {
			return false, "reads a secret (" + m + ")"
		}
		if roRecursiveSecretSearch(stripped) {
			return false, "recursively searches a sensitive tree"
		}
		if okSeg, r := roCheckSegment(stripped); !okSeg {
			return false, r
		}
	}
	if !checked {
		return false, "no command to inspect"
	}
	return true, ""
}

// roArgvSecret reports whether a segment names a credential file. Search
// patterns are not paths, but grep pattern-file options and search targets are.
func roArgvSecret(argv []string) (string, bool) {
	if len(argv) == 0 {
		return "", false
	}
	switch argv[0] {
	case "grep", "egrep", "fgrep", "rg":
		for i, a := range argv[1:] {
			var tok string
			switch {
			case (a == "-f" || a == "--file") && i+2 < len(argv):
				tok = argv[i+2]
			case strings.HasPrefix(a, "-f") && len(a) > 2:
				tok = a[2:]
			case strings.HasPrefix(a, "--file="):
				tok = strings.TrimPrefix(a, "--file=")
			}
			if m, bad := roSecretFile(tok); bad {
				return m, true
			}
		}
		_, targets := roSearchPatternTargets(argv[1:])
		for _, tok := range targets {
			if m, bad := roSecretFile(tok); bad {
				return m, true
			}
		}
		return "", false
	}
	for _, a := range argv[1:] {
		tok, ok := roArgToken(a)
		if !ok {
			continue
		}
		if m, bad := roSecretFile(tok); bad {
			return m, true
		}
	}
	return "", false
}

// roArgToken returns the value-bearing part of an argument. Short option
// values must be inspected because tools such as grep accept -fPATH.
func roArgToken(a string) (string, bool) {
	if strings.HasPrefix(a, "--") {
		if eq := strings.IndexByte(a, '='); eq >= 0 {
			return a[eq+1:], true
		}
		return "", false
	}
	if strings.HasPrefix(a, "-") && a != "-" {
		if len(a) > 2 {
			return a[2:], true
		}
		return "", false
	}
	return a, true
}

// roRecursiveSecretSearch rejects recursive searches of credential-bearing
// roots, and secret-word sweeps across any home path.
func roRecursiveSecretSearch(argv []string) bool {
	recursive := false
	switch filepath.Base(argv[0]) {
	case "rg":
		recursive = true // ripgrep recurses by default
	case "grep", "egrep", "fgrep":
		for _, a := range argv[1:] {
			if a == "-r" || a == "-R" || a == "--recursive" ||
				a == "--dereference-recursive" || roShortHas(a, 'r') || roShortHas(a, 'R') {
				recursive = true
			}
		}
	default:
		return false
	}
	if !recursive {
		return false
	}
	pattern, targets := roSearchPatternTargets(argv[1:])
	for _, target := range targets {
		if roSensitiveSearchRoot(target) {
			return true
		}
	}
	low := strings.ToLower(pattern)
	if low == "" {
		return false
	}
	secret := false
	for _, w := range []string{"token", "password", "passwd", "secret", "api_key", "apikey", "private"} {
		if strings.Contains(low, w) {
			secret = true
			break
		}
	}
	if !secret {
		return false
	}
	for _, t := range targets {
		if roUnderHome(t) {
			return true
		}
	}
	return false
}

func roSensitiveSearchRoot(target string) bool {
	clean := filepath.Clean(paths.Expand(target))
	switch clean {
	case "/", "/dev", "/proc", "/sys", "/home", "/root":
		return true
	}
	home := filepath.Clean(os.Getenv("HOME"))
	if home == "." || home == "" {
		return false
	}
	if clean == home || clean == filepath.Join(home, ".config") ||
		clean == filepath.Join(home, ".local") || clean == filepath.Join(home, ".var") {
		return true
	}
	return false
}

// roSearchPatternTargets splits grep/rg arguments into the search pattern and
// the paths it searches, honouring -e/--regexp and skipping flags.
func roSearchPatternTargets(args []string) (pattern string, targets []string) {
	havePattern := false
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-e" || a == "--regexp":
			if i+1 < len(args) {
				if !havePattern {
					pattern = args[i+1]
				}
				havePattern = true
			}
			i += 2
		case strings.HasPrefix(a, "--regexp="):
			if !havePattern {
				pattern = strings.TrimPrefix(a, "--regexp=")
			}
			havePattern = true
			i++
		case strings.HasPrefix(a, "-e") && len(a) > 2:
			if !havePattern {
				pattern = a[2:]
			}
			havePattern = true
			i++
		case a == "-f" || a == "--file":
			havePattern = true
			i += 2
		case strings.HasPrefix(a, "--file=") || strings.HasPrefix(a, "-f") && len(a) > 2:
			havePattern = true
			i++
		case strings.HasPrefix(a, "-") && a != "-":
			i++
		default:
			if !havePattern {
				pattern = a
				havePattern = true
			} else {
				targets = append(targets, a)
			}
			i++
		}
	}
	return pattern, targets
}

// roUnderHome reports whether a path argument points into the user's home,
// detected syntactically so the classifier needs no environment lookup.
func roUnderHome(t string) bool {
	switch {
	case t == "~" || strings.HasPrefix(t, "~/"):
		return true
	case t == "$HOME" || strings.HasPrefix(t, "$HOME/"):
		return true
	case t == "${HOME}" || strings.HasPrefix(t, "${HOME}/"):
		return true
	case strings.HasPrefix(t, "/home/") || strings.HasPrefix(t, "/root/"):
		return true
	}
	return false
}

// roLex turns a command line into per-segment argv slices. It honours single
// quotes, double quotes and backslash escapes; splits on unquoted |, ||, &&, ;,
// newlines; and returns a rejection reason for command or process substitution,
// subshell or brace groups, background &, heredocs, and any output redirection
// whose target is not /dev/null (input redirects and 2>&1 style dups are fine).
func roLex(cmd string) (segs [][]string, reads []string, reason string) {
	runes := []rune(cmd)
	n := len(runes)
	var cur []string
	flush := func() {
		segs = append(segs, cur)
		cur = nil
	}
	i := 0
	for i < n {
		for i < n && (runes[i] == ' ' || runes[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		r := runes[i]

		// A number glued to > or < is a file-descriptor prefix (2>, 1<), not a
		// word; parse the redirection it introduces.
		if r >= '0' && r <= '9' {
			j := i
			for j < n && runes[j] >= '0' && runes[j] <= '9' {
				j++
			}
			if j < n && (runes[j] == '>' || runes[j] == '<') {
				fd := string(runes[i:j])
				i = j
				var rj, tgt string
				if runes[i] == '>' {
					i, rj = roOutputRedirect(runes, i, fd)
				} else {
					i, tgt, rj = roInputRedirect(runes, i)
					if tgt != "" {
						reads = append(reads, tgt)
					}
				}
				if rj != "" {
					return nil, nil, rj
				}
				continue
			}
		}

		switch {
		case r == '\n' || r == ';':
			i++
			flush()
			continue
		case r == '|':
			if i+1 < n && runes[i+1] == '|' {
				i++
			}
			i++
			flush()
			continue
		case r == '&':
			if i+1 < n && runes[i+1] == '&' {
				i += 2
				flush()
				continue
			}
			if i+1 < n && runes[i+1] == '>' {
				i++ // consume '&', now positioned on '>'
				var rj string
				i, rj = roOutputRedirect(runes, i, "&")
				if rj != "" {
					return nil, nil, rj
				}
				continue
			}
			return nil, nil, "runs a command in the background"
		case r == '(' || r == ')':
			return nil, nil, "opens a subshell"
		case r == '{' || r == '}':
			// {} alone is the word find and xargs put a name in, not a group
			if r == '{' && i+1 < n && runes[i+1] == '}' && (i+2 == n || roWordEnd(runes[i+2])) {
				cur = append(cur, "{}")
				i += 2
				continue
			}
			return nil, nil, "opens a brace group"
		case r == '`':
			return nil, nil, "runs a command substitution"
		case r == '$' && i+1 < n && runes[i+1] == '(':
			return nil, nil, "runs a command substitution"
		case r == '>':
			var rj string
			i, rj = roOutputRedirect(runes, i, "")
			if rj != "" {
				return nil, nil, rj
			}
			continue
		case r == '<':
			var rj, tgt string
			i, tgt, rj = roInputRedirect(runes, i)
			if tgt != "" {
				reads = append(reads, tgt)
			}
			if rj != "" {
				return nil, nil, rj
			}
			continue
		}

		word, ni, rj := roReadWord(runes, i)
		if rj != "" {
			return nil, nil, rj
		}
		i = ni
		cur = append(cur, word)
	}
	flush()
	return segs, reads, ""
}

// roReadWord reads one word starting at i, honouring quotes and escapes and
// rejecting a command substitution found inside it. It stops before an unquoted
// operator or whitespace without consuming it.
func roReadWord(runes []rune, i int) (word string, newi int, reason string) {
	n := len(runes)
	var b strings.Builder
	for i < n {
		r := runes[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n' ||
			r == '|' || r == '&' || r == ';' ||
			r == '<' || r == '>' ||
			r == '(' || r == ')' || r == '{' || r == '}':
			return b.String(), i, ""
		case r == '\'':
			i++
			for i < n && runes[i] != '\'' {
				b.WriteRune(runes[i])
				i++
			}
			if i < n {
				i++
			}
		case r == '"':
			i++
			for i < n && runes[i] != '"' {
				if runes[i] == '\\' && i+1 < n {
					i++
					b.WriteRune(runes[i])
					i++
					continue
				}
				if runes[i] == '`' {
					return "", i, "runs a command substitution"
				}
				if runes[i] == '$' {
					if i+1 < n && runes[i+1] == '(' {
						return "", i, "runs a command substitution"
					}
					return "", i, "expands a shell variable"
				}
				b.WriteRune(runes[i])
				i++
			}
			if i < n {
				i++
			}
		case r == '\\':
			i++
			if i < n {
				b.WriteRune(runes[i])
				i++
			}
		case r == '`':
			return "", i, "runs a command substitution"
		case r == '$':
			if i+1 < n && runes[i+1] == '(' {
				return "", i, "runs a command substitution"
			}
			return "", i, "expands a shell variable"
		case r == '*' || r == '?' || r == '[':
			return "", i, "uses an unquoted glob"
		case r == '~' && b.Len() == 0 && i+1 < n && runes[i+1] != '/' && !roWordEnd(runes[i+1]):
			return "", i, "expands another user's home"
		default:
			b.WriteRune(r)
			i++
		}
	}
	return b.String(), i, ""
}

// roOutputRedirect parses an output redirection whose '>' is at runes[i], with
// fd already consumed ("" for stdout, a number, or "&" for &>). Only a target
// of /dev/null or a bare descriptor dup (2>&1, 1>&2, >&2) is read-only.
func roOutputRedirect(runes []rune, i int, fd string) (int, string) {
	n := len(runes)
	i++ // consume '>'
	switch {
	case i < n && runes[i] == '&':
		i++
		dup := ""
		for i < n && runes[i] >= '0' && runes[i] <= '9' {
			dup += string(runes[i])
			i++
		}
		op := fd + ">&" + dup
		if op == "2>&1" || op == "1>&2" || op == ">&2" {
			return i, ""
		}
		return i, "duplicates a file descriptor"
	case i < n && runes[i] == '(':
		return i, "uses process substitution"
	case i < n && runes[i] == '>':
		i++ // '>>'
	case i < n && runes[i] == '|':
		i++ // '>|'
	}
	for i < n && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	target, ni, rj := roReadWord(runes, i)
	if rj != "" {
		return ni, rj
	}
	if target == "/dev/null" {
		return ni, ""
	}
	return ni, "redirects output into a file"
}

// roInputRedirect parses an input redirection whose '<' is at runes[i]. Reading
// from a file (< file) and a here-string (<<<) are fine; a heredoc (<<) and
// process substitution (<() ask.
func roInputRedirect(runes []rune, i int) (newi int, target, reason string) {
	n := len(runes)
	i++ // consume '<'
	if i < n && runes[i] == '<' {
		i++
		if i < n && runes[i] == '<' {
			// here-string: the following word is data, not a file.
			i++
			for i < n && (runes[i] == ' ' || runes[i] == '\t') {
				i++
			}
			_, ni, rj := roReadWord(runes, i)
			return ni, "", rj
		}
		return i, "", "uses a heredoc"
	}
	if i < n && runes[i] == '(' {
		return i, "", "uses process substitution"
	}
	for i < n && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	tgt, ni, rj := roReadWord(runes, i)
	return ni, tgt, rj
}

// roStripAssignments accepts only presentation and locale variables. Every
// other assignment can alter a tool's configuration, code loading, or output
// destination, so it is outside the read-only proof.
func roStripAssignments(argv []string) ([]string, string) {
	i := 0
	for i < len(argv) && roIsAssignment(argv[i]) {
		name := argv[i][:strings.IndexByte(argv[i], '=')]
		if !roReadOnlyAssignment(name) {
			return nil, "`" + name + "` is not an allowed read-only environment variable"
		}
		i++
	}
	return argv[i:], ""
}

func roReadOnlyAssignment(name string) bool {
	switch name {
	case "LANG", "TZ", "NO_COLOR", "COLUMNS":
		return true
	}
	return strings.HasPrefix(name, "LC_") && len(name) > len("LC_")
}

func roIsAssignment(w string) bool {
	eq := strings.IndexByte(w, '=')
	if eq <= 0 {
		return false
	}
	for j, c := range w[:eq] {
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case j > 0 && c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return true
}

// roNeverCmds run other code or as another user; they never read-only, and a
// clear reason beats the generic allowlist miss.
var roNeverCmds = map[string]string{
	"sudo": "runs as another user", "doas": "runs as another user",
	"pkexec": "runs as another user", "su": "runs as another user",
	"run0": "runs as another user",
	"eval": "evaluates a built command", "exec": "replaces the shell",
	"source": "sources a script", ".": "sources a script",
	"printenv": "prints the environment",
}

// roCheckSegment allows one segment's command by name and argument rules.
// `env` may carry harmless variables ahead of an allowed command.
func roCheckSegment(argv []string) (bool, string) {
	if strings.ContainsRune(argv[0], '/') {
		return false, "path-qualified executables are not allowed"
	}
	base := argv[0]
	if reason, bad := roNeverCmds[base]; bad {
		return false, "`" + base + "` " + reason
	}
	if base == "env" {
		rest, assignmentReason := roStripAssignments(argv[1:])
		if assignmentReason != "" {
			return false, assignmentReason
		}
		if len(rest) == 0 {
			return false, "`env` prints the environment"
		}
		if strings.HasPrefix(rest[0], "-") {
			return false, "`env` with options is not read-only"
		}
		return roCheckSegment(rest)
	}
	rule, ok := roAllow[base]
	if !ok {
		return false, "`" + base + "` is not a read-only command"
	}
	return rule(argv[1:])
}

// roRule validates one command's arguments (argv without the command word).
type roRule func(args []string) (bool, string)

// roFlagSpec is an allowlist. The value is how many following arguments the
// flag consumes. A flag absent from the table is rejected.
type roFlagSpec map[string]int

func roFlags(noValue []string, oneValue []string) roFlagSpec {
	spec := make(roFlagSpec, len(noValue)+len(oneValue))
	for _, flag := range noValue {
		spec[flag] = 0
	}
	for _, flag := range oneValue {
		spec[flag] = 1
	}
	return spec
}

func roParseFlags(tool string, args []string, spec roFlagSpec, stopAtFirstPositional bool) ([]string, string) {
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return append(positional, args[i+1:]...), ""
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			if stopAtFirstPositional {
				return append(positional, args[i+1:]...), ""
			}
			continue
		}
		if strings.HasPrefix(arg, "--") {
			flag, attached, hasAttached := arg, "", false
			if before, after, found := strings.Cut(arg, "="); found {
				flag, attached, hasAttached = before, after, true
			}
			values, ok := spec[flag]
			if !ok {
				return nil, "`" + tool + " " + flag + "` is not an allowed read-only flag"
			}
			if values == 0 {
				if hasAttached {
					return nil, "`" + tool + " " + flag + "` does not take a value"
				}
				continue
			}
			if hasAttached {
				if attached == "" || values != 1 {
					return nil, "`" + tool + " " + flag + "` has an invalid value"
				}
				continue
			}
			if i+values >= len(args) {
				return nil, "`" + tool + " " + flag + "` needs a value"
			}
			i += values
			continue
		}

		if values, ok := spec[arg]; ok {
			if i+values >= len(args) {
				return nil, "`" + tool + " " + arg + "` needs a value"
			}
			i += values
			continue
		}
		for j := 1; j < len(arg); j++ {
			flag := "-" + string(arg[j])
			values, ok := spec[flag]
			if !ok {
				return nil, "`" + tool + " " + flag + "` is not an allowed read-only flag"
			}
			if values == 0 {
				continue
			}
			if j+1 < len(arg) {
				if values != 1 {
					return nil, "`" + tool + " " + flag + "` has an invalid value"
				}
				j = len(arg)
				break
			}
			if i+values >= len(args) {
				return nil, "`" + tool + " " + flag + "` needs a value"
			}
			i += values
			break
		}
	}
	return positional, ""
}

func roSimple(tool string, noValue []string, oneValue []string) roRule {
	spec := roFlags(noValue, oneValue)
	return func(args []string) (bool, string) {
		_, reason := roParseFlags(tool, args, spec, false)
		return reason == "", reason
	}
}

// roAllow maps a command basename to the rule that admits its read-only uses.
var roAllow map[string]roRule

func init() {
	// roAlways is reserved for commands whose argv cannot select a writer,
	// executor, configuration loader, or unbounded mode.
	roAllow = map[string]roRule{}
	for _, name := range []string{
		"rev", "printf", "true", "false", "test", "[", "whoami", "groups",
		"uptime", "nproc", "getconf", "locale", "pidof", "which", "whereis",
		"type", "fc-list", "fc-match",
	} {
		roAllow[name] = roAlways
	}
	roAllow["cat"] = roSimple("cat",
		[]string{"-A", "-b", "-e", "-E", "-n", "-s", "-t", "-T", "-u", "-v", "--show-all", "--number-nonblank", "--number", "--squeeze-blank", "--show-ends", "--show-tabs", "--show-nonprinting"},
		nil)
	roAllow["grep"] = roGrep
	roAllow["egrep"] = roGrep
	roAllow["fgrep"] = roGrep
	roAllow["ls"] = roSimple("ls",
		[]string{"-a", "-A", "-b", "-B", "-c", "-C", "-d", "-F", "-g", "-G", "-h", "-H", "-i", "-l", "-L", "-m", "-n", "-N", "-p", "-q", "-Q", "-r", "-R", "-s", "-S", "-t", "-u", "-v", "-x", "-X", "-1", "--all", "--almost-all", "--directory", "--classify", "--human-readable", "--dereference", "--numeric-uid-gid", "--reverse", "--recursive", "--size", "--inode", "--group-directories-first", "--zero"},
		[]string{"--color", "--format", "--time", "--sort", "--width", "--block-size", "--hide", "--ignore", "-w", "-I", "-T"})
	roAllow["eza"] = roAllow["ls"]
	roAllow["fd"] = roFd
	roAllow["cd"] = func(args []string) (bool, string) {
		positional, reason := roParseFlags("cd", args, roFlags([]string{"-L", "-P"}, nil), false)
		if reason != "" {
			return false, reason
		}
		if len(positional) > 1 {
			return false, "`cd` accepts one directory"
		}
		return true, ""
	}
	roAllow["pwd"] = roSimple("pwd", []string{"-L", "-P"}, nil)
	roAllow["stat"] = roSimple("stat",
		[]string{"-L", "-f", "-t", "--dereference", "--file-system", "--terse"},
		[]string{"-c", "--format", "--printf"})
	roAllow["file"] = roSimple("file",
		[]string{"-b", "-i", "-k", "-L", "-N", "-p", "-s", "-Z", "--brief", "--mime", "--keep-going", "--dereference", "--no-pad", "--preserve-date", "--special-files"},
		[]string{"-e", "--exclude", "--exclude-quiet", "-m", "--magic-file", "-P", "--parameter"})
	roAllow["wc"] = roSimple("wc",
		[]string{"-c", "-m", "-l", "-L", "-w", "--bytes", "--chars", "--lines", "--max-line-length", "--words", "--total"},
		nil)
	roAllow["cut"] = roSimple("cut",
		[]string{"-s", "-z", "--only-delimited", "--zero-terminated", "--complement"},
		[]string{"-b", "-c", "-f", "-d", "--bytes", "--characters", "--fields", "--delimiter", "--output-delimiter"})
	roAllow["tr"] = roSimple("tr",
		[]string{"-c", "-C", "-d", "-s", "-t", "--complement", "--delete", "--squeeze-repeats", "--truncate-set1"},
		nil)
	roAllow["nl"] = roSimple("nl",
		nil,
		[]string{"-b", "-d", "-f", "-h", "-i", "-l", "-n", "-p", "-s", "-v", "-w", "--body-numbering", "--section-delimiter", "--footer-numbering", "--header-numbering", "--line-increment", "--join-blank-lines", "--number-format", "--number-separator", "--starting-line-number", "--number-width"})
	roAllow["tac"] = roSimple("tac", []string{"-b", "-r", "--before", "--regex"}, []string{"-s", "--separator"})
	roAllow["column"] = roSimple("column",
		[]string{"-c", "-x", "-e", "-n", "-r", "-L", "-J", "--fillrows", "--table", "--table-noheadings", "--json"},
		[]string{"-s", "-t", "-o", "-N", "-H", "-R", "-W", "--separator", "--output-separator", "--table-columns", "--table-hide", "--table-right", "--table-wrap", "--output-width"})
	roAllow["fold"] = roSimple("fold", []string{"-b", "-s", "--bytes", "--spaces"}, []string{"-w", "--width"})
	roAllow["fmt"] = roSimple("fmt", []string{"-c", "-s", "-t", "-u", "--crown-margin", "--split-only", "--tagged-paragraph", "--uniform-spacing"}, []string{"-p", "-w", "-g", "--prefix", "--width", "--goal"})
	roAllow["comm"] = roSimple("comm", []string{"-1", "-2", "-3", "-z", "--zero-terminated", "--check-order", "--nocheck-order"}, []string{"--output-delimiter"})
	roAllow["join"] = roSimple("join", []string{"-a", "-v", "-e", "-i", "--ignore-case", "--check-order", "--nocheck-order", "-z", "--zero-terminated"}, []string{"-1", "-2", "-j", "-o", "-t"})
	roAllow["paste"] = roSimple("paste", []string{"-s", "-z", "--serial", "--zero-terminated"}, []string{"-d", "--delimiters"})
	roAllow["diff"] = roSimple("diff",
		[]string{"-a", "-b", "-B", "-d", "-E", "-i", "-q", "-r", "-s", "-t", "-T", "-u", "-w", "-y", "--text", "--ignore-space-change", "--ignore-blank-lines", "--minimal", "--ignore-tab-expansion", "--ignore-case", "--brief", "--recursive", "--report-identical-files", "--expand-tabs", "--initial-tab", "--unified", "--ignore-all-space", "--side-by-side", "--no-dereference"},
		[]string{"-C", "-U", "-I", "-S", "-W", "--context", "--unified", "--ignore-matching-lines", "--starting-file", "--width", "--tabsize", "--exclude", "--exclude-from"})
	roAllow["cmp"] = roSimple("cmp", []string{"-b", "-l", "-s", "--print-bytes", "--verbose", "--silent", "--quiet"}, []string{"-i", "-n", "--ignore-initial", "--bytes"})
	for _, name := range []string{"md5sum", "sha1sum", "sha256sum", "b2sum"} {
		roAllow[name] = roSimple(name,
			[]string{"-b", "--tag", "-t", "-z", "--binary", "--text", "--zero"},
			[]string{"-l", "--length"})
	}
	roAllow["basename"] = roSimple("basename", []string{"-a", "-z", "--multiple", "--zero"}, []string{"-s", "--suffix"})
	roAllow["dirname"] = roSimple("dirname", []string{"-z", "--zero"}, nil)
	roAllow["realpath"] = roSimple("realpath", []string{"-e", "-m", "-L", "-P", "-q", "-s", "-z", "--canonicalize-existing", "--canonicalize-missing", "--logical", "--physical", "--quiet", "--strip", "--zero"}, []string{"--relative-to", "--relative-base"})
	roAllow["readlink"] = roSimple("readlink", []string{"-f", "-e", "-m", "-n", "-q", "-s", "-v", "-z", "--canonicalize", "--canonicalize-existing", "--canonicalize-missing", "--no-newline", "--quiet", "--silent", "--verbose", "--zero"}, nil)
	roAllow["echo"] = roSimple("echo", []string{"-n", "-e", "-E"}, nil)
	roAllow["uname"] = roSimple("uname", []string{"-a", "-s", "-n", "-r", "-v", "-m", "-p", "-i", "-o", "--all", "--kernel-name", "--nodename", "--kernel-release", "--kernel-version", "--machine", "--processor", "--hardware-platform", "--operating-system"}, nil)
	roAllow["id"] = roSimple("id", []string{"-a", "-Z", "-g", "-G", "-n", "-r", "-u", "-z", "--context", "--group", "--groups", "--name", "--real", "--user", "--zero"}, nil)
	roAllow["free"] = roSimple("free", []string{"-b", "-k", "-m", "-g", "--tera", "-h", "-w", "-l", "-t", "-L", "--bytes", "--kilo", "--mega", "--giga", "--human", "--wide", "--lohi", "--total", "--line"}, []string{"-s", "-c", "--seconds", "--count"})
	roAllow["df"] = roSimple("df", []string{"-a", "-B", "-h", "-H", "-i", "-k", "-l", "-P", "-T", "-v", "--all", "--human-readable", "--si", "--inodes", "--local", "--portability", "--print-type"}, []string{"--block-size", "-t", "--type", "-x", "--exclude-type", "--output"})
	roAllow["du"] = roSimple("du", []string{"-a", "--all", "-h", "--human-readable", "-c", "--total", "-d", "-k", "-m", "-s", "--summarize", "-x", "--one-file-system", "-L", "--dereference", "-P", "--no-dereference", "--apparent-size", "--inodes"}, []string{"-B", "--block-size", "-t", "--threshold", "-X", "--exclude-from", "--exclude", "--max-depth", "--time", "--time-style"})
	roAllow["lsblk"] = roSimple("lsblk", []string{"-a", "-b", "-d", "-f", "-i", "-J", "-l", "-m", "-n", "-p", "-P", "-r", "-s", "-S", "-t", "-T", "-z", "--all", "--bytes", "--nodeps", "--fs", "--ascii", "--json", "--list", "--perms", "--noheadings", "--paths", "--pairs", "--raw", "--inverse", "--scsi", "--topology", "--zoned"}, []string{"-o", "--output", "-x", "--sort", "--include", "--exclude"})
	for _, name := range []string{"blkid", "findmnt", "lscpu", "lspci", "lsusb", "lsmod", "modinfo", "pgrep", "lsof", "strings", "od", "hexdump", "pw-dump"} {
		roAllow[name] = roSimple(name, nil, nil)
	}
	roAllow["jq"] = roJq
	roAllow["ps"] = roPs
	roAllow["head"] = roHeadTail("head")
	roAllow["tail"] = roHeadTail("tail")
	roAllow["rg"] = roRg
	roAllow["sort"] = roSort
	roAllow["uniq"] = roUniq
	roAllow["date"] = roDate
	roAllow["hostname"] = roHostname
	roAllow["command"] = roCommand
	roAllow["xxd"] = roXxd
	roAllow["bat"] = roBat
	roAllow["top"] = roTop
	roAllow["awk"] = roAwk
	roAllow["gawk"] = roAwk
	roAllow["mawk"] = roAwk
	roAllow["sed"] = roSed
	roAllow["find"] = roFind
	roAllow["xargs"] = roXargs
	roAllow["tree"] = roTree
	roAllow["ss"] = roSs
	roAllow["sensors"] = roSensors
	roAllow["dmidecode"] = roDmidecode
	roAllow["journalctl"] = roJournalctl
	roAllow["systemctl"] = roSystemctl
	roAllow["loginctl"] = roLoginctl
	roAllow["coredumpctl"] = roCoredumpctl
	roAllow["pacman"] = roPacman
	roAllow["yay"] = roPacman
	roAllow["paru"] = roPacman
	roAllow["dnf"] = roDNF
	roAllow["dnf5"] = roDNF
	roAllow["rpm"] = roRPM
	roAllow["xbps-query"] = roXBPSQuery
	roAllow["ip"] = roIP
	roAllow["nmcli"] = roNmcli
	roAllow["resolvectl"] = roResolvectl
	roAllow["timedatectl"] = roStatusShow
	roAllow["localectl"] = roStatusShow
	roAllow["hostnamectl"] = roStatusShow
	roAllow["systemd-analyze"] = roSystemdAnalyze
	roAllow["dmesg"] = roDmesg
	roAllow["fuser"] = roFuser
	roAllow["mount"] = roMount
	roAllow["swapon"] = roSwapon
	roAllow["zramctl"] = roZramctl
	roAllow["btrfs"] = roBtrfs
	roAllow["snapper"] = roSnapper
	roAllow["flatpak"] = roFlatpak
	roAllow["docker"] = roDocker
	roAllow["podman"] = roDocker
	roAllow["pactl"] = roPactl
	roAllow["wpctl"] = roWpctl
	roAllow["pw-cli"] = roPwCli
	roAllow["udevadm"] = roUdevadm
	roAllow["efibootmgr"] = roEfibootmgr
	roAllow["bootctl"] = roBootctl
	roAllow["fwupdmgr"] = roFwupdmgr
	roAllow["gsettings"] = roGsettings
	roAllow["dconf"] = roDconf
	roAllow["xdg-mime"] = roXdgMime
	roAllow["nvidia-smi"] = roNvidiaSmi
	roAllow["git"] = roGit
	roAllow["ryoku"] = roRyoku
	roAllow["rashin"] = roRashin
	roAllow["ryoku-shell"] = roRyokuShell
	roAllow["qs"] = roQs
}

func roAlways(args []string) (bool, string) { return true, "" }

// roFirstVerb returns the first non-flag argument, the subcommand for tools
// that dispatch on one.
func roFirstVerb(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}

func roNonFlags(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func roShortHas(a string, c rune) bool {
	return strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a, c)
}

func roGrep(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-E", "-F", "-G", "-P", "-i", "-v", "-w", "-x", "-c", "-l", "-L", "-n", "-H", "-h", "-o", "-q", "-s", "-r", "-R", "-a", "-I", "-U", "-z", "-Z", "--extended-regexp", "--fixed-strings", "--basic-regexp", "--perl-regexp", "--ignore-case", "--invert-match", "--word-regexp", "--line-regexp", "--count", "--files-with-matches", "--files-without-match", "--line-number", "--with-filename", "--no-filename", "--only-matching", "--quiet", "--silent", "--no-messages", "--recursive", "--dereference-recursive", "--text", "--binary-files-without-match", "--binary", "--null-data", "--null"},
		[]string{"-e", "-f", "-m", "-A", "-B", "-C", "-d", "-D", "--regexp", "--file", "--max-count", "--after-context", "--before-context", "--context", "--directories", "--devices", "--include", "--exclude", "--exclude-from", "--exclude-dir", "--binary-files", "--color", "--label"},
	)
	_, reason := roParseFlags("grep", args, spec, false)
	return reason == "", reason
}

func roJq(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-n", "-R", "-s", "-c", "-r", "-j", "-a", "-S", "-C", "-M", "-e", "--null-input", "--raw-input", "--slurp", "--compact-output", "--raw-output", "--join-output", "--ascii-output", "--sort-keys", "--color-output", "--monochrome-output", "--exit-status"},
		[]string{"-L", "--library-path"},
	)
	positional, reason := roParseFlags("jq", args, spec, false)
	if reason != "" {
		return false, reason
	}
	if len(positional) == 0 {
		return false, "`jq` needs a program to inspect"
	}
	program := positional[0]
	for _, forbidden := range []string{"$ENV", "ENV", "env", "input_filename"} {
		if jqProgramContains(program, forbidden) {
			return false, "`jq` program may expose the environment or a hidden input path"
		}
	}
	return true, ""
}

func jqProgramContains(program, token string) bool {
	for at := strings.Index(program, token); at >= 0; {
		beforeOK := at == 0 || !roIdentByte(program[at-1])
		end := at + len(token)
		afterOK := end == len(program) || !roIdentByte(program[end])
		if beforeOK && afterOK {
			return true
		}
		next := strings.Index(program[end:], token)
		if next < 0 {
			return false
		}
		at = end + next
	}
	return false
}

func roIdentByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func roHeadTail(tool string) roRule {
	rule := roSimple(tool,
		[]string{"-q", "-v", "-z", "--quiet", "--silent", "--verbose", "--zero-terminated"},
		[]string{"-c", "-n", "--bytes", "--lines"},
	)
	return func(args []string) (bool, string) {
		return rule(withoutLegacyCount(tool, args))
	}
}

// withoutLegacyCount drops the count written the old way, `head -20` or
// `tail +3`, which every head and tail still takes and most people type.
func withoutLegacyCount(tool string, args []string) []string {
	kept := make([]string, 0, len(args))
	for index, arg := range args {
		if arg == "--" {
			return append(kept, args[index:]...)
		}
		if len(arg) > 1 && (arg[0] == '-' || arg[0] == '+' && tool == "tail") && roDigits(arg[1:]) {
			continue
		}
		kept = append(kept, arg)
	}
	return kept
}

// roWordEnd reports a rune that ends a shell word, so a ~ before it stands alone.
func roWordEnd(r rune) bool {
	return strings.ContainsRune(" \t\n|&;<>(){}", r)
}

func roDigits(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	return text != ""
}

func roNoFollow(args []string) (bool, string) {
	return roHeadTail("tail")(args)
}

func roPs(args []string) (bool, string) {
	for _, arg := range args {
		low := strings.ToLower(arg)
		if strings.Contains(low, "environ") ||
			(!strings.HasPrefix(arg, "-") && strings.ContainsRune(arg, 'e')) ||
			(strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") &&
				len(arg) > 2 && strings.ContainsRune(arg, 'e') &&
				strings.ContainsAny(arg, "auxw")) {
			return false, "`ps e` exposes process environments"
		}
	}
	return true, ""
}

func roRg(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-i", "-v", "-w", "-x", "-F", "-L", "-U", "-z", "-0", "-l", "-c", "-n", "--ignore-case", "--invert-match", "--word-regexp", "--line-regexp", "--fixed-strings", "--follow", "--multiline", "--multiline-dotall", "--null", "--files-with-matches", "--files-without-match", "--count", "--count-matches", "--line-number", "--no-line-number", "--column", "--byte-offset", "--heading", "--no-heading", "--with-filename", "--no-filename", "--hidden", "--no-ignore", "--no-ignore-vcs", "--no-messages", "--stats", "--json", "--pcre2", "--crlf", "--text", "--binary"},
		[]string{"-e", "--regexp", "-f", "--file", "-g", "--glob", "-t", "--type", "-T", "--type-not", "-A", "--after-context", "-B", "--before-context", "-C", "--context", "-m", "--max-count", "--max-depth", "--max-filesize", "--sort", "--sortr", "--color", "--colors", "--encoding", "--engine", "--path-separator", "--field-context-separator", "--field-match-separator", "--threads"},
	)
	_, reason := roParseFlags("rg", args, spec, false)
	return reason == "", reason
}

func roFd(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-H", "--hidden", "-I", "--no-ignore", "--no-ignore-vcs", "-s", "--case-sensitive", "-i", "--ignore-case", "-g", "--glob", "--regex", "-a", "--absolute-path", "-l", "--list-details", "-L", "--follow", "-p", "--full-path", "-0", "--print0", "-1", "--strip-cwd-prefix"},
		[]string{"-d", "--max-depth", "--min-depth", "-t", "--type", "-e", "--extension", "-E", "--exclude", "--size", "--changed-within", "--changed-before", "--owner", "--search-path"},
	)
	_, reason := roParseFlags("fd", args, spec, false)
	return reason == "", reason
}

func roSort(args []string) (bool, string) {
	spec := roFlags([]string{"-r", "-n", "-u", "-h", "-V", "-f"}, []string{"-k", "-t"})
	_, reason := roParseFlags("sort", args, spec, false)
	return reason == "", reason
}

func roUniq(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-c", "-d", "-D", "-i", "-u", "-z", "--count", "--repeated", "--all-repeated", "--ignore-case", "--unique", "--zero-terminated"},
		[]string{"-f", "-s", "-w", "--skip-fields", "--skip-chars", "--check-chars", "--group"},
	)
	positional, reason := roParseFlags("uniq", args, spec, false)
	if reason != "" {
		return false, reason
	}
	if len(positional) > 1 {
		return false, "`uniq IN OUT` writes the second file"
	}
	return true, ""
}

func roDate(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-u", "--utc", "--universal", "--rfc-email"},
		[]string{"-d", "--date", "-f", "--file", "-I", "--iso-8601", "-R", "--reference", "--resolution", "--rfc-3339"},
	)
	_, reason := roParseFlags("date", args, spec, false)
	return reason == "", reason
}

func roHostname(args []string) (bool, string) {
	if len(args) == 0 {
		return true, ""
	}
	return false, "`hostname` with arguments changes the name"
}

func roCommand(args []string) (bool, string) {
	if len(args) >= 2 && (args[0] == "-v" || args[0] == "-V") {
		for _, arg := range args[1:] {
			if strings.HasPrefix(arg, "-") {
				return false, "`command` uses an unsupported flag"
			}
		}
		return true, ""
	}
	return false, "`command` runs its argument"
}

func roXxd(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-a", "-b", "-C", "-d", "-e", "-i", "-p", "-u", "-E", "--autoskip", "--bits", "--capitalize", "--decimal", "--little-endian", "--include", "--plain", "--uppercase"},
		[]string{"-c", "-g", "-l", "-o", "-s", "--cols", "--groupsize", "--len", "--offset", "--seek"},
	)
	positional, reason := roParseFlags("xxd", args, spec, false)
	if reason != "" {
		return false, reason
	}
	if len(positional) > 1 {
		return false, "`xxd IN OUT` writes the second file"
	}
	return true, ""
}

func roBat(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-p", "-n", "-A", "-P", "-H", "--plain", "--number", "--show-all", "--show-nonprintable", "--no-config", "--force-colorization"},
		[]string{"--paging", "--style", "--color", "--decorations", "--italic-text", "--theme", "--language", "--tabs", "--wrap", "--terminal-width", "--line-range", "--highlight-line", "--map-syntax"},
	)
	pagingNever := false
	for i, arg := range args {
		switch {
		case arg == "-p" || arg == "-pp":
			pagingNever = true
		case arg == "--paging=never":
			pagingNever = true
		case arg == "--paging":
			if i+1 >= len(args) || args[i+1] != "never" {
				return false, "`bat --paging` must be `never`"
			}
			pagingNever = true
		case strings.HasPrefix(arg, "--paging="):
			return false, "`bat --paging` must be `never`"
		}
	}
	_, reason := roParseFlags("bat", args, spec, false)
	if reason != "" {
		return false, reason
	}
	if !pagingNever {
		return false, "`bat` opens a pager; pass --paging=never"
	}
	return true, ""
}

func roTop(args []string) (bool, string) {
	positional, reason := roParseFlags("top", args, roFlags([]string{"-b"}, []string{"-n"}), false)
	if reason != "" || len(positional) != 0 {
		if reason != "" {
			return false, reason
		}
		return false, "`top` uses unsupported arguments"
	}
	hasBatch, count := false, 0
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-b":
			hasBatch = true
		case args[i] == "-n" && i+1 < len(args):
			count, _ = strconv.Atoi(args[i+1])
			i++
		case strings.HasPrefix(args[i], "-n") && len(args[i]) > 2:
			count, _ = strconv.Atoi(args[i][2:])
		}
	}
	if hasBatch && count > 0 && count <= 100 {
		return true, ""
	}
	return false, "`top` requires -b with -n between 1 and 100"
}

func roAwk(args []string) (bool, string) {
	positional, reason := roParseFlags("awk", args, roFlags(nil, []string{"-F", "-v"}), false)
	if reason != "" {
		return false, reason
	}
	if len(positional) == 0 {
		return false, "`awk` needs a program to inspect"
	}
	program := positional[0]
	compact := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(program, " ", ""), "\t", ""))
	for _, forbidden := range []string{">", "environ", "system", "getline", "|", "close(", "fflush(", "@include", "@load"} {
		if strings.Contains(compact, forbidden) {
			return false, "`awk` program may write or run a command"
		}
	}
	return true, ""
}

func roSed(args []string) (bool, string) {
	var scripts, positional []string
	haveExpr := false
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-i" || a == "--in-place" || strings.HasPrefix(a, "--in-place=") ||
			(!strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-i")):
			return false, "`sed -i` edits files in place"
		case a == "-f" || a == "--file" || strings.HasPrefix(a, "--file="):
			return false, "`sed -f` runs a script file"
		case a == "-e":
			if i+1 < len(args) {
				scripts = append(scripts, args[i+1])
				haveExpr = true
				i += 2
				continue
			}
			i++
		case strings.HasPrefix(a, "--expression="):
			scripts = append(scripts, strings.TrimPrefix(a, "--expression="))
			haveExpr = true
			i++
		case a == "-n" || a == "-E" || a == "-r" || a == "-s" || a == "-u" || a == "-z" ||
			a == "--quiet" || a == "--silent" || a == "--regexp-extended" ||
			a == "--separate" || a == "--unbuffered" || a == "--null-data":
			i++
		case !strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-") && a != "-" && roSedShortOK(a):
			i++
		case strings.HasPrefix(a, "-") && a != "-":
			return false, "`sed` uses an unsupported flag"
		default:
			positional = append(positional, a)
			i++
		}
	}
	if !haveExpr {
		if len(positional) == 0 {
			return false, "`sed` without a script"
		}
		scripts = append(scripts, positional[0])
	}
	for _, s := range scripts {
		if !roSedScriptSafe(s) {
			return false, "`sed` script does more than print"
		}
	}
	return true, ""
}

func roSedShortOK(a string) bool {
	for _, c := range a[1:] {
		if !strings.ContainsRune("nErsuz", c) {
			return false
		}
	}
	return true
}

// roSedScriptSafe accepts a sed script that is only addresses plus the print
// commands p, = and l. Any other command (s, w, e, r, y, d, q, ...) means the
// script can write, run, or transform, so it asks.
func roSedScriptSafe(script string) bool {
	r := []rune(script)
	n := len(r)
	i := 0

	spaces := func() {
		for i < n && (r[i] == ' ' || r[i] == '\t') {
			i++
		}
	}
	delimited := func(delim rune) bool {
		for i < n {
			if r[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if r[i] == delim {
				i++
				return true
			}
			i++
		}
		return false
	}
	digits := func() bool {
		if i >= n || r[i] < '0' || r[i] > '9' {
			return false
		}
		for i < n && r[i] >= '0' && r[i] <= '9' {
			i++
		}
		return true
	}
	// parseAddr returns whether an address was present and whether it parsed.
	parseAddr := func() (present, ok bool) {
		if i >= n {
			return false, true
		}
		switch {
		case r[i] >= '0' && r[i] <= '9':
			digits()
			if i < n && r[i] == '~' { // first~step
				i++
				if !digits() {
					return true, false
				}
			}
			return true, true
		case r[i] == '$':
			i++
			return true, true
		case r[i] == '/':
			i++
			if !delimited('/') {
				return true, false
			}
			for i < n && (r[i] == 'I' || r[i] == 'M') {
				i++
			}
			return true, true
		case r[i] == '\\': // \cREGEXc custom delimiter
			i++
			if i >= n {
				return true, false
			}
			delim := r[i]
			i++
			if !delimited(delim) {
				return true, false
			}
			for i < n && (r[i] == 'I' || r[i] == 'M') {
				i++
			}
			return true, true
		}
		return false, true
	}

	for {
		for i < n && (r[i] == ';' || r[i] == '\n' || r[i] == ' ' || r[i] == '\t') {
			i++
		}
		if i >= n {
			return true
		}
		present, ok := parseAddr()
		if !ok {
			return false
		}
		if present {
			spaces()
			if i < n && r[i] == ',' {
				i++
				spaces()
				if i < n && (r[i] == '+' || r[i] == '~') {
					i++
					if !digits() {
						return false
					}
				} else {
					p2, ok2 := parseAddr()
					if !ok2 || !p2 {
						return false
					}
				}
			}
		}
		spaces()
		for i < n && r[i] == '!' {
			i++
			spaces()
		}
		if i >= n {
			return false
		}
		switch r[i] {
		case 'p', '=':
			i++
		case 'l':
			i++
			digits()
		default:
			return false
		}
		spaces()
		if i < n && r[i] != ';' && r[i] != '\n' {
			return false
		}
	}
}

func roFind(args []string) (bool, string) {
	allowed := map[string]bool{
		"-H": true, "-L": true, "-P": true, "-D": true, "-O": true,
		"-amin": true, "-anewer": true, "-atime": true, "-cmin": true, "-cnewer": true, "-ctime": true,
		"-empty": true, "-executable": true, "-false": true, "-fstype": true, "-gid": true, "-group": true,
		"-ilname": true, "-iname": true, "-inum": true, "-ipath": true, "-iregex": true, "-iwholename": true,
		"-links": true, "-lname": true, "-maxdepth": true, "-mindepth": true, "-mmin": true, "-mount": true,
		"-mtime": true, "-name": true, "-newer": true, "-newerXY": true, "-nogroup": true, "-nouser": true,
		"-path": true, "-perm": true, "-readable": true, "-regex": true, "-regextype": true, "-samefile": true,
		"-size": true, "-true": true, "-type": true, "-uid": true, "-used": true, "-user": true,
		"-wholename": true, "-writable": true, "-xdev": true, "-xtype": true,
		"-print": true, "-print0": true, "-printf": true, "-ls": true, "-prune": true, "-quit": true,
		"-a": true, "-and": true, "-o": true, "-or": true, "-not": true,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-exec" || arg == "-execdir" {
			end := i + 1
			for end < len(args) && args[end] != ";" && args[end] != "+" {
				end++
			}
			if end == len(args) {
				return false, "`find " + arg + "` has no end"
			}
			if ok, reason := roRunsOnNames("find "+arg, args[i+1:end]); !ok {
				return false, reason
			}
			i = end
			continue
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		if _, err := strconv.Atoi(arg); err == nil {
			continue
		}
		if strings.HasPrefix(arg, "-O") && len(arg) > 2 {
			if _, err := strconv.Atoi(arg[2:]); err == nil {
				continue
			}
		}
		if !allowed[arg] {
			return false, "`find " + arg + "` is not an allowed read-only expression"
		}
	}
	return true, ""
}

// roOnNames are the tools find -exec and xargs may hand names to: each says
// something about a file (its size, kind, owner, checksum) and none prints
// what is in it, so a walk over a whole tree cannot spill a secret.
var roOnNames = map[string]bool{
	"du": true, "ls": true, "stat": true, "file": true, "wc": true,
	"basename": true, "dirname": true, "realpath": true, "readlink": true,
	"echo": true, "printf": true, "test": true,
	"md5sum": true, "sha1sum": true, "sha256sum": true, "b2sum": true,
}

// roRunsOnNames admits the command a walker runs on what it found, by the
// same rules that command meets when it stands alone.
func roRunsOnNames(walker string, argv []string) (bool, string) {
	if len(argv) == 0 {
		return false, "`" + walker + "` runs nothing"
	}
	if !roOnNames[argv[0]] {
		return false, "`" + walker + " " + argv[0] + "` may print or change what it is handed"
	}
	return roCheckSegment(argv)
}

func roXargs(args []string) (bool, string) {
	rest, reason := roParseFlags("xargs", args, roFlags(
		[]string{"-0", "--null", "-r", "--no-run-if-empty", "-t", "--verbose", "-x", "--exit"},
		[]string{"-d", "--delimiter", "-n", "--max-args", "-L", "--max-lines", "-P", "--max-procs", "-s", "--max-chars", "-I", "--replace", "-E"}), true)
	if reason != "" {
		return false, reason
	}
	if len(rest) == 0 {
		return true, ""
	}
	return roRunsOnNames("xargs", rest)
}

func roTree(args []string) (bool, string) {
	for _, a := range args {
		if a == "-o" || a == "--output" || strings.HasPrefix(a, "--output=") || roShortHas(a, 'o') {
			return false, "`tree -o` writes a file"
		}
	}
	return true, ""
}

func roSs(args []string) (bool, string) {
	for _, a := range args {
		if a == "-K" || a == "--kill" || roShortHas(a, 'K') {
			return false, "`ss -K` closes sockets"
		}
	}
	return true, ""
}

func roSensors(args []string) (bool, string) {
	for _, a := range args {
		if a == "-s" || a == "--set" || roShortHas(a, 's') {
			return false, "`sensors -s` sets limits"
		}
	}
	return true, ""
}

func roDmidecode(args []string) (bool, string) {
	for _, a := range args {
		if a == "--dump-bin" || strings.HasPrefix(a, "--dump-bin") {
			return false, "`dmidecode --dump-bin` writes a file"
		}
	}
	return true, ""
}

func roJournalctl(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-a", "-b", "-e", "-k", "-m", "-q", "-r", "-x", "--all", "--boot", "--pager-end", "--dmesg", "--merge", "--quiet", "--reverse", "--catalog", "--no-pager", "--no-full", "--utc", "--list-boots", "--disk-usage", "--verify", "--system", "--user"},
		[]string{"-n", "-u", "-t", "-p", "-o", "-S", "-U", "-g", "--lines", "--unit", "--user-unit", "--identifier", "--priority", "--output", "--since", "--until", "--grep", "--directory", "--file", "--root", "--image", "--namespace"},
	)
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		if len(arg) > 1 && arg[0] == '-' {
			if _, err := strconv.Atoi(arg); err == nil {
				normalized = append(normalized, "boot-offset")
				continue
			}
		}
		normalized = append(normalized, arg)
	}
	_, reason := roParseFlags("journalctl", normalized, spec, false)
	return reason == "", reason
}

func roSystemctl(args []string) (bool, string) {
	spec := roFlags(
		[]string{"--user", "--system", "--no-pager", "--no-legend", "--plain", "--full", "--all", "--reverse", "--failed", "--runtime", "--global", "--quiet"},
		[]string{"-p", "--property", "-t", "--type", "-s", "--state", "-o", "--output", "-H", "--host", "-M", "--machine", "-n", "--lines", "--root"},
	)
	parts, reason := roParseFlags("systemctl", args, spec, false)
	if reason != "" {
		return false, reason
	}
	if len(parts) == 0 {
		return true, ""
	}
	switch parts[0] {
	case "status", "show", "cat", "list-units", "list-unit-files", "list-timers",
		"list-sockets", "list-dependencies", "list-jobs", "is-active",
		"is-enabled", "is-failed", "is-system-running", "get-default":
		return true, ""
	}
	return false, "`systemctl " + parts[0] + "` is not a read-only command"
}

func roLoginctl(args []string) (bool, string) {
	spec := roFlags(
		[]string{"--no-pager", "--no-legend", "--full", "--all", "--value"},
		[]string{"-p", "--property", "-H", "--host", "-M", "--machine", "-n", "--lines", "-o", "--output", "-s", "--signal"},
	)
	parts, reason := roParseFlags("loginctl", args, spec, false)
	if reason != "" {
		return false, reason
	}
	if len(parts) == 0 || strings.HasPrefix(parts[0], "list-") || strings.HasPrefix(parts[0], "show-") ||
		parts[0] == "session-status" || parts[0] == "user-status" || parts[0] == "seat-status" {
		return true, ""
	}
	return false, "`loginctl " + parts[0] + "` is not a read-only command"
}

func roCoredumpctl(args []string) (bool, string) {
	spec := roFlags(
		[]string{"--no-pager", "--no-legend", "--reverse", "--json", "-q", "--quiet"},
		[]string{"-F", "--field", "-n", "--since", "--until", "--root", "--image"},
	)
	parts, reason := roParseFlags("coredumpctl", args, spec, false)
	if reason != "" {
		return false, reason
	}
	if len(parts) == 0 || parts[0] == "list" || parts[0] == "info" {
		return true, ""
	}
	return false, "`coredumpctl " + parts[0] + "` is not a read-only command"
}

func roPacman(args []string) (bool, string) {
	op := ""
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			op = a
			break
		}
	}
	if op == "" {
		return false, "needs a query operation"
	}
	if op == "-V" || op == "--version" {
		return true, ""
	}
	if strings.HasPrefix(op, "--") {
		switch op {
		case "--query", "--search", "--info":
			return true, ""
		}
		return false, "`" + op + "` is not a query"
	}
	mods := op[2:]
	switch op[1] {
	case 'Q':
		return true, ""
	case 'S':
		if mods == "" {
			return false, "`-S` installs packages"
		}
		for _, c := range mods {
			if !strings.ContainsRune("silg", c) {
				return false, "`-S" + string(c) + "` is not a query"
			}
		}
		return true, ""
	case 'F':
		for _, c := range mods {
			if !strings.ContainsRune("lxoq", c) {
				return false, "`-F" + string(c) + "` is not a query"
			}
		}
		return true, ""
	case 'D':
		if mods == "" {
			return false, "`-D` changes the database"
		}
		for _, c := range mods {
			if c != 'k' {
				return false, "`-D" + string(c) + "` changes the database"
			}
		}
		return true, ""
	}
	return false, "`" + op + "` is not a query"
}

func roDNF(args []string) (bool, string) {
	for _, arg := range args {
		flag := strings.SplitN(arg, "=", 2)[0]
		switch flag {
		case "--setopt", "--config", "-c", "--installroot", "--pluginpath",
			"--enableplugin", "--disableplugin", "--exec", "--downloaddir",
			"--destdir", "--downloadonly", "--download":
			return false, "`dnf " + flag + "` can load code, change configuration, or write files"
		}
		if strings.HasPrefix(arg, "-c") && !strings.HasPrefix(arg, "--") {
			return false, "`dnf -c` loads an alternate configuration"
		}
	}
	parts := roNonFlags(args)
	if len(parts) == 0 {
		return false, "needs a read-only subcommand"
	}
	switch parts[0] {
	case "list", "info", "search", "repoquery", "check-update":
		return true, ""
	case "history":
		if len(parts) == 1 || parts[1] == "list" || parts[1] == "info" || parts[1] == "userinstalled" {
			return true, ""
		}
	}
	return false, "`dnf " + parts[0] + "` is not a read-only command"
}

func roRPM(args []string) (bool, string) {
	query := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		flag := strings.SplitN(arg, "=", 2)[0]
		switch flag {
		case "--eval", "-E", "--define", "-D", "--undefine", "--load", "--macros",
			"--rcfile", "--rebuild", "--rebuilddb", "--resign", "--addsign",
			"--import", "--initdb", "--install", "--upgrade", "--freshen",
			"--erase", "--restore", "--dbpath", "--root", "--pipe":
			return false, "`rpm " + flag + "` can execute macros, load configuration, or change the RPM database"
		}
		if strings.HasPrefix(flag, "--delete") || strings.HasPrefix(flag, "--set") ||
			strings.HasPrefix(arg, "-U") || strings.HasPrefix(arg, "-F") ||
			strings.HasPrefix(arg, "-e") || strings.HasPrefix(arg, "-D") ||
			strings.HasPrefix(arg, "-i") && arg != "-i" {
			return false, "`rpm " + flag + "` is not a query"
		}
		switch {
		case arg == "-q" || arg == "--query":
			query = true
		case arg == "-qa" || arg == "-qi" || arg == "-ql" || arg == "-qf":
			query = true
		case arg == "-a" || arg == "--all" || arg == "-i" || arg == "--info" ||
			arg == "-l" || arg == "--list" || arg == "-f" || arg == "--file" ||
			arg == "--whatprovides" || arg == "--whatrequires" ||
			arg == "--requires" || arg == "--provides" || arg == "--changelog" ||
			arg == "--filesbypkg" || arg == "--scripts" || arg == "--triggers" ||
			arg == "--dump" || arg == "--docfiles" || arg == "--configfiles":
		case arg == "--qf" || arg == "--queryformat":
			if i+1 >= len(args) || !roSafeRPMQueryFormat(args[i+1]) {
				return false, "unsafe or missing RPM query format"
			}
			i++
		case strings.HasPrefix(arg, "--qf=") || strings.HasPrefix(arg, "--queryformat="):
			if !roSafeRPMQueryFormat(strings.SplitN(arg, "=", 2)[1]) {
				return false, "unsafe RPM query format"
			}
		case strings.HasPrefix(arg, "-"):
			return false, "`rpm " + arg + "` is not an allowed query flag"
		}
	}
	if !query {
		return false, "needs `-q` or `--query`"
	}
	return true, ""
}

func roSafeRPMQueryFormat(format string) bool {
	if strings.Contains(format, "%(") || strings.Contains(format, "%[") || strings.Contains(format, "%|") {
		return false
	}
	for rest := format; ; {
		start := strings.Index(rest, "%{")
		if start < 0 {
			return true
		}
		rest = rest[start+2:]
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			return false
		}
		tag := strings.TrimPrefix(rest[:end], "=")
		name, suffix, _ := strings.Cut(tag, ":")
		if name == "" || strings.Trim(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") != "" {
			return false
		}
		if suffix != "" {
			switch suffix {
			case "arraysize", "date", "day", "depflags", "hex", "humaniec", "humansi", "json", "octal", "perms", "shescape", "xml":
			default:
				return false
			}
		}
		rest = rest[end+1:]
	}
}

func roXBPSQuery(args []string) (bool, string) {
	for _, arg := range args {
		flag := strings.SplitN(arg, "=", 2)[0]
		if flag == "-r" || flag == "--rootdir" || flag == "-C" || flag == "--config" ||
			strings.HasPrefix(arg, "-r") && !strings.HasPrefix(arg, "--") ||
			strings.HasPrefix(arg, "-C") && !strings.HasPrefix(arg, "--") {
			return false, "`xbps-query " + flag + "` reads an alternate root or configuration"
		}
	}
	return true, ""
}

func roIP(args []string) (bool, string) {
	rest := roNonFlags(args)
	if len(rest) == 0 {
		return false, "`ip` needs an object"
	}
	switch rest[0] {
	case "addr", "address", "a", "link", "l", "route", "r", "neigh", "rule":
	default:
		return false, "`ip " + rest[0] + "` is not a read-only object"
	}
	if len(rest) == 1 {
		return true, ""
	}
	switch rest[1] {
	case "show", "list", "get":
		return true, ""
	}
	return false, "`ip " + rest[0] + " " + rest[1] + "` changes the network"
}

func roNmcli(args []string) (bool, string) {
	spec := roFlags(
		[]string{"-t", "-p", "--terse", "--pretty"},
		[]string{"-f", "--fields", "-g", "--get-values", "-e", "--escape", "-c", "--colors", "-m", "--mode", "-w", "--wait"},
	)
	rest, reason := roParseFlags("nmcli", args, spec, false)
	if reason != "" {
		return false, reason
	}
	if len(rest) == 0 {
		return true, ""
	}
	switch rest[0] {
	case "general", "g":
		if len(rest) == 1 || len(rest) == 2 && rest[1] == "status" {
			return true, ""
		}
	case "device", "dev", "d":
		if len(rest) == 1 || rest[1] == "status" || rest[1] == "show" {
			return true, ""
		}
		if (rest[1] == "wifi" || rest[1] == "w") && (len(rest) == 2 || len(rest) == 3 && rest[2] == "list") {
			return true, ""
		}
	case "connection", "con", "c":
		if len(rest) == 1 || len(rest) >= 2 && rest[1] == "show" {
			return true, ""
		}
	case "radio", "r":
		if len(rest) == 1 {
			return true, ""
		}
	}
	return false, "`nmcli " + strings.Join(rest, " ") + "` is not read-only"
}

func roResolvectl(args []string) (bool, string) {
	rest := roNonFlags(args)
	if len(rest) == 0 {
		return true, ""
	}
	switch rest[0] {
	case "status", "query", "statistics":
		return true, ""
	case "dns", "domain":
		if len(rest) <= 2 {
			return true, ""
		}
		return false, "`resolvectl " + rest[0] + "` with values sets DNS"
	}
	return false, "`resolvectl " + rest[0] + "` is not read-only"
}

func roStatusShow(args []string) (bool, string) {
	verb := roFirstVerb(args)
	if verb == "" || verb == "status" || verb == "show" || strings.HasPrefix(verb, "list-") {
		return true, ""
	}
	return false, "`" + verb + "` changes system state"
}

func roSystemdAnalyze(args []string) (bool, string) {
	verb := roFirstVerb(args)
	switch verb {
	case "", "blame", "critical-chain", "time", "cat-config", "calendar",
		"timespan", "timestamp", "verify", "security", "unit-paths", "dot", "plot":
		return true, ""
	}
	return false, "`systemd-analyze " + verb + "` is not read-only"
}

func roDmesg(args []string) (bool, string) {
	for _, a := range args {
		switch a {
		case "-C", "-c", "-D", "-E", "-n", "--clear", "--read-clear":
			return false, "`dmesg " + a + "` changes the ring buffer"
		}
		if strings.HasPrefix(a, "--console-") {
			return false, "`dmesg " + a + "` changes the console"
		}
		if strings.HasPrefix(a, "-n") && a != "-n" {
			return false, "`dmesg -n` changes the log level"
		}
	}
	return true, ""
}

func roFuser(args []string) (bool, string) {
	for _, a := range args {
		if a == "-k" || a == "--kill" || roShortHas(a, 'k') {
			return false, "`fuser -k` kills processes"
		}
	}
	return true, ""
}

func roMount(args []string) (bool, string) {
	if len(args) == 0 {
		return true, ""
	}
	return false, "`mount` with arguments changes mounts"
}

func roSwapon(args []string) (bool, string) {
	if len(args) == 0 {
		return false, "`swapon` enables swap"
	}
	for _, a := range args {
		if a != "--show" && !strings.HasPrefix(a, "--show=") {
			return false, "`swapon` without --show changes swap"
		}
	}
	return true, ""
}

func roZramctl(args []string) (bool, string) {
	for _, a := range args {
		if a != "--output" && !strings.HasPrefix(a, "--output=") {
			return false, "`zramctl` without --output changes zram"
		}
	}
	return true, ""
}

func roBtrfs(args []string) (bool, string) {
	rest := roNonFlags(args)
	if len(rest) < 2 {
		return false, "`btrfs` needs a read-only subcommand"
	}
	switch rest[0] {
	case "filesystem", "fi":
		if rest[1] == "show" || rest[1] == "usage" || rest[1] == "df" {
			return true, ""
		}
	case "subvolume", "subvol", "su":
		if rest[1] == "list" || rest[1] == "show" {
			return true, ""
		}
	case "device", "dev":
		if rest[1] == "stats" {
			return true, ""
		}
	case "scrub":
		if rest[1] == "status" {
			return true, ""
		}
	}
	return false, "`btrfs " + rest[0] + " " + rest[1] + "` is not read-only"
}

func roSnapper(args []string) (bool, string) {
	parts, reason := roParseFlags("snapper", args, roFlags(
		[]string{"--machine-readable", "--csvout", "--jsonout", "--utc"},
		[]string{"-c", "--config", "-t", "--table-style", "-r", "--root", "--separator"},
	), false)
	if reason != "" {
		return false, reason
	}
	if len(parts) == 0 {
		return false, "`snapper` needs a subcommand"
	}
	switch parts[0] {
	case "list", "get-config", "list-configs":
		return true, ""
	}
	return false, "`snapper " + parts[0] + "` is not read-only"
}

func roFlatpak(args []string) (bool, string) {
	verb := roFirstVerb(args)
	switch verb {
	case "list", "info", "ps", "remotes", "history":
		return true, ""
	}
	return false, "`flatpak " + verb + "` is not read-only"
}

func roDocker(args []string) (bool, string) {
	parts, reason := roParseFlags("docker", args, roFlags(
		[]string{"-D", "--debug"},
		[]string{"-l", "--log-level", "-H", "--host", "--context"},
	), true)
	if reason != "" {
		return false, reason
	}
	if len(parts) == 0 {
		return false, "`docker` needs a read-only subcommand"
	}
	verb, rest := parts[0], parts[1:]
	var spec roFlagSpec
	requireNoStream := false
	switch verb {
	case "ps":
		spec = roFlags([]string{"-a", "--all", "-q", "--quiet", "-s", "--size", "--no-trunc"}, []string{"-f", "--filter", "--format", "-n", "--last"})
	case "images":
		spec = roFlags([]string{"-a", "--all", "-q", "--quiet", "--digests", "--no-trunc"}, []string{"-f", "--filter", "--format"})
	case "inspect":
		spec = roFlags([]string{"-s", "--size"}, []string{"-f", "--format", "--type"})
	case "version", "info":
		spec = roFlags(nil, []string{"-f", "--format"})
	case "top":
		spec = roFlags(nil, nil)
	case "logs":
		spec = roFlags([]string{"-t", "--timestamps", "--details"}, []string{"--since", "--tail", "--until"})
	case "stats":
		spec = roFlags([]string{"-a", "--all", "--no-stream", "--no-trunc"}, []string{"--format"})
		requireNoStream = true
	default:
		return false, "`" + verb + "` is not a read-only command"
	}
	_, reason = roParseFlags("docker "+verb, rest, spec, false)
	if reason != "" {
		return false, reason
	}
	if requireNoStream {
		for _, arg := range rest {
			if arg == "--no-stream" {
				return true, ""
			}
		}
		return false, "`stats` streams forever without --no-stream"
	}
	return true, ""
}

func roPactl(args []string) (bool, string) {
	verb := roFirstVerb(args)
	if verb == "info" || verb == "list" || verb == "stat" || strings.HasPrefix(verb, "get-") {
		return true, ""
	}
	return false, "`pactl " + verb + "` changes audio state"
}

func roWpctl(args []string) (bool, string) {
	verb := roFirstVerb(args)
	switch verb {
	case "status", "inspect", "get-volume":
		return true, ""
	}
	return false, "`wpctl " + verb + "` changes audio state"
}

func roPwCli(args []string) (bool, string) {
	verb := roFirstVerb(args)
	switch verb {
	case "info", "ls", "list-objects":
		return true, ""
	}
	return false, "`pw-cli " + verb + "` is not read-only"
}

func roUdevadm(args []string) (bool, string) {
	verb := roFirstVerb(args)
	if verb == "info" {
		return true, ""
	}
	return false, "`udevadm " + verb + "` is not read-only"
}

func roEfibootmgr(args []string) (bool, string) {
	for _, a := range args {
		if a == "-v" || a == "--verbose" {
			continue
		}
		return false, "`efibootmgr " + a + "` changes boot entries"
	}
	return true, ""
}

func roBootctl(args []string) (bool, string) {
	verb := roFirstVerb(args)
	if verb == "" || verb == "status" || verb == "list" {
		return true, ""
	}
	return false, "`bootctl " + verb + "` changes the bootloader"
}

func roFwupdmgr(args []string) (bool, string) {
	verb := roFirstVerb(args)
	switch verb {
	case "get-devices", "get-updates", "get-history", "get-remotes":
		return true, ""
	}
	return false, "`fwupdmgr " + verb + "` is not read-only"
}

func roGsettings(args []string) (bool, string) {
	verb := roFirstVerb(args)
	if verb == "get" || verb == "describe" || verb == "range" || strings.HasPrefix(verb, "list-") {
		return true, ""
	}
	return false, "`gsettings " + verb + "` changes settings"
}

func roDconf(args []string) (bool, string) {
	verb := roFirstVerb(args)
	switch verb {
	case "read", "list", "dump":
		return true, ""
	}
	return false, "`dconf " + verb + "` changes settings"
}

func roXdgMime(args []string) (bool, string) {
	verb := roFirstVerb(args)
	if verb == "query" {
		return true, ""
	}
	return false, "`xdg-mime " + verb + "` changes associations"
}

func roNvidiaSmi(args []string) (bool, string) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-L" || a == "-q":
			i++
		case a == "-d" || a == "-i":
			i += 2
		case strings.HasPrefix(a, "-d") || strings.HasPrefix(a, "-i") ||
			strings.HasPrefix(a, "--query") || strings.HasPrefix(a, "--format="):
			i++
		default:
			return false, "`nvidia-smi " + a + "` is not a read-only query"
		}
	}
	return true, ""
}

func roGit(args []string) (bool, string) {
	parts, reason := roParseFlags("git", args, roFlags([]string{"--no-pager"}, []string{"-C"}), true)
	if reason != "" {
		return false, reason
	}
	if len(parts) == 0 {
		return false, "`git` needs a subcommand"
	}
	sub, rest := parts[0], parts[1:]
	parse := func(spec roFlagSpec, allowPositional bool) (bool, string) {
		positional, why := roParseFlags("git "+sub, rest, spec, false)
		if why != "" {
			return false, why
		}
		if !allowPositional && len(positional) != 0 {
			return false, "`git " + sub + "` with a name may modify refs"
		}
		return true, ""
	}
	switch sub {
	case "status":
		return parse(roFlags(
			[]string{"-s", "-b", "--short", "--branch", "--show-stash", "--no-renames", "--long", "-z"},
			[]string{"--porcelain", "-u", "--untracked-files", "--ignored", "--find-renames", "--column"},
		), true)
	case "rev-parse":
		return parse(roFlags(
			[]string{"--parseopt", "--sq-quote", "--revs-only", "--no-revs", "--flags", "--no-flags", "--default", "--verify", "-q", "--quiet", "--short", "--symbolic", "--symbolic-full-name", "--abbrev-ref", "--show-toplevel", "--show-prefix", "--show-cdup", "--show-superproject-working-tree", "--is-inside-git-dir", "--is-inside-work-tree", "--is-bare-repository", "--is-shallow-repository", "--show-object-format", "--show-ref-format", "--end-of-options"},
			[]string{"--path-format", "--git-path", "--resolve-git-dir", "--exclude-hidden"},
		), true)
	case "ls-files":
		return parse(roFlags(
			[]string{"-c", "--cached", "-d", "--deleted", "-m", "--modified", "-o", "--others", "-i", "--ignored", "-s", "--stage", "-u", "--unmerged", "-k", "--killed", "-v", "-t", "--full-name", "--recurse-submodules", "--error-unmatch", "--with-tree", "-z"},
			[]string{"-x", "--exclude", "-X", "--exclude-from", "--exclude-per-directory", "--format"},
		), true)
	case "describe":
		return parse(roFlags(
			[]string{"--all", "--tags", "--contains", "--always", "--long", "--exact-match", "--debug", "--broken", "--dirty"},
			[]string{"--match", "--exclude", "--candidates", "--abbrev"},
		), true)
	case "blame":
		return parse(roFlags(
			[]string{"-b", "--root", "--show-stats", "--progress", "--score-debug", "-f", "--show-name", "-n", "--show-number", "-p", "--porcelain", "--line-porcelain", "-s", "--suppress-author", "-e", "--show-email", "-w", "--ignore-rev", "--ignore-revs-file", "--color-lines", "--color-by-age", "--first-parent"},
			[]string{"-L", "--abbrev", "--contents", "--date"},
		), true)
	case "log", "diff", "show", "shortlog":
		normalized := make([]string, 0, len(rest))
		for _, arg := range rest {
			if len(arg) > 1 && arg[0] == '-' {
				if _, err := strconv.Atoi(arg); err == nil {
					continue
				}
			}
			normalized = append(normalized, arg)
		}
		rest = normalized
		return parse(roFlags(
			[]string{"-p", "-u", "--patch", "--no-patch", "--stat", "--numstat", "--shortstat", "--dirstat", "--summary", "--name-only", "--name-status", "--color", "--no-color", "--no-renames", "--full-index", "--binary", "--abbrev-commit", "--oneline", "--graph", "--decorate", "--no-decorate", "--follow", "--first-parent", "--reverse", "--no-ext-diff", "--no-textconv", "--raw", "--minimal", "-w", "--ignore-all-space", "-b", "--ignore-space-change", "--ignore-blank-lines", "--check", "--quiet", "--exit-code"},
			[]string{"--format", "--pretty", "--date", "--author", "--committer", "--grep", "--max-count", "-n", "--skip", "--since", "--after", "--until", "--before", "--diff-filter", "--find-renames", "--find-copies", "--submodule", "--word-diff", "--color-moved", "--relative"},
		), true)
	case "branch", "tag":
		return parse(roFlags(
			[]string{"-a", "-r", "-v", "--verbose", "--list", "-l", "--all", "--merged", "--no-merged", "--no-color", "--show-current"},
			[]string{"--color", "--sort", "--format", "--contains", "--no-contains", "--points-at", "--column"},
		), false)
	case "remote":
		if len(rest) == 0 || len(rest) == 1 && (rest[0] == "-v" || rest[0] == "--verbose") {
			return true, ""
		}
		if len(rest) >= 1 && rest[0] == "show" {
			return parse(roFlags([]string{"-n"}, nil), true)
		}
		return false, "`git remote` with that action changes remotes"
	}
	return false, "`git " + sub + "` is not a read-only command"
}

func roRyoku(args []string) (bool, string) {
	verb := roFirstVerb(args)
	switch verb {
	case "status", "version", "owner", "verify":
		return true, ""
	case "wm":
		rest := roNonFlags(args)
		if len(rest) >= 2 && rest[1] == "status" {
			return true, ""
		}
		return false, "`ryoku wm` with that action changes the compositor"
	case "doctor":
		for _, a := range args {
			if a == "--check" || a == "-n" || a == "--json" || a == "--dry-run" {
				return true, ""
			}
		}
		return false, "`ryoku doctor` runs reconcilers; add --check"
	}
	return false, "`ryoku " + verb + "` is not a read-only command"
}

func roRashin(args []string) (bool, string) {
	if len(args) == 0 || args[0] != "status" {
		return false, "`rashin` only allows the status command"
	}
	positional, reason := roParseFlags("rashin status", args[1:], roFlags([]string{"--json"}, nil), false)
	if reason != "" {
		return false, reason
	}
	if len(positional) != 0 {
		return false, "`rashin status` does not accept an action"
	}
	return true, ""
}

func roRyokuShell(args []string) (bool, string) {
	rest := roNonFlags(args)
	if len(rest) == 0 {
		return false, "`ryoku-shell` needs a read-only subcommand"
	}
	switch rest[0] {
	case "status":
		return true, ""
	case "dock":
		if len(rest) >= 2 && rest[1] == "list" {
			return true, ""
		}
		return false, "`ryoku-shell dock` with that action changes the dock"
	case "ipc":
		if roShellIpcRead(rest[1:]) {
			return true, ""
		}
		return false, "`ryoku-shell ipc` with that call can change the desktop"
	}
	return false, "`ryoku-shell " + rest[0] + "` is not a read-only command"
}

func roQs(args []string) (bool, string) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch a {
		case "-c", "--config", "-p", "--path", "-m", "--manifest", "--log-rules":
			i += 2
			continue
		}
		if strings.HasPrefix(a, "-") {
			i++
			continue
		}
		if a == "list" || a == "log" {
			return true, ""
		}
		if a == "ipc" {
			return false, "`qs ipc` bypasses the shell daemon; read through `ryoku-shell ipc`"
		}
		return false, "`qs " + a + "` is not a read-only command"
	}
	return false, "`qs` needs the list or log subcommand"
}

// any other function call can act on the desktop
func roShellIpcRead(rest []string) bool {
	if len(rest) == 3 && rest[0] == "get" {
		return true
	}
	if len(rest) != 3 || rest[0] != "call" || rest[1] != "qsbar" {
		return false
	}
	switch rest[2] {
	case "list", "catalog", "layout":
		return true
	}
	return false
}

// ReadOnly reports whether cmd can only inspect state and is safe for Ask to run.
func ReadOnly(cmd string) bool {
	ok, _ := readOnlyCommand(cmd)
	return ok
}

// WhyNotReadOnly says in a few words why ReadOnly turned cmd down, "" when it
// did not. A model told the real reason can ask another way; one told only
// that the command "may change the machine" repeats that to the user even
// when the command was turned down for reading a secret.
func WhyNotReadOnly(cmd string) string {
	if ok, reason := readOnlyCommand(cmd); !ok {
		return reason
	}
	return ""
}
