package fs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/glob.ts@c7fff67

// maxPaths is the most paths find_files returns even when asked for more.
const maxPaths = 200

type findTool struct{}

func (findTool) Name() string    { return tool.FindFiles }
func (findTool) Kind() tool.Kind { return protocol.ToolSearch }
func (findTool) ReadOnly() bool  { return true }
func (findTool) Parallel() bool  { return true }
func (findTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return spec(tool.FindFiles, "List files whose path matches a glob like src/**/*.go.",
			`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob such as **/*.md or a file name."},"path":{"type":"string","description":"Folder to search; empty for the working folder."}},"required":["pattern"]}`)
	}
	return spec(tool.FindFiles,
		"Find files by glob, newest first. `**` spans folders; a pattern without a slash matches a name at any depth; `{a,b}` picks either. "+
			"Git-ignored files and .git, node_modules and similar folders are skipped. A pattern ending in / matches folders too.",
		fmt.Sprintf(`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob such as src/**/*.go, *.md or a file name."},"path":{"type":"string","description":"Folder to search; empty for the working folder."},"hidden":{"type":"boolean","description":"Include dot files."},"limit":{"type":"integer","minimum":1,"maximum":%d}},"required":["pattern"]}`, maxPaths))
}

func (findTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	pattern := a.str("pattern", "glob", "name", "query")
	in := pattern
	if p := a.str(pathKeys...); p != "" {
		in = p + "  " + pattern
	}
	return tool.Presentation{Title: i18n.Tf("Finding files matching %s", pattern), Input: in}
}

func (findTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	pattern := strings.TrimSpace(a.str("pattern", "glob", "name", "query"))
	root := resolveFor(env, a.str(append(pathKeys, "dir", "root")...), env.Cwd)
	if pattern == "" {
		if !a.has(pathKeys...) {
			return tool.Failed("pattern is empty. Give a glob, for example {\"pattern\":\"**/*.go\"}.")
		}
		pattern = "**/*"
	}
	dirs := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	root, pattern = scope(ctx, env, root, pattern)
	if err := secret(root); err != nil {
		return tool.Failed(err.Error())
	}
	real, _, exists, err := target(ctx, env.Exec, root)
	if err != nil {
		return tool.Failed(describe(err, display(root, env.Cwd)))
	}
	if exists {
		root = real
	}
	lim := env.Limits
	if lim.Paths <= 0 {
		lim = env.Profile.Limits()
	}
	limit := lim.Paths
	if n, ok := a.integer("limit", "max"); ok && n > 0 {
		limit = min(n, maxPaths)
	}
	res, err := env.Exec.Glob(ctx, exec.GlobRequest{Root: root, Pattern: pattern, Hidden: a.boolean("hidden"), Dirs: dirs, Limit: limit})
	if err != nil {
		if errors.Is(err, exec.ErrBadPattern) {
			return tool.Failed(fmt.Sprintf("The glob %q is not valid: %v. Use * within a name, ** across folders.", pattern, err))
		}
		return tool.Failed(describe(err, display(root, env.Cwd)))
	}
	var b strings.Builder
	shown := 0
	for _, p := range res.Paths {
		if _, blocked := command.SecretPathAt(p, env.Cwd); blocked {
			continue
		}
		b.WriteString(display(p, env.Cwd))
		b.WriteByte('\n')
		shown++
	}
	if shown == 0 {
		return tool.Result{Text: fmt.Sprintf("No files found matching %s under %s. Git-ignored files are skipped; set hidden to include dot files.", pattern, display(root, env.Cwd))}
	}
	if res.Truncated || res.Total > len(res.Paths) {
		fmt.Fprintf(&b, "[Showing %d of %d paths, newest first. Narrow the pattern", shown, res.Total)
		if limit < maxPaths {
			fmt.Fprintf(&b, " or set limit up to %d", maxPaths)
		}
		b.WriteString(".]\n")
	}
	return tool.Result{Text: Shape(env, strings.TrimSuffix(b.String(), "\n"))}
}

// scope moves a pattern's literal leading folders into the root, so
// `/etc/**/*.conf` and `~/notes/*.md` search where they say, and turns a
// plain folder into "everything under it".
func scope(ctx context.Context, env tool.Env, root, pattern string) (string, string) {
	if pattern == "" {
		return root, "**/*"
	}
	if !hasGlob(pattern) {
		abs := resolveFor(env, pattern, root)
		if info, err := env.Exec.Stat(ctx, abs); err == nil && info.Kind == exec.KindDir {
			return abs, "**/*"
		}
		if strings.Contains(pattern, "/") {
			return resolveFor(env, filepath.Dir(pattern), root), filepath.Base(pattern)
		}
		return root, pattern
	}
	if !(filepath.IsAbs(pattern) || strings.HasPrefix(pattern, "~") || strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, "../")) {
		return root, pattern
	}
	parts := strings.Split(pattern, "/")
	i := 0
	for i < len(parts) && !hasGlob(parts[i]) {
		i++
	}
	base := strings.Join(parts[:i], "/")
	if base == "" && strings.HasPrefix(pattern, "/") {
		base = "/"
	}
	return resolveFor(env, base, root), strings.Join(parts[i:], "/")
}

func hasGlob(s string) bool { return strings.ContainsAny(s, "*?[{") }
