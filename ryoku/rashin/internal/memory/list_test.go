package memory

import (
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

func TestListQueryMatchesScopeNameAndSourceTitle(t *testing.T) {
	e, sessions, _, scopeA, _ := reachEngine(t)
	sessions.infos["s1"] = protocol.SessionInfo{ID: "s1", Kind: protocol.KindAgent, Project: "pa", Title: "Release checklist"}
	if err := call(t, e, core.MemoryCaller{Session: "s1"}, protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: "project", Text: "Tags go on main"}, nil); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]string{"alpha": scopeA, "checklist": "Tags go on main"} {
		var result protocol.MemoryListResult
		if err := call(t, e, app, protocol.MMemoryList, protocol.MemoryListParams{Query: query}, &result); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, m := range result.Memories {
			found = found || m.Scope == want || m.Text == want
			if m.Scope != scopeA {
				t.Fatalf("%q matched %+v", query, m)
			}
		}
		if !found {
			t.Fatalf("%q found %+v", query, result.Memories)
		}
	}
}
