package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type fakeAsker struct{ runner core.Runner }

func (f fakeAsker) NewAsk(core.AskOptions) core.Runner { return f.runner }

type fakeHarnesses struct {
	runner core.Runner
	opts   core.HarnessOptions
}

func (f *fakeHarnesses) List(context.Context, bool) ([]protocol.HarnessInfo, error) { return nil, nil }
func (f *fakeHarnesses) NewAgent(_ context.Context, opts core.HarnessOptions) (core.Runner, error) {
	f.opts = opts
	return f.runner, nil
}

type fakeWork struct {
	sessions     core.SessionControl
	prepared     protocol.SessionNewParams
	handleResult any
	handleErr    error
	prepareCalls int
	beforeCalls  int
	checkpoint   bool
	after        chan int
	watches      []string
}

func (f *fakeWork) Bind(s core.SessionControl) { f.sessions = s }
func (f *fakeWork) Handle(context.Context, string, json.RawMessage) (any, error) {
	if f.handleResult != nil {
		return f.handleResult, f.handleErr
	}
	return protocol.Empty{}, f.handleErr
}
func (f *fakeWork) Watch(project string, on bool) {
	if on {
		f.watches = append(f.watches, "+"+project)
	} else {
		f.watches = append(f.watches, "-"+project)
	}
}
func (f *fakeWork) Projects(context.Context) []protocol.ProjectInfo { return nil }
func (f *fakeWork) Tasks(context.Context) []protocol.TaskInfo       { return nil }
func (f *fakeWork) PrepareSession(_ context.Context, p protocol.SessionNewParams) (protocol.SessionNewParams, error) {
	f.prepareCalls++
	if f.prepared.Cwd != "" {
		p.Cwd = f.prepared.Cwd
	}
	if f.prepared.Worktree != "" {
		p.Worktree = f.prepared.Worktree
	}
	if f.prepared.Branch != "" {
		p.Branch = f.prepared.Branch
	}
	return p, nil
}
func (f *fakeWork) BeforeTurn(context.Context, protocol.SessionInfo, int) (bool, error) {
	f.beforeCalls++
	return f.checkpoint, nil
}
func (f *fakeWork) AfterTurn(_ context.Context, _ protocol.SessionInfo, turn int) {
	if f.after != nil {
		f.after <- turn
	}
}
func (f *fakeWork) SessionChanged(protocol.SessionInfo) {}
func (f *fakeWork) RevertTurn(context.Context, protocol.SessionInfo, int) (protocol.GitOpResult, error) {
	return protocol.GitOpResult{}, nil
}
func (f *fakeWork) Busy() bool   { return false }
func (f *fakeWork) Close() error { return nil }

type fakeIndexer struct {
	added              []protocol.IndexAddParams
	conflictAfterFirst bool
}

func (f *fakeIndexer) List(context.Context) ([]protocol.IndexInfo, error) { return nil, nil }
func (f *fakeIndexer) Add(_ context.Context, p protocol.IndexAddParams) (protocol.IndexInfo, error) {
	f.added = append(f.added, p)
	if f.conflictAfterFirst && len(f.added) > 1 {
		return protocol.IndexInfo{}, protocol.Errorf(protocol.CodeConflict, "Already registered.")
	}
	return protocol.IndexInfo{Root: p.Path, Name: p.Name}, nil
}
func (f *fakeIndexer) Remove(context.Context, protocol.IndexRef) error { return nil }
func (f *fakeIndexer) Refresh(context.Context, protocol.IndexRef) (protocol.IndexInfo, error) {
	return protocol.IndexInfo{}, nil
}
func (f *fakeIndexer) Status(context.Context, protocol.IndexRef) (protocol.IndexInfo, error) {
	return protocol.IndexInfo{}, nil
}
func (f *fakeIndexer) Search(context.Context, protocol.SearchParams) (protocol.SearchResult, error) {
	return protocol.SearchResult{}, nil
}
func (f *fakeIndexer) Find(context.Context, protocol.NameParams) (protocol.SymbolsResult, error) {
	return protocol.SymbolsResult{}, nil
}
func (f *fakeIndexer) Def(context.Context, protocol.NameParams) (protocol.DefResult, error) {
	return protocol.DefResult{}, nil
}
func (f *fakeIndexer) Outline(context.Context, protocol.PathParams) (protocol.SymbolsResult, error) {
	return protocol.SymbolsResult{}, nil
}
func (f *fakeIndexer) References(context.Context, protocol.NameParams) (protocol.ReferencesResult, error) {
	return protocol.ReferencesResult{}, nil
}
func (f *fakeIndexer) Peek(context.Context, protocol.PeekParams) (protocol.PeekResult, error) {
	return protocol.PeekResult{}, nil
}
func (f *fakeIndexer) Overview(context.Context, protocol.IndexRef) (protocol.Overview, error) {
	return protocol.Overview{}, nil
}
func (f *fakeIndexer) Impact(context.Context, protocol.PathParams) (protocol.Impact, error) {
	return protocol.Impact{}, nil
}
func (f *fakeIndexer) Wip(context.Context, protocol.WipParams) (protocol.WipResult, error) {
	return protocol.WipResult{}, nil
}
func (f *fakeIndexer) Changed(context.Context, protocol.WipParams) (protocol.WipResult, error) {
	return protocol.WipResult{}, nil
}
func (f *fakeIndexer) Busy() bool   { return false }
func (f *fakeIndexer) Release()     {}
func (f *fakeIndexer) Close() error { return nil }

