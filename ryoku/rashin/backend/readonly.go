package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
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
	case "read", "search", "think":
		for _, p := range roReadPaths(rawInput, title) {
			if m, bad := roSecretPath(p); bad {
				return false, "touches a secret (" + m + ")"
			}
		}
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

// roSecretFileMarkers name credential files and directories. A shell argument
// or redirect target containing one is a secret read, so an execute command
// that names one asks even though the command itself only reads.
var roSecretFileMarkers = []string{
	".ssh", ".gnupg", ".password-store", "keyrings", "id_rsa", "id_ed25519",
	".env", ".netrc", "/etc/shadow", "/etc/gshadow", "/etc/sudoers", ".hermes",
	"gateway/token", "logins.json", "key4.db", "cookies.sqlite",
	".docker/config.json", ".kube/config", "gh/hosts.yml", ".aws/credentials",
}

// roSecretWordMarkers are generic secret words. They only gate the read and
// search tool kinds (a query field), never the execute allowlist, where a
// grep for the word "token" is an everyday, harmless command.
var roSecretWordMarkers = []string{"credentials", "secrets", "token"}

func roPathSep(r rune) bool {
	return r == '/' || r == '\\' || r == ' ' || r == '\t' || r == '\n' || r == ':' || r == ','
}

// roSecretFile reports whether a path names a credential file, by strong
// name markers and the .pem/.key suffix. Safe to match anywhere because the
// markers are specific enough not to trip an ordinary word.
func roSecretFile(p string) (marker string, secret bool) {
	low := strings.ToLower(p)
	for _, m := range roSecretFileMarkers {
		if strings.Contains(low, m) {
			return m, true
		}
	}
	for _, seg := range strings.FieldsFunc(low, roPathSep) {
		if strings.HasSuffix(seg, ".pem") || strings.HasSuffix(seg, ".key") {
			return seg, true
		}
	}
	return "", false
}

// roSecretPath is the read/search gate: the file markers plus the generic
// secret words. It stays permissive about the daemon's own rashin.json, which
// carries only a port and matches no marker.
func roSecretPath(p string) (marker string, secret bool) {
	if m, bad := roSecretFile(p); bad {
		return m, true
	}
	low := strings.ToLower(p)
	for _, m := range roSecretWordMarkers {
		if strings.Contains(low, m) {
			return m, true
		}
	}
	return "", false
}

// roReadPaths pulls every path or pattern a read/search tool call names, so a
// secret in any of them is caught. It falls back to the title when the input
// carries none.
func roReadPaths(raw json.RawMessage, title string) []string {
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
	if len(out) == 0 && strings.TrimSpace(title) != "" {
		out = append(out, title)
	}
	return out
}

