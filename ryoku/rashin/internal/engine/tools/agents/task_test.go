package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// fakeSpawner runs each child as a script of steps and counts how many run
// at once.
type fakeSpawner struct {
	steps   int
	slots   int // 0: no Slots method in use
	fail    map[string]bool
	block   chan struct{}
	running atomic.Int32
	peak    atomic.Int32
	mu      sync.Mutex
	reqs    []tool.SpawnRequest
}

func (f *fakeSpawner) Spawn(ctx context.Context, req tool.SpawnRequest, progress func(protocol.AgentWork)) (tool.SpawnResult, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	now := f.running.Add(1)
	defer f.running.Add(-1)
	for {
		peak := f.peak.Load()
		if now <= peak || f.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	var steps []protocol.AgentStep
	for n := 1; n <= f.steps; n++ {
		title := fmt.Sprintf("Read file%d.go", n)
		steps = append(steps, protocol.AgentStep{Title: title})
		progress(protocol.AgentWork{Status: protocol.ToolRunning, Doing: title, Steps: steps, StepCount: n, Tokens: 100 * n, Model: "rashin-models/lfm3"})
		time.Sleep(time.Millisecond)
	}
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return tool.SpawnResult{}, ctx.Err()
		}
	}
	usage := llm.Usage{Input: 1000, Output: 200}
	if f.fail[req.ID] {
		return tool.SpawnResult{Output: "no such file", Failed: true, Usage: usage}, nil
	}
	return tool.SpawnResult{Output: "found it in " + req.ID, Usage: usage}, nil
}

type slottedSpawner struct{ *fakeSpawner }

func (s slottedSpawner) Slots(string) int { return s.slots }

// recorder keeps every agents report the task call emits.
type recorder struct {
	mu      sync.Mutex
	reports [][]protocol.AgentWork
}

func (r *recorder) emit(data protocol.ToolData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports = append(r.reports, data.Agents)
}

func batchArgs(n int) json.RawMessage {
	tasks := make([]map[string]string, n)
	for i := range tasks {
		tasks[i] = map[string]string{"name": fmt.Sprintf("Look%d", i+1), "task": fmt.Sprintf("Find thing %d", i+1)}
	}
	raw, _ := json.Marshal(map[string]any{"context": "# Goal\nmap it", "tasks": tasks})
	return raw
}

func envFor(spawner tool.Spawner, model string, class tool.Class, rec *recorder) tool.Env {
	return tool.Env{
		Session: protocol.SessionInfo{ID: "s1", Kind: protocol.KindAgent, Model: model, Approvals: protocol.ApprovalsAsk},
		Profile: tool.Profile{Mode: tool.ModeAgent, Class: class, Window: 32768},
		Spawner: spawner,
		Emit:    rec.emit,
	}
}

func TestLocalModelRunsChildrenOneAtATime(t *testing.T) {
	f := &fakeSpawner{steps: 2}
	rec := &recorder{}
	res := taskTool{}.Run(context.Background(), tool.Call{ID: "c1", Args: batchArgs(3)}, envFor(f, "rashin-models/lfm3", tool.ClassFull, rec))
	if res.IsError {
		t.Fatalf("task failed: %s", res.Text)
	}
	if peak := f.peak.Load(); peak != 1 {
		t.Fatalf("a local model ran %d children at once", peak)
	}
	if len(res.Agents) != 3 {
		t.Fatalf("got %d agents", len(res.Agents))
	}
}

func TestRemoteModelRunsUpToFourAtOnce(t *testing.T) {
	f := &fakeSpawner{block: make(chan struct{})}
	rec := &recorder{}
	done := make(chan tool.Result)
	go func() {
		done <- taskTool{}.Run(context.Background(), tool.Call{ID: "c1", Args: batchArgs(6)}, envFor(f, "openrouter/some-model", tool.ClassFull, rec))
	}()
	deadline := time.Now().Add(2 * time.Second)
	for f.running.Load() < MaxConcurrent && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	if running := f.running.Load(); running != MaxConcurrent {
		t.Fatalf("%d children running, want %d", running, MaxConcurrent)
	}
	close(f.block)
	res := <-done
	if peak := f.peak.Load(); peak != MaxConcurrent {
		t.Fatalf("peak %d, want %d", peak, MaxConcurrent)
	}
	if len(res.Agents) != 6 {
		t.Fatalf("got %d agents", len(res.Agents))
	}
}