type fakeRunner struct {
	prompt      func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error)
	cancel      func()
	permission  func(string, string) error
	stop        func() error
	mu          sync.Mutex
	permissions []string
	alive       bool
}

func (f *fakeRunner) Prompt(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
	return f.prompt(ctx, in, sink)
}
func (f *fakeRunner) Cancel() {
	f.mu.Lock()
	cancel := f.cancel
	f.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
func (f *fakeRunner) Permission(request, option string) error {
	f.mu.Lock()
	permission := f.permission
	if permission == nil {
		f.permissions = append(f.permissions, request+":"+option)
	}
	f.mu.Unlock()
	if permission != nil {
		return permission(request, option)
	}
	return nil
}
func (f *fakeRunner) SetModel(context.Context, string, core.Sink) error { return nil }
func (f *fakeRunner) SetMode(context.Context, string, core.Sink) error  { return nil }
func (f *fakeRunner) Alive() bool                                       { f.mu.Lock(); defer f.mu.Unlock(); return f.alive }
func (f *fakeRunner) Stop() error {
	f.mu.Lock()
	stop := f.stop
	f.alive = false
	f.mu.Unlock()
	if stop != nil {
		return stop()
	}
	return nil
}
func (f *fakeRunner) State() json.RawMessage { return nil }

type eventCollector struct {
	mu     sync.Mutex
	events []protocol.SessionEventData
	signal chan struct{}
}

func newCollector() *eventCollector { return &eventCollector{signal: make(chan struct{}, 32)} }
func (c *eventCollector) sendEvent(name string, data any) bool {
	if name == protocol.EvtSessionEvent {
		c.mu.Lock()
		c.events = append(c.events, data.(protocol.SessionEventData))
		c.mu.Unlock()
		c.signal <- struct{}{}
	}
	return true
}
func (c *eventCollector) snapshot() []protocol.SessionEventData {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]protocol.SessionEventData(nil), c.events...)
}

func isolateDaemon(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
}

func testManager(t *testing.T, runner core.Runner) (*sessionManager, protocol.SessionInfo) {
	t.Helper()
	isolateDaemon(t)
	settings := func() protocol.Settings {
		return protocol.Settings{Ask: protocol.AskSettings{Model: protocol.RouteAuto}, Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(fakeAsker{runner: runner}, nil, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginCLI})
	if err != nil {
		t.Fatal(err)
	}
	return manager, info
}

func waitTurnEnd(t *testing.T, collector *eventCollector) []protocol.SessionEventData {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		for _, event := range collector.snapshot() {
			if event.Kind == protocol.EvTurnEnd {
				return collector.snapshot()
			}
		}
		select {
		case <-collector.signal:
		case <-timer.C:
			t.Fatal("turn did not end")
		}
	}
}

func TestPromptFlowOrder(t *testing.T) {
	runner := &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "answer"})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, info := testManager(t, runner)
	collector := newCollector()
	if _, err := manager.attach(info.ID, 0, collector); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "question"}, nil); err != nil {
		t.Fatal(err)
	}
	events := waitTurnEnd(t, collector)
	want := []string{protocol.EvUser, protocol.EvTurnStart, protocol.EvText, protocol.EvTurnEnd}
	if len(events) != len(want) {
		t.Fatalf("events = %#v", events)
	}
	for i, kind := range want {
		if events[i].Kind != kind {
			t.Fatalf("event %d = %s, want %s", i, events[i].Kind, kind)
		}
	}
}

func TestAttachReplayThenLiveHasNoGapOrDuplicate(t *testing.T) {
	release := make(chan struct{})
	runner := &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "one"})
		<-release
		sink.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "two"})
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, info := testManager(t, runner)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "q"}, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	collector := newCollector()
	attached, err := manager.attach(info.ID, 0, collector)
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	live := waitTurnEnd(t, collector)
	seen := map[int64]bool{}
	for _, event := range attached.Events {
		if seen[event.Seq] {
			t.Fatalf("duplicate %d", event.Seq)
		}
		seen[event.Seq] = true
	}
	for _, event := range live {
		if seen[event.Seq] {
			t.Fatalf("duplicate %d", event.Seq)
		}
		seen[event.Seq] = true
	}
	for seq := int64(1); seq <= int64(len(seen)); seq++ {
		if !seen[seq] {
			t.Fatalf("missing seq %d", seq)
		}
	}
}

