package memory

import (
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// reachEngine is an engine with two projects, a team and facts in each, and
// an agent conversation s1 working in project A.
func reachEngine(t *testing.T) (e *Engine, sessions *recordSessions, ids map[string]string, scopeA, scopeB string) {
	t.Helper()
	base := t.TempDir()
	names := stubNames{projects: map[string]stubProject{
		"pa": {root: filepath.Join(base, "a"), name: "Alpha"},
		"pb": {root: filepath.Join(base, "b"), name: "Beta"},
	}}
	e, _, sessions, _ = liveEngine(t, true, nil, names)
	sessions.infos["s1"] = protocol.SessionInfo{ID: "s1", Kind: protocol.KindAgent, Project: "pa"}
	scopeA, _ = e.sessionScope(protocol.SessionInfo{Project: "pa"})
	scopeB, _ = e.sessionScope(protocol.SessionInfo{Project: "pb"})
	ids = map[string]string{}
	for scope, text := range map[string]string{protocol.MemoryYou: "Reports go out on Fridays", scopeA: "Alpha reports go in docs", scopeB: "Beta reports are private", "team:t": "Team reports are weekly"} {
		var change protocol.MemoryChange
		if err := call(t, e, app, protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: scope, Text: text}, &change); err != nil {
			t.Fatal(err)
		}
		ids[scope] = change.Items[0].ID
	}
	return e, sessions, ids, scopeA, scopeB
}

// searchScopesOf is the scopes a search by from found.
func searchScopesOf(t *testing.T, e *Engine, from core.MemoryCaller, scope string) map[string]bool {
	t.Helper()
	var found protocol.MemorySearchResult
	if err := call(t, e, from, protocol.MMemorySearch, protocol.MemorySearchParams{Query: "reports", Scope: scope}, &found); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, m := range found.Memories {
		out[m.Scope] = true
	}
	return out
}

func TestCallersReachOnlyTheirScopes(t *testing.T) {
	e, _, ids, scopeA, scopeB := reachEngine(t)
	agent := core.MemoryCaller{Session: "s1"}
	cli := core.MemoryCaller{}

	if got := searchScopesOf(t, e, agent, ""); len(got) != 2 || !got[protocol.MemoryYou] || !got[scopeA] {
		t.Fatalf("agent found %v", got)
	}
	if got := searchScopesOf(t, e, cli, ""); len(got) != 1 || !got[protocol.MemoryYou] {
		t.Fatalf("command line found %v", got)
	}
	for _, scope := range []string{scopeB, "team:t"} {
		if err := call(t, e, agent, protocol.MMemorySearch, protocol.MemorySearchParams{Query: "reports", Scope: scope}, nil); err == nil {
			t.Fatalf("agent searched %s", scope)
		}
		if err := call(t, e, agent, protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: scope, Text: "Planted"}, nil); err == nil {
			t.Fatalf("agent wrote %s", scope)
		}
		if err := call(t, e, agent, protocol.MMemoryForget, protocol.MemoryForgetParams{IDs: []string{ids[scope]}}, nil); err == nil {
			t.Fatalf("agent forgot in %s", scope)
		}
		if err := call(t, e, agent, protocol.MMemoryGet, protocol.MemoryRefParams{ID: ids[scope]}, nil); err == nil {
			t.Fatalf("agent read %s", scope)
		}
	}
	if err := call(t, e, cli, protocol.MMemoryGet, protocol.MemoryRefParams{ID: ids[scopeA]}, nil); err == nil {
		t.Fatal("the command line read a project")
	}
	var list protocol.MemoryListResult
	if err := call(t, e, cli, protocol.MMemoryList, protocol.MemoryListParams{}, &list); err != nil || list.Total != 1 || list.Memories[0].Scope != protocol.MemoryYou {
		t.Fatalf("command line list = %+v %v", list, err)
	}

	if err := call(t, e, app, protocol.MMemorySuppress, protocol.MemorySuppressParams{Session: "s1", ID: ids[scopeA]}, nil); err != nil {
		t.Fatal(err)
	}
	if got := searchScopesOf(t, e, agent, "project"); len(got) != 0 {
		t.Fatalf("a suppressed fact was found: %v", got)
	}
}

func TestConversationMemoryModeLimitsTheTools(t *testing.T) {
	e, sessions, _, _, _ := reachEngine(t)
	agent := core.MemoryCaller{Session: "s1"}
	sessions.infos["s1"] = protocol.SessionInfo{ID: "s1", Kind: protocol.KindAgent, Memory: protocol.MemoryUseOnly}
	if err := call(t, e, agent, protocol.MMemorySearch, protocol.MemorySearchParams{Query: "reports"}, nil); err != nil {
		t.Fatalf("a use-only conversation could not search: %v", err)
	}
	if err := call(t, e, agent, protocol.MMemoryAdd, protocol.MemoryAddParams{Text: "Likes tea"}, nil); err == nil {
		t.Fatal("a use-only conversation remembered")
	}
	sessions.infos["s1"] = protocol.SessionInfo{ID: "s1", Kind: protocol.KindAgent, Memory: protocol.MemoryNone, Temporary: true}
	for _, method := range []string{protocol.MMemorySearch, protocol.MMemoryList, protocol.MMemoryCard, protocol.MMemoryAdd} {
		if err := call(t, e, agent, method, struct{}{}, nil); err == nil {
			t.Fatalf("%s served a conversation without memory", method)
		}
	}
}
