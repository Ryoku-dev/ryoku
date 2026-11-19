package autolearn

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

type fakeStream struct {
	deltas []llm.Delta
	next   int
}

func (s *fakeStream) Next() (llm.Delta, error) {
	if s.next >= len(s.deltas) {
		return llm.Delta{}, io.EOF
	}
	s.next++
	return s.deltas[s.next-1], nil
}
func (s *fakeStream) Final() llm.Final { return llm.Final{Finish: llm.FinishStop} }
func (s *fakeStream) Close() error     { return nil }

// fakeClient answers every request with the same delta and keeps the
// requests.
type fakeClient struct {
	mu       sync.Mutex
	answer   llm.Delta
	requests []llm.Request
}

func (c *fakeClient) Stream(_ context.Context, req llm.Request) (llm.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	return &fakeStream{deltas: []llm.Delta{c.answer}}, nil
}

func (c *fakeClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.requests)
}

type fakeEvents struct {
	mu     sync.Mutex
	events []string
}

func (f *fakeEvents) Broadcast(event string, data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}

func autolearnOn() protocol.Settings {
	return protocol.Settings{Features: map[string]bool{protocol.FeatureAutolearn: true}}
}

func saveCall(name string) llm.Delta {
	args, _ := json.Marshal(map[string]string{
		"name": name, "description": "Use when the build fails on missing vendored modules.",
		"body": "1. Run go mod vendor.\n2. Build again with -mod=vendor.",
	})
	return llm.Delta{ToolCalls: []llm.ToolCall{{ID: "1", Name: saveTool, Arguments: string(args)}}}
}

// turn is a finished agent turn with own calls of its own and one
// subagent call that never counts.
func turn(own int) engine.TurnRecord {
	rec := engine.TurnRecord{
		TurnContext: engine.TurnContext{
			Session: protocol.SessionInfo{ID: "s1"},
			Profile: tool.Profile{Mode: tool.ModeAgent, Class: tool.ClassCompact},
			Prompt:  "the build is broken, fix it",
		},
		Stop: protocol.StopEnd,
		Text: "Fixed: vendored modules were missing.",
	}
	for range own {
		rec.Calls = append(rec.Calls, engine.CallRecord{Name: tool.Bash, Args: json.RawMessage(`{"command":"go build ./..."}`), Result: "ok"})
	}
	rec.Calls = append(rec.Calls, engine.CallRecord{Name: tool.Read, Parent: "task-1"})
	return rec
}

func observer(t *testing.T, answer llm.Delta, settings protocol.Settings) (*Observer, *fakeClient, *fakeEvents, *skills.Managed) {
	t.Helper()
	dir := t.TempDir()
	managed := skills.New(filepath.Join(dir, "builtin")).WithManaged(filepath.Join(dir, "managed"))
	client := &fakeClient{answer: answer}
	events := &fakeEvents{}
	return NewObserver(client, managed, func() protocol.Settings { return settings }, events), client, events, managed
}

func TestCaptureNeedsFiveCallsOfTheTurnsOwn(t *testing.T) {
	short, shortClient, _, _ := observer(t, saveCall("vendor-rebuild"), autolearnOn())
	short.TurnEnded(t.Context(), turn(MinToolCalls-1))
	short.Close()
	if shortClient.count() != 0 {
		t.Fatalf("a turn of %d calls was captured", MinToolCalls-1)
	}

	o, client, events, managed := observer(t, saveCall("vendor-rebuild"), autolearnOn())
	o.TurnEnded(t.Context(), turn(MinToolCalls))
	o.Close()
	if client.count() != 1 {
		t.Fatalf("captures = %d, want 1", client.count())
	}
	req := client.requests[0]
	if !req.Light || !req.LocalOnly || len(req.Tools) != 1 || req.Tools[0].Name != saveTool {
		t.Errorf("capture request = %+v", req)
	}
	if got := managed.Skills(); len(got) != 1 || got[0].Name != "vendor-rebuild" {
		t.Errorf("managed skills = %+v", got)
	}
	if len(events.events) != 1 || events.events[0] != protocol.EvtLearningChanged {
		t.Errorf("events = %v", events.events)
	}
}