func TestPermissionRoutingAndWaitingState(t *testing.T) {
	ready := make(chan struct{})
	release := make(chan struct{})
	runner := &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		sink.Emit(protocol.EvPermission, protocol.PermissionData{Request: "p1", Title: "write"})
		close(ready)
		<-release
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, info := testManager(t, runner)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "q"}, nil); err != nil {
		t.Fatal(err)
	}
	<-ready
	time.Sleep(time.Millisecond)
	s, _ := manager.get(info.ID)
	current := s.log.Info()
	if current.State != protocol.StateWaiting || current.Pending != 1 {
		t.Fatalf("info = %#v", current)
	}
	if err := manager.permission(protocol.PermissionParams{ID: info.ID, Request: "p1", Option: protocol.OptAllowOnce}); err != nil {
		t.Fatal(err)
	}
	runner.mu.Lock()
	routed := append([]string(nil), runner.permissions...)
	runner.mu.Unlock()
	if len(routed) != 1 || routed[0] != "p1:allow_once" {
		t.Fatalf("permissions = %#v", routed)
	}
	close(release)
}

func TestCancelEndsTurnCancelled(t *testing.T) {
	runner := &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		<-ctx.Done()
		return core.TurnResult{}, ctx.Err()
	}}
	manager, info := testManager(t, runner)
	collector := newCollector()
	_, _ = manager.attach(info.ID, 0, collector)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "q"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := manager.cancel(info.ID); err != nil {
		t.Fatal(err)
	}
	events := waitTurnEnd(t, collector)
	var end protocol.TurnEndData
	if json.Unmarshal(events[len(events)-1].Data, &end) != nil || end.Stop != protocol.StopCancelled {
		t.Fatalf("turn end = %#v", end)
	}
}

func TestSecondPromptConflicts(t *testing.T) {
	release := make(chan struct{})
	runner := &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		<-release
		return core.TurnResult{}, nil
	}}
	manager, info := testManager(t, runner)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "one"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "two"}, nil); err == nil {
		t.Fatal("second prompt succeeded")
	}
	close(release)
}

func TestIdlePredicate(t *testing.T) {
	isolateDaemon(t)
	d := New(Deps{Asker: fakeAsker{runner: &fakeRunner{}}, Version: "test"})
	if !d.Idle() {
		t.Fatal("new daemon is not idle")
	}
	d.mu.Lock()
	d.clients[&connection{}] = struct{}{}
	d.mu.Unlock()
	if d.Idle() {
		t.Fatal("daemon with a client is idle")
	}
}

func TestQueueSendsNextPromptAndUnqueueRemovesOne(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	next := make(chan string, 1)
	var mu sync.Mutex
	calls := 0
	runner := &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		if call == 1 {
			close(started)
			<-release
		} else {
			next <- in.Text
		}
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, info := testManager(t, runner)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "first"}, nil); err != nil {
		t.Fatal(err)
	}
	<-started
	queued, err := manager.queue(context.Background(), protocol.QueueParams{ID: info.ID, Text: "remove me"})
	if err != nil {
		t.Fatal(err)
	}
	if len(queued.Queue) != 1 {
		t.Fatalf("queue = %#v", queued.Queue)
	}
	queued, err = manager.queue(context.Background(), protocol.QueueParams{ID: info.ID, Text: "keep me"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.unqueue(protocol.UnqueueParams{ID: info.ID, Queue: queued.Queue[0].ID}); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case text := <-next:
		if text != "keep me" {
			t.Fatalf("next prompt = %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("queued prompt was not sent")
	}
}

func TestSteerCancelsThenPrompts(t *testing.T) {
	started := make(chan struct{})
	next := make(chan string, 1)
	var mu sync.Mutex
	calls := 0
	runner := &fakeRunner{prompt: func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		if call == 1 {
			close(started)
			<-ctx.Done()
			return core.TurnResult{}, ctx.Err()
		}
		next <- in.Text
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, info := testManager(t, runner)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "first"}, nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := manager.steer(context.Background(), protocol.PromptParams{ID: info.ID, Text: "new direction"}); err != nil {
		t.Fatal(err)
	}
	select {
	case text := <-next:
		if text != "new direction" {
			t.Fatalf("steered prompt = %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("steered prompt was not sent")
	}
}

func TestForkCopiesExactlyThroughSequence(t *testing.T) {
	manager, info := testManager(t, &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{}, nil
	}})
	source, _ := manager.get(info.ID)
	if err := source.coalescer.Emit(protocol.EvUser, protocol.UserData{Text: "one"}); err != nil {
		t.Fatal(err)
	}
	if err := source.coalescer.Emit(protocol.EvText, protocol.TextData{Message: "m", Delta: "two"}); err != nil {
		t.Fatal(err)
	}
	if err := source.coalescer.Emit(protocol.EvUser, protocol.UserData{Text: "three"}); err != nil {
		t.Fatal(err)
	}
	forked, err := manager.fork(context.Background(), protocol.ForkParams{ID: info.ID, Seq: 2})
	if err != nil {
		t.Fatal(err)
	}
	target, _ := manager.get(forked.ID)
	got, err := target.log.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	want, err := source.log.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	want = want[:2]
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forked events = %#v, want %#v", got, want)
	}
	if forked.Parent != info.ID {
		t.Fatalf("parent = %q", forked.Parent)
	}
}

func TestReadClearsUnread(t *testing.T) {
	manager, info := testManager(t, &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{}, nil
	}})
	s, _ := manager.get(info.ID)
	if err := s.log.Update(func(i *protocol.SessionInfo) { i.Unread = true }, nil); err != nil {
		t.Fatal(err)
	}
	if err := manager.read(info.ID); err != nil {
		t.Fatal(err)
	}
	if s.log.Info().Unread {
		t.Fatal("session remained unread")
	}
}

