// Package fs is the engine's file tools: read, write, edit, ls, find_files
// and grep, plus Ask's read_file, list_dir and write_file. Every file
// operation goes through tool.Env.Exec, so the same code serves a session on
// the user's machine and a team task inside its sandbox.
//
// The compact profile edits with find/replace ({path, old, new, all}); the
// full profile with hashline patches anchored to [PATH#TAG] headers that read
// prints. Output is shaped to Env.Limits; output that still runs over is
// spilled to local://N.log, which read pages like a file.
package fs

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/{read,write,grep,glob}.ts,
// packages/coding-agent/src/edit/ and crates/edit/@c7fff67

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Tools returns every file tool once, for tool.NewRegistry. The tools keep
// no per-session state of their own beyond the hashline tag and register
// memory, which is keyed by the session id.
func Tools() []tool.Tool {
	return []tool.Tool{
		readTool{}, writeTool{}, editTool{}, lsTool{}, findTool{}, grepTool{},
		readFileTool{}, listDirTool{}, writeFileTool{},
	}
}

// diffRunes caps each side of a Result.Diff, as Ask did.
const diffRunes = 4000

// maxLinks bounds symlink chains a write follows.
const maxLinks = 8

func spec(name, description, schema string) llm.Tool {
	return llm.Tool{Name: name, Description: description, Parameters: json.RawMessage(schema)}
}

// args reads a model's arguments leniently: aliases, numbers and booleans
// sent as strings, and a missing object all decode.
type args map[string]json.RawMessage

func decode(raw json.RawMessage) (args, error) {
	a := args{}
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return a, nil
	}
	if err := json.Unmarshal([]byte(text), &a); err != nil {
		return nil, fmt.Errorf("arguments are not a JSON object: %v", err)
	}
	return a, nil
}

func (a args) has(keys ...string) bool {
	for _, k := range keys {
		if v, ok := a[k]; ok && string(v) != "null" {
			return true
		}
	}
	return false
}

func (a args) str(keys ...string) string {
	for _, k := range keys {
		v, ok := a[k]
		if !ok || string(v) == "null" {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) == nil {
			return s
		}
		var n json.Number
		if json.Unmarshal(v, &n) == nil {
			return n.String()
		}
	}
	return ""
}

func (a args) integer(keys ...string) (int, bool) {
	for _, k := range keys {
		v, ok := a[k]
		if !ok || string(v) == "null" {
			continue
		}
		var n float64
		if json.Unmarshal(v, &n) == nil {
			return int(n), true
		}
		var s string
		if json.Unmarshal(v, &s) == nil {
			if i, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				return i, true
			}
		}
	}
	return 0, false
}

func (a args) boolean(keys ...string) bool {
	for _, k := range keys {
		v, ok := a[k]
		if !ok || string(v) == "null" {
			continue
		}
		var b bool
		if json.Unmarshal(v, &b) == nil {
			return b
		}
		var s string
		if json.Unmarshal(v, &s) == nil {
			b, _ = strconv.ParseBool(strings.TrimSpace(s))
			return b
		}
	}
	return false
}

// pathKeys are the names models use for a file path (omp accepts the same).
var pathKeys = []string{"path", "file_path", "filepath", "file", "filename"}

// A team task's tilde names its own scratch HOME, not the daemon's HOME.
func resolveFor(env tool.Env, path, cwd string) string {
	if env.Sandbox != nil && env.Sandbox.Env["HOME"] != "" {
		home := env.Sandbox.Env["HOME"]
		if path == "~" {
			path = home
		} else if strings.HasPrefix(path, "~/") {
			path = filepath.Join(home, path[2:])
		}
	}
	return resolve(path, cwd)
}

func resolve(path, cwd string) string {
	path = strings.TrimSpace(path)
	path = paths.Expand(path)
	if !filepath.IsAbs(path) {
		base := cwd
		if base == "" {
			base = paths.Home()
		}
		path = filepath.Join(base, path)
	}
	return filepath.Clean(path)
}

// display is how a path is shown to the model: relative inside the
// session's folder, absolute elsewhere.
func display(abs, cwd string) string {
	if cwd != "" {
		if rel, err := filepath.Rel(cwd, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			return filepath.ToSlash(rel)
		}
	}
	return abs
}

// secret refuses paths that hold credentials, by name and, for a link, by
// where it points.
func secret(abs string) error {
	if marker, blocked := command.SecretPathAt(abs, ""); blocked {
		return fmt.Errorf("refused: %s holds %s, which is never shown to a model", abs, marker)
	}
	return nil
}

