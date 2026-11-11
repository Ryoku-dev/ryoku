package daemon

import (
	"context"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// A session says when its running turn began and how many tool calls it made;
// the start goes when the turn ends and the count stays until the next turn,
// and both reach every session.info.
func TestASessionSaysHowLongAndHowMuchItWorks(t *testing.T) {
	isolateDaemon(t)
	var mu sync.Mutex
	var broadcast []protocol.SessionInfo
	var during protocol.SessionInfo
	var manager *sessionManager
	var id string
	turns := 0
	runner := &fakeRunner{alive: true}
	runner.prompt = func(_ context.Context, _ core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		turns++
		if turns == 1 {
			sink.Emit(protocol.EvTool, protocol.ToolData{Call: "read", Title: "Read notes", Status: protocol.ToolRunning})
			sink.Emit(protocol.EvTool, protocol.ToolData{Call: "read", Status: protocol.ToolDone})
			sink.Emit(protocol.EvTool, protocol.ToolData{Call: "write", Title: "Write summary", Status: protocol.ToolDone})
			seen, _ := manager.Info(id)
			mu.Lock()
			during = seen
			mu.Unlock()
		}
		return core.TurnResult{}, nil
	}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	record := func(event string, data any) {
		if event == protocol.EvtSessionInfo {
			mu.Lock()
			broadcast = append(broadcast, data.(protocol.SessionInfo))
			mu.Unlock()
		}
	}
	manager = newSessionManager(nil, &fakeHarnesses{runner: runner}, nil, settings, record, func() {}, func(string, ...any) {})
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	id = info.ID
	before := time.Now()
	if _, err := manager.Prompt(context.Background(), protocol.PromptParams{ID: id, Text: "Summarise the notes"}); err != nil {
		t.Fatal(err)
	}
	after := waitIdle(t, manager, id)
	mu.Lock()
	working := during
	mu.Unlock()
	if working.TurnStarted == nil || working.TurnStarted.Before(before) || working.Steps != 2 || working.Activity == "" {
		t.Fatalf("while working the session read %+v", working)
	}
	if after.TurnStarted != nil || after.Steps != 2 {
		t.Fatalf("after the turn the session read started %v, steps %d", after.TurnStarted, after.Steps)
	}
	mu.Lock()
	sawSteps, sawStart := false, false
	for _, sent := range broadcast {
		sawSteps = sawSteps || sent.Steps == 2
		sawStart = sawStart || sent.TurnStarted != nil
	}
	mu.Unlock()
	if !sawSteps || !sawStart {
		t.Fatalf("session.info never carried the presence: steps %v, start %v", sawSteps, sawStart)
	}

	if _, err := manager.Prompt(context.Background(), protocol.PromptParams{ID: id, Text: "Thanks"}); err != nil {
		t.Fatal(err)
	}
	if again := waitIdle(t, manager, id); again.Steps != 0 || again.TurnStarted != nil {
		t.Fatalf("a turn without tools read %+v", again)
	}
}

// waitIdle waits for the session's turn to end and returns how it reads.
func waitIdle(t *testing.T, manager *sessionManager, id string) protocol.SessionInfo {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info, _ := manager.Info(id)
		if info.State != protocol.StateWorking && info.TurnStarted == nil {
			return info
		}
		time.Sleep(5 * time.Millisecond)
	}
	info, _ := manager.Info(id)
	t.Fatalf("the turn never ended: %+v", info)
	return info
}
