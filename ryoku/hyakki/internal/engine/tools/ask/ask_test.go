package ask

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

type searchHost struct {
	indexes []protocol.IndexInfo
	method  string
	params  json.RawMessage
}

func (h *searchHost) Call(_ context.Context, method string, params, result any) error {
	h.method = method
	h.params, _ = json.Marshal(params)
	if method == protocol.MIndexList {
		body, _ := json.Marshal(protocol.IndexesResult{Indexes: h.indexes})
		return json.Unmarshal(body, result)
	}
	body, _ := json.Marshal(protocol.SearchResult{Index: "work", Hits: []protocol.SearchHit{{Path: "src/feature.go", Start: 2, End: 5}}})
	return json.Unmarshal(body, result)
}
func TestSearchCodeUsesOnlyRegisteredProjectIndex(t *testing.T) {
	h := &searchHost{indexes: []protocol.IndexInfo{{ID: "work", Kind: protocol.IndexProject, Root: "/code/work"}, {ID: "other", Kind: protocol.IndexProject, Root: "/code/other"}}}
	env := tool.Env{Cwd: "/code/work/subdir", Host: h}
	got := searchTool{tool.SearchCode}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"query":"feature"}`)}, env)
	if got.IsError || h.method != protocol.MIndexSearch || !strings.Contains(got.Text, "src/feature.go") {
		t.Fatalf("search: %+v method=%s", got, h.method)
	}
	var params protocol.SearchParams
	if err := json.Unmarshal(h.params, &params); err != nil {
		t.Fatal(err)
	}
	if params.ID != "work" || !params.Existing || !params.Compact || params.Limit != 8 {
		t.Fatalf("search scope: %+v", params)
	}
	h.method = ""
	noIndex := searchTool{tool.SearchCode}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"query":"feature","path":"/code/unindexed"}`)}, env)
	if noIndex.IsError || h.method != protocol.MIndexList || !strings.Contains(noIndex.Text, "not indexed") {
		t.Fatalf("non-indexed search: %+v method=%s", noIndex, h.method)
	}
}
func TestSearchSystemUsesExistingSystemIndex(t *testing.T) {
	h := &searchHost{}
	got := searchTool{tool.SearchSystem}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"query":"display"}`)}, tool.Env{Host: h})
	if got.IsError {
		t.Fatalf("search: %+v", got)
	}
	var params protocol.SearchParams
	if err := json.Unmarshal(h.params, &params); err != nil {
		t.Fatal(err)
	}
	if h.method != protocol.MIndexSearch || params.ID != protocol.IndexSystem || !params.Existing || !params.Compact {
		t.Fatalf("scope: %+v", params)
	}
}
func TestProposeClassifiesWithoutExecution(t *testing.T) {
	got := proposeTool{}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"summary":"Inspect logs","commands":[{"command":"","why":"skip"},{"command":"echo hello","why":"show"}]}`)}, tool.Env{})
	if got.IsError || got.Proposal == nil || !got.End || len(got.Proposal.Commands) != 1 || got.Proposal.Commands[0].Why != "show" {
		t.Fatalf("proposal: %+v", got)
	}
}
func TestRegisteredProjectUsesBoundaryAndDeepestRoot(t *testing.T) {
	ref, ok := registeredProject([]protocol.IndexInfo{{ID: "a", Kind: protocol.IndexProject, Root: "/code/app"}, {ID: "b", Kind: protocol.IndexProject, Root: "/code/app/lib"}}, "/code/app/lib/pkg")
	if !ok || ref.ID != "b" {
		t.Fatalf("wrong index: %+v", ref)
	}
	if _, ok := registeredProject([]protocol.IndexInfo{{ID: "a", Kind: protocol.IndexProject, Root: "/code/app"}}, "/code/apple"); ok {
		t.Fatal("matched sibling directory")
	}
}

type askExecutor struct {
	exec.Executor
	starts int
	output string
}

func (e *askExecutor) Start(_ context.Context, _ exec.StartRequest, _ exec.OutputFunc) (string, error) {
	e.starts++
	return "doctor", nil
}
func (e *askExecutor) Wait(context.Context, string) (exec.Exit, error) {
	return exec.Exit{Output: e.output}, nil
}
func (e *askExecutor) Release(context.Context, string) error { return nil }

func TestDoctorFiltersFindingsAndCachesTwoMinutes(t *testing.T) {
	e := &askExecutor{output: `[{"name":"good","status":"pass"},{"name":"needs work","status":"warn","detail":"repair"}]`}
	d := &doctorTool{}
	for range 2 {
		got := d.Run(context.Background(), tool.Call{Args: json.RawMessage(`{}`)}, tool.Env{Exec: e})
		if got.IsError || strings.Contains(got.Text, "good") || !strings.Contains(got.Text, "needs work") {
			t.Fatalf("filtered findings: %+v", got)
		}
	}
	if e.starts != 1 {
		t.Fatalf("doctor ran %d times instead of once", e.starts)
	}
}

func TestWikiRejectsTraversalBeforeReading(t *testing.T) {
	e := &askExecutor{}
	for _, page := range []string{"../secrets", "/etc/passwd"} {
		got := wikiTool{}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"page":` + quoteJSON(page) + `}`)}, tool.Env{Exec: e})
		if !got.IsError || !strings.Contains(got.Text, "invalid") {
			t.Fatalf("wiki accepted %s: %+v", page, got)
		}
	}
}

func quoteJSON(value string) string {
	body, _ := json.Marshal(filepath.ToSlash(value))
	return string(body)
}
