// Package command classifies shell commands without executing them.
package command

import (
	"os"
	"path/filepath"
	"strings"
)

// danger.go classifies a proposed shell command into a tier the terminal lane
// renders as a badge and `--run` uses as a confirmation gate. The classifier
// is deny-first and pessimistic: it never needs to be a full shell parser
// because an unknown or unparseable command classifies as write, never read.

type dangerTier int

const (
	tierRead dangerTier = iota
	tierWrite
	tierSystem
	tierDanger
)

func (t dangerTier) String() string {
	switch t {
	case tierRead:
		return "read"
	case tierWrite:
		return "write"
	case tierSystem:
		return "system"
	default:
		return "danger"
	}
}

// readOnly are binaries that look and never touch, absent a redirect.
var readOnly = map[string]bool{
	"ls": true, "eza": true, "lt": true, "cat": true, "bat": true, "less": true,
	"head": true, "tail": true, "fd": true, "find": true, "rg": true, "grep": true,
	"du": true, "df": true, "stat": true, "file": true, "wc": true, "sort": true,
	"uniq": true, "tr": true, "cut": true, "awk": true, "sed": true,
	"ps": true, "pgrep": true, "top": true, "btop": true, "free": true, "uname": true,
	"lsblk": true, "lspci": true, "lsusb": true, "id": true, "whoami": true,
	"printenv": true, "which": true, "type": true, "echo": true,
	"printf": true, "date": true, "cal": true, "uptime": true, "journalctl": true,
	"dmesg": true, "ip": true, "ss": true, "ping": true, "dig": true,
	"man": true, "tldr": true, "help": true, "cd": true, "pwd": true,
	"z": true, "zoxide": true, "fzf": true, "tree": true, "realpath": true,
	"basename": true, "dirname": true, "readlink": true, "sha256sum": true,
	"md5sum": true, "diff": true, "cmp": true, "hexdump": true, "xxd": true,
	"strings": true, "nproc": true, "lscpu": true, "sensors": true, "fastfetch": true,
	"ryoku-fastfetch": true, "wl-paste": true, "yazi": true,
	"jq": true, "column": true, "test": true, "true": true,
	"sleep": true, "getent": true,
}

// wrappers run another command; classification follows the wrapped argv.
// timeout also carries a duration argument before the command.
var wrappers = map[string]bool{
	"command": true, "nice": true, "ionice": true, "time": true, "watch": true,
	"stdbuf": true, "env": true, "timeout": true, "xargs": true, "nohup": true,
}

// readSubs marks tools whose tier depends on the subcommand: listed
// subcommands read, everything else falls to the tool's write tier.
var readSubs = map[string]map[string]bool{
	"git": {"status": true, "log": true, "diff": true, "show": true, "branch": true,
		"remote": true, "blame": true, "describe": true, "shortlog": true, "grep": true},
	"systemctl": {"status": true, "show": true, "list-units": true, "list-unit-files": true,
		"list-timers": true, "is-active": true, "is-enabled": true, "is-failed": true, "cat": true},
	"docker":       {"ps": true, "images": true, "logs": true, "inspect": true, "stats": true},
	"nmcli":        {"device": true, "connection": true, "general": true},
	"ryoku":        {"status": true, "snapshots": true},
	"ryoku-rashin": {"status": true},
	"rashin":       {"status": true},
	"snapper":      {"list": true, "status": true},
	"loginctl": {"list-sessions": true, "session-status": true, "show-session": true,
		"show-user": true, "user-status": true},
}

// sudoLike run their argument as another user; the wrapped command is what
// classifies, floored at system.
var sudoLike = map[string]bool{"sudo": true, "doas": true, "pkexec": true, "su": true, "run0": true}

