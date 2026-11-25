package code

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

type indexHost struct {
	method string
	params json.RawMessage
}

func (h *indexHost) Call(_ context.Context, method string, params, result any) error {
	h.method = method
	h.params, _ = json.Marshal(params)
	body, _ := json.Marshal(protocol.SearchResult{Index: "project", Hits: []protocol.SearchHit{{Path: "main.go", Start: 3, End: 4}}})
	return json.Unmarshal(body, result)
}
func TestSearchUsesSessionScopedHost(t *testing.T) {
	h := &indexHost{}
	result := indexTool{tool.Search, protocol.MIndexSearch, protocol.ToolSearch, "query", "Search"}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"query":"handler"}`)}, tool.Env{Cwd: "/team/project", Host: h})
	if result.IsError || h.method != protocol.MIndexSearch || !strings.Contains(result.Text, "main.go") {
		t.Fatalf("search: %+v method=%q", result, h.method)
	}
	var p protocol.SearchParams
	if err := json.Unmarshal(h.params, &p); err != nil {
		t.Fatal(err)
	}
	if p.Cwd != "/team/project" || p.ID != "" || p.Query != "handler" {
		t.Fatalf("scope: %+v", p)
	}
}

type gitExecutor struct {
	exec.Executor
	request exec.StartRequest
	output  string
}

func (e *gitExecutor) Start(_ context.Context, request exec.StartRequest, _ exec.OutputFunc) (string, error) {
	e.request = request
	return "git", nil
}
func (e *gitExecutor) Wait(context.Context, string) (exec.Exit, error) {
	return exec.Exit{Output: e.output}, nil
}
func (e *gitExecutor) Release(context.Context, string) error { return nil }
func TestGitStatusParsesPorcelainInsideSessionExecutor(t *testing.T) {
	e := &gitExecutor{output: "# branch.oid a123\n# branch.head main\x001 .M N... 100644 100644 100644 a a src/main.go\x00? scratch.txt\x00"}
	result := gitTool{tool.GitStatus, protocol.ToolRead, "status"}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{}`)}, tool.Env{Cwd: "/team/repo", Exec: e})
	if result.IsError || e.request.Dir != "/team/repo" || !strings.Contains(result.Text, "src/main.go") || !strings.Contains(result.Text, "scratch.txt") || !strings.Contains(result.Text, "main") {
		t.Fatalf("status: %+v request=%+v", result, e.request)
	}
	if !strings.Contains(strings.Join(e.request.Argv, " "), "--porcelain=v2 -z") {
		t.Fatalf("not machine format: %v", e.request.Argv)
	}
}
func TestGitDiffRejectsOutsideWorktree(t *testing.T) {
	e := &gitExecutor{}
	result := gitTool{tool.GitDiff, protocol.ToolRead, "diff"}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"path":"../secrets"}`)}, tool.Env{Cwd: "/team/repo", Exec: e})
	if !result.IsError || len(e.request.Argv) > 0 {
		t.Fatalf("outside worktree reached Git: %+v", result)
	}
}