func TestPrepareSessionCwdUsedByHarness(t *testing.T) {
	isolateDaemon(t)
	runner := &fakeRunner{alive: true, prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	harnesses := &fakeHarnesses{runner: runner}
	work := &fakeWork{prepared: protocol.SessionNewParams{Cwd: "/prepared", Worktree: "/prepared", Branch: "rashin/test"}}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(nil, harnesses, work, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.newSession(context.Background(), protocol.SessionNewParams{Kind: protocol.KindAgent, Project: "project", Worktree: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if work.prepareCalls != 1 || info.Cwd != "/prepared" || info.Worktree != "/prepared" || info.Branch != "rashin/test" {
		t.Fatalf("prepared info = %#v", info)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "go"}, nil); err != nil {
		t.Fatal(err)
	}
	if harnesses.opts.Cwd != "/prepared" {
		t.Fatalf("harness cwd = %q", harnesses.opts.Cwd)
	}
}

func TestDisconnectDropsProjectWatches(t *testing.T) {
	isolateDaemon(t)
	work := &fakeWork{}
	d := New(Deps{Work: work, Version: "test"})
	c := &connection{daemon: d, watches: make(map[string]int)}
	d.mu.Lock()
	d.clients[c] = struct{}{}
	d.mu.Unlock()
	c.setWatch("project", true)
	c.setWatch("project", true)
	d.disconnected(c)
	if !reflect.DeepEqual(work.watches, []string{"+project", "-project"}) {
		t.Fatalf("watch calls = %#v", work.watches)
	}
}

func TestWorkHooksCheckpointAndFinishAgentTurn(t *testing.T) {
	isolateDaemon(t)
	runner := &fakeRunner{alive: true, prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	harnesses := &fakeHarnesses{runner: runner}
	work := &fakeWork{checkpoint: true, after: make(chan int, 1)}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(nil, harnesses, work, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.newSession(context.Background(), protocol.SessionNewParams{Kind: protocol.KindAgent, Project: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "go"}, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case turn := <-work.after:
		if turn != 1 {
			t.Fatalf("after turn = %d", turn)
		}
	case <-time.After(time.Second):
		t.Fatal("after-turn hook was not called")
	}
	s, _ := manager.get(info.ID)
	if !s.log.Info().Checkpoints || work.beforeCalls != 1 {
		t.Fatalf("checkpoint info = %#v, calls = %d", s.log.Info(), work.beforeCalls)
	}
}

func TestProjectAddRegistersIndex(t *testing.T) {
	isolateDaemon(t)
	project := protocol.ProjectInfo{ID: "project", Name: "Example", Root: "/code/example"}
	work := &fakeWork{handleResult: project}
	indexer := &fakeIndexer{conflictAfterFirst: true}
	d := New(Deps{Work: work, Indexer: indexer, Version: "test"})
	raw, err := json.Marshal(protocol.ProjectAddParams{Path: project.Root})
	if err != nil {
		t.Fatal(err)
	}
	for attempt := range 2 {
		result, err := d.dispatch(context.Background(), &connection{}, protocol.MProjectsAdd, raw)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt+1, err)
		}
		if !reflect.DeepEqual(result, project) {
			t.Fatalf("result = %#v", result)
		}
	}
	want := []protocol.IndexAddParams{{Path: project.Root, Name: project.Name}, {Path: project.Root, Name: project.Name}}
	if !reflect.DeepEqual(indexer.added, want) {
		t.Fatalf("index registrations = %#v", indexer.added)
	}
}

func TestPermissionCallbackDoesNotHoldSessionLock(t *testing.T) {
	ready := make(chan struct{})
	var sink core.Sink
	runner := &fakeRunner{prompt: func(ctx context.Context, _ core.PromptInput, got core.Sink) (core.TurnResult, error) {
		sink = got
		got.Emit(protocol.EvPermission, protocol.PermissionData{Request: "permission"})
		close(ready)
		<-ctx.Done()
		return core.TurnResult{}, ctx.Err()
	}}
	runner.permission = func(request, option string) error {
		sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: request, Outcome: option})
		return nil
	}
	manager, info := testManager(t, runner)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "go"}, nil); err != nil {
		t.Fatal(err)
	}
	<-ready
	done := make(chan error, 1)
	go func() {
		done <- manager.permission(protocol.PermissionParams{ID: info.ID, Request: "permission", Option: protocol.OptAllowOnce})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("permission deadlocked")
	}
	_ = manager.cancel(info.ID)
}

