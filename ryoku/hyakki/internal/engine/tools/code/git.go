package code

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type gitTool struct{ name, kind, description string }

func (t gitTool) Name() string    { return t.name }
func (t gitTool) Kind() tool.Kind { return t.kind }
func (t gitTool) ReadOnly() bool  { return true }
func (t gitTool) Spec(p tool.Profile) llm.Tool {
	props := map[string]any{}
	switch t.name {
	case tool.GitDiff:
		props["mode"] = map[string]any{"type": "string", "enum": []string{protocol.DiffWorking, protocol.DiffStaged, protocol.DiffBase, protocol.DiffCommit, protocol.DiffRange}}
		for _, k := range []string{"base", "commit", "from", "to", "path"} {
			props[k] = map[string]any{"type": "string"}
		}
		props["context"] = map[string]any{"type": "integer"}
	case tool.GitLog:
		props["limit"] = map[string]any{"type": "integer"}
		props["search"] = map[string]any{"type": "string"}
		props["ref"] = map[string]any{"type": "string"}
	case tool.GitShow:
		props["hash"] = map[string]any{"type": "string"}
	}
	required := []string{}
	if t.name == tool.GitShow {
		required = []string{"hash"}
	}
	raw, _ := json.Marshal(map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false})
	return llm.Tool{Name: t.name, Description: t.description, Parameters: raw}
}
func (t gitTool) Present(raw json.RawMessage) tool.Presentation {
	var a struct {
		Hash string `json:"hash"`
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal(raw, &a)
	input := a.Path
	if input == "" {
		input = a.Hash
	}
	if input == "" {
		input = a.Mode
	}
	input, _ = tool.ClipRunes(input, 200)
	return tool.Presentation{Title: i18n.T("Reading Git history"), Input: input}
}
func (t gitTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	if env.Exec == nil {
		return tool.Failed(i18n.T("Git tools need a session executor."))
	}
	dir := env.Cwd
	if dir == "" {
		dir = env.Session.Cwd
	}
	if dir == "" {
		return tool.Failed(i18n.T("This session has no project folder."))
	}
	var a struct {
		Mode    string `json:"mode"`
		Base    string `json:"base"`
		Commit  string `json:"commit"`
		From    string `json:"from"`
		To      string `json:"to"`
		Path    string `json:"path"`
		Context int    `json:"context"`
		Limit   int    `json:"limit"`
		Search  string `json:"search"`
		Ref     string `json:"ref"`
		Hash    string `json:"hash"`
	}
	if err := json.Unmarshal(call.Args, &a); err != nil {
		return tool.Failed(err.Error())
	}
	if a.Path != "" && (filepath.IsAbs(a.Path) || strings.HasPrefix(filepath.Clean(a.Path), ".."+string(filepath.Separator)) || filepath.Clean(a.Path) == "..") {
		return tool.Failed(i18n.T("Git path must stay inside the session's worktree."))
	}
	var args []string
	switch t.name {
	case tool.GitStatus:
		args = []string{"status", "--porcelain=v2", "-z", "--branch", "--untracked-files=normal"}
	case tool.GitDiff:
		args = []string{"diff", "--no-ext-diff", "--no-textconv", "--patch"}
		if a.Context > 0 && a.Context <= 100 {
			args = append(args, "--unified="+strconv.Itoa(a.Context))
		}
		mode := a.Mode
		if mode == "" {
			mode = protocol.DiffWorking
		}
		switch mode {
		case protocol.DiffWorking:
		case protocol.DiffStaged:
			args = append(args, "--cached")
		case protocol.DiffBase:
			if !safeRevision(a.Base) {
				return tool.Failed(i18n.T("A valid base revision is required."))
			}
			args = append(args, a.Base)
		case protocol.DiffCommit:
			if !safeRevision(a.Commit) {
				return tool.Failed(i18n.T("A valid commit is required."))
			}
			args = []string{"show", "--no-ext-diff", "--no-textconv", "--format=", "--patch", "--end-of-options", a.Commit}
			if a.Context > 0 && a.Context <= 100 {
				args = append(args[:len(args)-2], append([]string{"--unified=" + strconv.Itoa(a.Context)}, args[len(args)-2:]...)...)
			}
		case protocol.DiffRange:
			if !safeRevision(a.From) || !safeRevision(a.To) {
				return tool.Failed(i18n.T("Two valid revisions are required."))
			}
			args = append(args, a.From, a.To)
		default:
			return tool.Failed(i18n.T("Unknown Git diff mode."))
		}
		args = append(args, "--")
		if a.Path != "" {
			args = append(args, a.Path)
		}
	case tool.GitLog:
		limit := a.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		args = []string{"log", "--no-ext-diff", "--no-show-signature", "--format=%H%x00%h%x00%an%x00%aI%x00%s%x00", "--max-count=" + strconv.Itoa(limit)}
		if a.Search != "" {
			args = append(args, "--fixed-strings", "--grep="+a.Search)
		}
		if a.Ref != "" {
			if !safeRevision(a.Ref) {
				return tool.Failed(i18n.T("Invalid Git revision."))
			}
			args = append(args, "--end-of-options", a.Ref)
		}
	case tool.GitShow:
		if !safeRevision(a.Hash) {
			return tool.Failed(i18n.T("A valid commit hash is required."))
		}
		args = []string{"show", "--no-ext-diff", "--no-textconv", "--no-show-signature", "--format=fuller", "--stat", "--end-of-options", a.Hash}
	}
	output, err := gitExec(ctx, env.Exec, dir, args...)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if t.name == tool.GitStatus {
		output = parseStatus(output)
	}
	if t.name == tool.GitLog {
		output = parseLog(output)
	}
	limits := env.Limits
	if limits.Told <= 0 {
		limits = env.Profile.Limits()
	}
	if len(output) > limits.Told {
		output = tool.HeadTail(output, limits.Head, limits.Tail)
	}
	return tool.Result{Text: output}
}
func safeRevision(value string) bool {
	return value != "" && !strings.HasPrefix(value, "-") && !strings.ContainsAny(value, "\x00\n\r")
}
func gitExec(ctx context.Context, e exec.Executor, dir string, args ...string) (string, error) {
	id, err := e.Start(ctx, exec.StartRequest{Argv: append([]string{"git", "-c", "core.pager=cat", "-c", "color.ui=false"}, args...), Dir: dir, Timeout: 30 * time.Second, Keep: 512 << 10}, nil)
	if err != nil {
		return "", err
	}
	defer e.Release(context.WithoutCancel(ctx), id)
	out, err := e.Wait(ctx, id)
	if err != nil {
		_ = e.Kill(context.WithoutCancel(ctx), id)
		return "", err
	}
	if out.Code != 0 {
		return "", fmt.Errorf("%s: %s", i18n.T("Git read failed"), strings.TrimSpace(out.Output))
	}
	if out.Truncated {
		return "", fmt.Errorf("%s", i18n.T("Git output was too large. Narrow the diff, log, or status query."))
	}
	return out.Output, nil
}
func parseStatus(raw string) string {
	type entry struct {
		Path     string `json:"path"`
		OldPath  string `json:"oldPath,omitempty"`
		Index    string `json:"index,omitempty"`
		Worktree string `json:"worktree,omitempty"`
		Kind     string `json:"kind"`
	}
	result := struct {
		Branch   string  `json:"branch,omitempty"`
		Head     string  `json:"head,omitempty"`
		Upstream string  `json:"upstream,omitempty"`
		Files    []entry `json:"files"`
	}{Files: []entry{}}
	lines := strings.Split(raw, "\x00")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			for _, field := range strings.Split(line, "\n") {
				switch {
				case strings.HasPrefix(field, "# branch.head "):
					result.Branch = strings.TrimPrefix(field, "# branch.head ")
				case strings.HasPrefix(field, "# branch.oid "):
					result.Head = strings.TrimPrefix(field, "# branch.oid ")
				case strings.HasPrefix(field, "# branch.upstream "):
					result.Upstream = strings.TrimPrefix(field, "# branch.upstream ")
				}
			}
			continue
		}
		switch line[0] {
		case '1', '2':
			n := 9
			if line[0] == '2' {
				n = 10
			}
			fields := strings.SplitN(line, " ", n)
			if len(fields) < n {
				continue
			}
			xy := fields[1]
			f := entry{Path: fields[n-1], Kind: "changed"}
			if len(xy) >= 2 {
				f.Index = xy[:1]
				f.Worktree = xy[1:2]
			}
			if line[0] == '2' && i+1 < len(lines) {
				i++
				f.OldPath = lines[i]
			}
			result.Files = append(result.Files, f)
		case 'u':
			fields := strings.SplitN(line, " ", 11)
			if len(fields) == 11 {
				result.Files = append(result.Files, entry{Path: fields[10], Kind: "conflict"})
			}
		case '?':
			result.Files = append(result.Files, entry{Path: strings.TrimPrefix(line, "? "), Kind: "untracked"})
		}
	}
	b, _ := json.Marshal(result)
	return string(b)
}
func parseLog(raw string) string {
	var entries []map[string]string
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) < 5 {
			continue
		}
		entries = append(entries, map[string]string{"hash": f[0], "short": f[1], "author": f[2], "date": f[3], "subject": f[4]})
	}
	if entries == nil {
		entries = []map[string]string{}
	}
	b, _ := json.Marshal(entries)
	return string(b)
}
