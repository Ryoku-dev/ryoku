package harness

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

type reentrantRunnerSink struct {
	runner *runner
	base   *testSink
}

func (s *reentrantRunnerSink) Emit(kind string, data any) {
	_ = s.runner.Alive()
	s.base.Emit(kind, data)
}

func (s *reentrantRunnerSink) Update(change func(*protocol.SessionInfo)) {
	_ = s.runner.State()
	s.base.Update(change)
}

func TestPermissionAndCancelEmitOutsideRunnerLocks(t *testing.T) {
	t.Run("permission", func(t *testing.T) {
		r, base := helperRunner(t, "generation", protocol.ApprovalsAsk, nil)
		sink := &reentrantRunnerSink{runner: r, base: base}
		promptDone := make(chan error, 1)
		go func() {
			_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "permission"}, sink)
			promptDone <- err
		}()
		permission := base.waitKind(t, protocol.EvPermission, 1)[0].data.(protocol.PermissionData)
		callDone := make(chan error, 1)
		go func() { callDone <- r.Permission(permission.Request, "allow") }()
		select {
		case err := <-callDone:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("permission emitted while holding the runner lock")
		}
		if err := <-promptDone; err != nil {
			t.Fatal(err)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		r, base := helperRunner(t, "cancel", protocol.ApprovalsAsk, nil)
		sink := &reentrantRunnerSink{runner: r, base: base}
		promptDone := make(chan error, 1)
		go func() {
			_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "cancel"}, sink)
			promptDone <- err
		}()
		base.waitKind(t, protocol.EvPermission, 2)
		callDone := make(chan struct{})
		go func() {
			r.Cancel()
			close(callDone)
		}()
		select {
		case <-callDone:
		case <-time.After(time.Second):
			t.Fatal("cancel emitted while holding the runner lock")
		}
		if err := <-promptDone; err != nil {
			t.Fatal(err)
		}
	})
}

func TestLoadDropsReplayedContentUpdates(t *testing.T) {
	resume := json.RawMessage(`{"acpSession":"past","loadSession":true}`)
	r, sink := helperRunner(t, "load-replay", protocol.ApprovalsAuto, resume)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "next"}, sink); err != nil {
		t.Fatal(err)
	}
	texts := sink.waitKind(t, protocol.EvText, 1)
	if len(texts) != 1 || texts[0].data.(protocol.TextData).Delta != "fresh" {
		t.Fatalf("session/load replay leaked into text events: %+v", texts)
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	counts := map[string]int{}
	for _, event := range sink.events {
		counts[event.kind]++
	}
	for _, kind := range []string{protocol.EvTool, protocol.EvPlan, protocol.EvCommands} {
		if counts[kind] != 0 {
			t.Fatalf("session/load replay leaked %s events: %d", kind, counts[kind])
		}
	}
	if counts[protocol.EvMode] == 0 || counts[protocol.EvModel] < 2 {
		t.Fatalf("session/load dropped live mode/model/config updates: %+v", counts)
	}
	foundConfig := false
	for _, event := range sink.events {
		if event.kind == protocol.EvModel && event.data.(protocol.ChoiceData).Current == "fast" {
			foundConfig = true
		}
	}
	if !foundConfig {
		t.Fatal("session/load dropped the config option update")
	}
}

func TestGeneratedIDsChangeAcrossAgentGenerations(t *testing.T) {
	r, sink := helperRunner(t, "generation", protocol.ApprovalsAsk, nil)
	prompt := func(turn int, permissionCount int) {
		t.Helper()
		done := make(chan error, 1)
		go func() {
			_, err := r.Prompt(context.Background(), core.PromptInput{Turn: turn, Text: "go"}, sink)
			done <- err
		}()
		permission := sink.waitKind(t, protocol.EvPermission, permissionCount)[permissionCount-1].data.(protocol.PermissionData)
		if err := r.Permission(permission.Request, "allow"); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	prompt(1, 1)
	firstText := sink.waitKind(t, protocol.EvText, 1)[0].data.(protocol.TextData).Message
	firstPermission := sink.waitKind(t, protocol.EvPermission, 1)[0].data.(protocol.PermissionData).Request
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	prompt(2, 2)
	texts := sink.waitKind(t, protocol.EvText, 2)
	permissions := sink.waitKind(t, protocol.EvPermission, 2)
	secondText := texts[1].data.(protocol.TextData).Message
	secondPermission := permissions[1].data.(protocol.PermissionData).Request
	if firstText == secondText || firstPermission == secondPermission {
		t.Fatalf("ids were reused across generations: text %q/%q permission %q/%q", firstText, secondText, firstPermission, secondPermission)
	}
	for _, id := range []string{firstText, secondText, firstPermission, secondPermission} {
		if !strings.Contains(id, "-") {
			t.Fatalf("id has no generation prefix: %q", id)
		}
	}
}
func TestTerminalAndFileIDsChangeAcrossAgentGenerations(t *testing.T) {
	r, sink := helperRunner(t, "synthetic-ids", protocol.ApprovalsAuto, nil)
	path := r.opts.Cwd + "/file.txt"
	t.Setenv("RASHIN_ACP_TEST_FILE", path)
	run := func(turn int) (string, string) {
		t.Helper()
		sink.mu.Lock()
		start := len(sink.events)
		sink.mu.Unlock()
		if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: turn, Text: "ids"}, sink); err != nil {
			t.Fatal(err)
		}
		sink.mu.Lock()
		defer sink.mu.Unlock()
		var terminalID, fileID string
		for _, event := range sink.events[start:] {
			switch event.kind {
			case protocol.EvTerminal:
				data := event.data.(protocol.TerminalData)
				if data.Command != "" {
					terminalID = data.Terminal
				}
			case protocol.EvTool:
				data := event.data.(protocol.ToolData)
				if strings.Contains(data.Call, "fs-write") {
					fileID = data.Call
				}
			}
		}
		if terminalID == "" || fileID == "" {
			t.Fatalf("missing generated ids: terminal=%q file=%q", terminalID, fileID)
		}
		return terminalID, fileID
	}
	firstTerminal, firstFile := run(1)
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	secondTerminal, secondFile := run(2)
	if firstTerminal == secondTerminal || firstFile == secondFile {
		t.Fatalf("generated ids were reused: terminal %q/%q file %q/%q", firstTerminal, secondTerminal, firstFile, secondFile)
	}
}