// systemLevel change packages, services, or machine state.
var systemLevel = map[string]bool{
	"pacman": true, "yay": true, "paru": true, "dnf": true, "dnf5": true,
	"rpm": true, "xbps-install": true, "xbps-remove": true,
	"systemctl": true, "mount": true, "umount": true, "modprobe": true,
	"sysctl": true, "timedatectl": true, "hostnamectl": true, "localectl": true,
	"mkinitcpio": true, "grub-mkconfig": true, "useradd": true, "usermod": true,
	"groupadd": true, "passwd": true, "ryoku": true, "snapper": true,
	"reboot": true, "poweroff": true, "shutdown": true, "halt": true,
}

// wipers destroy data by design; any invocation is danger.
var wipers = map[string]bool{
	"mkfs": true, "wipefs": true, "shred": true, "blkdiscard": true,
	"sgdisk": true, "sfdisk": true, "userdel": true, "groupdel": true,
}

// systemRoots are path prefixes whose recursive mutation is a machine-wide
// event, not a user-file edit.
var systemRoots = []string{"/etc", "/usr", "/var", "/boot", "/opt", "/srv", "/bin", "/sbin", "/lib"}

// shells that turn piped text into execution.
var shellNames = map[string]bool{"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true}

// classify returns the highest tier across a command line's pipeline
// segments, with a short reason for anything above read.
func classify(cmd string) (dangerTier, string) {
	if hasUnquotedSubstitution(cmd) {
		return tierDanger, "command substitution is not allowed"
	}
	segs := splitSegments(cmd)
	tier, reason := tierRead, ""
	bump := func(t dangerTier, r string) {
		if t > tier || t == tier && reason == "" && r != "" {
			tier, reason = t, r
		}
	}
	if strings.Contains(strings.ReplaceAll(cmd, " ", ""), ":(){") {
		return tierDanger, "fork bomb"
	}
	for i, seg := range segs {
		argv := shellFields(seg.text)
		argv = stripEnvAssignments(argv)
		if len(argv) == 0 {
			continue
		}
		floor := tierRead
		if sudoLike[argv[0]] {
			floor = tierSystem
			argv = unwrapSudo(argv)
			if len(argv) == 0 {
				bump(tierSystem, "runs as root")
				continue
			}
		}
		t, r := classifyArgv(argv)
		if t == tierRead {
			if ok, _ := readOnlyCommand(seg.text); !ok {
				t, r = tierWrite, ""
			}
		}
		if t < floor {
			t, r = floor, "runs as root"
		}
		bump(t, r)
		// Piping a downloader into a shell executes whatever the network
		// serves; the pair is the hazard, not either segment alone.
		if seg.pipedInto && i > 0 && shellNames[argv[0]] {
			prev := shellFields(segs[i-1].text)
			prev = stripEnvAssignments(prev)
			if len(prev) > 0 && (prev[0] == "curl" || prev[0] == "wget") {
				bump(tierDanger, "pipes downloaded content into a shell")
			}
		}
		// Redirects escalate an otherwise read-only segment.
		if dst := redirectTarget(seg.text); dst != "" && dst != "/dev/null" {
			switch {
			case strings.HasPrefix(dst, "/dev/sd"), strings.HasPrefix(dst, "/dev/nvme"),
				strings.HasPrefix(dst, "/dev/vd"), strings.HasPrefix(dst, "/dev/mmcblk"):
				bump(tierDanger, "writes to a block device")
			case underAny(dst, systemRoots):
				bump(tierSystem, "writes under "+dst)
			default:
				bump(tierWrite, "writes "+dst)
			}
		}
	}
	return tier, reason
}

// classifyArgv tiers one command's argv (env prefixes already stripped). It
// unwraps runner prefixes (xargs, env, timeout, ...) and exec flags (fd -x,
// find -exec) so the command that actually runs is the one that classifies.
func classifyArgv(argv []string) (dangerTier, string) {
	argv = unwrap(argv)
	if len(argv) == 0 {
		return tierWrite, "wrapper has no command"
	}
	name := filepath.Base(argv[0])
	sub := ""
	for _, a := range argv[1:] {
		if !strings.HasPrefix(a, "-") {
			sub = a
			break
		}
	}
	switch {
	case wipers[name] || strings.HasPrefix(name, "mkfs."):
		return tierDanger, name + " destroys data"
	case name == "rm":
		return classifyRm(argv)
	case name == "dd":
		for _, a := range argv[1:] {
			if strings.HasPrefix(a, "of=/dev/") {
				return tierDanger, "dd onto a device"
			}
		}
		return tierWrite, "dd writes files"
	case name == "chmod" || name == "chown" || name == "chgrp":
		if hasRecursiveFlag(argv) && firstPathArg(argv) != "" &&
			(firstPathArg(argv) == "/" || underAny(firstPathArg(argv), systemRoots)) {
			return tierDanger, "recursive " + name + " on a system tree"
		}
		return tierWrite, name + " changes permissions"
	case name == "parted" || name == "fdisk" || name == "cfdisk" || name == "gdisk":
		return tierDanger, "edits partition tables"
	case name == "pacman" || name == "yay" || name == "paru":
		op := ""
		if len(argv) > 1 {
			op = argv[1]
		}
		for _, read := range []string{"-Q", "-F", "-Ss", "-Si", "-T", "-h", "--query", "--help"} {
			if strings.HasPrefix(op, read) {
				return tierRead, ""
			}
		}
		return tierSystem, "changes installed packages"
	case name == "dnf" || name == "dnf5":
		if ok, _ := roDNF(argv[1:]); ok {
			return tierRead, ""
		}
		return tierSystem, "changes installed packages"
	case name == "rpm":
		if ok, _ := roRPM(argv[1:]); ok {
			return tierRead, ""
		}
		return tierSystem, "changes installed packages"
	case name == "xbps-query":
		return tierRead, ""
	case name == "xbps-install" || name == "xbps-remove":
		return tierSystem, "changes installed packages"
	case name == "ip":
		if ok, _ := roIP(argv[1:]); ok {
			return tierRead, ""
		}
		return tierWrite, "ip arguments may change the network"
	case name == "journalctl":
		if ok, _ := roJournalctl(argv[1:]); ok {
			return tierRead, ""
		}
		return tierWrite, "journalctl arguments change or continuously read the journal"
	case name == "systemctl":
		if ok, _ := roSystemctl(argv[1:]); ok {
			return tierRead, ""
		}
		return tierSystem, "systemctl arguments change system state"
	case name == "loginctl":
		if ok, _ := roLoginctl(argv[1:]); ok {
			return tierRead, ""
		}
		return tierSystem, "loginctl arguments change system state"
	case name == "snapper":
		if ok, _ := roSnapper(argv[1:]); ok {
			return tierRead, ""
		}
		return tierSystem, "snapper arguments change snapshots"
	case name == "nmcli":
		if ok, _ := roNmcli(argv[1:]); ok {
			return tierRead, ""
		}
		return tierWrite, "nmcli arguments may change network state"
	case name == "sed":
		if ok, why := roSed(argv[1:]); ok {
			return tierRead, ""
		} else if strings.Contains(why, "in place") {
			return tierWrite, "sed edits in place"
		}
		return tierWrite, "sed script may write, execute, or transform input"
	case name == "awk" || name == "gawk" || name == "mawk":
		if ok, _ := roAwk(argv[1:]); ok {
			return tierRead, ""
		}
		return tierWrite, "awk program may write or run a command"
	case name == "docker" || name == "podman":
		if ok, _ := roDocker(argv[1:]); ok {
			return tierRead, ""
		}
		return tierWrite, name + " arguments are not a finite read"
	case name == "tail":
		if ok, _ := roNoFollow(argv[1:]); ok {
			return tierRead, ""
		}
		return tierWrite, "tail follows and never exits"
	case name == "top":
		if ok, _ := roTop(argv[1:]); ok {
			return tierRead, ""
		}
		return tierWrite, "top is interactive or never exits"
	case name == "curl" || name == "wget":
		for _, a := range argv[1:] {
			if a == "-o" || a == "-O" || strings.HasPrefix(a, "--output") || a == "--remote-name" {
				return tierWrite, name + " downloads to a file"
			}
		}
		return tierRead, ""
	case name == "fd" || name == "find":
		if tail := execTail(argv); tail != nil {
			t, r := classifyArgv(tail)
			if t > tierRead {
				return t, name + " runs: " + r
			}
			return t, r
		}
		return tierRead, ""
	case sub != "" && readSubs[name] != nil:
		if readSubs[name][sub] {
			return tierRead, ""
		}
		if systemLevel[name] {
			return tierSystem, name + " " + sub + " changes system state"
		}
		return tierWrite, name + " " + sub
	case systemLevel[name]:
		return tierSystem, name + " changes system state"
	case readOnly[name]:
		return tierRead, ""
	default:
		// Unknown binaries never classify as read.
		return tierWrite, ""
	}
}

// unwrap strips runner prefixes so `xargs -0 mv`, `env FOO=1 cmd`, and
// `timeout 5 cmd` classify as the wrapped command.
func unwrap(argv []string) []string {
	for len(argv) > 0 && wrappers[filepath.Base(argv[0])] {
		name := filepath.Base(argv[0])
		argv = argv[1:]
		switch name {
		case "timeout":
			argv = unwrapOptions(argv, map[string]bool{"-s": true, "--signal": true, "-k": true, "--kill-after": true})
			if len(argv) > 0 {
				argv = argv[1:]
			}
		case "nice":
			argv = unwrapOptions(argv, map[string]bool{"-n": true, "--adjustment": true})
		case "ionice":
			argv = unwrapOptions(argv, map[string]bool{"-c": true, "--class": true, "-n": true, "--classdata": true, "-p": true, "--pid": true, "-P": true, "--pgid": true, "-u": true, "--uid": true})
		case "stdbuf":
			argv = unwrapOptions(argv, map[string]bool{"-i": true, "--input": true, "-o": true, "--output": true, "-e": true, "--error": true})
		case "env":
			argv = unwrapOptions(argv, map[string]bool{"-u": true, "--unset": true, "-C": true, "--chdir": true, "-S": true, "--split-string": true})
			argv = stripEnvAssignments(argv)
		case "xargs":
			argv = unwrapOptions(argv, map[string]bool{"-a": true, "--arg-file": true, "-E": true, "--eof": true, "-I": true, "--replace": true, "-L": true, "--max-lines": true, "-n": true, "--max-args": true, "-P": true, "--max-procs": true, "-s": true, "--max-chars": true})
		default:
			argv = unwrapOptions(argv, nil)
		}
		argv = stripEnvAssignments(argv)
	}
	return argv
}

func unwrapSudo(argv []string) []string {
	if len(argv) == 0 {
		return nil
	}
	return stripEnvAssignments(unwrapOptions(argv[1:], map[string]bool{
		"-u": true, "--user": true, "-g": true, "--group": true, "-C": true, "--close-from": true,
		"-D": true, "--chdir": true, "-h": true, "--host": true, "-p": true, "--prompt": true,
		"-r": true, "--role": true, "-t": true, "--type": true, "-U": true, "--other-user": true,
	}))
}

func unwrapOptions(args []string, valueFlags map[string]bool) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-" {
		arg := args[0]
		if arg == "--" {
			return args[1:]
		}
		flag, attached := arg, false
		if before, _, found := strings.Cut(arg, "="); found {
			flag, attached = before, true
		} else if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			short := arg[:2]
			if valueFlags[short] {
				flag, attached = short, true
			}
		}
		args = args[1:]
		if valueFlags[flag] && !attached && len(args) > 0 {
			args = args[1:]
		}
	}
	return args
}