// target checks every symlink on the path, including an existing parent
// when the file itself has not been created yet.
func target(ctx context.Context, x exec.Executor, abs string) (string, exec.FileInfo, bool, error) {
	cur := filepath.Clean(abs)
	if cur == string(filepath.Separator) {
		info, err := x.Stat(ctx, cur)
		return cur, info, err == nil, err
	}
	for range maxLinks {
		parts := strings.Split(strings.TrimPrefix(cur, string(filepath.Separator)), string(filepath.Separator))
		path := string(filepath.Separator)
		var found exec.FileInfo
		exists := false
		followed := false
		for i, part := range parts {
			if part == "" {
				continue
			}
			path = filepath.Join(path, part)
			info, err := x.Stat(ctx, path)
			if errors.Is(err, exec.ErrNotFound) {
				return cur, exec.FileInfo{}, false, nil
			}
			if err != nil {
				return cur, info, false, err
			}
			if info.Kind == exec.KindSymlink {
				next := info.Target
				if !filepath.IsAbs(next) {
					next = filepath.Join(filepath.Dir(path), next)
				}
				cur = filepath.Join(append([]string{next}, parts[i+1:]...)...)
				if err := secret(cur); err != nil {
					return cur, info, false, err
				}
				followed = true
				break
			}
			found, exists = info, true
		}
		if !followed {
			return cur, found, exists, nil
		}
	}
	return cur, exec.FileInfo{}, false, fmt.Errorf("%s is a chain of more than %d links", abs, maxLinks)
}

func targetName(abs, real, cwd string) string {
	shown := display(abs, cwd)
	if abs != real {
		return shown + " -> " + real
	}
	return shown
}

// describe turns an executor failure into a sentence that names the fix.
func describe(err error, shown string) string {
	var e *exec.Error
	if !errors.As(err, &e) {
		return err.Error()
	}
	switch e.Code {
	case exec.CodeNotFound:
		return fmt.Sprintf("%s does not exist. Check the path with ls or find_files.", shown)
	case exec.CodeIsDir:
		return fmt.Sprintf("%s is a folder. Use ls to list it.", shown)
	case exec.CodeNotDir:
		return fmt.Sprintf("%s is not a folder.", shown)
	case exec.CodeNotRegular:
		return fmt.Sprintf("%s is not a regular file (a device, socket or pipe).", shown)
	case exec.CodeDenied:
		return fmt.Sprintf("Permission denied for %s.", shown)
	case exec.CodeExists:
		return fmt.Sprintf("%s already exists.", shown)
	case exec.CodeChanged:
		return fmt.Sprintf("%s changed while this call ran. Read it again, then retry.", shown)
	case exec.CodeTooLarge:
		return fmt.Sprintf("%s is too large for this tool.", shown)
	case exec.CodeUnavailable:
		return "The file system is not available right now: " + e.Error()
	}
	return e.Error()
}

// allow asks the session's approvals before a change; the Result is set
// when the change must not run.
func allow(ctx context.Context, env tool.Env, call tool.Call, title, input string, diff *protocol.Diff) (tool.Result, bool) {
	if env.Emit != nil && diff != nil {
		env.Emit(protocol.ToolData{Title: title, Kind: protocol.ToolEdit, Status: protocol.ToolPending, Input: input, Diff: diff})
	}
	if env.Permissions == nil {
		return tool.Failed("File changes are unavailable without the session's permission gate."), false
	}
	d, err := env.Permissions.Allow(ctx, tool.Request{Call: call, Title: title, Kind: protocol.ToolEdit, Input: input, Tier: protocol.TierWrite, Diff: diff})
	if err != nil {
		r := tool.Failed("Cancelled before it was written.")
		r.Shown = i18n.T("Cancelled before it was written.")
		return r, false
	}
	if !d.Allowed {
		reason := d.Reason
		if reason == "" {
			reason = "The user refused this. Do not try the same change another way. Tell them it was not done and ask what they want instead."
		}
		r := tool.Failed(reason)
		r.Shown = i18n.T("Not written: you refused it.")
		return r, false
	}
	return tool.Result{}, true
}

// backup keeps what a file held before a change; "" when the engine keeps
// no backups (a temporary session).
func backup(env tool.Env, abs string, previous []byte) (string, error) {
	if env.Backup == nil {
		return "", nil
	}
	return env.Backup(abs, previous)
}