func TestCancelCallbackDoesNotHoldSessionLock(t *testing.T) {
	ready := make(chan struct{})
	var sink core.Sink
	runner := &fakeRunner{prompt: func(ctx context.Context, _ core.PromptInput, got core.Sink) (core.TurnResult, error) {
		sink = got
		got.Emit(protocol.EvPermission, protocol.PermissionData{Request: "permission"})
		close(ready)
		<-ctx.Done()
		return core.TurnResult{}, ctx.Err()
	}}
	runner.cancel = func() {
		sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: "permission", Outcome: "cancelled"})
	}
	manager, info := testManager(t, runner)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "go"}, nil); err != nil {
		t.Fatal(err)
	}
	<-ready
	done := make(chan error, 1)
	go func() { done <- manager.cancel(info.ID) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel deadlocked")
	}
}

func TestSlowConnectionQueueOverflowDisconnects(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	connection := newConnection(&Daemon{}, server)
	defer connection.close()
	for range outboundQueueSize + 4 {
		connection.sendEvent(protocol.EvtNotice, protocol.Notice{Text: "queued"})
	}
	select {
	case <-connection.done:
	case <-time.After(time.Second):
		t.Fatal("overflowing connection stayed open")
	}
}

func TestStopDoesNotStartQueuedPrompt(t *testing.T) {
	started := make(chan struct{})
	var calls int
	var callsMu sync.Mutex
	runner := &fakeRunner{alive: true, prompt: func(ctx context.Context, _ core.PromptInput, _ core.Sink) (core.TurnResult, error) {
		callsMu.Lock()
		calls++
		call := calls
		callsMu.Unlock()
		if call == 1 {
			close(started)
		}
		<-ctx.Done()
		return core.TurnResult{}, ctx.Err()
	}}
	manager, info := testManager(t, runner)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "first"}, nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := manager.queue(context.Background(), protocol.QueueParams{ID: info.ID, Text: "queued"}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- manager.stop(info.ID) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not finish")
	}
	time.Sleep(20 * time.Millisecond)
	callsMu.Lock()
	got := calls
	callsMu.Unlock()
	if got != 1 {
		t.Fatalf("prompt calls = %d, want 1", got)
	}
}