// execTail returns the command an fd -x / find -exec invocation runs per
// hit, nil when the walk only lists.
func execTail(argv []string) []string {
	for i, a := range argv[1:] {
		if a == "-x" || a == "-X" || a == "--exec" || a == "--exec-batch" ||
			a == "-exec" || a == "-execdir" || a == "-ok" || a == "-delete" {
			if a == "-delete" {
				return []string{"rm"}
			}
			tail := argv[i+2:]
			var out []string
			for _, t := range tail {
				if t == ";" || t == "+" || t == "\\;" {
					break
				}
				if t == "{}" {
					continue
				}
				out = append(out, t)
			}
			if len(out) > 0 {
				return out
			}
			return nil
		}
	}
	return nil
}

// classifyRm: recursive/forced removal of a root, a home, or their immediate
// children is irreversible enough to demand the danger gate.
func classifyRm(argv []string) (dangerTier, string) {
	recursive, force := false, false
	for _, a := range argv[1:] {
		if strings.HasPrefix(a, "--") {
			recursive = recursive || a == "--recursive"
			force = force || a == "--force"
			continue
		}
		if strings.HasPrefix(a, "-") {
			recursive = recursive || strings.ContainsAny(a, "rR")
			force = force || strings.Contains(a, "f")
		}
	}
	if !recursive && !force {
		return tierWrite, "removes files"
	}
	for _, a := range argv[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		clean := filepath.Clean(expandHomeRef(a))
		bare := expandHomeRef(a) // for glob suffix checks
		switch {
		case clean == "/" || clean == home() || clean == "/*" || a == "*":
			return tierDanger, "rm -rf on " + a
		case underAny(clean, systemRoots) && strings.Count(clean, "/") <= 2:
			return tierDanger, "rm -rf near a system root"
		case clean == filepath.Dir(home()) || strings.HasSuffix(bare, "/*") && filepath.Dir(clean) == home():
			return tierDanger, "rm -rf across the home directory"
		}
	}
	return tierWrite, "removes files recursively"
}

