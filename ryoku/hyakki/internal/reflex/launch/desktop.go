package launch

import (
	"bufio"
	"errors"
	"io"
	"path"
	"strings"
)

// Entry is one launchable application from a desktop entry file.
type Entry struct {
	// ID is the desktop file ID: the path below an applications directory
	// with "/" turned into "-", e.g. "org.example.Notes.desktop".
	ID   string
	Name string
	// Aliases are other names the app answers to, such as the last part of
	// a reverse-domain ID.
	Aliases    []string
	Categories []string
	MimeTypes  []string
	// tokens is the Exec line after quoting, field codes still in place.
	tokens []string
}

// Argv is the command the entry runs with every field code stripped.
func (e Entry) Argv() []string { return expand(e.tokens, "") }

// WithURL is the command with url given where the entry takes a file or
// address (%u, %U, %f or %F standing alone), or appended when it takes
// none. Every other field code is stripped. url is passed as one argument.
func (e Entry) WithURL(url string) []string { return expand(e.tokens, url) }

// Browser reports whether the entry says it is a web browser.
func (e Entry) Browser() bool {
	for _, c := range e.Categories {
		if c == "WebBrowser" {
			return true
		}
	}
	for _, m := range e.MimeTypes {
		if m == "x-scheme-handler/https" || m == "x-scheme-handler/http" {
			return true
		}
	}
	return false
}

// maxEntryBytes bounds how much of one desktop file is read.
const maxEntryBytes = 64 << 10

var errEntry = errors.New("launch: not a launchable desktop entry")

// refusedPrograms are programs an entry may not run: a shell or a command
// runner would turn a desktop entry into a script, and the privilege tools
// would ask for a password on Oni's behalf.
var refusedPrograms = map[string]bool{
	"sh": true, "bash": true, "dash": true, "zsh": true, "fish": true, "ksh": true,
	"csh": true, "tcsh": true, "mksh": true, "busybox": true, "env": true,
	"sudo": true, "doas": true, "pkexec": true, "su": true, "run0": true,
	"systemd-run": true, "xdg-open": true,
}

// entryFields are the [Desktop Entry] keys the catalog reads.
type entryFields struct {
	kind, name, exec, tryExec string
	noDisplay, hidden, term   bool
	categories, mimeTypes     []string
	onlyShowIn, notShowIn     []string
}

// parseEntry reads the [Desktop Entry] group of one file. Localised keys
// are ignored: the catalog matches the plain name.
func parseEntry(r io.Reader) (entryFields, error) {
	var f entryFields
	sc := bufio.NewScanner(io.LimitReader(r, maxEntryBytes))
	sc.Buffer(make([]byte, 0, 4096), maxEntryBytes)
	in, seen := false, false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if in {
				break
			}
			in = line == "[Desktop Entry]"
			seen = seen || in
			continue
		}
		if !in {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "Type":
			f.kind = value
		case "Name":
			f.name = unescape(value)
		case "Exec":
			f.exec = unescape(value)
		case "TryExec":
			f.tryExec = unescape(value)
		case "NoDisplay":
			f.noDisplay = value == "true"
		case "Hidden":
			f.hidden = value == "true"
		case "Terminal":
			f.term = value == "true"
		case "Categories":
			f.categories = list(value)
		case "MimeType":
			f.mimeTypes = list(value)
		case "OnlyShowIn":
			f.onlyShowIn = list(value)
		case "NotShowIn":
			f.notShowIn = list(value)
		}
	}
	if err := sc.Err(); err != nil {
		return f, err
	}
	if !seen {
		return f, errEntry
	}
	return f, nil
}

// unescape applies the string escapes of the desktop entry format.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 's':
			b.WriteByte(' ')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func list(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ";") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// splitExec splits an Exec value into arguments by the format's quoting
// rules: arguments are separated by spaces, and a double-quoted argument
// may hold spaces, with \", \`, \$ and \\ standing for the character
// itself. An unterminated quote or a bad escape makes the entry invalid.
func splitExec(s string) ([]string, error) {
	var (
		out    []string
		cur    strings.Builder
		have   bool
		quoted bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quoted && c == '\\':
			if i+1 == len(s) || !strings.ContainsRune("\"`$\\", rune(s[i+1])) {
				return nil, errEntry
			}
			i++
			cur.WriteByte(s[i])
		case quoted && c == '"':
			quoted = false
		case quoted:
			cur.WriteByte(c)
		case c == '"':
			quoted, have = true, true
		case c == ' ' || c == '\t':
			if have {
				out = append(out, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteByte(c)
			have = true
		}
	}
	if quoted {
		return nil, errEntry
	}
	if have {
		out = append(out, cur.String())
	}
	return out, nil
}

// fieldCodes are the codes the format defines; any other % sequence makes
// the entry invalid.
const fieldCodes = "fFuUdDnNickvm"

// checkCodes reports whether every % in the tokens is a known field code
// or "%%".
func checkCodes(tokens []string) bool {
	for _, t := range tokens {
		for i := 0; i < len(t); i++ {
			if t[i] != '%' {
				continue
			}
			if i+1 == len(t) || (t[i+1] != '%' && !strings.ContainsRune(fieldCodes, rune(t[i+1]))) {
				return false
			}
			i++
		}
	}
	return true
}

// expand builds argv from tokens. A token that is exactly %u, %U, %f or %F
// becomes url when url is set (once; later ones are dropped). Every other
// field code is removed, "%%" becomes "%", and a token left empty by
// removing codes is dropped. When url is set and no token took it, url is
// appended.
func expand(tokens []string, url string) []string {
	out := make([]string, 0, len(tokens)+1)
	placed := false
	for _, t := range tokens {
		switch t {
		case "%u", "%U", "%f", "%F":
			if url != "" && !placed {
				out = append(out, url)
				placed = true
			}
			continue
		}
		var b strings.Builder
		coded := false
		for i := 0; i < len(t); i++ {
			if t[i] == '%' && i+1 < len(t) {
				i++
				if t[i] == '%' {
					b.WriteByte('%')
				} else {
					coded = true
				}
				continue
			}
			b.WriteByte(t[i])
		}
		if coded && b.Len() == 0 {
			continue
		}
		out = append(out, b.String())
	}
	if url != "" && !placed {
		out = append(out, url)
	}
	return out
}

// program is the base name of argv[0].
func program(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	return path.Base(argv[0])
}