func TestCancelWaitsForCancelledPromptResponse(t *testing.T) {
	r, sink := helperRunner(t, "cancel-late", protocol.ApprovalsAuto, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct {
		result core.TurnResult
		err    error
	}, 1)
	go func() {
		result, err := r.Prompt(ctx, core.PromptInput{Turn: 1, Text: "first"}, sink)
		done <- struct {
			result core.TurnResult
			err    error
		}{result: result, err: err}
	}()
	sink.waitKind(t, protocol.EvPlan, 1)
	started := time.Now()
	cancel()
	turn := <-done
	if turn.err != nil || turn.result.Stop != protocol.StopCancelled {
		t.Fatalf("cancelled prompt returned %+v, %v", turn.result, turn.err)
	}
	if time.Since(started) < 90*time.Millisecond {
		t.Fatal("cancel returned before the agent answered the prompt")
	}
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "second"}, sink); err != nil {
		t.Fatal(err)
	}
	texts := sink.waitKind(t, protocol.EvText, 2)
	if texts[0].data.(protocol.TextData).Delta != "late-before-cancelled" || texts[1].data.(protocol.TextData).Delta != "second" {
		t.Fatalf("cancelled output crossed into the next turn: %+v", texts)
	}
}

func TestCancelKillsAgentAfterGrace(t *testing.T) {
	r, sink := helperRunner(t, "cancel-ignore", protocol.ApprovalsAuto, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(ctx, core.PromptInput{Turn: 1, Text: "first"}, sink)
		done <- err
	}()
	sink.waitKind(t, protocol.EvPlan, 1)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("agent that ignored cancel returned without a cancellation error")
	}
	if r.Alive() {
		t.Fatal("agent stayed alive after the cancel grace timeout")
	}
}

func TestClosedConnectionRestartsAfterChildHoldsStderr(t *testing.T) {
	r, sink := helperRunner(t, "crash-child", protocol.ApprovalsAuto, nil)
	marker := r.opts.Cwd + "/crashed"
	t.Setenv("RASHIN_ACP_TEST_MARKER", marker)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "crash"}, sink); err == nil {
		t.Fatal("crashing agent prompt succeeded")
	}
	if r.Alive() {
		t.Fatal("closed agent connection remained alive")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "restart"}, sink); err != nil {
		t.Fatal(err)
	}
	texts := sink.waitKind(t, protocol.EvText, 1)
	if texts[len(texts)-1].data.(protocol.TextData).Delta != "restarted" {
		t.Fatalf("agent did not restart after connection close: %+v", texts)
	}
}

func TestTerminalSurvivesBetweenPrompts(t *testing.T) {
	r, sink := helperRunner(t, "terminal-persist", protocol.ApprovalsAuto, nil)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "start"}, sink); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	terminals := len(r.terminals)
	r.mu.Unlock()
	if terminals != 1 {
		t.Fatalf("terminal was discarded at prompt completion: %d", terminals)
	}
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "check"}, sink); err != nil {
		t.Fatal(err)
	}
	texts := sink.waitKind(t, protocol.EvText, 1)
	if texts[len(texts)-1].data.(protocol.TextData).Delta != "kept" {
		t.Fatalf("terminal did not survive into the next prompt: %+v", texts)
	}
}

func TestResumeFallbackEmitsNotice(t *testing.T) {
	resume := json.RawMessage(`{"acpSession":"past","loadSession":true}`)
	r, sink := helperRunner(t, "resume-new", protocol.ApprovalsAuto, resume)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "resume"}, sink); err != nil {
		t.Fatal(err)
	}
	notices := sink.waitKind(t, protocol.EvtNotice, 1)
	if notices[0].data.(protocol.Notice).Level != protocol.NoticeWarn {
		t.Fatalf("unexpected fallback notice: %+v", notices[0].data)
	}
}

func TestCancelDoesNotStopTerminals(t *testing.T) {
	r, sink := helperRunner(t, "terminal-cancel", protocol.ApprovalsAuto, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(ctx, core.PromptInput{Turn: 1, Text: "start"}, sink)
		done <- err
	}()
	sink.waitKind(t, protocol.EvPlan, 1)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	terminals := len(r.terminals)
	r.mu.Unlock()
	if terminals != 1 {
		t.Fatalf("cancel discarded the running terminal: %d", terminals)
	}
}