// expandHomeRef resolves ~, $HOME, and ${HOME} prefixes to the home path, so
// `rm -rf $HOME` and `rm -rf "$HOME"` classify like `rm -rf ~` (shellFields
// has already stripped the quotes). Other variables are left unexpanded: an
// unknown $VAR path is treated literally, which never under-classifies.
func expandHomeRef(a string) string {
	switch {
	case a == "~" || strings.HasPrefix(a, "~/"):
		return home() + a[1:]
	case a == "$HOME" || strings.HasPrefix(a, "$HOME/"):
		return home() + a[len("$HOME"):]
	case a == "${HOME}" || strings.HasPrefix(a, "${HOME}/"):
		return home() + a[len("${HOME}"):]
	}
	return a
}

type segment struct {
	text      string
	pipedInto bool // this segment consumes the previous one's stdout
}

// hasUnquotedSubstitution rejects shell constructs that run a command before
// the visible argv is executed. Single quotes suppress expansion; double
// quotes still expand command substitutions.
func hasUnquotedSubstitution(cmd string) bool {
	runes := []rune(cmd)
	quote := rune(0)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			}
			continue
		}
		if r == '\\' {
			i++
			continue
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
				continue
			}
			if r == '`' || r == '$' && i+1 < len(runes) && runes[i+1] == '(' {
				return true
			}
			continue
		}
		if r == '\'' {
			quote = '\''
			continue
		}
		if r == '"' {
			quote = '"'
			continue
		}
		if r == '`' || r == '$' && i+1 < len(runes) && runes[i+1] == '(' {
			return true
		}
		if (r == '<' || r == '>') && i+1 < len(runes) && runes[i+1] == '(' {
			return true
		}
	}
	return false
}

