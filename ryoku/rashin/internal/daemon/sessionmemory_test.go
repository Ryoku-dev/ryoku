package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	store "ryoku-rashin/internal/session"
)

// hookMemory records the session manager's memory hooks.
type hookMemory struct {
	ended   chan protocol.SessionInfo
	deleted chan string
}

func newHookMemory() *hookMemory {
	return &hookMemory{ended: make(chan protocol.SessionInfo, 8), deleted: make(chan string, 8)}
}

func (h *hookMemory) Bind(core.MemorySessions) {}
func (h *hookMemory) Handle(context.Context, core.MemoryCaller, string, json.RawMessage) (any, error) {
	return nil, nil
}
func (h *hookMemory) Status(context.Context) protocol.MemoryStatus { return protocol.MemoryStatus{} }
func (h *hookMemory) Recall(context.Context, protocol.SessionInfo, string) core.Recall {
	return core.Recall{}
}
func (h *hookMemory) Brief(protocol.SessionInfo) string   { return "" }
func (h *hookMemory) TeamBrief(protocol.Team) string      { return "" }
func (h *hookMemory) TurnEnded(info protocol.SessionInfo) { h.ended <- info }
func (h *hookMemory) SessionDeleted(id string)            { h.deleted <- id }
func (h *hookMemory) Settle()                             {}
func (h *hookMemory) Busy() bool                          { return false }
func (h *hookMemory) Release()                            {}
func (h *hookMemory) Close() error                        { return nil }

// memoryManager is a session manager in an isolated state dir with memory
// hooks recorded, and no session made yet.
func memoryManager(t *testing.T, runner core.Runner) (*sessionManager, *hookMemory) {
	t.Helper()
	isolateDaemon(t)
	settings := func() protocol.Settings {
		return protocol.Settings{Ask: protocol.AskSettings{Model: protocol.RouteAuto}, Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(fakeAsker{runner: runner}, nil, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	hooks := newHookMemory()
	manager.memory = hooks
	return manager, hooks
}

func answering(text string) *fakeRunner {
	return &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: text})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
}

// finishTurn sends text and waits until the turn has ended for an attached
// collector.
func finishTurn(t *testing.T, manager *sessionManager, id, text string) {
	t.Helper()
	collector := newCollector()
	if _, err := manager.attach(id, 1<<62, collector); err != nil {
		t.Fatal(err)
	}
	defer manager.detach(id, collector)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: id, Text: text}, nil); err != nil {
		t.Fatal(err)
	}
	waitTurnEnd(t, collector)
}

func waitEnded(t *testing.T, hooks *hookMemory) protocol.SessionInfo {
	t.Helper()
	select {
	case info := <-hooks.ended:
		return info
	case <-time.After(time.Second):
		t.Fatal("memory did not hear the turn end")
	}
	return protocol.SessionInfo{}
}

func codeOf(err error) string {
	var protocolErr *protocol.Error
	if errors.As(err, &protocolErr) {
		return protocolErr.Code
	}
	return ""
}

// A temporary conversation with a full turn, a memory line and a narrowing
// leaves nothing under the state dir, is listed only while the manager holds
// it, never reaches memory's turn hook, and is gone once deleted.
func TestTemporarySessionNeverTouchesDisk(t *testing.T) {
	manager, hooks := memoryManager(t, answering("an answer"))
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginApp, Temporary: true})
	if err != nil {
		t.Fatal(err)
	}
	if !info.Temporary || info.Memory != protocol.MemoryUseOnly {
		t.Fatalf("info = %#v", info)
	}
	finishTurn(t, manager, info.ID, "a question")
	manager.EmitMemory(info.ID, protocol.MemoryEventData{Kind: protocol.MemoryRefused, Reason: protocol.MemoryRefusedOff})
	if _, err := manager.setMemory(protocol.SetMemoryParams{ID: info.ID, Memory: protocol.MemoryNone}); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(paths.StateDir(), func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if strings.Contains(path, info.ID) || strings.HasPrefix(path, paths.SessionsDir()+string(filepath.Separator)) {
			t.Errorf("a temporary session wrote %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	held, err := manager.get(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if events, _ := held.log.Since(0); len(events) != 6 {
		t.Fatalf("held events = %#v", events)
	}
	if events, err := manager.Events(info.ID); err != nil || len(events) != 0 {
		t.Fatalf("stored events = %#v, %v", events, err)
	}
	select {
	case ended := <-hooks.ended:
		t.Fatalf("memory heard a temporary turn: %#v", ended)
	default:
	}
	if listed := manager.List(protocol.KindAsk); len(listed) != 0 {
		t.Fatalf("List = %#v", listed)
	}
	if stored, _ := store.List(""); len(stored) != 0 {
		t.Fatalf("stored = %#v", stored)
	}
	if listed, _ := manager.list(protocol.SessionListParams{Kind: protocol.KindAsk}); len(listed.Sessions) != 1 || listed.Sessions[0].ID != info.ID {
		t.Fatalf("session.list = %#v", listed.Sessions)
	}
	if err := manager.delete(info.ID); err != nil {
		t.Fatal(err)
	}
	if id := <-hooks.deleted; id != info.ID {
		t.Fatalf("deleted %s", id)
	}
	if listed, _ := manager.list(protocol.SessionListParams{Kind: protocol.KindAsk}); len(listed.Sessions) != 0 {
		t.Fatalf("session.list after delete = %#v", listed.Sessions)
	}
	if _, ok := manager.Info(info.ID); ok {
		t.Fatal("a deleted temporary session still has info")
	}
	if _, err := manager.get(info.ID); codeOf(err) != protocol.CodeNotFound {
		t.Fatalf("get after delete = %v", err)
	}
	if _, err := os.Stat(paths.SessionDir(info.ID)); !os.IsNotExist(err) {
		t.Fatalf("session dir = %v", err)
	}
}

// A team task's conversation is never temporary.
func TestTeamTaskCannotBeTemporary(t *testing.T) {
	manager, _ := memoryManager(t, answering(""))
	_, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: os.Getenv("HOME"), Team: "team", TeamTask: "task", Temporary: true})
	if codeOf(err) != protocol.CodeBadRequest {
		t.Fatalf("err = %v", err)
	}
}

