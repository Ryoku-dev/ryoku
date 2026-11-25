package engine

// Derived from oh-my-pi (MIT), packages/coding-agent/src/discovery/at-imports.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/system/project-prompt.md@c7fff67

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/sandbox"
)

// projectFileNames are the project context files one folder may hold, best
// first: a folder contributes only the first one it has.
var projectFileNames = []string{
	filepath.Join(".rashin", "AGENTS.md"),
	"AGENTS.md",
	"CLAUDE.md",
	filepath.Join(".claude", "CLAUDE.md"),
}

const (
	// importHops is how deep @path imports nest.
	importHops = 5
	// importMax is how many imports one prompt expands.
	importMax = 64
)

type projectFile struct {
	path    string // relative to the root
	content string
}

// projectContext finds the project's context files from cwd up to the
// project root (cwd alone outside Git), one per folder, expands their @path
// imports and renders them farthest first, so the closest folder's rules
// read last and weigh most. budget caps the bytes of all files together;
// closer files are kept first when it runs short. "" when there are none.
func projectContext(cwd, project string, budget int, specs ...*sandbox.Spec) string {
	const intro = "Follow these files from the project in all work here; a deeper folder's file refines the ones above it.\n"
	if cwd == "" || budget <= len(intro) {
		return ""
	}
	cwd = filepath.Clean(cwd)
	root := cwd
	if project != "" {
		if p := filepath.Clean(project); pathWithin(cwd, p) {
			root = p
		}
	}
	var dirs []string
	for dir := cwd; ; dir = filepath.Dir(dir) {
		dirs = append(dirs, dir)
		if dir == root || dir == filepath.Dir(dir) {
			break
		}
	}
	realRoot := promptRealPath(root)
	im := &importer{root: realRoot, left: importMax}
	if len(specs) > 0 {
		im.sandbox = specs[0]
	}
	remaining := budget - len(intro)
	var found []projectFile
	for _, dir := range dirs {
		for _, name := range projectFileNames {
			path := filepath.Join(dir, name)
			if im.hidden(path) {
				continue
			}
			if !pathWithin(promptRealPath(path), realRoot) {
				continue
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				continue
			}
			open := fmt.Sprintf("<file path=%q>\n", rel)
			const close = "\n</file>\n"
			available := remaining - len(open) - len(close)
			if available <= 0 {
				break
			}
			body, ok := readContextFile(path, available)
			if !ok {
				continue
			}
			im.seen = map[string]bool{promptRealPath(path): true}
			body = promptClip(strings.TrimSpace(im.expand(body, filepath.Dir(path), 0, available)), available)
			if body == "" {
				continue
			}
			if containedIn(body, found) {
				break
			}
			found = append(found, projectFile{path: rel, content: body})
			remaining -= len(open) + len(body) + len(close)
			break
		}
		if remaining <= 0 {
			break
		}
	}
	if len(found) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(intro)
	for i := len(found) - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "<file path=%q>\n%s\n</file>\n", found[i].path, found[i].content)
	}
	return strings.TrimRight(b.String(), "\n")
}

// containedIn reports a farther file whose text a closer one already holds
// whole, such as a CLAUDE.md that only repeats AGENTS.md.
func containedIn(body string, closer []projectFile) bool {
	for _, f := range closer {
		if strings.Contains(f.content, body) {
			return true
		}
	}
	return false
}

// importer expands @path references in context files.
type importer struct {
	root    string
	left    int
	seen    map[string]bool
	sandbox *sandbox.Spec
}

// expand appends each @path a text line names, as an <import> block after
// that line. Imports stay inside the root, skip code, and leave a target
// that is missing, outside, a cycle or past the caps as written.
func (im *importer) expand(text, dir string, depth, budget int) string {
	if depth >= importHops || !strings.Contains(text, "@") || budget <= len(text) {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	fenced := false
	used := len(text)
	for _, line := range lines {
		out = append(out, line)
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, ref := range importRefs(line) {
			if im.left <= 0 || budget-used < 32 {
				break
			}
			path, ok := im.resolve(ref, dir)
			if !ok || im.seen[path] {
				continue
			}
			if im.hidden(path) {
				continue
			}
			rel, err := filepath.Rel(im.root, path)
			if err != nil {
				continue
			}
			open := fmt.Sprintf("<import path=%q>\n", rel)
			const close = "\n</import>"
			available := budget - used - len(open) - len(close) - 1
			if available <= 0 {
				continue
			}
			body, ok := readContextFile(path, available)
			if !ok {
				continue
			}
			im.left--
			im.seen[path] = true
			body = promptClip(strings.TrimSpace(im.expand(body, filepath.Dir(path), depth+1, available)), available)
			delete(im.seen, path)
			if body == "" {
				continue
			}
			block := open + body + close
			used += len(block) + 1
			out = append(out, block)
		}
	}
	return strings.Join(out, "\n")
}

// resolve turns an @path reference into a real path inside the root.
func (im *importer) resolve(ref, dir string) (string, bool) {
	switch {
	case strings.HasPrefix(ref, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		ref = filepath.Join(home, ref[2:])
	case !filepath.IsAbs(ref):
		ref = filepath.Join(dir, ref)
	}
	if im.hidden(ref) {
		return "", false
	}
	path := promptRealPath(filepath.Clean(ref))
	if !pathWithin(path, im.root) {
		return "", false
	}
	return path, true
}

func (im *importer) hidden(path string) bool {
	return im.sandbox != nil && (im.sandbox.Hides(path) || im.sandbox.Hides(promptRealPath(path)))
}

// importRefs are the @path references of one line outside inline code: an
// @ at the start or after a space, so mail addresses and git@host stay text.
func importRefs(line string) []string {
	var refs []string
	for i, part := range strings.Split(line, "`") {
		if i%2 == 1 {
			continue
		}
		for _, field := range strings.Fields(part) {
			if !strings.HasPrefix(field, "@") || len(field) < 2 {
				continue
			}
			ref := strings.TrimRight(field[1:], ".,;:!?)]}\"'")
			if ref == "" || strings.Contains(ref, "@") {
				continue
			}
			refs = append(refs, ref)
		}
	}
	return refs
}

// readContextFile reads a regular text file of at most limit bytes (cut,
// and marked, when longer). False for anything missing, empty or binary.
func readContextFile(path string, limit int) (string, bool) {
	if limit <= 0 {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return "", false
	}
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || bytes.IndexByte(data, 0) >= 0 {
		return "", false
	}
	text := string(data)
	if len(data) > limit {
		text = promptClip(text, limit)
	}
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	return text, true
}

// promptRealPath resolves symlinks where it can, so a link cannot lead an
// import out of the root.
func promptRealPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// pathWithin reports whether path is dir or below it.
func pathWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
