package harness

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

func briefedRunner(t *testing.T, mode string, resume json.RawMessage) (*runner, *testSink) {
	t.Helper()
	t.Setenv("RASHIN_ACP_TEST_MODE", mode)
	r := newRunner(core.HarnessOptions{
		Cwd:       t.TempDir(),
		Approvals: func() string { return protocol.ApprovalsAsk },
		Brief:     func() string { return "You run inside Rashin." },
		Resume:    resume,
	}, []string{os.Args[0], "-test.run=^TestACPHelperProcess$"})
	t.Cleanup(func() { _ = r.Stop() })
	return r, newTestSink()
}

func promptBlocks(t *testing.T, r *runner, sink *testSink, text string, turn int) []map[string]any {
	t.Helper()
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: turn, Text: text}, sink); err != nil {
		t.Fatal(err)
	}
	said := lastOf[protocol.TextData](t, sink, protocol.EvText)
	var blocks []map[string]any
	if err := json.Unmarshal([]byte(said.Delta), &blocks); err != nil {
		t.Fatalf("the agent echoed %q", said.Delta)
	}
	return blocks
}

// An agent hears where it runs once, ahead of its first prompt and never
// again, and the user's words stay their own block.
func TestAnAgentIsBriefedAheadOfItsFirstPromptOnly(t *testing.T) {
	r, sink := briefedRunner(t, "echo", nil)
	first := promptBlocks(t, r, sink, "Fix the build", 1)
	if len(first) != 2 || first[0]["text"] != "You run inside Rashin." || first[1]["text"] != "Fix the build" {
		t.Fatalf("the first prompt was handed as %v", first)
	}
	second := promptBlocks(t, r, sink, "And the tests", 2)
	if len(second) != 1 || second[0]["text"] != "And the tests" {
		t.Fatalf("the second prompt was handed as %v", second)
	}
	var state resumeState
	if err := json.Unmarshal(r.State(), &state); err != nil || state.Briefed != "fake-session" {
		t.Fatalf("the runner's saved state reads %s", r.State())
	}
}

// A session the agent loads back still holds the brief in its own history, so
// a restart of the daemon does not say it twice. One the agent could not load
// starts empty and hears it again.
func TestOnlyAFreshAgentSessionIsBriefedAgain(t *testing.T) {
	resume, _ := json.Marshal(resumeState{ACPSession: "fake-session", Load: true, Briefed: "fake-session"})
	r, sink := briefedRunner(t, "echo-load", resume)
	if blocks := promptBlocks(t, r, sink, "Carry on", 3); len(blocks) != 1 || blocks[0]["text"] != "Carry on" {
		t.Fatalf("a loaded session was handed %v", blocks)
	}

	lost, _ := json.Marshal(resumeState{ACPSession: "older-session", Load: true, Briefed: "older-session"})
	r, sink = briefedRunner(t, "echo", lost)
	if blocks := promptBlocks(t, r, sink, "Carry on", 3); len(blocks) != 2 || blocks[0]["text"] != "You run inside Rashin." {
		t.Fatalf("a session that started over was handed %v", blocks)
	}
}