func TestStopStaysStoppingThroughQueuedPromptDecision(t *testing.T) {
	isolateDaemon(t)
	started := make(chan struct{})
	queuedStarted := make(chan struct{}, 1)
	var calls int
	var callsMu sync.Mutex
	runner := &fakeRunner{alive: true, prompt: func(ctx context.Context, _ core.PromptInput, _ core.Sink) (core.TurnResult, error) {
		callsMu.Lock()
		calls++
		call := calls
		callsMu.Unlock()
		if call == 1 {
			close(started)
			<-ctx.Done()
			return core.TurnResult{}, ctx.Err()
		}
		queuedStarted <- struct{}{}
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	after := make(chan int)
	work := &fakeWork{after: after}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(nil, &fakeHarnesses{runner: runner}, work, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	collector := newCollector()
	if _, err := manager.attach(info.ID, 0, collector); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "first"}, nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := manager.queue(context.Background(), protocol.QueueParams{ID: info.ID, Text: "QUEUED_MUST_NOT_RUN"}); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- manager.stop(info.ID) }()
	waitTurnEnd(t, collector)
	select {
	case err := <-stopped:
		t.Fatalf("stop returned before queue decision: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if turn := <-after; turn != 1 {
		t.Fatalf("after turn = %d", turn)
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not finish after queue decision")
	}
	select {
	case <-queuedStarted:
		t.Fatal("queued prompt started during stop")
	case <-time.After(20 * time.Millisecond):
	}
	session, _ := manager.get(info.ID)
	if queue := session.log.Info().Queue; len(queue) != 1 || queue[0].Text != "QUEUED_MUST_NOT_RUN" {
		t.Fatalf("queued prompt was removed: %#v", queue)
	}
}

func TestAttachPagesBeforeJoiningLiveEvents(t *testing.T) {
	manager, info := testManager(t, &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{}, nil
	}})
	s, _ := manager.get(info.ID)
	for index := range 5 {
		if err := manager.persist(s, protocol.EvUser, protocol.UserData{Text: fmt.Sprintf("%d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	collector := newCollector()
	var first protocol.SessionAttachResult
	manager.attachQueued(info.ID, 0, 2, collector, func(result protocol.SessionAttachResult, err error) bool {
		if err != nil {
			t.Fatal(err)
		}
		first = result
		return true
	})
	if !first.More || len(first.Events) != 2 {
		t.Fatalf("first page = %#v", first)
	}
	if len(collector.snapshot()) != 0 {
		t.Fatal("joined live events before final page")
	}
	since := first.Events[len(first.Events)-1].Seq
	for first.More {
		manager.attachQueued(info.ID, since, 2, collector, func(result protocol.SessionAttachResult, err error) bool {
			if err != nil {
				t.Fatal(err)
			}
			first = result
			return true
		})
		if len(first.Events) > 0 {
			since = first.Events[len(first.Events)-1].Seq
		}
	}
	if err := manager.persist(s, protocol.EvUser, protocol.UserData{Text: "live"}); err != nil {
		t.Fatal(err)
	}
	if len(collector.snapshot()) != 1 {
		t.Fatal("final page did not join live events")
	}
}

func TestOversizedResponseReturnsInternalError(t *testing.T) {
	server, client := net.Pipe()
	connection := newConnection(&Daemon{}, server)
	defer connection.close()
	defer client.Close()
	connection.respond(7, strings.Repeat("x", protocol.MaxLine+1), nil)
	frame, err := protocol.NewDecoder(client).Next()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Error == nil || frame.Error.Code != protocol.CodeInternal {
		t.Fatalf("response = %#v", frame)
	}
}

func TestSetModelAgentCanIdleStop(t *testing.T) {
	isolateDaemon(t)
	runner := &fakeRunner{alive: true, prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{}, nil
	}}
	harnesses := &fakeHarnesses{runner: runner}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly, IdleStopMinutes: 1}}
	}
	manager := newSessionManager(nil, harnesses, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.setModel(context.Background(), protocol.SetModelParams{ID: info.ID, Model: "model"}); err != nil {
		t.Fatal(err)
	}
	manager.idleStop(time.Now().Add(2 * time.Minute))
	deadline := time.Now().Add(time.Second)
	for runner.Alive() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if runner.Alive() {
		t.Fatal("agent started by set_model was not idle-stopped")
	}
}

func TestAgentCwdDefaultsHomeAndRejectsRelative(t *testing.T) {
	isolateDaemon(t)
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(nil, &fakeHarnesses{}, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	if info.Cwd != os.Getenv("HOME") {
		t.Fatalf("cwd = %q", info.Cwd)
	}
	if _, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: "relative"}); err == nil {
		t.Fatal("relative cwd was accepted")
	}
}

type orderedTarget struct {
	mu    sync.Mutex
	order []string
	done  chan struct{}
}

func (t *orderedTarget) sendEvent(name string, _ any) bool {
	if name != protocol.EvtSessionEvent {
		return true
	}
	t.mu.Lock()
	t.order = append(t.order, "event")
	t.mu.Unlock()
	select {
	case t.done <- struct{}{}:
	default:
	}
	return true
}

func TestAttachResponseQueuedBeforeLaterEvent(t *testing.T) {
	manager, info := testManager(t, &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{}, nil
	}})
	s, _ := manager.get(info.ID)
	target := &orderedTarget{done: make(chan struct{}, 1)}
	manager.attachQueued(info.ID, 0, 10, target, func(protocol.SessionAttachResult, error) bool {
		target.mu.Lock()
		target.order = append(target.order, "response")
		target.mu.Unlock()
		return true
	})
	if err := manager.persist(s, protocol.EvUser, protocol.UserData{Text: "later"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-target.done:
	case <-time.After(time.Second):
		t.Fatal("later event was not delivered")
	}
	target.mu.Lock()
	got := append([]string(nil), target.order...)
	target.mu.Unlock()
	if !reflect.DeepEqual(got, []string{"response", "event"}) {
		t.Fatalf("order = %#v", got)
	}
}