// splitSegments cuts a command line at top-level ;, &&, ||, | and newlines,
// tracking quotes so a | inside a string never splits.
func splitSegments(cmd string) []segment {
	var segs []segment
	var cur strings.Builder
	quote := rune(0)
	piped := false
	flush := func(nextPiped bool) {
		if s := strings.TrimSpace(cur.String()); s != "" {
			segs = append(segs, segment{text: s, pipedInto: piped})
		}
		cur.Reset()
		piped = nextPiped
	}
	runes := []rune(cmd)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
			cur.WriteRune(r)
		case r == '\\' && i+1 < len(runes):
			cur.WriteRune(r)
			i++
			cur.WriteRune(runes[i])
		case r == ';' || r == '\n':
			flush(false)
		case r == '&':
			// 2>&1 and &> point a stream somewhere, they end nothing
			if i > 0 && runes[i-1] == '>' || i+1 < len(runes) && runes[i+1] == '>' {
				cur.WriteRune(r)
				continue
			}
			if i+1 < len(runes) && runes[i+1] == '&' {
				i++
			}
			flush(false)
		case r == '|':
			if i+1 < len(runes) && runes[i+1] == '|' {
				i++
				flush(false)
			} else {
				flush(true)
			}
		default:
			cur.WriteRune(r)
		}
	}
	flush(false)
	return segs
}