func TestSpawnerSlotsSetConcurrencyWithinBounds(t *testing.T) {
	for _, c := range []struct{ slots, want int }{{2, 2}, {9, MaxConcurrent}, {0, 1}} {
		f := &fakeSpawner{slots: c.slots}
		env := envFor(slottedSpawner{f}, "rashin-models/lfm3", tool.ClassFull, &recorder{})
		if got := concurrency(env, env.Session.Model); got != c.want {
			t.Fatalf("slots %d: concurrency %d, want %d", c.slots, got, c.want)
		}
	}
	f := &fakeSpawner{}
	if got := concurrency(envFor(f, "openrouter/x", tool.ClassCompact, &recorder{}), "openrouter/x"); got != 1 {
		t.Fatalf("a compact profile ran %d at once", got)
	}
}

func TestBatchOverMaxTasksIsRefused(t *testing.T) {
	f := &fakeSpawner{}
	res := taskTool{}.Run(context.Background(), tool.Call{ID: "c1", Args: batchArgs(MaxTasks + 1)}, envFor(f, "openrouter/x", tool.ClassFull, &recorder{}))
	if !res.IsError || len(f.reqs) != 0 {
		t.Fatalf("a batch of %d ran: %q", MaxTasks+1, res.Text)
	}
}

func TestAgentWorkFollowsEachChildStep(t *testing.T) {
	f := &fakeSpawner{steps: 11, fail: map[string]bool{"Look2": true}}
	rec := &recorder{}
	res := taskTool{}.Run(context.Background(), tool.Call{ID: "c1", Args: batchArgs(2)}, envFor(f, "rashin-models/lfm3", tool.ClassFull, rec))

	if len(rec.reports) < 2*11 {
		t.Fatalf("only %d reports for 22 steps", len(rec.reports))
	}
	first := rec.reports[0]
	if first[0].Status != protocol.ToolPending || first[1].Status != protocol.ToolPending {
		t.Fatalf("first report %+v, want both pending", first)
	}
	sawDoing := false
	for _, report := range rec.reports {
		if report[0].Status == protocol.ToolRunning && report[0].Doing != "" {
			sawDoing = true
		}
		for _, w := range report {
			if len(w.Steps) > MaxSteps {
				t.Fatalf("a report kept %d steps", len(w.Steps))
			}
		}
	}
	if !sawDoing {
		t.Fatal("no report said what the running child was doing")
	}

	done, failed := res.Agents[0], res.Agents[1]
	if done.ID != "Look1" || done.Name != Worker || done.Task != "Find thing 1" {
		t.Fatalf("identity %+v", done)
	}
	if done.Status != protocol.ToolDone || done.Doing != "" {
		t.Fatalf("done child %+v", done)
	}
	if done.StepCount != 11 || len(done.Steps) != MaxSteps || done.Steps[MaxSteps-1].Title != "Read file11.go" {
		t.Fatalf("steps %d %+v", done.StepCount, done.Steps)
	}
	if done.Tokens != 1100 || done.Model != "rashin-models/lfm3" || done.Output != "found it in Look1" {
		t.Fatalf("facts %+v", done)
	}
	if done.Millis <= 0 {
		t.Fatal("no running time")
	}
	if failed.Status != protocol.ToolFailed || failed.Output != "no such file" {
		t.Fatalf("failed child %+v", failed)
	}
	if res.IsError {
		t.Fatal("one failed child failed the whole call")
	}
	if res.Usage.Input != 2000 || res.Usage.Output != 400 {
		t.Fatalf("usage %+v", res.Usage)
	}
	if !strings.Contains(res.Text, `<task-result id="Look1" agent="worker" status="done"`) || !strings.Contains(res.Text, "found it in Look1") {
		t.Fatalf("result text %q", res.Text)
	}
	for _, req := range f.reqs {
		if req.Parent != "c1" || req.Context != "# Goal\nmap it" || !req.Write {
			t.Fatalf("request %+v", req)
		}
	}
}