func TestListenCreatesMissingSocketParent(t *testing.T) {
	isolateDaemon(t)
	socket := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "missing", "parent", "rashin.sock")
	t.Setenv("RASHIN_SOCKET", socket)
	listener, bound, lock, err := listen(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		listener.Close()
		if bound {
			os.Remove(socket)
		}
		if lock != nil {
			lock.Close()
		}
	}()
	info, err := os.Stat(filepath.Dir(socket))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("socket parent mode = %o", info.Mode().Perm())
	}
}

func TestSessionShutdownHonorsDeadline(t *testing.T) {
	isolateDaemon(t)
	release := make(chan struct{})
	runner := &fakeRunner{
		alive: true,
		prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
			return core.TurnResult{}, nil
		},
		stop: func() error { <-release; return nil },
	}
	harnesses := &fakeHarnesses{runner: runner}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(nil, harnesses, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.setModel(context.Background(), protocol.SetModelParams{ID: info.ID, Model: "model"}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	manager.shutdown(20 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("shutdown exceeded deadline: %v", elapsed)
	}
	close(release)
}

func TestCloseAllAlwaysUnlinksSocketAndReleasesLock(t *testing.T) {
	isolateDaemon(t)
	lock, acquired, err := AcquireLock()
	if err != nil || !acquired {
		t.Fatalf("acquire lock: acquired=%v err=%v", acquired, err)
	}
	listener, bound, lock, err := listen(lock)
	if err != nil {
		t.Fatal(err)
	}
	daemon := New(Deps{Lock: lock})
	daemon.mu.Lock()
	daemon.listener, daemon.bound, daemon.lock = listener, bound, lock
	daemon.mu.Unlock()
	daemon.Shutdown()
	if err := daemon.closeAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.Socket()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket still exists: %v", err)
	}
	lock, acquired, err = AcquireLock()
	if err != nil || !acquired {
		t.Fatalf("reacquire lock: acquired=%v err=%v", acquired, err)
	}
	lock.Close()
}

func TestAttachPagesHistoryLargerThanFrame(t *testing.T) {
	manager, info := testManager(t, &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{}, nil
	}})
	s, _ := manager.get(info.ID)
	payload := strings.Repeat("x", 1<<20)
	for range 17 {
		if err := manager.persist(s, protocol.EvUser, protocol.UserData{Text: payload}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := attachPage(s, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.More || len(result.Events) == 0 || len(result.Events) >= 17 {
		t.Fatalf("first page has %d events, more=%v", len(result.Events), result.More)
	}
	encoded, err := json.Marshal(protocol.Response{ID: 1, Result: mustMarshal(t, result)})
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > protocol.MaxLine {
		t.Fatalf("page frame = %d bytes", len(encoded))
	}
}

func mustMarshal(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := protocol.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPromptImmediatelyAfterTurnEndIsAccepted(t *testing.T) {
	releaseSecond := make(chan struct{})
	var calls int
	var mu sync.Mutex
	runner := &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		if call == 2 {
			<-releaseSecond
		}
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	manager, info := testManager(t, runner)
	collector := newCollector()
	if _, err := manager.attach(info.ID, 0, collector); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "one"}, nil); err != nil {
		t.Fatal(err)
	}
	waitTurnEnd(t, collector)
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "two"}, nil); err != nil {
		t.Fatalf("prompt after turn_end: %v", err)
	}
	close(releaseSecond)
}

func TestForkAtMidTurnCutsOpenTurn(t *testing.T) {
	manager, info := testManager(t, &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{}, nil
	}})
	source, _ := manager.get(info.ID)
	if err := manager.persist(source, protocol.EvUser, protocol.UserData{Text: "question"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.persist(source, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}); err != nil {
		t.Fatal(err)
	}
	if err := manager.persist(source, protocol.EvText, protocol.TextData{Message: "m", Delta: "partial"}); err != nil {
		t.Fatal(err)
	}
	forked, err := manager.fork(context.Background(), protocol.ForkParams{ID: info.ID, Seq: 3})
	if err != nil {
		t.Fatal(err)
	}
	target, _ := manager.get(forked.ID)
	events, err := target.log.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("fork retained open turn: %#v", events)
	}
}

func TestRevertRefusesWaitingSessionInWorktree(t *testing.T) {
	isolateDaemon(t)
	work := &fakeWork{}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(nil, &fakeHarnesses{}, work, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	running, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: "/work", Worktree: "/work"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: "/work", Worktree: "/work"})
	if err != nil {
		t.Fatal(err)
	}
	session, _ := manager.get(running.ID)
	session.mu.Lock()
	session.running = true
	_ = session.log.Update(func(info *protocol.SessionInfo) { info.State = protocol.StateWaiting }, nil)
	session.mu.Unlock()
	if _, err := manager.revertTurn(context.Background(), protocol.TurnParams{ID: other.ID, Turn: 1}); err == nil {
		t.Fatal("revert succeeded while another session waited")
	}
}

