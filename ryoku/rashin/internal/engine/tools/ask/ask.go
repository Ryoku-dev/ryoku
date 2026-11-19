// Package ask holds the native engine tools offered only in Ask mode.
package ask

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/command"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Tools returns Ask's proposal, doctor, wiki and index search tools.
func Tools() []tool.Tool {
	return []tool.Tool{proposeTool{}, processesTool{}, &doctorTool{}, wikiTool{}, searchTool{tool.SearchSystem}, searchTool{tool.SearchCode}}
}
func askSpec(name, description, properties string, required string) llm.Tool {
	return llm.Tool{Name: name, Description: description, Parameters: json.RawMessage(`{"type":"object","properties":` + properties + `,"required":` + required + `,"additionalProperties":false}`)}
}
func askPresent(title string, raw json.RawMessage, key string) tool.Presentation {
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	input, _ := fields[key].(string)
	input, _ = tool.ClipRunes(input, 200)
	return tool.Presentation{Title: title, Input: input}
}
func askResult(text string, env tool.Env) tool.Result {
	limit := env.Limits
	if limit.Told <= 0 {
		limit = env.Profile.Limits()
	}
	if len(text) > limit.Told {
		text = tool.HeadTail(text, limit.Head, limit.Tail)
	}
	return tool.Result{Text: text}
}

type proposeTool struct{}

