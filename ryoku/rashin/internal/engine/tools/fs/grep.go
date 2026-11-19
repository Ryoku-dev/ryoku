package fs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/grep.ts@c7fff67

const (
	// singleFileMatches caps matches when grep searches one file.
	singleFileMatches = 50
	maxContext        = 3
)

type grepTool struct{}

func (grepTool) Name() string    { return tool.Grep }
func (grepTool) Kind() tool.Kind { return protocol.ToolSearch }
func (grepTool) ReadOnly() bool  { return true }
func (grepTool) Parallel() bool  { return true }
func (grepTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return spec(tool.Grep, "Search file contents with a regex. Give path to search one folder or file.",
			`{"type":"object","properties":{"pattern":{"type":"string","description":"Regular expression to find."},"path":{"type":"string","description":"File, folder or glob; empty for the working folder."},"ignore_case":{"type":"boolean"}},"required":["pattern"]}`)
	}
	return spec(tool.Grep,
		"Search file contents with an RE2 regular expression (a pattern that is not valid regex is searched as plain text). "+
			"Results are grouped by file as `LINE:text` rows. Git-ignored files and .git, node_modules and similar folders are skipped. "+
			"path takes a file, a folder or a glob such as src/**/*.go; glob limits which files are searched; use skip to page through more files.",
		`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string","description":"File, folder or glob; empty for the working folder."},"glob":{"type":"string","description":"Only files matching this glob, such as *.go."},"ignore_case":{"type":"boolean"},"literal":{"type":"boolean","description":"Match the pattern as plain text."},"context":{"type":"integer","minimum":0,"maximum":3,"description":"Lines shown around each match."},"hidden":{"type":"boolean","description":"Include dot files."},"skip":{"type":"integer","minimum":0,"description":"Files to skip, for the next page."}},"required":["pattern"]}`)
}

func (grepTool) Present(raw json.RawMessage) tool.Presentation {
	a, _ := decode(raw)
	pattern := a.str("pattern", "query", "regex", "search")
	in := pattern
	if p := a.str(pathKeys...); p != "" {
		in = pattern + "  " + p
	}
	return tool.Presentation{Title: i18n.Tf("Searching for %s", pattern), Input: in}
}

func (grepTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	a, err := decode(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	pattern := a.str("pattern", "query", "regex", "search")
	if pattern == "" {
		return tool.Failed("pattern is empty. Give the text or regex to find, for example {\"pattern\":\"func main\"}.")
	}
	lim := env.Limits
	if lim.GrepFiles <= 0 {
		lim = env.Profile.Limits()
	}
	root := resolveFor(env, a.str(append(pathKeys, "dir", "root")...), env.Cwd)
	include := a.str("glob", "include", "files")
	if p := a.str(pathKeys...); hasGlob(p) {
		var glob string
		root, glob = scope(ctx, env, resolveFor(env, ".", env.Cwd), p)
		if include == "" {
			include = glob
		}
	}
	if err := secret(root); err != nil {
		return tool.Failed(err.Error())
	}
	real, info, exists, err := target(ctx, env.Exec, root)
	if err != nil {
		return tool.Failed(describe(err, display(root, env.Cwd)))
	}
	if !exists {
		return tool.Failed(describe(exec.ErrNotFound, display(root, env.Cwd)))
	}
	root = real
	skip, _ := a.integer("skip", "offset")
	skip = max(skip, 0)
	req := exec.GrepRequest{
		Root:       root,
		Pattern:    pattern,
		Literal:    a.boolean("literal", "fixed_strings"),
		IgnoreCase: a.boolean("ignore_case", "case_insensitive", "i", "-i") || (a.has("case") && !a.boolean("case")),
		Include:    include,
		Hidden:     a.boolean("hidden"),
		MaxFiles:   skip + lim.GrepFiles,
		MaxPerFile: lim.GrepPerFile,
	}
	if !env.Profile.Compact() {
		c, _ := a.integer("context", "C")
		req.Context = min(max(c, 0), maxContext)
	}
	if info.Kind == exec.KindFile {
		req.MaxFiles = 1
		req.MaxPerFile = singleFileMatches
	}
	note := ""
	res, err := env.Exec.Grep(ctx, req)
	if errors.Is(err, exec.ErrBadPattern) && !req.Literal {
		req.Literal = true
		note = fmt.Sprintf("[%q is not a valid regular expression, so it was searched as plain text.]\n", pattern)
		res, err = env.Exec.Grep(ctx, req)
	}
	if err != nil {
		return tool.Failed(describe(err, display(root, env.Cwd)))
	}
	out, files := grepText(res, skip, lim.LineBytes, env.Cwd)
	if files == 0 {
		if skip > 0 && res.Files > 0 {
			return tool.Result{Text: note + fmt.Sprintf("No more results (%d files matched; skip=%d is past the end).", res.Files, skip)}
		}
		return tool.Result{Text: note + "No matches found."}
	}
	text := note + out
	if res.Truncated {
		next := skip + files
		if env.Profile.Compact() {
			text += fmt.Sprintf("\n[Matches in %d files; more were left out. Narrow path or pattern.]", res.Files)
		} else {
			text += fmt.Sprintf("\n[Matches in %d files; showing files %d-%d with at most %d matches each. Use skip=%d for the next page, or narrow path or pattern.]",
				res.Files, skip+1, next, req.MaxPerFile, next)
		}
	}
	return tool.Result{Text: Shape(env, text)}
}

// grepText groups matches by file, skipping the first skip files and any
// secret path. It returns the text and how many files it shows.
func grepText(res exec.GrepResult, skip, lineBytes int, cwd string) (string, int) {
	var b strings.Builder
	file := ""
	index := -1
	files := 0
	for _, m := range res.Matches {
		if m.Path != file {
			file = m.Path
			index++
			if index < skip {
				continue
			}
			if _, blocked := command.SecretPathAt(m.Path, cwd); blocked {
				continue
			}
			if files > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(display(m.Path, cwd))
			b.WriteByte('\n')
			files++
		}
		if index < skip {
			continue
		}
		if _, blocked := command.SecretPathAt(m.Path, cwd); blocked {
			continue
		}
		for i, c := range m.Before {
			writeRow(&b, m.Line-len(m.Before)+i, '-', c, lineBytes)
		}
		writeRow(&b, m.Line, ':', m.Text, lineBytes)
		for i, c := range m.After {
			writeRow(&b, m.Line+1+i, '-', c, lineBytes)
		}
	}
	return strings.TrimSuffix(b.String(), "\n"), files
}

func writeRow(b *strings.Builder, n int, sep byte, text string, lineBytes int) {
	text, _ = clipLine(strings.TrimRight(text, "\r"), lineBytes)
	b.WriteString(strconv.Itoa(n))
	b.WriteByte(sep)
	b.WriteString(text)
	b.WriteByte('\n')
}