func TestConcurrentSettingsSetKeepsBothUpdates(t *testing.T) {
	isolateDaemon(t)
	initial := config.Defaults()
	initial.Enabled = true
	if err := config.Save(initial); err != nil {
		t.Fatal(err)
	}
	daemon := New(Deps{Version: "test"})
	daemon.mu.Lock()
	daemon.settings = initial
	daemon.mu.Unlock()
	patches := []json.RawMessage{
		json.RawMessage(`{"ask":{"habits":false}}`),
		json.RawMessage(`{"agent":{"approvals":"auto"}}`),
	}
	start := make(chan struct{})
	errors := make(chan error, len(patches))
	var wg sync.WaitGroup
	for _, patch := range patches {
		wg.Add(1)
		go func(patch json.RawMessage) {
			defer wg.Done()
			<-start
			raw, err := json.Marshal(protocol.SettingsSetParams{Patch: patch})
			if err == nil {
				_, err = daemon.dispatch(context.Background(), &connection{}, protocol.MSettingsSet, raw)
			}
			errors <- err
		}(patch)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := daemon.Settings()
	if got.Ask.Habits || got.Agent.Approvals != protocol.ApprovalsAuto {
		t.Fatalf("settings = %#v", got)
	}
}

func TestDispatchRejectsRelativeRegistrationAndWorktreePaths(t *testing.T) {
	isolateDaemon(t)
	daemon := New(Deps{Indexer: &fakeIndexer{}, Work: &fakeWork{}})
	cases := []struct {
		method string
		params any
	}{
		{protocol.MIndexAdd, protocol.IndexAddParams{Path: "."}},
		{protocol.MProjectsAdd, protocol.ProjectAddParams{Path: "project"}},
		{protocol.MWorktreesPreviewRemove, protocol.WorktreeRef{Path: "worktree"}},
		{protocol.MWorktreesRemove, protocol.WorktreeRemoveParams{Path: "worktree"}},
		{protocol.MWorktreesLock, protocol.WorktreeLockParams{Path: "worktree"}},
		{protocol.MWorktreesMove, protocol.WorktreeMoveParams{Path: "/worktree", To: "moved"}},
		{protocol.MWorktreesSetup, protocol.WorktreeSetupParams{Path: "worktree"}},
		{protocol.MWorktreesTrust, protocol.TrustParams{Path: "worktree"}},
		{protocol.MWorktreesPorts, protocol.WorktreeRef{Path: "worktree"}},
	}
	for _, test := range cases {
		t.Run(test.method, func(t *testing.T) {
			raw, err := json.Marshal(test.params)
			if err != nil {
				t.Fatal(err)
			}
			_, err = daemon.dispatch(context.Background(), &connection{}, test.method, raw)
			var methodErr *protocol.Error
			if !errors.As(err, &methodErr) || methodErr.Code != protocol.CodeBadRequest {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestFirstAskQuestionSetsSingleLineTitle(t *testing.T) {
	manager, info := testManager(t, &fakeRunner{prompt: func(context.Context, core.PromptInput, core.Sink) (core.TurnResult, error) {
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}})
	question := "  Why does this title\ncontain whitespace and enough words to exceed the normal sixty character table title limit?  "
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: question}, nil); err != nil {
		t.Fatal(err)
	}
	session, _ := manager.get(info.ID)
	if got, want := session.log.Info().Title, questionTitle(question); got != want || got == "" || strings.Contains(got, "\n") {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestAnAgentSessionIsNamedAfterWhatItWasFirstAskedUntilItsHarnessNamesIt(t *testing.T) {
	isolateDaemon(t)
	runner := &fakeRunner{alive: true, prompt: func(_ context.Context, _ core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		return core.TurnResult{Stop: protocol.StopEnd}, nil
	}}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager := newSessionManager(nil, &fakeHarnesses{runner: runner}, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: info.ID, Text: "Fix the build on Void"}, nil); err != nil {
		t.Fatal(err)
	}
	session, _ := manager.get(info.ID)
	if title := session.log.Info().Title; title != "Fix the build on Void" {
		t.Fatalf("an agent session nobody named is called %q after its first prompt", title)
	}

	named, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Title: "Void port"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.prompt(context.Background(), protocol.PromptParams{ID: named.ID, Text: "Fix the build on Void"}, nil); err != nil {
		t.Fatal(err)
	}
	session, _ = manager.get(named.ID)
	if title := session.log.Info().Title; title != "Void port" {
		t.Fatalf("a session that had a name was renamed to %q by its first prompt", title)
	}
}