func (proposeTool) Name() string    { return tool.Propose }
func (proposeTool) Kind() tool.Kind { return protocol.ToolThink }
func (proposeTool) ReadOnly() bool  { return true }
func (proposeTool) Spec(tool.Profile) llm.Tool {
	return askSpec(tool.Propose, "Propose exact commands for the user to run; never execute them.", `{"summary":{"type":"string"},"commands":{"type":"array","items":{"type":"object","properties":{"command":{"type":"string"},"why":{"type":"string"}},"required":["command"],"additionalProperties":false}}}`, `["commands"]`)
}
func (proposeTool) Present(raw json.RawMessage) tool.Presentation {
	return askPresent(i18n.T("Proposing a plan"), raw, "summary")
}
func (proposeTool) Run(_ context.Context, call tool.Call, _ tool.Env) tool.Result {
	var a struct {
		Summary  string `json:"summary"`
		Commands []struct {
			Command string `json:"command"`
			Why     string `json:"why"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(call.Args, &a); err != nil {
		return tool.Failed(err.Error())
	}
	plan := &protocol.CommandPlan{Summary: strings.TrimSpace(a.Summary)}
	for _, item := range a.Commands {
		cmd := strings.TrimSpace(item.Command)
		if cmd == "" {
			continue
		}
		tier, _ := command.Classify(cmd)
		_, terminal := command.NeedsTerminal(cmd)
		plan.Commands = append(plan.Commands, protocol.PlannedCommand{Command: cmd, Why: strings.TrimSpace(item.Why), Tier: tier, Terminal: terminal})
	}
	if len(plan.Commands) == 0 {
		return tool.Failed(i18n.T("The proposed plan has no commands."))
	}
	body, _ := json.Marshal(plan)
	return tool.Result{Text: string(body), Proposal: plan, End: true}
}

type searchTool struct{ name string }

func (t searchTool) Name() string    { return t.name }
func (t searchTool) Kind() tool.Kind { return protocol.ToolSearch }
func (t searchTool) ReadOnly() bool  { return true }
func (t searchTool) Spec(tool.Profile) llm.Tool {
	if t.name == tool.SearchSystem {
		return askSpec(t.name, "Search the indexed Ryoku system and vault.", `{"query":{"type":"string"}}`, `["query"]`)
	}
	return askSpec(t.name, "Search a previously indexed project rooted at path or the session folder.", `{"query":{"type":"string"},"path":{"type":"string"}}`, `["query"]`)
}
func (t searchTool) Present(raw json.RawMessage) tool.Presentation {
	return askPresent(i18n.T("Searching the index"), raw, "query")
}
func (t searchTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	var a struct {
		Query string `json:"query"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(call.Args, &a); err != nil {
		return tool.Failed(err.Error())
	}
	if strings.TrimSpace(a.Query) == "" {
		return tool.Failed(i18n.T("A search query is required."))
	}
	if env.Host == nil {
		return tool.Failed(i18n.T("The code index is unavailable."))
	}
	ref := protocol.IndexRef{ID: protocol.IndexSystem}
	if t.name == tool.SearchCode {
		cwd := env.Cwd
		if cwd == "" {
			cwd = env.Session.Cwd
		}
		if cwd == "" {
			return tool.Failed(i18n.T("This session has no project folder."))
		}
		target := cwd
		if a.Path != "" {
			target = a.Path
			if !filepath.IsAbs(target) {
				target = filepath.Join(cwd, target)
			}
		}
		var indexes protocol.IndexesResult
		if err := env.Host.Call(ctx, protocol.MIndexList, protocol.Empty{}, &indexes); err != nil {
			return tool.Failed(err.Error())
		}
		var ok bool
		ref, ok = registeredProject(indexes.Indexes, target)
		if !ok {
			return askResult(i18n.Tf("This project is not indexed. Add it in Rashin's Index page or run `rashin index add %s`.", quoteArg(target)), env)
		}
	}
	var result protocol.SearchResult
	if err := env.Host.Call(ctx, protocol.MIndexSearch, protocol.SearchParams{IndexRef: ref, Query: a.Query, Limit: 8, Compact: true, Existing: true}, &result); err != nil {
		return tool.Failed(err.Error())
	}
	body, _ := json.Marshal(result)
	return askResult(string(body), env)
}
func registeredProject(indexes []protocol.IndexInfo, target string) (protocol.IndexRef, bool) {
	target = filepath.Clean(target)
	best := ""
	depth := -1
	for _, index := range indexes {
		if index.Kind != protocol.IndexProject || index.ID == "" || index.ID == protocol.IndexSystem {
			continue
		}
		roots := append([]string{index.Root}, index.Roots...)
		for _, root := range roots {
			if root == "" {
				continue
			}
			root = filepath.Clean(root)
			rel, err := filepath.Rel(root, target)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && len(root) > depth {
				best = index.ID
				depth = len(root)
			}
		}
	}
	if best == "" {
		return protocol.IndexRef{}, false
	}
	return protocol.IndexRef{ID: best}, true
}
func quoteArg(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

type wikiTool struct{}

func (wikiTool) Name() string    { return tool.ReadWiki }
func (wikiTool) Kind() tool.Kind { return protocol.ToolRead }
func (wikiTool) ReadOnly() bool  { return true }
func (wikiTool) Spec(tool.Profile) llm.Tool {
	return askSpec(tool.ReadWiki, "Read one named page from the Ryoku vault wiki.", `{"page":{"type":"string"}}`, `["page"]`)
}
func (wikiTool) Present(raw json.RawMessage) tool.Presentation {
	return askPresent(i18n.T("Reading the wiki"), raw, "page")
}
func (wikiTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	var a struct {
		Page string `json:"page"`
	}
	if err := json.Unmarshal(call.Args, &a); err != nil {
		return tool.Failed(err.Error())
	}
	if env.Exec == nil {
		return tool.Failed(i18n.T("The wiki is unavailable."))
	}
	page := strings.TrimSpace(a.Page)
	if filepath.Ext(page) == "" {
		page += ".md"
	}
	clean := filepath.Clean(filepath.FromSlash(page))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return tool.Failed(i18n.T("The wiki page name is invalid."))
	}
	root := paths.VaultDir()
	if _, err := env.Exec.Stat(ctx, root); err != nil {
		root = paths.LegacyVaultDir()
	}
	wiki := filepath.Join(root, "wiki")
	path := filepath.Join(wiki, clean)
	if err := noSymlinks(ctx, env.Exec, wiki, clean); err == nil {
		read, err := env.Exec.Read(ctx, exec.ReadRequest{Path: path, Limit: 64*1024 + 1})
		if err == nil {
			body := read.Data
			if len(body) > 64*1024 || read.Truncated {
				if len(body) > 64*1024 {
					body = body[:64*1024]
				}
				return askResult(string(body)+"\n[trimmed]", env)
			}
			if read.Binary {
				return tool.Failed(i18n.T("The wiki page is not text."))
			}
			return askResult(string(body), env)
		}
	}
	pages := wikiPages(ctx, env.Exec, wiki, "", 0)
	sort.Strings(pages)
	return tool.Failed(i18n.Tf("Wiki page %q was not found. Available pages: %s", page, strings.Join(pages, ", ")))
}
func noSymlinks(ctx context.Context, e exec.Executor, root, relative string) error {
	info, err := e.Stat(ctx, root)
	if err != nil {
		return err
	}
	if info.Kind == exec.KindSymlink {
		return fmt.Errorf("symlink")
	}
	path := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		path = filepath.Join(path, part)
		info, err = e.Stat(ctx, path)
		if err != nil {
			return err
		}
		if info.Kind == exec.KindSymlink {
			return fmt.Errorf("symlink")
		}
	}
	return nil
}
func wikiPages(ctx context.Context, e exec.Executor, root, relative string, depth int) []string {
	if depth >= 8 {
		return nil
	}
	listing, err := e.List(ctx, exec.ListRequest{Path: filepath.Join(root, relative), Limit: 2048})
	if err != nil {
		return nil
	}
	var pages []string
	for _, entry := range listing.Entries {
		if entry.Kind == exec.KindSymlink {
			continue
		}
		name := filepath.Join(relative, entry.Name)
		if entry.Kind == exec.KindDir {
			pages = append(pages, wikiPages(ctx, e, root, name, depth+1)...)
		} else if entry.Kind == exec.KindFile && strings.EqualFold(filepath.Ext(name), ".md") {
			pages = append(pages, filepath.ToSlash(name))
		}
	}
	return pages
}

type doctorTool struct {
	mu      sync.Mutex
	at      time.Time
	out     string
	err     error
	loading chan struct{}
}

func (*doctorTool) Name() string    { return tool.Doctor }
func (*doctorTool) Kind() tool.Kind { return protocol.ToolRead }
func (*doctorTool) ReadOnly() bool  { return true }
func (*doctorTool) Spec(tool.Profile) llm.Tool {
	return askSpec(tool.Doctor, "Run Ryoku doctor; show only todo, warn and fail findings.", `{}`, `[]`)
}
func (*doctorTool) Present(json.RawMessage) tool.Presentation {
	return tool.Presentation{Title: i18n.T("Checking system health")}
}
func (t *doctorTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	if env.Exec == nil {
		return tool.Failed(i18n.T("Ryoku doctor is unavailable."))
	}
	for {
		t.mu.Lock()
		if !t.at.IsZero() && time.Since(t.at) < 2*time.Minute {
			out, err := t.out, t.err
			t.mu.Unlock()
			if err != nil {
				return tool.Failed(err.Error())
			}
			return askResult(out, env)
		}
		if t.loading != nil {
			loading := t.loading
			t.mu.Unlock()
			select {
			case <-ctx.Done():
				return tool.Failed(ctx.Err().Error())
			case <-loading:
				continue
			}
		}
		t.loading = make(chan struct{})
		t.mu.Unlock()
		out, err := runDoctor(ctx, env.Exec)
		t.mu.Lock()
		if ctx.Err() == nil {
			t.at, t.out, t.err = time.Now(), out, err
		}
		close(t.loading)
		t.loading = nil
		t.mu.Unlock()
		if err != nil {
			return tool.Failed(err.Error())
		}
		return askResult(out, env)
	}
}

type doctorFinding struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

func runDoctor(ctx context.Context, e exec.Executor) (string, error) {
	id, err := e.Start(ctx, exec.StartRequest{Argv: []string{"ryoku", "doctor", "--json"}, Dir: paths.Home(), Timeout: 90 * time.Second, Keep: 1024 * 1024}, nil)
	if err != nil {
		return "", err
	}
	defer e.Release(context.WithoutCancel(ctx), id)
	output, err := e.Wait(ctx, id)
	if err != nil {
		_ = e.Kill(context.WithoutCancel(ctx), id)
		return "", err
	}
	var findings []doctorFinding
	if err := json.Unmarshal([]byte(output.Output), &findings); err != nil {
		if output.Code != 0 {
			return output.Output, fmt.Errorf("%s", output.Output)
		}
		return output.Output, fmt.Errorf(i18n.T("Ryoku doctor returned invalid JSON: %v"), err)
	}
	issues := make([]doctorFinding, 0, len(findings))
	for _, f := range findings {
		switch f.Status {
		case "todo", "warn", "fail":
			issues = append(issues, f)
		}
	}
	body, _ := json.Marshal(issues)
	return string(body), nil
}
