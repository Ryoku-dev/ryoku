package todo

import (
	"context"
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

func run(t *testing.T, raw string) tool.Result {
	t.Helper()
	return todoTool{}.Run(context.Background(), tool.Call{Args: json.RawMessage(raw)}, tool.Env{})
}

func TestCompactMarkersBecomeThePlan(t *testing.T) {
	res := run(t, `{"items":["[x] Read the config","- [>] Fix the parser","2. [ ] Run the tests","[-] Rewrite it all"]}`)
	if res.IsError || res.Plan == nil {
		t.Fatalf("no plan: %q", res.Text)
	}
	want := []protocol.PlanEntry{
		{Content: "Read the config", Status: Completed},
		{Content: "Fix the parser", Status: InProgress},
		{Content: "Run the tests", Status: Pending},
	}
	if len(res.Plan.Entries) != len(want) {
		t.Fatalf("entries %+v", res.Plan.Entries)
	}
	for i, entry := range res.Plan.Entries {
		if entry != want[i] {
			t.Fatalf("entry %d %+v, want %+v", i, entry, want[i])
		}
	}
}

func TestOneTaskStaysInProgress(t *testing.T) {
	res := run(t, `{"todos":[{"content":"a","status":"done"},{"content":"b","status":"in-progress"},{"content":"c","status":"active"}]}`)
	got := []string{res.Plan.Entries[0].Status, res.Plan.Entries[1].Status, res.Plan.Entries[2].Status}
	if got[0] != Completed || got[1] != InProgress || got[2] != Pending {
		t.Fatalf("statuses %v", got)
	}
	res = run(t, `{"todos":[{"content":"a","status":"completed"},{"content":"b","status":"pending"}]}`)
	if res.Plan.Entries[1].Status != InProgress {
		t.Fatalf("the next pending task did not start: %+v", res.Plan.Entries)
	}
}

func TestBadListsAreRefusedWithoutAPlan(t *testing.T) {
	for _, raw := range []string{`{}`, `{"todos":[{"content":"a","status":"maybe"}]}`, `{"items":[""]}`} {
		if res := run(t, raw); !res.IsError || res.Plan != nil {
			t.Fatalf("%s: %+v", raw, res)
		}
	}
	if res := run(t, `{"todos":[]}`); res.IsError || res.Plan == nil || len(res.Plan.Entries) != 0 {
		t.Fatalf("an empty list did not clear the plan: %+v", res)
	}
}