// shellFields splits one segment into words, honoring quotes (kept content,
// dropped delimiters) and backslash escapes.
func shellFields(s string) []string {
	var out []string
	var cur strings.Builder
	quote := rune(0)
	pending := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			pending = true
		case r == '\\' && i+1 < len(runes):
			i++
			cur.WriteRune(runes[i])
		case r == ' ' || r == '\t':
			if cur.Len() > 0 || pending {
				out = append(out, cur.String())
				cur.Reset()
				pending = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 || pending {
		out = append(out, cur.String())
	}
	return out
}

// stripEnvAssignments drops leading K=V words so `FOO=1 cmd` classifies cmd.
func stripEnvAssignments(argv []string) []string {
	for len(argv) > 0 {
		eq := strings.IndexByte(argv[0], '=')
		if eq <= 0 || strings.ContainsAny(argv[0][:eq], "/-.") {
			break
		}
		argv = argv[1:]
	}
	return argv
}

// redirectTarget returns the path a top-level > or >> writes, "" when none.
func redirectTarget(seg string) string {
	quote := rune(0)
	runes := []rune(seg)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '>':
			j := i + 1
			if j < len(runes) && runes[j] == '>' {
				j++
			}
			rest := strings.TrimSpace(string(runes[j:]))
			if rest == "" {
				return ""
			}
			f := shellFields(rest)
			if len(f) == 0 {
				return ""
			}
			// >&2 copies a stream, >& file and >&file write one
			if target, both := strings.CutPrefix(f[0], "&"); both {
				switch {
				case target == "" && len(f) > 1:
					return expandHomeRef(f[1])
				case target == "" || target == "-" || roDigits(target):
					return ""
				}
				return expandHomeRef(target)
			}
			return expandHomeRef(f[0])
		}
	}
	return ""
}

func hasRecursiveFlag(argv []string) bool {
	for _, a := range argv[1:] {
		if a == "--recursive" || (strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rR")) {
			return true
		}
	}
	return false
}

func hasFlag(argv []string, flag string) bool {
	for _, a := range argv[1:] {
		if a == flag || strings.HasPrefix(a, flag) && !strings.HasPrefix(a, "--") {
			return true
		}
	}
	return false
}

func firstPathArg(argv []string) string {
	for _, a := range argv[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return filepath.Clean(expandHomeRef(a))
	}
	return ""
}

func underAny(p string, roots []string) bool {
	for _, r := range roots {
		if p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

func home() string {
	if value := os.Getenv("HOME"); value != "" {
		return value
	}
	value, _ := os.UserHomeDir()
	return value
}

// Elevates reports whether cmd runs something through sudo, doas or su, each
// of which asks for a password on a terminal. pkexec and run0 are left out:
// they ask through the desktop's own prompt.
func Elevates(cmd string) bool {
	for _, seg := range splitSegments(cmd) {
		argv := stripEnvAssignments(shellFields(seg.text))
		if len(argv) != 0 && (argv[0] == "sudo" || argv[0] == "doas" || argv[0] == "su") {
			return true
		}
	}
	return false
}

// Classify returns the highest safety tier across a shell command and a short reason.
func Classify(cmd string) (tier, reason string) {
	t, reason := classify(cmd)
	return t.String(), reason
}