// makeDiff shows the changed region of a file: whole when small, else the
// lines that differ with a little context, each side cut at diffRunes.
func makeDiff(path, old, new string) *protocol.Diff {
	d := &protocol.Diff{Path: path}
	if len(old)+len(new) > 2*diffRunes {
		old, new = window(old, new, 3)
		d.Truncated = true
	}
	var a, b bool
	d.Old, a = tool.ClipRunes(old, diffRunes)
	d.New, b = tool.ClipRunes(new, diffRunes)
	d.Truncated = d.Truncated || a || b
	return d
}

// window keeps the lines between the first and last difference of old and
// new, with ctx lines around them.
func window(old, new string, ctx int) (string, string) {
	ol := strings.SplitAfter(old, "\n")
	nl := strings.SplitAfter(new, "\n")
	head := 0
	for head < len(ol) && head < len(nl) && ol[head] == nl[head] {
		head++
	}
	tail := 0
	for tail < len(ol)-head && tail < len(nl)-head && ol[len(ol)-1-tail] == nl[len(nl)-1-tail] {
		tail++
	}
	from := max(head-ctx, 0)
	oEnd := min(len(ol)-tail+ctx, len(ol))
	nEnd := min(len(nl)-tail+ctx, len(nl))
	return strings.Join(ol[from:oEnd], ""), strings.Join(nl[from:nEnd], "")
}

// isText reports content a text tool may show or write.
func isText(b []byte) bool {
	return utf8.Valid(b) && !strings.ContainsRune(string(b), 0)
}

// textFile preserves original bytes for backups and each line's ending
// separately so a local edit cannot rewrite unrelated lines.
type textFile struct {
	Text    string
	Raw     []byte
	BOM     bool
	Endings []string
}

func splitText(data []byte) textFile {
	s := string(data)
	f := textFile{Raw: data}
	if strings.HasPrefix(s, "\uFEFF") {
		f.BOM = true
		s = strings.TrimPrefix(s, "\uFEFF")
	}
	var out strings.Builder
	for _, part := range strings.SplitAfter(s, "\n") {
		if part == "" {
			continue
		}
		switch {
		case strings.HasSuffix(part, "\r\n"):
			out.WriteString(strings.TrimSuffix(part, "\r\n"))
			out.WriteByte('\n')
			f.Endings = append(f.Endings, "\r\n")
		case strings.HasSuffix(part, "\n"):
			out.WriteString(part)
			f.Endings = append(f.Endings, "\n")
		default:
			out.WriteString(part)
			f.Endings = append(f.Endings, "")
		}
	}
	f.Text = out.String()
	return f
}

func (f textFile) join(text string) []byte {
	if text == f.Text {
		return f.Raw
	}
	original, updated := lines(f.Text), lines(text)
	locations := make(map[string][]int, len(original))
	for i, line := range original {
		locations[line] = append(locations[line], i)
	}
	used := make([]bool, len(original))
	assigned := make([]int, len(updated))
	for i := range assigned {
		assigned[i] = -1
		if i < len(original) && updated[i] == original[i] {
			assigned[i] = i
			used[i] = true
		}
	}
	next := make(map[string]int)
	for i, line := range updated {
		if assigned[i] >= 0 {
			continue
		}
		at := locations[line]
		for next[line] < len(at) && used[at[next[line]]] {
			next[line]++
		}
		if next[line] < len(at) {
			assigned[i] = at[next[line]]
			used[assigned[i]] = true
			next[line]++
		}
	}
	for i := range assigned {
		if assigned[i] < 0 && i < len(original) && !used[i] {
			assigned[i] = i
			used[i] = true
		}
	}
	style := "\n"
	if len(f.Endings) > 0 && f.Endings[0] != "" {
		style = f.Endings[0]
	}
	var b strings.Builder
	if f.BOM {
		b.WriteString("\uFEFF")
	}
	for i, line := range updated {
		b.WriteString(line)
		if i == len(updated)-1 && !strings.HasSuffix(text, "\n") {
			break
		}
		ending := style
		if index := assigned[i]; index >= 0 && f.Endings[index] != "" {
			ending = f.Endings[index]
		}
		b.WriteString(ending)
	}
	return []byte(b.String())
}

// lines splits LF text into addressable lines: a final newline does not
// make an empty last line.
func lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// clipLine cuts one shown line at max bytes with a marker.
func clipLine(s string, max int) (string, bool) {
	if max <= 0 || len(s) <= max {
		return s, false
	}
	cut, _ := tool.ClipBytes(s, max)
	return cut + "…", true
}

// countLines counts the lines of text, a final line without newline
// included.
func countLines(text string) int {
	if text == "" {
		return 0
	}
	n := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		n++
	}
	return n
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