func TestCancelledBatchStopsChildren(t *testing.T) {
	f := &fakeSpawner{block: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan tool.Result)
	go func() {
		done <- taskTool{}.Run(ctx, tool.Call{ID: "c1", Args: batchArgs(3)}, envFor(f, "rashin-models/lfm3", tool.ClassFull, &recorder{}))
	}()
	for f.running.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	cancel()
	res := <-done
	for _, w := range res.Agents {
		if w.Status != protocol.ToolStopped {
			t.Fatalf("after cancel %s is %s", w.ID, w.Status)
		}
	}
	if len(f.reqs) != 1 {
		t.Fatalf("%d children started after cancel", len(f.reqs))
	}
}

func TestChildrenStayReadOnlyWhereTheParentCannotWrite(t *testing.T) {
	for name, env := range map[string]tool.Env{
		"plan":      {Profile: tool.Profile{Mode: tool.ModePlan, ReadOnly: true}},
		"read-only": {Session: protocol.SessionInfo{Approvals: protocol.ApprovalsReadOnly}},
	} {
		f := &fakeSpawner{}
		env.Spawner = f
		taskTool{}.Run(context.Background(), tool.Call{ID: "c", Args: batchArgs(1)}, env)
		if len(f.reqs) != 1 || f.reqs[0].Write {
			t.Fatalf("%s: child may write: %+v", name, f.reqs)
		}
	}
	f := &fakeSpawner{}
	raw := json.RawMessage(`{"tasks":[{"agent":"scout","task":"find it"}]}`)
	taskTool{}.Run(context.Background(), tool.Call{ID: "c", Args: raw}, tool.Env{Spawner: f, Session: protocol.SessionInfo{Approvals: protocol.ApprovalsAuto}})
	if f.reqs[0].Write {
		t.Fatal("a scout may write")
	}
}

func TestBatchShapesModelsWrite(t *testing.T) {
	for raw, want := range map[string][]string{
		`{"tasks":["read the parser","fix the parser"]}`:         {"ReadTheParser", "FixTheParser"},
		`{"tasks":"[{\"task\":\"one\"},{\"task\":\"one\"}]"}`:    {"One", "One2"},
		`{"name":"Solo","task":"do it"}`:                         {"Solo"},
		`{"tasks":[{"prompt":"check 2 files","agent":"Scout"}]}`: {"Check2Files"},
	} {
		b, err := parseBatch(json.RawMessage(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		var names []string
		for _, item := range b.tasks {
			names = append(names, item.name)
		}
		if strings.Join(names, ",") != strings.Join(want, ",") {
			t.Fatalf("%s: names %v, want %v", raw, names, want)
		}
	}
	for _, raw := range []string{`{}`, `{"tasks":[]}`, `{"tasks":[{"task":"x","agent":"reviewer"}]}`, `{"tasks":[{"name":"x"}]}`} {
		if _, err := parseBatch(json.RawMessage(raw)); err == nil {
			t.Fatalf("%s parsed", raw)
		}
	}
}

func TestYieldEndsTheRunWithDataOrError(t *testing.T) {
	cases := []struct {
		raw     string
		end     bool
		isError bool
		text    string
	}{
		{`{"data":"found it"}`, true, false, "found it"},
		{`{"result":"found it"}`, true, false, "found it"},
		{`{"data":{"files":["a.go"]}}`, true, false, "{\n  \"files\": [\n    \"a.go\"\n  ]\n}"},
		{`{"error":"no access"}`, true, true, "no access"},
		{`{"data":"x","error":"y"}`, false, true, ""},
		{`{}`, false, true, ""},
	}
	for _, c := range cases {
		res := yieldTool{}.Run(context.Background(), tool.Call{Args: json.RawMessage(c.raw)}, tool.Env{})
		if res.End != c.end || res.IsError != c.isError || (c.text != "" && res.Text != c.text) {
			t.Fatalf("%s: %+v", c.raw, res)
		}
	}
}