func memoryEvents(t *testing.T, manager *sessionManager, id string) []protocol.MemoryEventData {
	t.Helper()
	events, err := manager.Events(id)
	if err != nil {
		t.Fatal(err)
	}
	var out []protocol.MemoryEventData
	for _, event := range events {
		if event.Kind == protocol.EvMemory {
			var data protocol.MemoryEventData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatal(err)
			}
			out = append(out, data)
		}
	}
	return out
}

// Before the first message any memory mode may be picked and nothing is
// written; after it only a narrower one, each narrowing written where it
// was made.
func TestSetMemoryNarrowsAfterTheFirstMessage(t *testing.T) {
	manager, _ := memoryManager(t, answering("an answer"))
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginApp, Memory: protocol.MemoryNone})
	if err != nil {
		t.Fatal(err)
	}
	if info.Memory != protocol.MemoryNone {
		t.Fatalf("created with %q", info.Memory)
	}
	for _, mode := range []string{protocol.MemoryUseOnly, protocol.MemoryRemember} {
		got, err := manager.setMemory(protocol.SetMemoryParams{ID: info.ID, Memory: mode})
		if err != nil || got.Memory != mode {
			t.Fatalf("set %q before the first message = %#v, %v", mode, got.Memory, err)
		}
	}
	if _, err := manager.setMemory(protocol.SetMemoryParams{ID: info.ID, Memory: "always"}); codeOf(err) != protocol.CodeBadRequest {
		t.Fatalf("unknown mode = %v", err)
	}
	if lines := memoryEvents(t, manager, info.ID); len(lines) != 0 {
		t.Fatalf("lines before the first message = %#v", lines)
	}
	finishTurn(t, manager, info.ID, "a question")
	if got, err := manager.setMemory(protocol.SetMemoryParams{ID: info.ID, Memory: protocol.MemoryUseOnly}); err != nil || got.Memory != protocol.MemoryUseOnly {
		t.Fatalf("narrow to use = %#v, %v", got.Memory, err)
	}
	if _, err := manager.setMemory(protocol.SetMemoryParams{ID: info.ID, Memory: protocol.MemoryRemember}); codeOf(err) != protocol.CodeConflict {
		t.Fatalf("widen after the first message = %v", err)
	}
	if got, err := manager.setMemory(protocol.SetMemoryParams{ID: info.ID, Memory: protocol.MemoryNone}); err != nil || got.Memory != protocol.MemoryNone {
		t.Fatalf("narrow to off = %#v, %v", got.Memory, err)
	}
	if _, err := manager.setMemory(protocol.SetMemoryParams{ID: info.ID, Memory: protocol.MemoryUseOnly}); codeOf(err) != protocol.CodeConflict {
		t.Fatalf("widen to use = %v", err)
	}
	lines := memoryEvents(t, manager, info.ID)
	if len(lines) != 2 || lines[0].Kind != protocol.MemoryNarrowed || lines[0].Memory != protocol.MemoryUseOnly || lines[1].Memory != protocol.MemoryNone {
		t.Fatalf("narrowed lines = %#v", lines)
	}
	if stored, ok := manager.Info(info.ID); !ok || stored.Memory != protocol.MemoryNone {
		t.Fatalf("stored memory = %#v", stored.Memory)
	}

	temporary, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginApp, Temporary: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.setMemory(protocol.SetMemoryParams{ID: temporary.ID, Memory: protocol.MemoryRemember}); codeOf(err) != protocol.CodeBadRequest {
		t.Fatalf("remember in a temporary conversation = %v", err)
	}
	if got, err := manager.setMemory(protocol.SetMemoryParams{ID: temporary.ID, Memory: protocol.MemoryNone}); err != nil || got.Memory != protocol.MemoryNone {
		t.Fatalf("temporary to off = %#v, %v", got.Memory, err)
	}
}