// roFetchReadOnly allows a web fetch that is a plain GET with no body. A method
// other than GET, or any request body or upload, changes the far end and asks.
func roFetchReadOnly(raw json.RawMessage) (bool, string) {
	m := roDecode(raw)
	if s, ok := roString(m, "method"); ok {
		if u := strings.ToUpper(strings.TrimSpace(s)); u != "" && u != "GET" {
			return false, "`" + u + "` is not a plain read"
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
		stripped := roStripAssignments(argv)
		if len(stripped) == 0 {
			return false, "sets variables without running a command"
		}
		checked = true
		if m, bad := roArgvSecret(stripped); bad {
			return false, "reads a secret (" + m + ")"
		}
		if roRecursiveSecretSearch(stripped) {
			return false, "searches the home directory for secrets"
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

// roArgvSecret reports whether any argument of a segment names a credential
// file, checking the value part of a --flag=value and skipping the command
// word and bare flags.
func roArgvSecret(argv []string) (string, bool) {
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

// roArgToken returns the path-bearing part of an argument: a positional word as
// is, or the value of a --flag=value. Bare flags carry no path.
func roArgToken(a string) (string, bool) {
	if strings.HasPrefix(a, "--") {
		if eq := strings.IndexByte(a, '='); eq >= 0 {
			return a[eq+1:], true
		}
		return "", false
	}
	if strings.HasPrefix(a, "-") && a != "-" {
		return "", false
	}
	return a, true
}

// roRecursiveSecretSearch reports a grep or rg that recursively scans the home
// directory for a secret word. Reading one named file is fine; sweeping
// ~/.config or ~ for "token" or "password" harvests every credential at once.
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

// roSearchPatternTargets splits grep/rg arguments into the search pattern and
// the paths it searches, honouring -e/--regexp and skipping flags.
func roSearchPatternTargets(args []string) (pattern string, targets []string) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-e" || a == "--regexp":
			if i+1 < len(args) && pattern == "" {
				pattern = args[i+1]
			}
			i += 2
		case strings.HasPrefix(a, "--regexp="):
			if pattern == "" {
				pattern = strings.TrimPrefix(a, "--regexp=")
			}
			i++
		case a == "-f" || a == "--file":
			i += 2 // pattern comes from a file; leave pattern empty
		case strings.HasPrefix(a, "-") && a != "-":
			i++
		default:
			if pattern == "" {
				pattern = a
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
				if runes[i] == '`' || (runes[i] == '$' && i+1 < n && runes[i+1] == '(') {
					return "", i, "runs a command substitution"
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
		case r == '$' && i+1 < n && runes[i+1] == '(':
			return "", i, "runs a command substitution"
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

// roStripAssignments drops leading NAME=value words so `FOO=1 cmd` classifies
// as cmd.
func roStripAssignments(argv []string) []string {
	i := 0
	for i < len(argv) && roIsAssignment(argv[i]) {
		i++
	}
	return argv[i:]
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

// roCheckSegment allows one segment's command by its basename and argument
// rules. `env` is the one wrapper allowed: it may set variables ahead of an
// allowed command but never run bare, which would print the environment.
func roCheckSegment(argv []string) (bool, string) {
	base := filepath.Base(argv[0])
	if reason, bad := roNeverCmds[base]; bad {
		return false, "`" + base + "` " + reason
	}
	if base == "env" {
		rest := argv[1:]
		j := 0
		for j < len(rest) && roIsAssignment(rest[j]) {
			j++
		}
		rest = rest[j:]
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

// roAllow maps a command basename to the rule that admits its read-only uses.
var roAllow map[string]roRule

func init() {
	roAllow = map[string]roRule{}
	// Commands that only ever read; a redirect or substitution around them is
	// already caught by the lexer, so their arguments need no further check.
	for _, c := range []string{
		"cat", "grep", "egrep", "fgrep", "ls", "stat", "file", "wc",
		"cut", "tr", "nl", "tac", "rev", "column", "fold", "fmt", "comm",
		"join", "paste", "diff", "cmp", "md5sum", "sha1sum", "sha256sum",
		"b2sum", "basename", "dirname", "realpath", "readlink", "echo",
		"printf", "true", "false", "test", "[", "uname", "whoami", "id",
		"groups", "uptime", "free", "df", "du", "lsblk", "blkid", "findmnt",
		"lscpu", "lspci", "lsusb", "lsmod", "modinfo", "lshw",
		"ps", "pgrep", "pidof", "lsof", "which", "whereis",
		"type", "strings", "od", "hexdump", "jq", "fc-list", "fc-match",
		"nproc", "getconf", "locale", "fastfetch", "inxi", "pw-dump",
		"checkupdates",
	} {
		roAllow[c] = roAlways
	}
	roAllow["head"] = roNoFollow
	roAllow["tail"] = roNoFollow
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
	roAllow["curl"] = roCurl
	roAllow["ryoku"] = roRyoku
	roAllow["ryoku-rashin"] = roRashin
	roAllow["rashin"] = roRashin
	roAllow["ryoku-shell"] = roRyokuShell
	roAllow["qs"] = roQs
	roAllow["prowl"] = roProwl
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

func roNoFollow(args []string) (bool, string) {
	for _, a := range args {
		if a == "-f" || a == "-F" || a == "--follow" || roShortHas(a, 'f') || roShortHas(a, 'F') {
			return false, "`-f` follows and never exits"
		}
	}
	return true, ""
}

func roRg(args []string) (bool, string) {
	for _, a := range args {
		if a == "--pre" || strings.HasPrefix(a, "--pre=") || strings.HasPrefix(a, "--pre-") {
			return false, "`rg --pre` runs a preprocessor command"
		}
	}
	return true, ""
}

func roSort(args []string) (bool, string) {
	for _, a := range args {
		if a == "--output" || strings.HasPrefix(a, "--output=") || roShortHas(a, 'o') {
			return false, "`sort -o` writes to a file"
		}
	}
	return true, ""
}

func roUniq(args []string) (bool, string) {
	files, expectVal := 0, false
	for _, a := range args {
		if expectVal {
			expectVal = false
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			switch a {
			case "-f", "-s", "-w", "--skip-fields", "--skip-chars", "--check-chars", "--group":
				expectVal = true
			}
			continue
		}
		files++
	}
	if files > 1 {
		return false, "`uniq IN OUT` writes the second file"
	}
	return true, ""
}

func roDate(args []string) (bool, string) {
	for _, a := range args {
		if a == "-s" || a == "--set" || strings.HasPrefix(a, "--set=") || strings.HasPrefix(a, "-s") {
			return false, "`date -s` sets the clock"
		}
	}
	return true, ""
}

func roHostname(args []string) (bool, string) {
	if len(args) == 0 {
		return true, ""
	}
	return false, "`hostname` with arguments changes the name"
}

func roCommand(args []string) (bool, string) {
	for _, a := range args {
		if a == "-v" || a == "-V" {
			return true, ""
		}
	}
	return false, "`command` runs its argument"
}

func roXxd(args []string) (bool, string) {
	for _, a := range args {
		if a == "-r" || a == "--revert" || roShortHas(a, 'r') {
			return false, "`xxd -r` writes binary back"
		}
	}
	return true, ""
}

func roBat(args []string) (bool, string) {
	for _, a := range args {
		if a == "--paging=never" || a == "-pp" {
			return true, ""
		}
	}
	return false, "`bat` opens a pager; pass --paging=never"
}

func roTop(args []string) (bool, string) {
	hasB, hasN := false, false
	for _, a := range args {
		if roShortHas(a, 'b') {
			hasB = true
		}
		if roShortHas(a, 'n') {
			hasN = true
		}
	}
	if hasB && hasN {
		return true, ""
	}
	return false, "`top` without -b -n never exits"
}

func roAwk(args []string) (bool, string) {
	prog, haveProg := "", false
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-f" || a == "--file" || strings.HasPrefix(a, "--file="):
			return false, "`awk -f` runs a program file"
		case a == "-i" || a == "--include" || strings.HasPrefix(a, "--include=") ||
			a == "--in-place" || (!strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-i")):
			return false, "`awk -i` can edit files"
		case a == "-F" || a == "-v":
			i += 2
		case strings.HasPrefix(a, "-") && a != "-":
			i++
		default:
			prog, haveProg = a, true
			i = len(args)
		}
	}
	if !haveProg {
		return false, "`awk` needs a program to inspect"
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(prog, " ", ""), "\t", "")
	for _, bad := range []string{"system", "getline", "|", "close(", "fflush(", "print>", "printf>"} {
		if strings.Contains(compact, bad) {
			return false, "`awk` program writes or runs a command"
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
	bad := map[string]bool{
		"-delete": true, "-exec": true, "-execdir": true, "-ok": true,
		"-okdir": true, "-fprint": true, "-fprint0": true, "-fprintf": true,
		"-fls": true,
	}
	for _, a := range args {
		if bad[a] {
			return false, "`find " + a + "` changes files or runs commands"
		}
	}
	return true, ""
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
	for _, a := range args {
		if a == "-f" || a == "--follow" {
			return false, "`journalctl -f` never exits"
		}
		if strings.HasPrefix(a, "--vacuum-") {
			return false, "`journalctl --vacuum` deletes logs"
		}
		switch a {
		case "--rotate", "--flush", "--relinquish-var", "--smart-relinquish-var",
			"--sync", "--setup-keys", "--update-catalog":
			return false, "`journalctl " + a + "` changes the journal"
		}
	}
	return true, ""
}

func roSystemctl(args []string) (bool, string) {
	allowed := map[string]bool{
		"status": true, "show": true, "cat": true, "list-units": true,
		"list-unit-files": true, "list-timers": true, "list-sockets": true,
		"list-dependencies": true, "list-jobs": true, "is-active": true,
		"is-enabled": true, "is-failed": true, "is-system-running": true,
		"get-default": true, "show-environment": true,
	}
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "-n" {
			i += 2
			continue
		}
		if strings.HasPrefix(a, "-") {
			i++
			continue
		}
		if allowed[a] {
			return true, ""
		}
		return false, "`systemctl " + a + "` is not a read-only command"
	}
	return true, "" // no verb lists units
}

func roLoginctl(args []string) (bool, string) {
	verb := roFirstVerb(args)
	if verb == "" || strings.HasPrefix(verb, "list-") || strings.HasPrefix(verb, "show-") ||
		verb == "session-status" || verb == "user-status" || verb == "seat-status" {
		return true, ""
	}
	return false, "`loginctl " + verb + "` is not a read-only command"
}

func roCoredumpctl(args []string) (bool, string) {
	for _, a := range args {
		if a == "-o" || a == "--output" || strings.HasPrefix(a, "--output=") {
			return false, "`coredumpctl -o` writes a file"
		}
	}
	verb := roFirstVerb(args)
	if verb == "" || verb == "list" || verb == "info" {
		return true, ""
	}
	return false, "`coredumpctl " + verb + "` is not a read-only command"
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
	rest := roNonFlags(args)
	if len(rest) == 0 {
		return true, ""
	}
	switch rest[0] {
	case "general", "g":
		return true, ""
	case "device", "dev", "d":
		if len(rest) == 1 {
			return true, ""
		}
		switch rest[1] {
		case "status", "show":
			return true, ""
		case "wifi", "w":
			if len(rest) == 2 || rest[2] == "list" {
				return true, ""
			}
			return false, "`nmcli device wifi " + rest[2] + "` may change state"
		}
		return false, "`nmcli device " + rest[1] + "` is not read-only"
	case "connection", "con", "c":
		if len(rest) == 1 || rest[1] == "show" {
			return true, ""
		}
		return false, "`nmcli connection " + rest[1] + "` changes connections"
	case "radio", "r":
		if len(rest) == 1 {
			return true, ""
		}
		return false, "`nmcli radio " + rest[1] + "` toggles the radio"
	}
	return false, "`nmcli " + rest[0] + "` is not read-only"
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
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "-c" || a == "--config" {
			i += 2
			continue
		}
		if strings.HasPrefix(a, "-") {
			i++
			continue
		}
		switch a {
		case "list", "get-config", "list-configs":
			return true, ""
		}
		return false, "`snapper " + a + "` is not read-only"
	}
	return false, "`snapper` needs a subcommand"
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
	verb := roFirstVerb(args)
	switch verb {
	case "ps", "images", "inspect", "version", "info", "top":
		return true, ""
	case "logs":
		for _, a := range args {
			if a == "-f" || a == "--follow" {
				return false, "`logs -f` never exits"
			}
		}
		return true, ""
	case "stats":
		for _, a := range args {
			if a == "--no-stream" {
				return true, ""
			}
		}
		return false, "`stats` streams forever without --no-stream"
	}
	return false, "`" + verb + "` is not a read-only command"
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
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-c" || a == "--config-env" || strings.HasPrefix(a, "--config-env="):
			return false, "`git -c` can run code"
		case a == "--exec-path" || strings.HasPrefix(a, "--exec-path="):
			return false, "`git --exec-path` can run code"
		case a == "--git-dir" || strings.HasPrefix(a, "--git-dir=") ||
			a == "--work-tree" || strings.HasPrefix(a, "--work-tree="):
			return false, "`git --git-dir` retargets the repo"
		case a == "-C":
			i += 2
			continue
		case strings.HasPrefix(a, "-"):
			i++
			continue
		}
		break // the first non-flag word is the subcommand
	}
	if i >= len(args) {
		return false, "`git` needs a subcommand"
	}
	sub := args[i]
	rest := args[i+1:]
	switch sub {
	case "status", "rev-parse", "ls-files", "describe", "blame":
		return true, ""
	case "log", "diff", "show", "shortlog":
		for _, a := range rest {
			if a == "--output" || strings.HasPrefix(a, "--output=") {
				return false, "`git " + sub + " --output` writes a file"
			}
			if a == "--ext-diff" || a == "--textconv" {
				return false, "`git " + sub + " " + a + "` runs an external program"
			}
		}
		return true, ""
	case "branch", "tag":
		listFlags := map[string]bool{
			"-a": true, "-r": true, "-v": true, "-vv": true, "--list": true,
			"-l": true, "--all": true, "--merged": true, "--no-merged": true,
			"--color": true, "--no-color": true,
		}
		for _, a := range rest {
			if strings.HasPrefix(a, "-") {
				if listFlags[a] || strings.HasPrefix(a, "--sort=") ||
					strings.HasPrefix(a, "--format=") || strings.HasPrefix(a, "--contains=") {
					continue
				}
				return false, "`git " + sub + " " + a + "` may modify refs"
			}
			return false, "`git " + sub + "` with a name creates or moves a ref"
		}
		return true, ""
	case "remote":
		if len(rest) == 0 || rest[0] == "-v" || rest[0] == "--verbose" || rest[0] == "show" {
			return true, ""
		}
		return false, "`git remote " + rest[0] + "` changes remotes"
	}
	return false, "`git " + sub + "` is not a read-only command"
}

func roCurl(args []string) (bool, string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "-O" || a == "--output" || a == "--remote-name" ||
			strings.HasPrefix(a, "--output="):
			return false, "`curl -o` writes a file"
		case a == "-T" || a == "--upload-file" || strings.HasPrefix(a, "--upload-file="):
			return false, "`curl -T` uploads a file"
		case a == "-d" || strings.HasPrefix(a, "-d") || strings.HasPrefix(a, "--data") ||
			strings.HasPrefix(a, "--json"):
			return false, "`curl --data` sends a request body"
		case a == "-F" || a == "--form" || strings.HasPrefix(a, "--form="):
			return false, "`curl -F` posts form data"
		case a == "-K" || a == "--config" || strings.HasPrefix(a, "--config="):
			return false, "`curl -K` reads a config file"
		case a == "-X" || a == "--request":
			if i+1 < len(args) && strings.ToUpper(args[i+1]) != "GET" {
				return false, "`curl -X " + args[i+1] + "` is not a plain read"
			}
			i++
		case strings.HasPrefix(a, "-X"):
			if strings.ToUpper(a[2:]) != "GET" {
				return false, "`curl -X` is not a plain read"
			}
		case strings.HasPrefix(a, "--request="):
			if strings.ToUpper(strings.TrimPrefix(a, "--request=")) != "GET" {
				return false, "`curl` uses a non-GET method"
			}
		}
	}
	return true, ""
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
	verb := roFirstVerb(args)
	switch verb {
	case "status", "paths", "logs":
		return true, ""
	case "agent":
		if len(roNonFlags(args)) == 1 {
			return true, ""
		}
		return false, "`ryoku-rashin agent use` switches the backend"
	}
	return false, "`ryoku-rashin " + verb + "` is not a read-only command"
}

func roRyokuShell(args []string) (bool, string) {
	rest := roNonFlags(args)
	if len(rest) == 0 {
		return false, "`ryoku-shell` needs a read-only subcommand"
	}
	switch rest[0] {
	case "status":
		return true, ""
	case "bar":
		if len(rest) >= 2 && (rest[1] == "list" || rest[1] == "catalog") {
			return true, ""
		}
		return false, "`ryoku-shell bar` with that action changes the bar"
	case "dock":
		if len(rest) >= 2 && rest[1] == "list" {
			return true, ""
		}
		return false, "`ryoku-shell dock` with that action changes the dock"
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
		return false, "`qs " + a + "` is not a read-only command"
	}
	return false, "`qs` needs the list or log subcommand"
}

func roProwl(args []string) (bool, string) {
	verb := roFirstVerb(args)
	switch verb {
	case "search", "find", "def", "outline", "references", "peek", "overview",
		"impact", "changed", "wip", "brief", "doctor", "callers", "callees",
		"clusters":
		return true, ""
	}
	return false, "`prowl " + verb + "` is not on the read-only list"
}