func TestCaptureSkipsTurnsThatAreNotTheUsers(t *testing.T) {
	cases := map[string]func(*engine.TurnRecord){
		"plan review":       func(r *engine.TurnRecord) { r.Profile.Mode = tool.ModePlan },
		"subagent":          func(r *engine.TurnRecord) { r.Profile.Mode = tool.ModeSubagent },
		"cancelled":         func(r *engine.TurnRecord) { r.Stop = protocol.StopCancelled },
		"goal continuation": func(r *engine.TurnRecord) { r.Goal = true },
		"temporary":         func(r *engine.TurnRecord) { r.Session.Temporary = true },
		"already learned":   func(r *engine.TurnRecord) { r.Calls[0].Name = tool.ManageSkill },
		"team task":         func(r *engine.TurnRecord) { r.Profile.Mode = tool.ModeTeam },
		"dream manager":     func(r *engine.TurnRecord) { r.Profile.Mode = tool.ModeDreamManager },
	}
	for name, change := range cases {
		o, client, _, _ := observer(t, saveCall("vendor-rebuild"), autolearnOn())
		rec := turn(MinToolCalls + 1)
		change(&rec)
		o.TurnEnded(t.Context(), rec)
		o.Close()
		if client.count() != 0 {
			t.Errorf("%s: captured", name)
		}
	}
	o, client, _, _ := observer(t, saveCall("vendor-rebuild"), protocol.Settings{})
	o.TurnEnded(t.Context(), turn(MinToolCalls+1))
	o.Close()
	if client.count() != 0 {
		t.Error("captured with autolearn off")
	}
}

func TestCaptureWritesNothingWhenNothingWasLearned(t *testing.T) {
	for name, answer := range map[string]llm.Delta{
		"none":          {Text: "none"},
		"built-in name": saveCall("brainstorming"),
		"no body":       {ToolCalls: []llm.ToolCall{{Name: saveTool, Arguments: `{"name":"x","description":"Use when.","body":""}`}}},
	} {
		o, client, events, managed := observer(t, answer, autolearnOn())
		o.TurnEnded(t.Context(), turn(MinToolCalls))
		o.Close()
		if client.count() != 1 {
			t.Fatalf("%s: captures = %d", name, client.count())
		}
		if got := managed.Skills(); len(got) != 0 || len(events.events) != 0 {
			t.Errorf("%s: wrote %+v, events %v", name, got, events.events)
		}
	}
}

func TestCaptureReadsACallWrittenAsText(t *testing.T) {
	text := `save_skill {"name": "Vendor-Rebuild", "description": "Use when vendored modules are missing.", "body": "1. Run go mod vendor.\n2. Build again."}`
	o, _, _, managed := observer(t, llm.Delta{Text: text}, autolearnOn())
	o.TurnEnded(t.Context(), turn(MinToolCalls))
	o.Close()
	if got := managed.Skills(); len(got) != 1 || got[0].Name != "vendor-rebuild" {
		t.Errorf("managed skills = %+v", got)
	}
}

func TestCaptureImprovesTheSkillOfTheSameName(t *testing.T) {
	o, client, _, managed := observer(t, saveCall("vendor-rebuild"), autolearnOn())
	if _, err := managed.Create("vendor-rebuild", "Use when modules are missing.", "1. Old step that is long enough to keep."); err != nil {
		t.Fatal(err)
	}
	o.TurnEnded(t.Context(), turn(MinToolCalls))
	o.Close()
	if !strings.Contains(client.requests[0].Messages[1].Content, "- vendor-rebuild: Use when modules are missing.") {
		t.Errorf("the capture was not shown the existing skill:\n%s", client.requests[0].Messages[1].Content)
	}
	if got := managed.Skills(); len(got) != 1 || got[0].Description != "Use when the build fails on missing vendored modules." {
		t.Errorf("managed skills = %+v", got)
	}
}

func TestCaptureIgnoresOtherCallsAndRedactsTurn(t *testing.T) {
	rec := turn(MinToolCalls)
	rec.Prompt = "Fix the build with api_key=secret-for-tests"
	rec.Calls[0].Result = "Authorization: Bearer secret-for-tests"
	o, client, _, managed := observer(t, llm.Delta{
		ToolCalls: []llm.ToolCall{{Name: "not_save_skill", Arguments: `{"name":"bad","description":"Use when.","body":"1. Do this forever."}`}},
	}, autolearnOn())
	o.TurnEnded(t.Context(), rec)
	o.Close()
	if got := managed.Skills(); len(got) != 0 {
		t.Errorf("captured an unrelated tool call: %+v", got)
	}
	if content := client.requests[0].Messages[1].Content; strings.Contains(content, "secret-for-tests") {
		t.Errorf("capture leaked a credential: %s", content)
	}
}