// A memory line reaches an attached client outside a turn and in the middle
// of one, after the text streamed before it and without ending the turn.
func TestEmitMemoryStreamsDuringAndOutsideATurn(t *testing.T) {
	streamed, release := make(chan struct{}), make(chan struct{})
	runner := &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "one "})
		close(streamed)
		<-release
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "two"})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, _ := memoryManager(t, runner)
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginApp})
	if err != nil {
		t.Fatal(err)
	}
	collector := newCollector()
	if _, err := manager.attach(info.ID, 0, collector); err != nil {
		t.Fatal(err)
	}
	manager.EmitMemory(info.ID, protocol.MemoryEventData{Kind: protocol.MemoryUpdated})
	if events := collector.snapshot(); len(events) != 1 || events[0].Kind != protocol.EvMemory || events[0].Seq != 1 {
		t.Fatalf("outside a turn = %#v", events)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "q"}, nil); err != nil {
		t.Fatal(err)
	}
	<-streamed
	manager.EmitMemory(info.ID, protocol.MemoryEventData{Kind: protocol.MemoryUsed, Card: true})
	during := collector.snapshot()
	last := during[len(during)-1]
	if last.Kind != protocol.EvMemory || during[len(during)-2].Kind != protocol.EvText {
		t.Fatalf("during a turn = %#v", during)
	}
	if current, _ := manager.Info(info.ID); current.State != protocol.StateWorking {
		t.Fatalf("the memory line moved the turn to %s", current.State)
	}
	close(release)
	events := waitTurnEnd(t, collector)
	want := []string{protocol.EvMemory, protocol.EvUser, protocol.EvTurnStart, protocol.EvText, protocol.EvMemory, protocol.EvText, protocol.EvTurnEnd}
	if len(events) != len(want) {
		t.Fatalf("events = %#v", events)
	}
	for index, kind := range want {
		if events[index].Kind != kind || events[index].Seq != int64(index+1) {
			t.Fatalf("event %d = %s seq %d, want %s", index, events[index].Kind, events[index].Seq, kind)
		}
	}
	stored, err := manager.Events(info.ID)
	if err != nil || len(stored) != len(want) {
		t.Fatalf("stored = %#v, %v", stored, err)
	}
}

// Memory hears each turn end once, with the session's info as the turn
// left it.
func TestTurnEndedOncePerTurnWithFinalInfo(t *testing.T) {
	manager, hooks := memoryManager(t, answering("an answer"))
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginApp})
	if err != nil {
		t.Fatal(err)
	}
	for turn := 1; turn <= 2; turn++ {
		finishTurn(t, manager, info.ID, "a question")
		ended := waitEnded(t, hooks)
		current, _ := manager.Info(info.ID)
		if ended.ID != info.ID || ended.Turns != turn || ended.State != protocol.StateIdle || ended.LastSeq != current.LastSeq || !ended.Unread {
			t.Fatalf("turn %d ended with %#v, now %#v", turn, ended, current)
		}
	}
	select {
	case extra := <-hooks.ended:
		t.Fatalf("an extra turn end: %#v", extra)
	case <-time.After(50 * time.Millisecond):
	}
	if err := manager.delete(info.ID); err != nil {
		t.Fatal(err)
	}
	if id := <-hooks.deleted; id != info.ID {
		t.Fatalf("deleted %s", id)
	}
}

// A fork carries the conversation's memory mode, and a fork of a temporary
// conversation is a stored one: that is how one is kept.
func TestForkCopiesMemoryButNotTemporary(t *testing.T) {
	manager, _ := memoryManager(t, answering("an answer"))
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginApp, Temporary: true, Memory: protocol.MemoryNone})
	if err != nil {
		t.Fatal(err)
	}
	finishTurn(t, manager, info.ID, "a question")
	current, _ := manager.Info(info.ID)
	forked, err := manager.fork(context.Background(), protocol.ForkParams{ID: info.ID, Seq: current.LastSeq})
	if err != nil {
		t.Fatal(err)
	}
	if forked.Temporary || forked.Memory != protocol.MemoryNone || forked.Turns != 1 {
		t.Fatalf("fork = %#v", forked)
	}
	stored, err := store.List(protocol.KindAsk)
	if err != nil || len(stored) != 1 || stored[0].ID != forked.ID || stored[0].Memory != protocol.MemoryNone {
		t.Fatalf("stored = %#v, %v", stored, err)
	}
	if events, err := manager.Events(forked.ID); err != nil || len(events) != int(current.LastSeq) {
		t.Fatalf("forked events = %d, %v", len(events), err)
	}
}
