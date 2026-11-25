package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const (
	// subEntries and subDirs bound the second level of a folder listing.
	subEntries = 12
	subDirs    = 16
)

type lsTool struct{}

func (lsTool) Name() string    { return tool.Ls }
func (lsTool) Kind() tool.Kind { return protocol.ToolRead }
func (lsTool) ReadOnly() bool  { return true }
func (lsTool) Parallel() bool  { return true }
func (lsTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return spec(tool.Ls, "List a folder's files and folders.",
			`{"type":"object","properties":{"path":{"type":"string","description":"Folder path; empty for the working folder."}}}`)
	}
	return spec(tool.Ls,
		"List a folder: folders first (ending in /), then files with their size and age. Set depth 2 to look one level into each subfolder; hidden includes dot entries.",
		`{"type":"object","properties":{"path":{"type":"string","description":"Folder path; empty for the working folder."},"depth":{"type":"integer","minimum":1,"maximum":2},"hidden":{"type":"boolean"}}}`)
}

func (lsTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	p := a.str(pathKeys...)
	if p == "" {
		p = "."
	}
	return tool.Presentation{Title: i18n.Tf("Listing %s", p), Input: p, Locations: []protocol.Location{{Path: p}}}
}

func (lsTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	p := strings.TrimSpace(a.str(append(pathKeys, "dir", "directory", "folder")...))
	if p == "" {
		p = "."
	}
	abs := resolveFor(env, p, env.Cwd)
	shown := display(abs, env.Cwd)
	if err := secret(abs); err != nil {
		return tool.Failed(err.Error())
	}
	real, info, exists, err := target(ctx, env.Exec, abs)
	if err != nil {
		return tool.Failed(describe(err, shown))
	}
	if !exists {
		return tool.Failed(fmt.Sprintf("%s does not exist. Check the path with find_files.", shown))
	}
	if info.Kind != exec.KindDir {
		return tool.Failed(fmt.Sprintf("%s is a file, not a folder. Use read to see it.", shown))
	}
	depth, _ := a.integer("depth")
	depth = min(max(depth, 1), 2)
	out, err := listing(ctx, env, real, shown, depth, a.boolean("hidden", "all"))
	if err != nil {
		return tool.Failed(describe(err, shown))
	}
	return tool.Result{Text: Shape(env, out), Locations: []protocol.Location{{Path: real}}}
}

// listing prints a folder for the model, depth 1 or 2.
func listing(ctx context.Context, env tool.Env, abs, shown string, depth int, hidden bool) (string, error) {
	lim := env.Limits
	if lim.Entries <= 0 {
		lim = env.Profile.Limits()
	}
	res, err := env.Exec.List(ctx, exec.ListRequest{Path: abs, Hidden: hidden, Limit: lim.Entries})
	if err != nil {
		return "", err
	}
	if res.Total == 0 {
		return fmt.Sprintf("%s/ (empty folder)", strings.TrimSuffix(shown, "/")), nil
	}
	now := time.Now()
	ages := !env.Profile.Compact()
	var b strings.Builder
	fmt.Fprintf(&b, "%s/\n", strings.TrimSuffix(shown, "/"))
	expanded := 0
	for _, e := range res.Entries {
		b.WriteString("  " + entryLine(e, now, ages) + "\n")
		if depth < 2 || e.Kind != exec.KindDir || expanded >= subDirs || skipDir(e.Name) {
			continue
		}
		if _, blocked := command.SecretPathAt(e.Path, env.Cwd); blocked {
			continue
		}
		expanded++
		sub, err := env.Exec.List(ctx, exec.ListRequest{Path: e.Path, Hidden: hidden, Limit: subEntries})
		if err != nil {
			continue
		}
		for _, s := range sub.Entries {
			b.WriteString("    " + entryLine(s, now, ages) + "\n")
		}
		if sub.Total > len(sub.Entries) {
			fmt.Fprintf(&b, "    … %d more\n", sub.Total-len(sub.Entries))
		}
	}
	if res.Truncated || res.Total > len(res.Entries) {
		fmt.Fprintf(&b, "[%d of %d entries shown. Narrow with find_files.]\n", len(res.Entries), res.Total)
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func skipDir(name string) bool {
	for _, s := range exec.SkipDirs {
		if s == name {
			return true
		}
	}
	return false
}

func entryLine(e exec.FileInfo, now time.Time, ages bool) string {
	switch e.Kind {
	case exec.KindDir:
		return e.Name + "/"
	case exec.KindSymlink:
		return e.Name + " -> " + e.Target
	}
	line := e.Name + "  " + size(e.Size)
	if ages && !e.ModTime.IsZero() {
		line += "  " + age(now.Sub(e.ModTime))
	}
	return line
}

func age(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
}

// listDirTool is Ask's list_dir, kept under its name, schema and output.
type listDirTool struct{}

func (listDirTool) Name() string    { return tool.ListDir }
func (listDirTool) Kind() tool.Kind { return protocol.ToolRead }
func (listDirTool) ReadOnly() bool  { return true }
func (listDirTool) Parallel() bool  { return true }
func (listDirTool) Spec(tool.Profile) llm.Tool {
	return spec(tool.ListDir, "List a local directory without changing it.",
		`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)
}

func (listDirTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	p := a.str(pathKeys...)
	return tool.Presentation{Title: i18n.Tf("Listing %s", p), Input: p, Locations: []protocol.Location{{Path: p}}}
}

func (listDirTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	abs := resolveFor(env, a.str(pathKeys...), env.Cwd)
	if marker, blocked := command.SecretPathAt(abs, env.Cwd); blocked {
		return tool.Failed(i18n.Tf("Refused to list %s because it contains %s.", abs, marker))
	}
	real, info, exists, err := target(ctx, env.Exec, abs)
	if err != nil {
		return tool.Failed(describe(err, abs))
	}
	if !exists {
		return tool.Failed(fmt.Sprintf("%s does not exist.", abs))
	}
	if info.Kind != exec.KindDir {
		return tool.Failed(i18n.T("Refused to list a path that is not a directory."))
	}
	res, err := env.Exec.List(ctx, exec.ListRequest{Path: real, Hidden: true, Limit: 256})
	if err != nil {
		return tool.Failed(describe(err, abs))
	}
	var b strings.Builder
	for _, e := range res.Entries {
		kind := "file"
		switch e.Kind {
		case exec.KindDir:
			kind = "dir"
		case exec.KindSymlink:
			kind = "link"
		}
		fmt.Fprintf(&b, "%s\t%s\n", kind, e.Name)
	}
	if res.Truncated {
		b.WriteString("[more entries omitted]\n")
	}
	return tool.Result{Text: Shape(env, strings.TrimSpace(b.String())), Locations: []protocol.Location{{Path: filepath.Clean(real)}}}
}
